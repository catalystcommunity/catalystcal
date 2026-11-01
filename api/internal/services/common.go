// Package services implements the generated CSIL service interfaces. There
// is one file per service. Every authorization decision goes through
// access.Effective (via the resolver in this file).
package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/actas"
	"github.com/catalystcommunity/catalystcal/api/internal/apppresent"
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/config"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	"github.com/catalystcommunity/catalystcal/api/internal/subscription"
	log "github.com/sirupsen/logrus"
)

// Deps is what every service needs.
type Deps struct {
	Store  *store.Store
	Cfg    *config.Config
	Secret []byte
	// PKI is the LinkKeys RP client. Nil disables Complete.
	PKI PKIClient
	// ActAs is the act-as audience. Nil makes the act-as ops answer 503.
	ActAs actas.Provider
	// Apps verifies application presentations. Nil makes the application
	// ops answer 503.
	Apps    *apppresent.Verifier
	Fetcher *subscription.Fetcher
	Revoked *RevokedGrants
	// Now is the clock.
	Now func() time.Time
}

func (d *Deps) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

// Limits.
const (
	// MaxRangeDays is the widest ListEvents / free-busy range.
	MaxRangeDays = 400
	// MaxOccurrences caps the instances of one ListEvents call.
	MaxOccurrences = 5000
)

// ---- errors -------------------------------------------------------------

func internal(err error, what string) error {
	log.WithError(err).Error(what)
	return csilrpc.Internal("internal error")
}

func errNotFound(kind string) error { return csilrpc.NotFound(kind + " not found") }

// errHandleUnknown is the message docs/access-model.md requires.
var errHandleUnknown = csilrpc.NotFound("that person has not signed in to this catalystcal server yet; use their account id")

func errDelegatedForbidden() error {
	return csilrpc.Forbidden("a delegated bearer cannot do this; use your own sign-in")
}

func requireIdentity(ctx context.Context) (*auth.Identity, error) {
	id := auth.IdentityFromContext(ctx)
	if id == nil {
		return nil, csilrpc.Unauthorized("missing identity")
	}
	return id, nil
}

func requireSelf(ctx context.Context) (*auth.Identity, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if id.IsDelegated() {
		return nil, errDelegatedForbidden()
	}
	return id, nil
}

// ---- small conversions ----------------------------------------------------

func ptr[T any](v T) *T { return &v }

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ts(t time.Time) csil.Timestamp { return csil.Timestamp(store.FormatTime(t)) }

func tsPtr(t *time.Time) *csil.Timestamp {
	if t == nil {
		return nil
	}
	v := ts(*t)
	return &v
}

func parseTS(field string, v csil.Timestamp) (time.Time, error) {
	t, err := store.ParseTime(string(v))
	if err != nil {
		return time.Time{}, csilrpc.BadRequest(field + " must be an RFC3339 UTC time like 2026-10-15T17:00:00Z")
	}
	return t, nil
}

func person(u store.User) csil.PersonSummary {
	return csil.PersonSummary{
		UserId:        csil.UserID(u.ID),
		Domain:        u.Domain,
		Subject:       u.Subject,
		ApplicationId: optStr(u.ApplicationID),
		Handle:        optStr(u.Handle),
		DisplayName:   optStr(u.DisplayName),
	}
}

func userOut(u store.User) csil.User {
	return csil.User{
		UserId:        csil.UserID(u.ID),
		Domain:        u.Domain,
		Subject:       u.Subject,
		ApplicationId: optStr(u.ApplicationID),
		Handle:        optStr(u.Handle),
		DisplayName:   optStr(u.DisplayName),
		Email:         optStr(u.Email),
		Timezone:      optStr(u.Timezone),
		CreatedAt:     ts(u.CreatedAt),
		UpdatedAt:     ts(u.UpdatedAt),
	}
}

func (d *Deps) personByID(ctx context.Context, id string) (*csil.PersonSummary, error) {
	if id == "" {
		return nil, nil
	}
	u, err := d.Store.GetUser(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p := person(u)
	return &p, nil
}

// ---- identities ----------------------------------------------------------

// resolveRef turns an IdentityRef into a principal and the user, when the
// person (or the application principal) signed in here. A user_id needs no
// sign-in; a handle does. An application_id needs a user_id: a handle names
// a person, not an application.
func (d *Deps) resolveRef(ctx context.Context, ref csil.IdentityRef) (store.Principal, *store.User, error) {
	domain := strings.ToLower(strings.TrimSpace(ref.Domain))
	if domain == "" {
		return store.Principal{}, nil, csilrpc.BadRequest("identity.domain is required")
	}
	app := strings.TrimSpace(deref(ref.ApplicationId))
	if ref.UserId != nil && *ref.UserId != "" {
		p := store.Principal{Domain: domain, Subject: *ref.UserId, Application: app}
		found, err := d.Store.GetPrincipal(ctx, p.Domain, p.Subject, p.Application)
		if err == nil {
			return p, &found, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return store.Principal{}, nil, internal(err, "resolve identity")
		}
		return p, nil, nil
	}
	if app != "" {
		return store.Principal{}, nil, csilrpc.BadRequest("identity with application_id needs user_id")
	}
	if ref.Handle != nil && *ref.Handle != "" {
		found, err := d.Store.GetUserByHandle(ctx, domain, strings.TrimPrefix(*ref.Handle, "@"))
		if errors.Is(err, store.ErrNotFound) {
			return store.Principal{}, nil, errHandleUnknown
		}
		if err != nil {
			return store.Principal{}, nil, internal(err, "resolve handle")
		}
		return store.Principal{Domain: domain, Subject: found.Subject}, &found, nil
	}
	return store.Principal{}, nil, csilrpc.BadRequest("identity needs user_id or handle")
}

// identityRef is the output form of a principal: the account id, and the
// handle when the person signed in here.
func identityRef(p store.Principal, u *store.User) csil.IdentityRef {
	ref := csil.IdentityRef{Domain: p.Domain, UserId: ptr(p.Subject), ApplicationId: optStr(p.Application)}
	if u != nil && u.Handle != "" {
		ref.Handle = ptr(u.Handle)
	}
	return ref
}

// ---- the access resolver ---------------------------------------------------

// resolver gathers the facts access.Effective needs for one caller and
// caches them for the call.
type resolver struct {
	d  *Deps
	id *auth.Identity
	// The effective user's identity (the principal for a delegated bearer).
	// effApp is set for an application principal.
	effDomain, effSubject, effApp string
	delegationCalendars           []string
	shareRoles                    map[string]access.Role
	// groups holds the ids of the groups that contain the effective user.
	// Nil until the first share lookup.
	groups []string
}

func (d *Deps) resolver(ctx context.Context, id *auth.Identity) (*resolver, error) {
	r := &resolver{d: d, id: id, effDomain: strings.ToLower(id.Domain), effSubject: id.Subject, effApp: id.ApplicationID,
		shareRoles: map[string]access.Role{}}
	if id.EffectiveUserID() != id.UserID {
		u, err := d.Store.GetUser(ctx, id.EffectiveUserID())
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, csilrpc.Unauthorized("principal no longer exists")
			}
			return nil, internal(err, "load principal")
		}
		r.effDomain, r.effSubject, r.effApp = u.Domain, u.Subject, u.ApplicationID
	}
	if id.Via == auth.ViaDelegation && id.DelegationID != "" {
		cals, err := d.Store.DelegationCalendars(ctx, id.DelegationID)
		if err != nil {
			return nil, internal(err, "load delegation calendars")
		}
		r.delegationCalendars = cals
	}
	return r, nil
}

// principal is the effective user's identity.
func (r *resolver) principal() store.Principal {
	return store.Principal{Domain: r.effDomain, Subject: r.effSubject, Application: r.effApp}
}

// groupIDs returns the groups that contain the effective user. Group
// membership is read at each call, so a removal works at once.
func (r *resolver) groupIDs(ctx context.Context) ([]string, error) {
	if r.groups != nil {
		return r.groups, nil
	}
	ids, err := r.d.Store.GroupsContaining(ctx, r.principal())
	if err != nil {
		return nil, internal(err, "load groups")
	}
	if ids == nil {
		ids = []string{}
	}
	r.groups = ids
	return ids, nil
}

// highestRole is the highest of the share roles (direct and through
// groups).
func highestRole(roles []string) access.Role {
	best := access.None
	for _, s := range roles {
		best = max(best, access.FromShareRole(s))
	}
	return best
}

// calendar returns the caller's effective access on a calendar.
func (r *resolver) calendar(ctx context.Context, c store.Calendar) (access.Access, error) {
	role, ok := r.shareRoles[c.ID]
	if !ok {
		role = access.None
		if c.OwnerID != r.id.EffectiveUserID() {
			groups, err := r.groupIDs(ctx)
			if err != nil {
				return access.Access{}, err
			}
			roles, err := r.d.Store.ShareRoles(ctx, c.ID, r.principal(), groups)
			if err != nil {
				return access.Access{}, internal(err, "load share role")
			}
			role = highestRole(roles)
		}
		r.shareRoles[c.ID] = role
	}
	return access.Effective(r.id, access.Facts{
		CalendarID:          c.ID,
		OwnerID:             c.OwnerID,
		ShareRole:           role,
		DelegationCalendars: r.delegationCalendars,
	}), nil
}

// loadCalendar reads a calendar the caller can see. None access is 404.
func (r *resolver) loadCalendar(ctx context.Context, id string) (store.Calendar, access.Access, error) {
	c, err := r.d.Store.GetCalendar(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.Calendar{}, access.Access{}, errNotFound("calendar")
	}
	if err != nil {
		return store.Calendar{}, access.Access{}, internal(err, "load calendar")
	}
	acc, err := r.calendar(ctx, c)
	if err != nil {
		return store.Calendar{}, access.Access{}, err
	}
	if !acc.CanSee() {
		return store.Calendar{}, access.Access{}, errNotFound("calendar")
	}
	return c, acc, nil
}

// visibleCalendars returns every calendar the caller can see: the
// effective user's own, then those shared with the effective user.
func (r *resolver) visibleCalendars(ctx context.Context) ([]store.Calendar, []access.Access, error) {
	own, err := r.d.Store.ListCalendarsByOwner(ctx, r.id.EffectiveUserID())
	if err != nil {
		return nil, nil, internal(err, "list calendars")
	}
	groups, err := r.groupIDs(ctx)
	if err != nil {
		return nil, nil, err
	}
	shared, err := r.d.Store.ListCalendarsSharedWith(ctx, r.principal(), groups)
	if err != nil {
		return nil, nil, internal(err, "list shared calendars")
	}
	// One calendar can have more than one share for the caller: keep the
	// highest role, and list the calendar once.
	best := map[string]access.Role{}
	for _, sc := range shared {
		best[sc.Calendar.ID] = max(best[sc.Calendar.ID], access.FromShareRole(sc.Role))
	}
	var cals []store.Calendar
	var accs []access.Access
	for _, c := range own {
		a, err := r.calendar(ctx, c)
		if err != nil {
			return nil, nil, err
		}
		if a.CanSee() {
			cals, accs = append(cals, c), append(accs, a)
		}
	}
	listed := map[string]bool{}
	for _, sc := range shared {
		if sc.Calendar.OwnerID == r.id.EffectiveUserID() || listed[sc.Calendar.ID] {
			continue
		}
		listed[sc.Calendar.ID] = true
		r.shareRoles[sc.Calendar.ID] = best[sc.Calendar.ID]
		a, err := r.calendar(ctx, sc.Calendar)
		if err != nil {
			return nil, nil, err
		}
		if a.CanSee() {
			cals, accs = append(cals, sc.Calendar), append(accs, a)
		}
	}
	return cals, accs, nil
}

// isAttendee reports whether the effective user is an attendee of ev.
func (r *resolver) isAttendee(ev csil.Event) bool {
	for _, a := range ev.Attendees {
		if a.Identity != nil && a.Identity.UserId != nil &&
			strings.EqualFold(a.Identity.Domain, r.effDomain) && *a.Identity.UserId == r.effSubject {
			return true
		}
	}
	return false
}

// eventAccess returns the calendar, the calendar access, and the event view
// of one row. A row the caller cannot see is 404.
func (r *resolver) eventAccess(ctx context.Context, e store.Event) (store.Calendar, access.Access, access.EventView, error) {
	c, err := r.d.Store.GetCalendar(ctx, string(e.Ev.CalendarId))
	if err != nil {
		return store.Calendar{}, access.Access{}, access.EventView{}, internal(err, "load event calendar")
	}
	acc, err := r.calendar(ctx, c)
	if err != nil {
		return store.Calendar{}, access.Access{}, access.EventView{}, err
	}
	view := access.ViewEvent(r.id, acc, access.IsPrivate(e.Ev.Visibility), r.isAttendee(e.Ev))
	if !view.CanSee() {
		return store.Calendar{}, access.Access{}, access.EventView{}, errNotFound("event")
	}
	return c, acc, view, nil
}

// loadEvent reads an event row the caller can see.
func (r *resolver) loadEvent(ctx context.Context, id string) (store.Event, store.Calendar, access.Access, access.EventView, error) {
	e, err := r.d.Store.GetEvent(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.Event{}, store.Calendar{}, access.Access{}, access.EventView{}, errNotFound("event")
	}
	if err != nil {
		return store.Event{}, store.Calendar{}, access.Access{}, access.EventView{}, internal(err, "load event")
	}
	c, acc, view, err := r.eventAccess(ctx, e)
	return e, c, acc, view, err
}
