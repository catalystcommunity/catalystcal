package services

import (
	"context"
	"errors"
	"strings"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// ShareService implements csil.ShareService.
type ShareService struct{ *Deps }

var _ csil.ShareService = (*ShareService)(nil)

func (s *ShareService) shareOut(ctx context.Context, sh store.Share) (csil.Share, error) {
	out := csil.Share{
		ShareId:    csil.ShareID(sh.ID),
		CalendarId: csil.CalendarID(sh.CalendarID),
		Grantee:    csil.IdentityRef{Domain: sh.GranteeDomain, UserId: ptr(sh.GranteeSubject)},
		Role:       csil.ShareRole(sh.Role),
		CreatedAt:  ts(sh.CreatedAt),
	}
	u, err := s.Store.GetUserByIdentity(ctx, sh.GranteeDomain, sh.GranteeSubject)
	if err == nil {
		p := person(u)
		out.GranteePerson = &p
		if u.Handle != "" {
			out.Grantee.Handle = ptr(u.Handle)
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return csil.Share{}, internal(err, "load grantee")
	}
	return out, nil
}

func (s *ShareService) managed(ctx context.Context, calendarID string) (store.Calendar, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return store.Calendar{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return store.Calendar{}, err
	}
	c, acc, err := r.loadCalendar(ctx, calendarID)
	if err != nil {
		return store.Calendar{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	if !acc.Manage {
		return store.Calendar{}, csilrpc.Forbidden("you cannot manage the sharing of this calendar")
	}
	return c, nil
}

// ListShares lists the shares of a calendar (manage access).
func (s *ShareService) ListShares(ctx context.Context, req csil.CalendarID) (csil.ShareList, error) {
	c, err := s.managed(ctx, string(req))
	if err != nil {
		return csil.ShareList{}, err
	}
	shares, err := s.Store.ListShares(ctx, c.ID)
	if err != nil {
		return csil.ShareList{}, internal(err, "list shares")
	}
	out := csil.ShareList{Shares: []csil.Share{}}
	for _, sh := range shares {
		so, err := s.shareOut(ctx, sh)
		if err != nil {
			return csil.ShareList{}, err
		}
		out.Shares = append(out.Shares, so)
	}
	return out, nil
}

// PutShare creates or changes the share for a grantee (manage access).
func (s *ShareService) PutShare(ctx context.Context, req csil.Share) (csil.Share, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.Share{}, err
	}
	c, err := s.managed(ctx, string(req.CalendarId))
	if err != nil {
		return csil.Share{}, err
	}
	if access.FromShareRole(string(req.Role)) == access.None {
		return csil.Share{}, badReq("role must be free_busy, read, write, or manage")
	}
	domain, subject, _, err := s.resolveRef(ctx, req.Grantee)
	if err != nil {
		return csil.Share{}, err
	}
	owner, err := s.Store.GetUser(ctx, c.OwnerID)
	if err != nil {
		return csil.Share{}, internal(err, "load owner")
	}
	if strings.EqualFold(owner.Domain, domain) && owner.Subject == subject {
		return csil.Share{}, badReq("the owner already has every right on this calendar")
	}
	sh, err := s.Store.PutShare(ctx, store.Share{
		CalendarID: c.ID, GranteeDomain: domain, GranteeSubject: subject, Role: string(req.Role), CreatedBy: id.EffectiveUserID(),
	})
	if err != nil {
		return csil.Share{}, internal(err, "put share")
	}
	csilrpc.SetAuditTarget(ctx, "share:"+sh.ID)
	return s.shareOut(ctx, sh)
}

// DeleteShare removes a share. A manager of the calendar can, and so can
// the grantee (leaving a calendar).
func (s *ShareService) DeleteShare(ctx context.Context, req csil.ShareID) (csil.EmptyResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	sh, err := s.Store.GetShare(ctx, string(req))
	if errors.Is(err, store.ErrNotFound) {
		return csil.EmptyResponse{}, errNotFound("share")
	}
	if err != nil {
		return csil.EmptyResponse{}, internal(err, "load share")
	}
	csilrpc.SetAuditTarget(ctx, "share:"+sh.ID)
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	self := !id.IsDelegated() && strings.EqualFold(sh.GranteeDomain, r.effDomain) && sh.GranteeSubject == r.effSubject
	if !self {
		if _, err := s.managed(ctx, sh.CalendarID); err != nil {
			return csil.EmptyResponse{}, err
		}
	}
	if err := s.Store.DeleteShare(ctx, sh.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		return csil.EmptyResponse{}, internal(err, "delete share")
	}
	return csil.EmptyResponse{}, nil
}
