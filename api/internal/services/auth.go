package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/actas"
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/linkkeys"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	log "github.com/sirupsen/logrus"
)

// PKIClient is the part of the LinkKeys RP client the browser flow uses.
// linkkeys.TCPClient satisfies it; tests use fakes.
type PKIClient interface {
	SignRequest(callbackURL, nonce string) (string, error)
	DecryptToken(encryptedToken string) (string, error)
	VerifyAssertion(signedAssertion, expectedDomain string) (*linkkeys.Assertion, error)
}

// UserInfoFetcher is the optional claims call (linkkeys.TCPClient has it).
type UserInfoFetcher interface {
	FetchUserInfo(token, domain string) (*linkkeys.UserInfo, error)
}

// SessionCookie is the browser session cookie name.
const SessionCookie = "catalystcal_session"

const (
	sessionCookiePath = "/api"
	// sessionReuseGrace lets two tabs resume with the same cookie.
	sessionReuseGrace = 60 * time.Second
	// actAsSkew is added to the nonce hold time.
	actAsSkew = 60 * time.Second
)

// AuthService implements csil.AuthService.
type AuthService struct{ *Deps }

var _ csil.AuthService = (*AuthService)(nil)

// ---- tokens --------------------------------------------------------------

func (d *Deps) bearerTTL() time.Duration {
	if d.Cfg != nil && d.Cfg.BearerTTL > 0 {
		return d.Cfg.BearerTTL
	}
	return auth.DefaultTTL
}

func (d *Deps) sessionTTL() time.Duration {
	if d.Cfg != nil && d.Cfg.SessionTTL > 0 {
		return d.Cfg.SessionTTL
	}
	return 720 * time.Hour
}

func scopesOut(in []string) []csil.Scope {
	if len(in) == 0 {
		return nil
	}
	out := make([]csil.Scope, len(in))
	for i, s := range in {
		out[i] = csil.Scope(s)
	}
	return out
}

// mint signs an identity and builds the LoginResponse.
func (d *Deps) mint(id auth.Identity, ttl time.Duration, user csil.PersonSummary, actingAs *csil.PersonSummary) (csil.LoginResponse, error) {
	if len(d.Secret) == 0 {
		return csil.LoginResponse{}, csilrpc.Internal("auth not configured")
	}
	tok, err := auth.Mint(d.Secret, id, ttl)
	if err != nil {
		return csil.LoginResponse{}, internal(err, "mint bearer")
	}
	v, err := auth.Verify(d.Secret, tok)
	if err != nil {
		return csil.LoginResponse{}, internal(err, "verify minted bearer")
	}
	return csil.LoginResponse{
		Token:     tok,
		ExpiresAt: ts(time.Unix(v.ExpiresAt, 0)),
		User:      user,
		ActingAs:  actingAs,
		Via:       csil.ActingVia(v.Via),
		Scopes:    scopesOut(v.Scopes),
	}, nil
}

func selfIdentity(u store.User, sessionID string) auth.Identity {
	return auth.Identity{UserID: u.ID, Domain: u.Domain, Subject: u.Subject, DisplayName: u.DisplayName, Via: auth.ViaSelf, SessionID: sessionID}
}

// ---- browser sessions ----------------------------------------------------

func randomSecret(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (d *Deps) setSessionCookie(ctx context.Context, token string, lifetime time.Duration) {
	csilrpc.SetCookie(ctx, &http.Cookie{
		Name: SessionCookie, Value: token, Path: sessionCookiePath,
		MaxAge: int(lifetime / time.Second), HttpOnly: true,
		Secure: d.Cfg == nil || d.Cfg.SecureCookies(), SameSite: http.SameSiteLaxMode,
	})
}

func (d *Deps) clearSessionCookie(ctx context.Context) {
	csilrpc.SetCookie(ctx, &http.Cookie{
		Name: SessionCookie, Value: "", Path: sessionCookiePath, MaxAge: -1, HttpOnly: true,
		Secure: d.Cfg == nil || d.Cfg.SecureCookies(), SameSite: http.SameSiteLaxMode,
	})
}

// browserLogin starts a browser session for u (revoking a session cookie
// the browser already holds) and mints a self bearer bound to it.
func (d *Deps) browserLogin(ctx context.Context, u store.User) (csil.LoginResponse, error) {
	if old := csilrpc.RequestCookie(ctx, SessionCookie); old != "" {
		if _, err := d.Store.RevokeSessionByHash(ctx, hashSecret(old)); err != nil && !errors.Is(err, store.ErrSessionInvalid) {
			log.WithError(err).Warn("auth: revoke previous session failed")
		}
	}
	secret, err := randomSecret("dz_s_")
	if err != nil {
		return csil.LoginResponse{}, internal(err, "session secret")
	}
	lifetime := d.sessionTTL()
	sess, err := d.Store.CreateSession(ctx, u.ID, hashSecret(secret), csilrpc.RequestUserAgent(ctx), lifetime)
	if err != nil {
		return csil.LoginResponse{}, internal(err, "create session")
	}
	d.setSessionCookie(ctx, secret, lifetime)
	d.securityEvent(ctx, u.ID, "auth.Login")
	return d.mint(selfIdentity(u, sess.ID), min(d.bearerTTL(), lifetime), person(u), nil)
}

// securityEvent writes an audit entry for a public auth op (the dispatcher
// audits only authenticated calls).
func (d *Deps) securityEvent(ctx context.Context, userID, op string) {
	if err := d.Store.InsertAudit(ctx, store.AuditRow{
		ActorID: userID, PrincipalID: userID, Via: auth.ViaSelf, Operation: op, Target: "user:" + userID, Outcome: "ok",
	}); err != nil {
		log.WithError(err).Warn("audit: security event failed")
	}
}

// ---- Complete ------------------------------------------------------------

// Complete finishes the LinkKeys browser flow: decrypt the sealed token,
// verify the assertion, check the login nonce, the audience, and the
// sign-in policy, then upsert the user and start a session.
func (s *AuthService) Complete(ctx context.Context, req csil.CompleteRequest) (csil.LoginResponse, error) {
	if s.PKI == nil {
		return csil.LoginResponse{}, csilrpc.NewError(http.StatusServiceUnavailable, "LinkKeys sign-in is not configured on this server")
	}
	if strings.TrimSpace(req.EncryptedToken) == "" {
		return csil.LoginResponse{}, csilrpc.BadRequest("encrypted_token is required")
	}
	signed, err := s.PKI.DecryptToken(req.EncryptedToken)
	if err != nil {
		log.WithError(err).Error("auth.Complete: decrypt-token failed")
		return csil.LoginResponse{}, csilrpc.NewError(http.StatusBadGateway, "could not decrypt token")
	}
	a, err := s.PKI.VerifyAssertion(signed, s.Cfg.LinkkeysIDPDomain)
	if err != nil {
		log.WithError(err).Info("auth.Complete: assertion refused")
		return csil.LoginResponse{}, csilrpc.Unauthorized("assertion verification failed")
	}
	if err := auth.VerifyNonce(s.Secret, a.Nonce); err != nil {
		return csil.LoginResponse{}, csilrpc.Unauthorized("login nonce invalid or expired")
	}
	if want := s.Cfg.RPDomain(); a.Audience != "" && want != "" && !strings.EqualFold(a.Audience, want) {
		return csil.LoginResponse{}, csilrpc.Unauthorized("assertion audience mismatch")
	}
	if a.Domain == "" || a.UserID == "" {
		return csil.LoginResponse{}, csilrpc.Unauthorized("assertion has no identity")
	}
	if !s.Cfg.SignInAllowed(a.Domain, a.UserID) {
		return csil.LoginResponse{}, csilrpc.Forbidden("this catalystcal server does not accept sign-in from " + strings.ToLower(a.Domain))
	}
	claims := s.fetchClaims(signed, a)
	in := store.UserInput{Domain: a.Domain, Subject: a.UserID, DisplayName: a.DisplayName}
	if v := claims["display_name"]; v != "" {
		in.DisplayName = v
	}
	in.Handle = claims["handle"]
	in.Email = claims["email"]
	u, _, err := s.Store.UpsertUser(ctx, in)
	if err != nil {
		return csil.LoginResponse{}, internal(err, "upsert user")
	}
	return s.browserLogin(ctx, u)
}

func (s *AuthService) fetchClaims(token string, a *linkkeys.Assertion) map[string]string {
	f, ok := s.PKI.(UserInfoFetcher)
	if !ok {
		return nil
	}
	info, err := f.FetchUserInfo(token, a.Domain)
	if err != nil || info == nil {
		if err != nil {
			log.WithError(err).Warn("auth: userinfo-fetch failed; continuing without claims")
		}
		return nil
	}
	claims := info.ClaimValues()
	if claims == nil {
		claims = map[string]string{}
	}
	if info.DisplayName != "" && claims["display_name"] == "" {
		claims["display_name"] = info.DisplayName
	}
	return claims
}

// ---- sessions ------------------------------------------------------------

// ResumeSession rotates the session cookie and mints a new self bearer.
func (s *AuthService) ResumeSession(ctx context.Context, _ csil.EmptyRequest) (csil.LoginResponse, error) {
	presented := csilrpc.RequestCookie(ctx, SessionCookie)
	if presented == "" {
		return csil.LoginResponse{}, csilrpc.Unauthorized("no browser session")
	}
	next, err := randomSecret("dz_s_")
	if err != nil {
		return csil.LoginResponse{}, internal(err, "session secret")
	}
	lifetime := s.sessionTTL()
	sess, err := s.Store.RotateSession(ctx, hashSecret(presented), hashSecret(next), lifetime, sessionReuseGrace)
	rotated := true
	switch {
	case err == nil:
	case errors.Is(err, store.ErrSessionRecentlyRotated):
		rotated = false
	case errors.Is(err, store.ErrSessionReuse), errors.Is(err, store.ErrSessionInvalid):
		if errors.Is(err, store.ErrSessionReuse) {
			log.Warn("auth: browser session secret reuse; session revoked")
		}
		s.clearSessionCookie(ctx)
		return csil.LoginResponse{}, csilrpc.Unauthorized("browser session is invalid or expired")
	default:
		return csil.LoginResponse{}, internal(err, "rotate session")
	}
	u, err := s.Store.GetUser(ctx, sess.UserID)
	if err != nil {
		s.clearSessionCookie(ctx)
		return csil.LoginResponse{}, csilrpc.Unauthorized("browser session is invalid or expired")
	}
	if rotated {
		s.setSessionCookie(ctx, next, lifetime)
	}
	remaining := sess.ExpiresAt.Sub(s.now())
	return s.mint(selfIdentity(u, sess.ID), min(s.bearerTTL(), remaining), person(u), nil)
}

// EndSession revokes the session of the cookie and clears it.
func (s *AuthService) EndSession(ctx context.Context, _ csil.EmptyRequest) (csil.EmptyResponse, error) {
	s.clearSessionCookie(ctx)
	presented := csilrpc.RequestCookie(ctx, SessionCookie)
	if presented == "" {
		return csil.EmptyResponse{}, nil
	}
	sess, err := s.Store.RevokeSessionByHash(ctx, hashSecret(presented))
	if err == nil {
		s.securityEvent(ctx, sess.UserID, "auth.EndSession")
	} else if !errors.Is(err, store.ErrSessionInvalid) {
		log.WithError(err).Warn("auth: end session failed")
	}
	return csil.EmptyResponse{}, nil
}

// ---- Me and delegation bearers -----------------------------------------

// Me returns the real caller, whose authority the bearer uses, and the
// people who gave the caller a delegation that is in effect now.
func (s *AuthService) Me(ctx context.Context, _ csil.EmptyRequest) (csil.MeResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.MeResponse{}, err
	}
	u, err := s.Store.GetUser(ctx, id.UserID)
	if err != nil {
		return csil.MeResponse{}, csilrpc.Unauthorized("user no longer exists")
	}
	out := csil.MeResponse{User: userOut(u), Via: csil.ActingVia(id.Via), Scopes: scopesOut(id.Scopes), Principals: []csil.PersonSummary{}}
	if id.IsDelegated() {
		if out.ActingAs, err = s.personByID(ctx, id.EffectiveUserID()); err != nil {
			return csil.MeResponse{}, internal(err, "load principal")
		}
	}
	if id.Via != auth.ViaActAsGrant {
		ds, err := s.Store.ListDelegationsByDelegate(ctx, u.Domain, u.Subject)
		if err != nil {
			return csil.MeResponse{}, internal(err, "list received delegations")
		}
		now := s.now()
		seen := map[string]bool{}
		for _, dl := range ds {
			if !dl.LiveAt(now) || seen[dl.PrincipalID] {
				continue
			}
			seen[dl.PrincipalID] = true
			p, err := s.personByID(ctx, dl.PrincipalID)
			if err != nil {
				return csil.MeResponse{}, internal(err, "load principal")
			}
			if p != nil {
				out.Principals = append(out.Principals, *p)
			}
		}
	}
	return out, nil
}

// AssumePrincipal mints a delegated bearer. Only a self bearer may call it
// (no chains). The bearer lives at most one hour and never past the
// delegation's expiry.
func (s *AuthService) AssumePrincipal(ctx context.Context, req csil.AssumePrincipalRequest) (csil.LoginResponse, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.LoginResponse{}, err
	}
	if req.PrincipalUserId == "" {
		return csil.LoginResponse{}, csilrpc.BadRequest("principal_user_id is required")
	}
	if string(req.PrincipalUserId) == id.UserID {
		return csil.LoginResponse{}, csilrpc.BadRequest("you cannot assume yourself")
	}
	dl, err := s.Store.FindLiveDelegation(ctx, string(req.PrincipalUserId), id.Domain, id.Subject)
	if errors.Is(err, store.ErrNotFound) {
		return csil.LoginResponse{}, csilrpc.NotFound("no delegation from that person is in effect for you")
	}
	if err != nil {
		return csil.LoginResponse{}, internal(err, "find delegation")
	}
	principal, err := s.Store.GetUser(ctx, dl.PrincipalID)
	if err != nil {
		return csil.LoginResponse{}, errNotFound("principal")
	}
	actor, err := s.Store.GetUser(ctx, id.UserID)
	if err != nil {
		return csil.LoginResponse{}, csilrpc.Unauthorized("user no longer exists")
	}
	ttl := auth.DelegatedTTL
	if dl.ExpiresAt != nil {
		ttl = min(ttl, dl.ExpiresAt.Sub(s.now()))
		if ttl <= 0 {
			return csil.LoginResponse{}, csilrpc.NotFound("no delegation from that person is in effect for you")
		}
	}
	ident := auth.Identity{
		UserID: actor.ID, Domain: actor.Domain, Subject: actor.Subject, DisplayName: actor.DisplayName,
		Via: auth.ViaDelegation, PrincipalID: principal.ID, DelegationID: dl.ID, Scopes: dl.Scopes, SessionID: id.SessionID,
	}
	p := person(principal)
	return s.mint(ident, ttl, person(actor), &p)
}

// ReleasePrincipal mints a self bearer for the real caller again. An
// act-as grant bearer cannot: the grantee is an application, not the user.
func (s *AuthService) ReleasePrincipal(ctx context.Context, _ csil.EmptyRequest) (csil.LoginResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.LoginResponse{}, err
	}
	if id.Via == auth.ViaActAsGrant {
		return csil.LoginResponse{}, csilrpc.Forbidden("an act-as grant bearer cannot release to a self bearer")
	}
	u, err := s.Store.GetUser(ctx, id.UserID)
	if err != nil {
		return csil.LoginResponse{}, csilrpc.Unauthorized("user no longer exists")
	}
	ttl := s.bearerTTL()
	if id.SessionID != "" {
		sess, err := s.Store.GetSession(ctx, id.SessionID)
		if err != nil || sess.RevokedAt != nil || !sess.ExpiresAt.After(s.now()) {
			return csil.LoginResponse{}, csilrpc.Unauthorized("browser session is invalid or expired")
		}
		ttl = min(ttl, sess.ExpiresAt.Sub(s.now()))
	}
	return s.mint(selfIdentity(u, id.SessionID), ttl, person(u), nil)
}

// ---- act-as (catalystcal is the audience) ---------------------------------------

func errActAsOff() error {
	return csilrpc.NewError(http.StatusServiceUnavailable, "act-as is not configured on this server")
}

// GetActAsScopeSet signs the scope set catalystcal offers a grantee application.
func (s *AuthService) GetActAsScopeSet(ctx context.Context, req csil.ScopeSetRequest) (csil.ScopeSetResponse, error) {
	if s.ActAs == nil {
		return csil.ScopeSetResponse{}, errActAsOff()
	}
	if req.GranteeDomain == "" || req.GranteeUserId == "" || req.GranteeApplicationId == "" {
		return csil.ScopeSetResponse{}, csilrpc.BadRequest("grantee_domain, grantee_user_id, and grantee_application_id are required")
	}
	raw, err := s.ActAs.SignScopeSet(ctx, actas.ScopeSetRequest{
		GranteeDomain: req.GranteeDomain, GranteeUserID: req.GranteeUserId,
		GranteeApplicationID: req.GranteeApplicationId, Languages: req.Languages,
	})
	if err != nil {
		return csil.ScopeSetResponse{}, internal(err, "sign scope set")
	}
	return csil.ScopeSetResponse{SignedScopeSet: raw}, nil
}

// ExchangeActAsCredential verifies a LinkKeys act-as credential, burns its
// presentation nonce, and mints a bearer for the user with only the
// approved catalystcal scopes. The bearer never outlives the grant.
func (s *AuthService) ExchangeActAsCredential(ctx context.Context, req csil.ExchangeActAsCredentialRequest) (csil.LoginResponse, error) {
	if s.ActAs == nil {
		return csil.LoginResponse{}, errActAsOff()
	}
	if len(req.Credential) == 0 {
		return csil.LoginResponse{}, csilrpc.BadRequest("credential is required")
	}
	var known []string
	if s.Revoked != nil {
		known = s.Revoked.List()
	}
	v, err := s.ActAs.VerifyCredential(ctx, req.Credential, known)
	if err != nil {
		log.WithError(err).Info("auth: act-as credential refused")
		return csil.LoginResponse{}, csilrpc.Unauthorized("act-as credential refused: " + err.Error())
	}
	if s.Revoked != nil && s.Revoked.Has(v.GrantID) {
		return csil.LoginResponse{}, csilrpc.Unauthorized("act-as grant is revoked")
	}
	now := s.now()
	if !v.ExpiresAt.After(now) {
		return csil.LoginResponse{}, csilrpc.Unauthorized("act-as grant has expired")
	}
	hold := s.ActAs.MaxPresentationAge() + 2*actAsSkew
	if err := s.Store.RecordActAsNonce(ctx, v.GrantID+"/"+v.Nonce, now.Add(hold)); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return csil.LoginResponse{}, csilrpc.Unauthorized("act-as presentation was already used")
		}
		return csil.LoginResponse{}, internal(err, "record act-as nonce")
	}
	if !s.Cfg.SignInAllowed(v.SubjectDomain, v.SubjectUserID) {
		return csil.LoginResponse{}, csilrpc.Forbidden("this catalystcal server does not accept users from " + v.SubjectDomain)
	}
	if len(v.Scopes) == 0 {
		return csil.LoginResponse{}, csilrpc.Forbidden("the grant approves no catalystcal scope")
	}
	u, _, err := s.Store.UpsertUser(ctx, store.UserInput{Domain: v.SubjectDomain, Subject: v.SubjectUserID})
	if err != nil {
		return csil.LoginResponse{}, internal(err, "upsert act-as user")
	}
	if err := s.Store.UpsertActAsGrant(ctx, store.ActAsGrant{
		GrantID: v.GrantID, SubjectDomain: v.SubjectDomain, UserID: u.ID, Application: v.Application, ExpiresAt: v.ExpiresAt,
	}); err != nil {
		return csil.LoginResponse{}, internal(err, "record act-as grant")
	}
	scopes := make([]string, 0, len(v.Scopes))
	for _, sc := range v.Scopes {
		if access.ValidScope(sc) {
			scopes = append(scopes, sc)
		}
	}
	ttl := min(s.bearerTTL(), v.ExpiresAt.Sub(now))
	ident := auth.Identity{
		UserID: u.ID, Domain: u.Domain, Subject: u.Subject, DisplayName: u.DisplayName,
		Via: auth.ViaActAsGrant, PrincipalID: u.ID, GrantID: v.GrantID, Application: v.Application, Scopes: scopes,
	}
	if err := s.Store.InsertAudit(ctx, store.AuditRow{
		ActorID: u.ID, PrincipalID: u.ID, Via: auth.ViaActAsGrant, Application: v.Application,
		Operation: "auth.ExchangeActAsCredential", Target: "grant:" + v.GrantID, Outcome: "ok",
	}); err != nil {
		log.WithError(err).Warn("audit: act-as exchange")
	}
	p := person(u)
	return s.mint(ident, ttl, p, &p)
}
