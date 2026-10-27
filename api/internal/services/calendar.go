package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/ical"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	log "github.com/sirupsen/logrus"
)

// CalendarService implements csil.CalendarService.
type CalendarService struct{ *Deps }

var _ csil.CalendarService = (*CalendarService)(nil)

// FeedPath is the public path of a calendar feed.
func FeedPath(token string) string { return "/ics/" + token + ".ics" }

func (d *Deps) calendarOut(ctx context.Context, c store.Calendar, acc access.Access, owners map[string]*csil.PersonSummary) (csil.Calendar, error) {
	owner, ok := owners[c.OwnerID]
	if !ok {
		var err error
		owner, err = d.personByID(ctx, c.OwnerID)
		if err != nil {
			return csil.Calendar{}, internal(err, "load calendar owner")
		}
		owners[c.OwnerID] = owner
	}
	out := csil.Calendar{
		CalendarId:   csil.CalendarID(c.ID),
		Name:         c.Name,
		Description:  optStr(c.Description),
		Color:        optStr(c.Color),
		Timezone:     optStr(c.Timezone),
		Kind:         csil.CalendarKind(c.Kind),
		SourceUrl:    optStr(c.SourceURL),
		LastSyncedAt: tsPtr(c.LastSyncedAt),
		SyncError:    optStr(c.SyncError),
		IsDefault:    c.IsDefault,
		Access:       acc.Role.CSIL(),
		AccessPath:   acc.Path,
		CreatedAt:    ts(c.CreatedAt),
		UpdatedAt:    ts(c.UpdatedAt),
	}
	if owner != nil {
		out.Owner = *owner
	}
	if acc.Manage && c.FeedToken != "" {
		out.FeedPath = ptr(FeedPath(c.FeedToken))
	}
	if !acc.Manage {
		// The source URL of a subscription can hold a secret token.
		out.SourceUrl = nil
	}
	return out, nil
}

func (s *CalendarService) calendarOf(ctx context.Context, r *resolver, c store.Calendar) (csil.Calendar, error) {
	acc, err := r.calendar(ctx, c)
	if err != nil {
		return csil.Calendar{}, err
	}
	return s.calendarOut(ctx, c, acc, map[string]*csil.PersonSummary{})
}

// ListCalendars returns every calendar the caller can see.
func (s *CalendarService) ListCalendars(ctx context.Context, _ csil.EmptyRequest) (csil.CalendarList, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.CalendarList{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.CalendarList{}, err
	}
	cals, accs, err := r.visibleCalendars(ctx)
	if err != nil {
		return csil.CalendarList{}, err
	}
	owners := map[string]*csil.PersonSummary{}
	out := csil.CalendarList{Calendars: []csil.Calendar{}}
	for i, c := range cals {
		cc, err := s.calendarOut(ctx, c, accs[i], owners)
		if err != nil {
			return csil.CalendarList{}, err
		}
		out.Calendars = append(out.Calendars, cc)
	}
	return out, nil
}

// GetCalendar returns one visible calendar.
func (s *CalendarService) GetCalendar(ctx context.Context, req csil.CalendarID) (csil.Calendar, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.Calendar{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.Calendar{}, err
	}
	c, acc, err := r.loadCalendar(ctx, string(req))
	if err != nil {
		return csil.Calendar{}, err
	}
	return s.calendarOut(ctx, c, acc, map[string]*csil.PersonSummary{})
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 256 {
		return "", csilrpc.BadRequest("name must have 1 to 256 bytes")
	}
	return name, nil
}

// requireCalendarCreate allows a self bearer, or a delegated bearer with
// calendars.manage, to make calendars for the effective user.
func requireCalendarCreate(ctx context.Context) (string, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return "", err
	}
	if id.IsDelegated() && !id.HasScope(access.ScopeCalendarsManage) {
		return "", csilrpc.Forbidden("this bearer cannot make calendars (needs calendars.manage)")
	}
	return id.EffectiveUserID(), nil
}

// CreateCalendar makes a local calendar for the effective user.
func (s *CalendarService) CreateCalendar(ctx context.Context, req csil.CreateCalendarRequest) (csil.Calendar, error) {
	owner, err := requireCalendarCreate(ctx)
	if err != nil {
		return csil.Calendar{}, err
	}
	name, err := validName(req.Name)
	if err != nil {
		return csil.Calendar{}, err
	}
	if err := checkZone(deref(req.Timezone)); err != nil {
		return csil.Calendar{}, err
	}
	c, err := s.Store.CreateCalendar(ctx, store.Calendar{
		OwnerID: owner, Name: name, Description: deref(req.Description), Color: deref(req.Color),
		Timezone: deref(req.Timezone), Kind: "local",
	})
	if err != nil {
		return csil.Calendar{}, internal(err, "create calendar")
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	id, _ := requireIdentity(ctx)
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.Calendar{}, err
	}
	return s.calendarOf(ctx, r, c)
}

// manageable loads a calendar the caller can manage. Seen but not
// manageable is 403; unseen is 404.
func (s *CalendarService) manageable(ctx context.Context, calendarID string) (*resolver, store.Calendar, access.Access, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, store.Calendar{}, access.Access{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return nil, store.Calendar{}, access.Access{}, err
	}
	c, acc, err := r.loadCalendar(ctx, calendarID)
	if err != nil {
		return nil, store.Calendar{}, access.Access{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	if !acc.Manage {
		return nil, store.Calendar{}, access.Access{}, csilrpc.Forbidden("you cannot change the settings of this calendar")
	}
	return r, c, acc, nil
}

// UpdateCalendar changes calendar settings (manage access).
func (s *CalendarService) UpdateCalendar(ctx context.Context, req csil.UpdateCalendarRequest) (csil.Calendar, error) {
	r, c, _, err := s.manageable(ctx, string(req.CalendarId))
	if err != nil {
		return csil.Calendar{}, err
	}
	patch := store.CalendarPatch{Description: req.Description, Color: req.Color, Timezone: req.Timezone}
	if req.Name != nil {
		name, err := validName(*req.Name)
		if err != nil {
			return csil.Calendar{}, err
		}
		patch.Name = &name
	}
	if req.Timezone != nil {
		if err := checkZone(*req.Timezone); err != nil {
			return csil.Calendar{}, err
		}
	}
	c, err = s.Store.UpdateCalendar(ctx, c.ID, patch)
	if err != nil {
		return csil.Calendar{}, internal(err, "update calendar")
	}
	return s.calendarOf(ctx, r, c)
}

// DeleteCalendar removes a calendar. Only the owner with a self bearer can,
// and never the default calendar.
func (s *CalendarService) DeleteCalendar(ctx context.Context, req csil.CalendarID) (csil.EmptyResponse, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	c, acc, err := r.loadCalendar(ctx, string(req))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	if !acc.DeleteCalendar {
		return csil.EmptyResponse{}, csilrpc.Forbidden("only the owner can delete a calendar")
	}
	if c.IsDefault {
		return csil.EmptyResponse{}, csilrpc.Forbidden("you cannot delete your default calendar")
	}
	if err := s.Store.DeleteCalendar(ctx, c.ID); err != nil {
		return csil.EmptyResponse{}, internal(err, "delete calendar")
	}
	return csil.EmptyResponse{}, nil
}

// SubscribeCalendar makes a read-only calendar that mirrors a remote
// iCalendar feed and imports it at once.
func (s *CalendarService) SubscribeCalendar(ctx context.Context, req csil.SubscribeCalendarRequest) (csil.Calendar, error) {
	owner, err := requireCalendarCreate(ctx)
	if err != nil {
		return csil.Calendar{}, err
	}
	name, err := validName(req.Name)
	if err != nil {
		return csil.Calendar{}, err
	}
	src, err := s.Fetcher.NormalizeURL(req.SourceUrl)
	if err != nil {
		return csil.Calendar{}, csilrpc.BadRequest(err.Error())
	}
	c, err := s.Store.CreateCalendar(ctx, store.Calendar{
		OwnerID: owner, Name: name, Color: deref(req.Color), Kind: "subscription", SourceURL: src,
	})
	if err != nil {
		return csil.Calendar{}, internal(err, "create subscription calendar")
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	_ = s.SyncSubscription(ctx, c) // a failure is kept in sync_error
	if c, err = s.Store.GetCalendar(ctx, c.ID); err != nil {
		return csil.Calendar{}, internal(err, "reload calendar")
	}
	id, _ := requireIdentity(ctx)
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.Calendar{}, err
	}
	return s.calendarOf(ctx, r, c)
}

// RefreshSubscription syncs a subscription calendar now.
func (s *CalendarService) RefreshSubscription(ctx context.Context, req csil.CalendarID) (csil.Calendar, error) {
	r, c, _, err := s.manageable(ctx, string(req))
	if err != nil {
		return csil.Calendar{}, err
	}
	if c.Kind != "subscription" {
		return csil.Calendar{}, csilrpc.BadRequest("this calendar is not a subscription")
	}
	_ = s.SyncSubscription(ctx, c)
	if c, err = s.Store.GetCalendar(ctx, c.ID); err != nil {
		return csil.Calendar{}, internal(err, "reload calendar")
	}
	return s.calendarOf(ctx, r, c)
}

// SetFeed turns the secret public feed on (with a new secret) or off.
func (s *CalendarService) SetFeed(ctx context.Context, req csil.SetFeedRequest) (csil.Calendar, error) {
	r, c, _, err := s.manageable(ctx, string(req.CalendarId))
	if err != nil {
		return csil.Calendar{}, err
	}
	token := ""
	if req.Enabled {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return csil.Calendar{}, internal(err, "feed token")
		}
		token = base64.RawURLEncoding.EncodeToString(b)
	}
	c, err = s.Store.SetFeedToken(ctx, c.ID, token)
	if err != nil {
		return csil.Calendar{}, internal(err, "set feed token")
	}
	return s.calendarOf(ctx, r, c)
}

// SyncSubscription fetches a subscription feed and replaces every event
// of the calendar with it. A failure is stored in sync_error and returned.
func (d *Deps) SyncSubscription(ctx context.Context, c store.Calendar) error {
	fail := func(err error) error {
		msg := err.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		if serr := d.Store.SetSyncError(ctx, c.ID, msg); serr != nil {
			log.WithError(serr).Error("subscription: record sync error")
		}
		return err
	}
	if d.Fetcher == nil {
		return fail(errors.New("subscriptions are not configured"))
	}
	src, err := d.Fetcher.NormalizeURL(c.SourceURL)
	if err != nil {
		return fail(err)
	}
	body, err := d.Fetcher.Fetch(ctx, src)
	if err != nil {
		return fail(fmt.Errorf("fetch: %w", err))
	}
	parsed, err := ical.Parse(body)
	if err != nil {
		return fail(fmt.Errorf("parse: %w", err))
	}
	owner, _ := d.Store.GetUser(ctx, c.OwnerID)
	rows := make([]store.Event, 0, len(parsed.Events))
	for _, ev := range parsed.Events {
		ev.Attendees = withoutAttendee(ev.Attendees, owner.Domain, owner.Subject)
		row, err := d.importedRow(ev, c.ID, "")
		if err != nil {
			continue
		}
		rows = append(rows, row)
	}
	if _, err := d.Store.ReplaceCalendarEvents(ctx, c.ID, rows); err != nil {
		return fail(fmt.Errorf("store: %w", err))
	}
	return nil
}

// SyncDueSubscriptions refreshes every subscription last synced more than
// interval ago. The background worker calls it.
func (d *Deps) SyncDueSubscriptions(ctx context.Context, interval time.Duration) {
	cals, err := d.Store.ListSubscriptionsDue(ctx, d.now().Add(-interval))
	if err != nil {
		log.WithError(err).Error("subscription worker: list due")
		return
	}
	for _, c := range cals {
		if ctx.Err() != nil {
			return
		}
		if err := d.SyncSubscription(ctx, c); err != nil {
			log.WithError(err).WithField("calendar_id", c.ID).Warn("subscription sync failed")
		}
	}
}
