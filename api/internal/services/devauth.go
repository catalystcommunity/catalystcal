package services

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	log "github.com/sirupsen/logrus"
)

// DevDomain is the domain of users made by dev-login.
const DevDomain = "dev.local"

// DevAuthService implements csil.DevAuthService. It is registered only
// when dev auth is allowed (CATALYSTCAL_DEV_AUTH_ENABLED in dev or nonprod).
type DevAuthService struct{ *Deps }

var _ csil.DevAuthService = (*DevAuthService)(nil)

var devHandle = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// ListDevUsers lists every user, for the dev sign-in picker.
func (s *DevAuthService) ListDevUsers(ctx context.Context, _ csil.EmptyRequest) (csil.DevUsersResponse, error) {
	users, err := s.Store.ListUsers(ctx)
	if err != nil {
		return csil.DevUsersResponse{}, internal(err, "list users")
	}
	out := csil.DevUsersResponse{Users: []csil.DevUser{}}
	for _, u := range users {
		out.Users = append(out.Users, csil.DevUser{
			UserId: csil.UserID(u.ID), Handle: optStr(u.Handle), DisplayName: optStr(u.DisplayName), Domain: u.Domain,
		})
	}
	return out, nil
}

// DevLogin signs in as an existing user, or makes a dev.local user with
// the handle (and a default calendar).
func (s *DevAuthService) DevLogin(ctx context.Context, req csil.DevLoginRequest) (csil.LoginResponse, error) {
	var u store.User
	var err error
	switch {
	case req.UserId != nil && *req.UserId != "":
		u, err = s.Store.GetUser(ctx, string(*req.UserId))
		if errors.Is(err, store.ErrNotFound) {
			return csil.LoginResponse{}, errNotFound("user")
		}
		if err != nil {
			return csil.LoginResponse{}, internal(err, "dev-login load user")
		}
	case req.Handle != nil && *req.Handle != "":
		handle := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(*req.Handle), "@"))
		if !devHandle.MatchString(handle) {
			return csil.LoginResponse{}, csilrpc.BadRequest("handle must be 1-63 letters, digits, '.', '_', or '-'")
		}
		name := deref(req.DisplayName)
		if name == "" {
			name = handle
		}
		u, _, err = s.Store.UpsertUser(ctx, store.UserInput{Domain: DevDomain, Subject: handle, Handle: handle, DisplayName: name})
		if err != nil {
			return csil.LoginResponse{}, internal(err, "dev-login upsert user")
		}
	default:
		return csil.LoginResponse{}, csilrpc.BadRequest("user_id or handle is required")
	}
	log.WithFields(log.Fields{"user_id": u.ID, "domain": u.Domain}).Warn("DEV-AUTH: signed in without LinkKeys")
	return s.browserLogin(ctx, u)
}
