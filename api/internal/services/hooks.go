package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	log "github.com/sirupsen/logrus"
)

// RevokedGrants is the in-memory set of revoked act-as grant ids. The
// dispatcher's identity check reads it on every act-as call; the revocation
// worker fills it.
type RevokedGrants struct {
	mu  sync.RWMutex
	ids map[string]bool
}

// NewRevokedGrants makes an empty set.
func NewRevokedGrants() *RevokedGrants { return &RevokedGrants{ids: map[string]bool{}} }

// Has reports whether a grant is revoked.
func (r *RevokedGrants) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ids[id]
}

// Add marks grants revoked.
func (r *RevokedGrants) Add(ids ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		r.ids[id] = true
	}
}

// List returns every revoked id.
func (r *RevokedGrants) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.ids))
	for id := range r.ids {
		out = append(out, id)
	}
	return out
}

// LoadRevoked fills the set from the store at startup.
func (d *Deps) LoadRevoked(ctx context.Context) error {
	ids, err := d.Store.ListRevokedActAsGrantIDs(ctx)
	if err != nil {
		return err
	}
	d.Revoked.Add(ids...)
	return nil
}

// RefreshRevocations asks the home domains about every live grant and
// records the revoked ones. The worker calls it on a clock.
func (d *Deps) RefreshRevocations(ctx context.Context) error {
	if d.ActAs == nil {
		return nil
	}
	grants, err := d.Store.ListLiveActAsGrants(ctx)
	if err != nil {
		return err
	}
	byDomain := map[string][]string{}
	for _, g := range grants {
		byDomain[g.SubjectDomain] = append(byDomain[g.SubjectDomain], g.GrantID)
	}
	var firstErr error
	for domain, ids := range byDomain {
		revoked, err := d.ActAs.RevokedGrants(ctx, domain, ids)
		if err != nil {
			log.WithError(err).WithField("domain", domain).Warn("act-as: revocation lookup failed")
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(revoked) == 0 {
			continue
		}
		if err := d.Store.MarkActAsGrantsRevoked(ctx, revoked); err != nil {
			return err
		}
		d.Revoked.Add(revoked...)
		log.WithFields(log.Fields{"domain": domain, "revoked": len(revoked)}).Info("act-as: grants revoked")
	}
	return firstErr
}

// CheckIdentity is the dispatcher's per-call bearer re-check:
//
//   - self: the user still exists;
//   - delegation: the delegation is live (not revoked, not expired), it
//     still joins this principal and this delegate, and both users exist.
//     The bearer's scopes shrink to the delegation's current scopes, so an
//     update-delegation that removes a scope works at once;
//   - act_as_grant: the grant is not revoked and the user exists.
func (d *Deps) CheckIdentity(ctx context.Context, id *auth.Identity) error {
	if _, err := d.Store.GetUser(ctx, id.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errors.New("user no longer exists")
		}
		return fmt.Errorf("identity check: %w", err)
	}
	switch id.Via {
	case auth.ViaSelf, "":
		if id.PrincipalID != "" && id.PrincipalID != id.UserID {
			return errors.New("self bearer names a principal")
		}
		return nil
	case auth.ViaDelegation:
		if id.DelegationID == "" || id.PrincipalID == "" {
			return errors.New("delegated bearer without a delegation")
		}
		dl, err := d.Store.GetDelegation(ctx, id.DelegationID)
		if err != nil {
			return errors.New("delegation is no longer in effect")
		}
		if dl.PrincipalID != id.PrincipalID ||
			!strings.EqualFold(dl.DelegateDomain, id.Domain) || dl.DelegateSubject != id.Subject ||
			!dl.LiveAt(d.now()) {
			return errors.New("delegation is no longer in effect")
		}
		if _, err := d.Store.GetUser(ctx, id.PrincipalID); err != nil {
			return errors.New("principal no longer exists")
		}
		id.Scopes = intersect(id.Scopes, dl.Scopes)
		return nil
	case auth.ViaActAsGrant:
		if id.GrantID == "" {
			return errors.New("act-as bearer without a grant")
		}
		if d.Revoked != nil && d.Revoked.Has(id.GrantID) {
			return errors.New("act-as grant is revoked")
		}
		return nil
	}
	return errors.New("unknown bearer kind")
}

func intersect(a, b []string) []string {
	in := map[string]bool{}
	for _, s := range b {
		in[s] = true
	}
	var out []string
	for _, s := range a {
		if in[s] {
			out = append(out, s)
		}
	}
	return out
}

// AuditRecorder writes dispatcher audit entries to audit_log.
type AuditRecorder struct{ Store *store.Store }

// RecordAudit implements csilrpc.AuditRecorder. The actor is the real
// caller; the principal is whose authority the call used.
func (a AuditRecorder) RecordAudit(ctx context.Context, e csilrpc.AuditEntry) error {
	id := e.Identity
	return a.Store.InsertAudit(ctx, store.AuditRow{
		ActorID:     id.UserID,
		PrincipalID: id.EffectiveUserID(),
		Via:         viaOrSelf(id.Via),
		Application: id.Application,
		Operation:   e.Service + "." + e.Method,
		Target:      e.Target,
		Outcome:     e.Outcome,
	})
}

func viaOrSelf(v string) string {
	if v == "" {
		return auth.ViaSelf
	}
	return v
}
