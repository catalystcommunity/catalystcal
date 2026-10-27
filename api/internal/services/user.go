package services

import (
	"context"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
)

// UserService implements csil.UserService.
type UserService struct{ *Deps }

var _ csil.UserService = (*UserService)(nil)

// GetProfile returns the effective user's profile (the principal for a
// delegated bearer: the call runs as the principal).
func (s *UserService) GetProfile(ctx context.Context, _ csil.EmptyRequest) (csil.User, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.User{}, err
	}
	u, err := s.Store.GetUser(ctx, id.EffectiveUserID())
	if err != nil {
		return csil.User{}, errNotFound("user")
	}
	return userOut(u), nil
}

// UpdateProfile changes the caller's display name and time zone. A
// delegated bearer cannot.
func (s *UserService) UpdateProfile(ctx context.Context, req csil.UpdateProfileRequest) (csil.User, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.User{}, err
	}
	if req.DisplayName != nil {
		v := strings.TrimSpace(*req.DisplayName)
		if len(v) > 256 {
			return csil.User{}, csilrpc.BadRequest("display_name is longer than 256 bytes")
		}
		req.DisplayName = &v
	}
	if req.Timezone != nil && *req.Timezone != "" {
		if err := checkZone(*req.Timezone); err != nil {
			return csil.User{}, err
		}
	}
	u, err := s.Store.UpdateProfile(ctx, id.UserID, req.DisplayName, req.Timezone)
	if err != nil {
		return csil.User{}, internal(err, "update profile")
	}
	csilrpc.SetAuditTarget(ctx, "user:"+u.ID)
	return userOut(u), nil
}

// LookupPerson finds a person who signed in to this server.
func (s *UserService) LookupPerson(ctx context.Context, req csil.LookupPersonRequest) (csil.PersonSummary, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return csil.PersonSummary{}, err
	}
	_, _, u, err := s.resolveRef(ctx, req.Identity)
	if err != nil {
		return csil.PersonSummary{}, err
	}
	if u == nil {
		return csil.PersonSummary{}, errHandleUnknown
	}
	return person(*u), nil
}

func checkZone(tz string) error {
	if tz == "" {
		return nil
	}
	if _, err := time.LoadLocation(tz); err != nil || strings.EqualFold(tz, "local") {
		return csilrpc.BadRequest("unknown IANA time zone " + tz)
	}
	return nil
}
