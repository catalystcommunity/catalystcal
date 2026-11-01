package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/catalystcommunity/catalystcal/api/internal/apppresent"
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	log "github.com/sirupsen/logrus"
)

// ---- application principals ------------------------------------------------

func errAppsOff() error {
	return csilrpc.NewError(http.StatusServiceUnavailable, "application sign-in is not configured on this server")
}

// GetApplicationIdentity returns catalystcal's own application identity:
// the audience that an application presentation must name.
func (s *AuthService) GetApplicationIdentity(_ context.Context, _ csil.EmptyRequest) (csil.ApplicationIdentityResponse, error) {
	if s.Apps == nil {
		return csil.ApplicationIdentityResponse{}, errAppsOff()
	}
	return csil.ApplicationIdentityResponse{Audience: s.Apps.Own.CSIL()}, nil
}

// ExchangeApplicationPresentation verifies an application presentation,
// records its nonce, and mints a self bearer for the application
// principal. The bearer is short: a revoked key or a removed application
// stops at the next exchange, and the dispatcher re-checks the allow list
// on each call.
func (s *AuthService) ExchangeApplicationPresentation(ctx context.Context, req csil.ExchangeApplicationPresentationRequest) (csil.LoginResponse, error) {
	if s.Apps == nil {
		return csil.LoginResponse{}, errAppsOff()
	}
	v, err := s.Apps.Verify(ctx, req.Presentation)
	switch {
	case errors.Is(err, apppresent.ErrStale):
		return csil.LoginResponse{}, csilrpc.NewError(http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, apppresent.ErrNotAllowed):
		return csil.LoginResponse{}, csilrpc.Forbidden(err.Error())
	case err != nil:
		log.WithError(err).Info("auth: application presentation refused")
		return csil.LoginResponse{}, csilrpc.Unauthorized(err.Error())
	}
	now := s.now()
	if err := s.Store.RecordActAsNonce(ctx, v.NonceKey(), now.Add(s.Apps.NonceHold())); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return csil.LoginResponse{}, csilrpc.Unauthorized("application presentation was already used")
		}
		return csil.LoginResponse{}, internal(err, "record application nonce")
	}
	u, _, err := s.Store.UpsertUser(ctx, store.UserInput{
		Domain: v.Signer.SubjectDomain, Subject: v.Signer.SubjectUserID, ApplicationID: v.Signer.ApplicationID,
	})
	if err != nil {
		return csil.LoginResponse{}, internal(err, "upsert application principal")
	}
	app := v.Signer.ApplicationID + "@" + v.Signer.SubjectDomain
	ident := auth.Identity{
		UserID: u.ID, Domain: u.Domain, Subject: u.Subject, DisplayName: u.DisplayName,
		ApplicationID: u.ApplicationID, Application: app, Via: auth.ViaSelf,
	}
	if err := s.Store.InsertAudit(ctx, store.AuditRow{
		ActorID: u.ID, PrincipalID: u.ID, Via: auth.ViaSelf, Application: app,
		Operation: "auth.ExchangeApplicationPresentation", Target: "instance:" + v.InstanceID, Outcome: "ok",
	}); err != nil {
		log.WithError(err).Warn("audit: application exchange")
	}
	ttl := auth.DefaultTTL
	if s.Cfg != nil && s.Cfg.ApplicationBearerTTL > 0 {
		ttl = s.Cfg.ApplicationBearerTTL
	}
	return s.mint(ident, ttl, person(u), nil)
}
