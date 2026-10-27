// Package actas makes catalystcal a LinkKeys act-as AUDIENCE with the regular-RP
// Go SDK. It signs catalystcal's scope sets and verifies the credentials that
// grantee applications present to AuthService.exchange-act-as-credential.
//
// The services see only the Provider interface, so they are testable with a
// fake. Verifier is the SDK-backed Provider; its key material comes through
// the KeySource interface, so the SDK conformance vectors can drive it
// without a network.
package actas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	api "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

// RequestDigestInput is the text whose SHA-256 is the presentation request
// digest of exchange-act-as-credential (csil/catalystcal.csil).
const RequestDigestInput = "catalystcal/AuthService/exchange-act-as-credential"

// RequestDigest returns SHA-256(RequestDigestInput).
func RequestDigest() []byte {
	sum := sha256.Sum256([]byte(RequestDigestInput))
	return sum[:]
}

// ScopeDescriptions are the English descriptions catalystcal offers in its scope
// sets. The keys are the catalystcal Scope words.
var ScopeDescriptions = map[string]string{
	"calendars.read":   "See your calendars and event details, including private events.",
	"freebusy.read":    "See when you are busy and free.",
	"events.write":     "Create, change, and delete events as you.",
	"events.respond":   "Answer invitations as you.",
	"calendars.manage": "Change calendar settings, feeds, and sharing.",
}

// ScopeOrder is the order of entries in a scope set.
var ScopeOrder = []string{"calendars.read", "freebusy.read", "events.write", "events.respond", "calendars.manage"}

// Verified is an accepted credential, reduced to what catalystcal needs.
type Verified struct {
	GrantID string
	// SubjectUserID is the LinkKeys account id of the user at SubjectDomain.
	SubjectUserID string
	SubjectDomain string
	// Application names the grantee, for the audit log:
	// "<application_id>@<domain>" or "local-rp:<fingerprint>".
	Application string
	// Scopes are the approved scopes that are catalystcal Scope words. Unknown
	// words are dropped.
	Scopes    []string
	ExpiresAt time.Time
	// Nonce is the presentation nonce (hex). The caller records it.
	Nonce string
}

// ScopeSetRequest names the grantee application that asks for a scope set.
type ScopeSetRequest struct {
	GranteeDomain        string
	GranteeUserID        string
	GranteeApplicationID string
	Languages            []string
}

// Provider is what AuthService needs from act-as.
type Provider interface {
	// SignScopeSet returns CBOR(SignedActAsScopeSet).
	SignScopeSet(ctx context.Context, req ScopeSetRequest) ([]byte, error)
	// VerifyCredential checks CBOR(ActAsCredential). knownRevoked holds
	// grant ids the caller already knows are revoked.
	VerifyCredential(ctx context.Context, credential []byte, knownRevoked []string) (Verified, error)
	// RevokedGrants returns the ids among grantIDs that subjectDomain
	// revoked.
	RevokedGrants(ctx context.Context, subjectDomain string, grantIDs []string) ([]string, error)
	// MaxPresentationAge is how long a presentation nonce must be held.
	MaxPresentationAge() time.Duration
}

// KeySource supplies verified key material.
type KeySource interface {
	// OwnScopeSetKeys returns ALL attested keys of catalystcal's own instance,
	// including expired and revoked ones (AttestedKeyRefs).
	OwnScopeSetKeys(ctx context.Context, own api.ApplicationRef, instanceID string) ([]regularrp.ApplicationKeyRef, error)
	// GranteeInstanceKeys returns the usable keys of a grantee instance.
	GranteeInstanceKeys(ctx context.Context, ref regularrp.InstanceRef) ([]regularrp.ApplicationKeyRef, error)
	// DomainKeys returns the signing keys of a home domain, with its key
	// revocations applied.
	DomainKeys(ctx context.Context, domain string) ([]api.DomainPublicKey, error)
	// RevokedGrants returns the ids of grants with a verified revocation.
	RevokedGrants(ctx context.Context, domain string, grantIDs []string) ([]string, error)
}

// AppKeys is the JSON file named by CATALYSTCAL_ACT_AS_APP_KEYS_FILE: catalystcal's
// LinkKeys application identity and its Ed25519 signing keys.
type AppKeys struct {
	Domain        string       `json:"domain"`
	UserID        string       `json:"user_id"`
	ApplicationID string       `json:"application_id"`
	InstanceID    string       `json:"instance_id"`
	SigningKeys   []SigningKey `json:"signing_keys"`
}

// SigningKey is one Ed25519 key: its key id and its 32-byte seed in hex.
type SigningKey struct {
	KeyID   string `json:"key_id"`
	SeedHex string `json:"seed_hex"`
}

// LoadAppKeys reads and checks the application key file that
// CATALYSTCAL_ACT_AS_APP_KEYS_FILE names (in the container:
// /run/secrets/catalystcal/act-as/app-keys.json). The file is JSON:
//
//	{
//	  "domain":         "catalystcal.example",              // LinkKeys domain of catalystcal's account
//	  "user_id":        "018f...",                   // that account's LinkKeys user id
//	  "application_id": "catalystcal",                      // catalystcal's application id
//	  "instance_id":    "catalystcal-prod-1",               // this enrolled instance
//	  "signing_keys": [                              // every CURRENT Ed25519 signing key
//	    {"key_id": "k-2026-10", "seed_hex": "<64 hex chars: the 32-byte Ed25519 seed>"}
//	  ]
//	}
//
// Every signing key signs each scope set, so one expired or revoked key
// does not break grants. Keep the file secret: the seeds are private keys.
func LoadAppKeys(path string) (AppKeys, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return AppKeys{}, fmt.Errorf("read act-as app keys: %w", err)
	}
	var k AppKeys
	if err := json.Unmarshal(raw, &k); err != nil {
		return AppKeys{}, fmt.Errorf("parse act-as app keys %s: %w", path, err)
	}
	if k.Domain == "" || k.UserID == "" || k.ApplicationID == "" || k.InstanceID == "" {
		return AppKeys{}, errors.New("act-as app keys need domain, user_id, application_id, and instance_id")
	}
	if len(k.SigningKeys) == 0 {
		return AppKeys{}, errors.New("act-as app keys need at least one signing key")
	}
	if _, err := k.Signers(); err != nil {
		return AppKeys{}, err
	}
	return k, nil
}

// Ref returns catalystcal's application reference.
func (k AppKeys) Ref() api.ApplicationRef {
	return api.ApplicationRef{SubjectUserId: k.UserID, SubjectDomain: strings.ToLower(k.Domain), ApplicationId: k.ApplicationID}
}

// Signers converts the keys for the SDK.
func (k AppKeys) Signers() ([]regularrp.ApplicationSigner, error) {
	out := make([]regularrp.ApplicationSigner, 0, len(k.SigningKeys))
	for _, sk := range k.SigningKeys {
		seed, err := hex.DecodeString(sk.SeedHex)
		if err != nil || len(seed) != 32 {
			return nil, fmt.Errorf("act-as signing key %q: seed_hex must be 32 bytes of hex", sk.KeyID)
		}
		if sk.KeyID == "" {
			return nil, errors.New("act-as signing key without key_id")
		}
		out = append(out, regularrp.ApplicationSigner{KeyID: sk.KeyID, Algorithm: "ed25519", PrivateKeyBytes: seed})
	}
	return out, nil
}

// Verifier is the SDK-backed Provider.
type Verifier struct {
	Own        api.ApplicationRef
	InstanceID string
	Signers    []regularrp.ApplicationSigner
	Keys       KeySource
	// Digest is the expected presentation request digest. Default:
	// RequestDigest().
	Digest      []byte
	MaxAge      time.Duration
	SkewSeconds int64
	// ScopeSetTTL is the life of a signed scope set. Default 24 hours.
	ScopeSetTTL time.Duration
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

var _ Provider = (*Verifier)(nil)

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// MaxPresentationAge implements Provider.
func (v *Verifier) MaxPresentationAge() time.Duration { return v.MaxAge }

// SignScopeSet implements Provider. It offers every catalystcal scope.
func (v *Verifier) SignScopeSet(_ context.Context, req ScopeSetRequest) ([]byte, error) {
	if req.GranteeDomain == "" || req.GranteeUserID == "" || req.GranteeApplicationID == "" {
		return nil, errors.New("grantee_domain, grantee_user_id, and grantee_application_id are required")
	}
	now := v.now().UTC()
	ttl := v.ScopeSetTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	entries := make([]api.ActAsScopeEntry, 0, len(ScopeOrder))
	for _, s := range ScopeOrder {
		d := ScopeDescriptions[s]
		entries = append(entries, api.ActAsScopeEntry{Scope: s, Description: &d})
	}
	lang := "en"
	set := api.ActAsScopeSet{
		Audience: v.Own,
		Grantee: api.GranteeRef{Application: &api.ApplicationRef{
			SubjectUserId: req.GranteeUserID,
			SubjectDomain: strings.ToLower(req.GranteeDomain),
			ApplicationId: req.GranteeApplicationID,
		}},
		Entries:   entries,
		Language:  &lang,
		IssuedAt:  regularrp.FormatActAsTime(now),
		ExpiresAt: regularrp.FormatActAsTime(now.Add(ttl)),
	}
	signed, err := regularrp.SignScopeSet(set, v.InstanceID, v.Signers)
	if err != nil {
		return nil, err
	}
	return api.EncodeSignedActAsScopeSet(signed), nil
}

// VerifyCredential implements Provider: the SDK's audience checklist plus
// key resolution and a live revocation lookup.
func (v *Verifier) VerifyCredential(ctx context.Context, raw []byte, knownRevoked []string) (Verified, error) {
	cred, err := api.DecodeActAsCredential(raw)
	if err != nil {
		return Verified{}, fmt.Errorf("credential does not decode: %w", err)
	}
	grant, err := regularrp.DecodeGrant(cred.Grant)
	if err != nil {
		return Verified{}, err
	}
	signerInstance, err := regularrp.CredentialScopeSetSigner(cred)
	if err != nil {
		return Verified{}, err
	}
	ownKeys, err := v.Keys.OwnScopeSetKeys(ctx, v.Own, signerInstance)
	if err != nil {
		return Verified{}, fmt.Errorf("resolve own scope-set keys: %w", err)
	}
	var granteeKeys []regularrp.ApplicationKeyRef
	if inst, ok := regularrp.CredentialSignerInstance(cred); ok && grant.Grantee.Application != nil {
		app := grant.Grantee.Application
		granteeKeys, err = v.Keys.GranteeInstanceKeys(ctx, regularrp.InstanceRef{
			SubjectUserID: app.SubjectUserId,
			SubjectDomain: app.SubjectDomain,
			ApplicationID: app.ApplicationId,
			InstanceID:    inst,
		})
		if err != nil {
			return Verified{}, fmt.Errorf("resolve grantee keys: %w", err)
		}
	}
	domainKeys, err := v.Keys.DomainKeys(ctx, grant.SubjectDomain)
	if err != nil {
		return Verified{}, fmt.Errorf("resolve home domain keys: %w", err)
	}
	revoked := append([]string{}, knownRevoked...)
	live, err := v.Keys.RevokedGrants(ctx, grant.SubjectDomain, []string{grant.GrantId})
	if err != nil {
		return Verified{}, fmt.Errorf("resolve grant revocations: %w", err)
	}
	revoked = append(revoked, live...)
	digest := v.Digest
	if digest == nil {
		digest = RequestDigest()
	}
	skew := v.SkewSeconds
	if skew <= 0 {
		skew = 60
	}
	verified, err := regularrp.VerifyCredential(cred, regularrp.AudienceContext{
		OwnApplication:            v.Own,
		OwnScopeSetKeys:           ownKeys,
		IssuerDomainKeys:          domainKeys,
		GranteeInstanceKeys:       granteeKeys,
		ExpectedRequestDigest:     digest,
		RevokedGrantIDs:           revoked,
		MaxPresentationAgeSeconds: int64(v.MaxAge / time.Second),
		Now:                       v.now(),
		SkewSeconds:               skew,
		RevokedKeyPolicy:          regularrp.AcceptBeforeRevocation,
	})
	if err != nil {
		return Verified{}, err
	}
	expires, err := time.Parse(time.RFC3339, verified.ExpiresAt)
	if err != nil {
		return Verified{}, fmt.Errorf("grant expires_at: %w", err)
	}
	return Verified{
		GrantID:       verified.GrantID,
		SubjectUserID: grant.UserId,
		SubjectDomain: strings.ToLower(grant.SubjectDomain),
		Application:   GranteeName(grant.Grantee),
		Scopes:        MapScopes(verified.ApprovedScope),
		ExpiresAt:     expires.UTC(),
		Nonce:         hex.EncodeToString(verified.Nonce),
	}, nil
}

// RevokedGrants implements Provider.
func (v *Verifier) RevokedGrants(ctx context.Context, domain string, grantIDs []string) ([]string, error) {
	return v.Keys.RevokedGrants(ctx, domain, grantIDs)
}

// GranteeName is the audit name of a grantee.
func GranteeName(g api.GranteeRef) string {
	if g.Application != nil {
		return g.Application.ApplicationId + "@" + strings.ToLower(g.Application.SubjectDomain)
	}
	if g.LocalRpDescriptorFingerprint != nil {
		return "local-rp:" + *g.LocalRpDescriptorFingerprint
	}
	return "unknown"
}

// MapScopes keeps the approved scopes that are catalystcal Scope words, in the
// CSIL order, without duplicates.
func MapScopes(approved []string) []string {
	want := map[string]bool{}
	for _, s := range approved {
		want[s] = true
	}
	var out []string
	for _, s := range ScopeOrder {
		if want[s] {
			out = append(out, s)
		}
	}
	return out
}

// RPKeySource is the production KeySource: catalystcal's own LinkKeys RP over
// the SDK's pinned transport, with the SDK's CachedResolver for
// application keys.
type RPKeySource struct {
	Transport api.Transport
	Resolver  *regularrp.CachedResolver
}

// NewRPKeySource builds an RPKeySource over a transport.
func NewRPKeySource(t api.Transport) *RPKeySource {
	return &RPKeySource{Transport: t, Resolver: regularrp.NewCachedResolver(t, regularrp.CachedResolverOptions{})}
}

// OwnScopeSetKeys implements KeySource with AttestedKeyRefs (NOT
// UsableKeyRefs: a scope set signed before a key rotation must still
// verify).
func (r *RPKeySource) OwnScopeSetKeys(ctx context.Context, own api.ApplicationRef, instanceID string) ([]regularrp.ApplicationKeyRef, error) {
	res, err := r.Resolver.Resolve(ctx, regularrp.InstanceRef{
		SubjectUserID: own.SubjectUserId,
		SubjectDomain: own.SubjectDomain,
		ApplicationID: own.ApplicationId,
		InstanceID:    instanceID,
	}, nil)
	if err != nil {
		return nil, err
	}
	return res.Keys.AttestedKeyRefs(), nil
}

// GranteeInstanceKeys implements KeySource with UsableKeyRefs.
func (r *RPKeySource) GranteeInstanceKeys(ctx context.Context, ref regularrp.InstanceRef) ([]regularrp.ApplicationKeyRef, error) {
	res, err := r.Resolver.Resolve(ctx, ref, nil)
	if err != nil {
		return nil, err
	}
	return res.Keys.UsableKeyRefs(), nil
}

// DomainKeys implements KeySource through Rp/resolve-domain-keys. A key
// named by any revocation certificate in the response is dropped. That
// can only refuse more, never accept more, so it is safe without checking
// the certificate signatures here.
func (r *RPKeySource) DomainKeys(ctx context.Context, domain string) ([]api.DomainPublicKey, error) {
	resp, err := api.NewRpClient(r.Transport).ResolveDomainKeys(ctx, api.RpResolveDomainKeysRequest{Domain: domain})
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(resp.Domain, domain) {
		return nil, errors.New("RP domain-key response names another domain")
	}
	revoked := map[string]bool{}
	for _, c := range resp.Revocations {
		revoked[c.TargetKeyId] = true
	}
	var out []api.DomainPublicKey
	for _, k := range resp.Keys {
		if !revoked[k.KeyId] {
			out = append(out, k)
		}
	}
	return out, nil
}

// RevokedGrants implements KeySource with the SDK's verified lookup.
func (r *RPKeySource) RevokedGrants(ctx context.Context, domain string, grantIDs []string) ([]string, error) {
	var out []string
	for start := 0; start < len(grantIDs); start += regularrp.MaxRevocationLookupIDs {
		chunk := grantIDs[start:min(start+regularrp.MaxRevocationLookupIDs, len(grantIDs))]
		revs, err := regularrp.ResolveGrantRevocations(ctx, r.Transport, domain, chunk)
		if err != nil {
			return nil, err
		}
		for _, rv := range revs {
			out = append(out, rv.GrantId)
		}
	}
	return out, nil
}
