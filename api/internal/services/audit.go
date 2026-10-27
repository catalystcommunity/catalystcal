package services

import (
	"context"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// AuditService implements csil.AuditService.
type AuditService struct{ *Deps }

var _ csil.AuditService = (*AuditService)(nil)

// ListAudit returns entries where the caller is the principal or the
// actor. The caller is the real caller (for an act-as grant, the user).
// delegated_only keeps the entries where somebody else used the caller's
// authority.
func (s *AuditService) ListAudit(ctx context.Context, req csil.AuditQuery) (csil.AuditList, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.AuditList{}, err
	}
	limit := 100
	if req.Limit != nil && *req.Limit > 0 {
		limit = int(min(*req.Limit, 1000))
	}
	delegatedOnly := req.DelegatedOnly != nil && *req.DelegatedOnly
	rows, err := s.Store.ListAudit(ctx, id.UserID, delegatedOnly, limit)
	if err != nil {
		return csil.AuditList{}, internal(err, "list audit")
	}
	pp := s.people()
	out := csil.AuditList{Entries: []csil.AuditEntry{}}
	for _, a := range rows {
		e := csil.AuditEntry{
			AuditId:     csil.AuditID(a.ID),
			At:          ts(a.At),
			Via:         csil.ActingVia(a.Via),
			Application: optStr(a.Application),
			Operation:   a.Operation,
			Target:      optStr(a.Target),
			Outcome:     a.Outcome,
		}
		actor, err := pp.id(ctx, a.ActorID)
		if err != nil {
			return csil.AuditList{}, internal(err, "load actor")
		}
		principal, err := pp.id(ctx, a.PrincipalID)
		if err != nil {
			return csil.AuditList{}, internal(err, "load principal")
		}
		if actor != nil {
			e.Actor = *actor
		} else {
			e.Actor = csil.PersonSummary{UserId: csil.UserID(a.ActorID)}
		}
		if principal != nil {
			e.Principal = *principal
		} else {
			e.Principal = csil.PersonSummary{UserId: csil.UserID(a.PrincipalID)}
		}
		out.Entries = append(out.Entries, e)
	}
	return out, nil
}
