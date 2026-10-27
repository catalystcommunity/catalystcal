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

// DelegationService implements csil.DelegationService. Every op needs a
// self bearer: a delegate cannot grant, extend, or see delegations of the
// principal.
type DelegationService struct{ *Deps }

var _ csil.DelegationService = (*DelegationService)(nil)

func (s *DelegationService) delegationOut(ctx context.Context, pp *people, d store.Delegation) (csil.Delegation, error) {
	principal, err := pp.id(ctx, d.PrincipalID)
	if err != nil {
		return csil.Delegation{}, internal(err, "load principal")
	}
	delegate, err := pp.identity(ctx, d.DelegateDomain, d.DelegateSubject)
	if err != nil {
		return csil.Delegation{}, internal(err, "load delegate")
	}
	out := csil.Delegation{
		DelegationId:   csil.DelegationID(d.ID),
		Delegate:       csil.IdentityRef{Domain: d.DelegateDomain, UserId: ptr(d.DelegateSubject)},
		DelegatePerson: delegate,
		Scopes:         scopesOut(d.Scopes),
		Note:           optStr(d.Note),
		ExpiresAt:      tsPtr(d.ExpiresAt),
		CreatedAt:      ts(d.CreatedAt),
		RevokedAt:      tsPtr(d.RevokedAt),
	}
	if principal != nil {
		out.Principal = *principal
	}
	if delegate != nil && delegate.Handle != nil {
		out.Delegate.Handle = delegate.Handle
	}
	for _, c := range d.CalendarIDs {
		out.CalendarIds = append(out.CalendarIds, csil.CalendarID(c))
	}
	return out, nil
}

func (s *DelegationService) list(ctx context.Context, ds []store.Delegation) (csil.DelegationList, error) {
	pp := s.people()
	out := csil.DelegationList{Delegations: []csil.Delegation{}}
	for _, d := range ds {
		do, err := s.delegationOut(ctx, pp, d)
		if err != nil {
			return csil.DelegationList{}, err
		}
		out.Delegations = append(out.Delegations, do)
	}
	return out, nil
}

// ListGranted lists the delegations the caller gave.
func (s *DelegationService) ListGranted(ctx context.Context, _ csil.EmptyRequest) (csil.DelegationList, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.DelegationList{}, err
	}
	ds, err := s.Store.ListDelegationsByPrincipal(ctx, id.UserID)
	if err != nil {
		return csil.DelegationList{}, internal(err, "list delegations")
	}
	return s.list(ctx, ds)
}

// ListReceived lists the delegations given to the caller.
func (s *DelegationService) ListReceived(ctx context.Context, _ csil.EmptyRequest) (csil.DelegationList, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.DelegationList{}, err
	}
	ds, err := s.Store.ListDelegationsByDelegate(ctx, id.Domain, id.Subject)
	if err != nil {
		return csil.DelegationList{}, internal(err, "list delegations")
	}
	return s.list(ctx, ds)
}

// checkScopesAndCalendars validates the writable parts of a delegation.
func (s *DelegationService) checkScopesAndCalendars(ctx context.Context, principalID string, req csil.Delegation) ([]string, []string, error) {
	if len(req.Scopes) == 0 {
		return nil, nil, badReq("scopes must not be empty")
	}
	want := map[string]bool{}
	for _, sc := range req.Scopes {
		if !access.ValidScope(string(sc)) {
			return nil, nil, badReq("unknown scope " + string(sc))
		}
		want[string(sc)] = true
	}
	var scopes []string
	for _, sc := range access.AllScopes {
		if want[sc] {
			scopes = append(scopes, sc)
		}
	}
	var cals []string
	for _, cid := range req.CalendarIds {
		c, err := s.Store.GetCalendar(ctx, string(cid))
		if err != nil || c.OwnerID != principalID {
			return nil, nil, badReq("calendar_ids must name your own calendars (" + string(cid) + ")")
		}
		cals = append(cals, c.ID)
	}
	return scopes, cals, nil
}

// CreateDelegation lets another person act as the caller.
func (s *DelegationService) CreateDelegation(ctx context.Context, req csil.Delegation) (csil.Delegation, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.Delegation{}, err
	}
	domain, subject, _, err := s.resolveRef(ctx, req.Delegate)
	if err != nil {
		return csil.Delegation{}, err
	}
	if strings.EqualFold(domain, id.Domain) && subject == id.Subject {
		return csil.Delegation{}, badReq("you cannot delegate to yourself")
	}
	scopes, cals, err := s.checkScopesAndCalendars(ctx, id.UserID, req)
	if err != nil {
		return csil.Delegation{}, err
	}
	d := store.Delegation{
		PrincipalID: id.UserID, DelegateDomain: domain, DelegateSubject: subject,
		Scopes: scopes, CalendarIDs: cals, Note: strings.TrimSpace(deref(req.Note)),
	}
	if req.ExpiresAt != nil {
		t, err := parseTS("expires_at", *req.ExpiresAt)
		if err != nil {
			return csil.Delegation{}, err
		}
		if !t.After(s.now()) {
			return csil.Delegation{}, badReq("expires_at must be in the future")
		}
		d.ExpiresAt = &t
	}
	d, err = s.Store.CreateDelegation(ctx, d)
	if errors.Is(err, store.ErrConflict) {
		return csil.Delegation{}, csilrpc.Conflict("a delegation to this person is already in effect; update it instead")
	}
	if err != nil {
		return csil.Delegation{}, internal(err, "create delegation")
	}
	csilrpc.SetAuditTarget(ctx, "delegation:"+d.ID)
	return s.delegationOut(ctx, s.people(), d)
}

func (s *DelegationService) own(ctx context.Context, delegationID string) (store.Delegation, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return store.Delegation{}, err
	}
	d, err := s.Store.GetDelegation(ctx, delegationID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && d.PrincipalID != id.UserID) {
		return store.Delegation{}, errNotFound("delegation")
	}
	if err != nil {
		return store.Delegation{}, internal(err, "load delegation")
	}
	csilrpc.SetAuditTarget(ctx, "delegation:"+d.ID)
	return d, nil
}

// UpdateDelegation changes scopes, calendars, note, and expiry. The
// delegate cannot be changed.
func (s *DelegationService) UpdateDelegation(ctx context.Context, req csil.Delegation) (csil.Delegation, error) {
	d, err := s.own(ctx, string(req.DelegationId))
	if err != nil {
		return csil.Delegation{}, err
	}
	if d.RevokedAt != nil {
		return csil.Delegation{}, csilrpc.Conflict("this delegation is revoked")
	}
	scopes, cals, err := s.checkScopesAndCalendars(ctx, d.PrincipalID, req)
	if err != nil {
		return csil.Delegation{}, err
	}
	d.Scopes, d.CalendarIDs, d.Note, d.ExpiresAt = scopes, cals, strings.TrimSpace(deref(req.Note)), nil
	if req.ExpiresAt != nil {
		t, err := parseTS("expires_at", *req.ExpiresAt)
		if err != nil {
			return csil.Delegation{}, err
		}
		d.ExpiresAt = &t
	}
	d, err = s.Store.UpdateDelegation(ctx, d)
	if err != nil {
		return csil.Delegation{}, internal(err, "update delegation")
	}
	return s.delegationOut(ctx, s.people(), d)
}

// RevokeDelegation ends a delegation. Every delegated bearer from it fails
// on its next call.
func (s *DelegationService) RevokeDelegation(ctx context.Context, req csil.DelegationID) (csil.EmptyResponse, error) {
	d, err := s.own(ctx, string(req))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.RevokeDelegation(ctx, d.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		return csil.EmptyResponse{}, internal(err, "revoke delegation")
	}
	return csil.EmptyResponse{}, nil
}
