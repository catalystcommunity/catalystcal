package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/ical"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// EventService implements csil.EventService.
type EventService struct{ *Deps }

var _ csil.EventService = (*EventService)(nil)

func (s *EventService) begin(ctx context.Context) (*auth.Identity, *resolver, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return nil, nil, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return id, r, nil
}

// requireWritable refuses event writes the caller may not make. The
// caller already passed the 404 check.
func requireWritable(c store.Calendar, acc access.Access) error {
	if !acc.CanWrite() {
		return csilrpc.Forbidden("you cannot change events on this calendar")
	}
	if c.Kind == "subscription" {
		return csilrpc.Forbidden("subscription calendars are read-only")
	}
	return nil
}

func parseRange(start, end csil.Timestamp) (time.Time, time.Time, error) {
	st, err := parseTS("range_start", start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	en, err := parseTS("range_end", end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !en.After(st) {
		return time.Time{}, time.Time{}, badReq("range_end must be after range_start")
	}
	if en.Sub(st) > MaxRangeDays*24*time.Hour {
		return time.Time{}, time.Time{}, badReq(fmt.Sprintf("the range can be at most %d days", MaxRangeDays))
	}
	return st, en, nil
}

// ---- reads ---------------------------------------------------------------

// ListEvents expands every visible calendar (or the requested ones) and,
// optionally, the caller's invitations in a time range.
func (s *EventService) ListEvents(ctx context.Context, req csil.ListEventsRequest) (csil.OccurrenceList, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.OccurrenceList{}, err
	}
	start, end, err := parseRange(req.RangeStart, req.RangeEnd)
	if err != nil {
		return csil.OccurrenceList{}, err
	}
	var cals []store.Calendar
	var accs []access.Access
	if len(req.CalendarIds) > 0 {
		for _, cid := range req.CalendarIds {
			c, a, err := r.loadCalendar(ctx, string(cid))
			if err != nil {
				return csil.OccurrenceList{}, err
			}
			cals, accs = append(cals, c), append(accs, a)
		}
	} else if cals, accs, err = r.visibleCalendars(ctx); err != nil {
		return csil.OccurrenceList{}, err
	}
	includeInv := len(req.CalendarIds) == 0
	if req.IncludeInvitations != nil {
		includeInv = *req.IncludeInvitations
	}

	accByCal := map[string]access.Access{}
	var ids []string
	for i, c := range cals {
		accByCal[c.ID] = accs[i]
		ids = append(ids, c.ID)
	}
	rows, err := s.Store.ListEventsInRange(ctx, ids, start, end)
	if err != nil {
		return csil.OccurrenceList{}, internal(err, "list events")
	}
	if includeInv {
		inv, err := s.Store.InvitationRows(ctx, r.effDomain, r.effSubject, id.EffectiveUserID())
		if err != nil {
			return csil.OccurrenceList{}, internal(err, "list invitations")
		}
		for _, e := range inv {
			if _, already := accByCal[string(e.Ev.CalendarId)]; already {
				continue
			}
			rows = append(rows, e)
		}
	}
	masters, overrides := groupRows(rows)
	byID := map[string]store.Event{}
	for _, e := range rows {
		byID[e.ID()] = e
	}
	pp := s.people()
	out := csil.OccurrenceList{Occurrences: []csil.Occurrence{}}
	remaining := MaxOccurrences
	calCache := map[string]access.Access{}
	for k, v := range accByCal {
		calCache[k] = v
	}
	for _, m := range masters {
		if remaining <= 0 {
			out.Truncated = true
			break
		}
		insts, trunc, err := expandRow(m, overrides[m.ID()], start, end, remaining)
		if err != nil {
			return csil.OccurrenceList{}, internal(err, "expand series "+m.ID())
		}
		if trunc {
			out.Truncated = true
		}
		for _, in := range insts {
			row, ok := byID[string(in.Event.EventId)]
			if !ok {
				row = m
			}
			acc, ok := calCache[string(row.Ev.CalendarId)]
			if !ok {
				c, err := s.Store.GetCalendar(ctx, string(row.Ev.CalendarId))
				if err != nil {
					return csil.OccurrenceList{}, internal(err, "load calendar")
				}
				if acc, err = r.calendar(ctx, c); err != nil {
					return csil.OccurrenceList{}, err
				}
				calCache[c.ID] = acc
			}
			view := access.ViewEvent(id, acc, access.IsPrivate(row.Ev.Visibility), r.isAttendee(row.Ev))
			if !view.CanSee() {
				continue
			}
			ev, err := pp.viewOut(ctx, row, view)
			if err != nil {
				return csil.OccurrenceList{}, err
			}
			// ical gives a series instance the instance's own times.
			ev.Start, ev.End, ev.StartDate, ev.EndDate = in.Event.Start, in.Event.End, in.Event.StartDate, in.Event.EndDate
			occ := csil.Occurrence{
				Event: ev, Start: ts(in.Start), End: ts(in.End),
				Access: view.Role.CSIL(), BusyOnly: view.BusyOnly, Invited: view.Invited,
			}
			if in.RecurrenceID != nil {
				occ.RecurrenceId = ptr(ts(*in.RecurrenceID))
			}
			out.Occurrences = append(out.Occurrences, occ)
			remaining--
		}
	}
	sort.SliceStable(out.Occurrences, func(i, j int) bool {
		return out.Occurrences[i].Start < out.Occurrences[j].Start
	})
	return out, nil
}

// GetEvent returns one event the caller can see.
func (s *EventService) GetEvent(ctx context.Context, req csil.EventID) (csil.Event, error) {
	_, r, err := s.begin(ctx)
	if err != nil {
		return csil.Event{}, err
	}
	e, _, _, view, err := r.loadEvent(ctx, string(req))
	if err != nil {
		return csil.Event{}, err
	}
	return s.people().viewOut(ctx, e, view)
}

// ---- create --------------------------------------------------------------

// CreateEvent makes an event. The organizer, the creator, and the updater
// are the effective user: the principal for a delegated bearer, the
// grantee for a share.
func (s *EventService) CreateEvent(ctx context.Context, req csil.Event) (csil.Event, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.Event{}, err
	}
	ev := req
	clearReceiveOnly(&ev)
	if ev.CalendarId == "" {
		return csil.Event{}, badReq("calendar_id is required")
	}
	c, acc, err := r.loadCalendar(ctx, string(ev.CalendarId))
	if err != nil {
		return csil.Event{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	if err := requireWritable(c, acc); err != nil {
		return csil.Event{}, err
	}
	if err := validateContent(&ev); err != nil {
		return csil.Event{}, err
	}
	if ev.Uid == nil || strings.TrimSpace(*ev.Uid) == "" {
		ev.Uid = ptr(store.NewID() + "@catalystcal")
	}
	eff := id.EffectiveUserID()
	var by []string
	ev.Attendees, by = mergeAnswers(ev.Attendees, nil, nil, r.effDomain, r.effSubject)
	ev.UpdatedVia = viaOf(id)
	row := store.Event{Ev: ev, OrganizerID: eff, CreatedByID: eff, UpdatedByID: eff, RespondedBy: by, SeriesUntil: seriesUntil(ev)}
	if err := s.Store.CreateEvent(ctx, &row); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return csil.Event{}, csilrpc.Conflict("an event with this uid is already on the calendar")
		}
		return csil.Event{}, internal(err, "create event")
	}
	csilrpc.SetAuditTarget(ctx, "event:"+row.ID())
	return s.reload(ctx, row.ID())
}

func (s *EventService) reload(ctx context.Context, id string) (csil.Event, error) {
	e, err := s.Store.GetEvent(ctx, id)
	if err != nil {
		return csil.Event{}, internal(err, "reload event")
	}
	return s.people().eventOut(ctx, e)
}

// ---- update --------------------------------------------------------------

func scopeOf(p *csil.RecurrenceScope) string {
	if p == nil || *p == "" {
		return "all"
	}
	return string(*p)
}

// nextContent builds a row from the client's new content and the stored
// row: identity, organizer, creator, and attendee answers stay.
func nextContent(id *auth.Identity, r *resolver, prev store.Event, content csil.Event) store.Event {
	ev := content
	ev.EventId = prev.Ev.EventId
	ev.CalendarId = prev.Ev.CalendarId
	ev.Uid = prev.Ev.Uid
	ev.MasterEventId = prev.Ev.MasterEventId
	ev.RecurrenceId = prev.Ev.RecurrenceId
	ev.CreatedAt = prev.Ev.CreatedAt
	ev.Sequence = prev.Ev.Sequence + 1
	ev.UpdatedVia = viaOf(id)
	var by []string
	ev.Attendees, by = mergeAnswers(ev.Attendees, prev.Ev.Attendees, prev.RespondedBy, r.effDomain, r.effSubject)
	return store.Event{
		Ev: ev, OrganizerID: prev.OrganizerID, CreatedByID: prev.CreatedByID, UpdatedByID: id.EffectiveUserID(),
		RespondedBy: by, SeriesUntil: seriesUntil(ev),
	}
}

func splitTimes(in []csil.Timestamp, cut time.Time) (before, after []csil.Timestamp) {
	c := ts(cut)
	for _, v := range in {
		if v < c {
			before = append(before, v)
		} else {
			after = append(after, v)
		}
	}
	return before, after
}

// UpdateEvent changes an event: the whole series (all), one instance
// (this: an override row), or the instance and every later one
// (this_and_future: the master ends before it and a new series starts).
func (s *EventService) UpdateEvent(ctx context.Context, req csil.UpdateEventRequest) (csil.Event, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.Event{}, err
	}
	target := string(req.Event.EventId)
	if target == "" {
		return csil.Event{}, badReq("event.event_id is required")
	}
	prev, c, acc, _, err := r.loadEvent(ctx, target)
	if err != nil {
		return csil.Event{}, err
	}
	csilrpc.SetAuditTarget(ctx, "event:"+prev.ID())
	if err := requireWritable(c, acc); err != nil {
		return csil.Event{}, err
	}
	content := req.Event
	clearReceiveOnly(&content)
	if err := validateContent(&content); err != nil {
		return csil.Event{}, err
	}
	scope := scopeOf(req.Scope)
	if prev.MasterID() != "" {
		scope = "override" // an override row is one instance already
	}
	if prev.Ev.Recurrence == nil && scope != "override" {
		scope = "all"
	}
	switch scope {
	case "all", "override":
		if scope == "override" {
			content.Recurrence = nil
		}
		row := nextContent(id, r, prev, content)
		if err := s.Store.UpdateEvent(ctx, &row); err != nil {
			return csil.Event{}, internal(err, "update event")
		}
		return s.reload(ctx, row.ID())
	case "this":
		if req.RecurrenceId == nil {
			return csil.Event{}, badReq("recurrence_id is required for scope this")
		}
		rid, err := parseTS("recurrence_id", *req.RecurrenceId)
		if err != nil {
			return csil.Event{}, err
		}
		content.Recurrence = nil
		ov, err := s.Store.FindOverride(ctx, prev.ID(), rid)
		switch {
		case err == nil:
			row := nextContent(id, r, ov, content)
			if err := s.Store.UpdateEvent(ctx, &row); err != nil {
				return csil.Event{}, internal(err, "update override")
			}
			return s.reload(ctx, row.ID())
		case errors.Is(err, store.ErrNotFound):
			base := prev
			base.Ev.EventId = ""
			base.Ev.MasterEventId = ptr(prev.Ev.EventId)
			base.Ev.RecurrenceId = ptr(ts(rid))
			row := nextContent(id, r, base, content)
			row.Ev.EventId = ""
			row.SeriesUntil = nil
			if err := s.Store.CreateEvent(ctx, &row); err != nil {
				if errors.Is(err, store.ErrConflict) {
					return csil.Event{}, csilrpc.Conflict("that instance changed at the same time; try again")
				}
				return csil.Event{}, internal(err, "create override")
			}
			return s.reload(ctx, row.ID())
		default:
			return csil.Event{}, internal(err, "find override")
		}
	case "this_and_future":
		if req.RecurrenceId == nil {
			return csil.Event{}, badReq("recurrence_id is required for scope this_and_future")
		}
		rid, err := parseTS("recurrence_id", *req.RecurrenceId)
		if err != nil {
			return csil.Event{}, err
		}
		mstart, _, err := store.Bounds(prev.Ev)
		if err != nil {
			return csil.Event{}, internal(err, "master bounds")
		}
		if !rid.After(mstart) {
			row := nextContent(id, r, prev, content)
			if err := s.Store.UpdateEvent(ctx, &row); err != nil {
				return csil.Event{}, internal(err, "update event")
			}
			return s.reload(ctx, row.ID())
		}
		old, err := truncatedMaster(id, prev, rid)
		if err != nil {
			return csil.Event{}, err
		}
		// The new series: the client's content, from the instance onward.
		if content.Recurrence == nil {
			_, exAfter := splitTimes(prev.Ev.Recurrence.Exdates, rid)
			_, rdAfter := splitTimes(prev.Ev.Recurrence.Rdates, rid)
			content.Recurrence = &csil.Recurrence{Rrule: prev.Ev.Recurrence.Rrule, Exdates: exAfter, Rdates: rdAfter}
		}
		base := prev
		base.Ev.EventId = ""
		base.Ev.Uid = ptr(store.NewID() + "@catalystcal")
		base.Ev.Sequence = -1
		next := nextContent(id, r, base, content)
		next.Ev.EventId = ""
		if err := s.Store.ApplyEventWrites(ctx, []store.EventWrite{
			{Update: &old},
			{DeleteOverridesOf: prev.ID(), From: rid},
			{Insert: &next},
		}); err != nil {
			return csil.Event{}, internal(err, "split series")
		}
		csilrpc.SetAuditTarget(ctx, "event:"+next.ID())
		return s.reload(ctx, next.ID())
	}
	return csil.Event{}, badReq("unknown scope " + scope)
}

// truncatedMaster returns the master changed to end before cut.
func truncatedMaster(id *auth.Identity, prev store.Event, cut time.Time) (store.Event, error) {
	old := prev
	ev := old.Ev
	rec := *ev.Recurrence
	if rec.Rrule != "" {
		tz := ""
		if ev.Timezone != nil {
			tz = *ev.Timezone
		}
		mstart, _, err := store.Bounds(ev)
		if err != nil {
			return store.Event{}, internal(err, "master bounds")
		}
		rule, err := ical.TruncateRRule(rec.Rrule, mstart, tz, cut)
		if err != nil {
			return store.Event{}, internal(err, "truncate rrule")
		}
		rec.Rrule = rule
	}
	rec.Exdates, _ = splitTimes(rec.Exdates, cut)
	rec.Rdates, _ = splitTimes(rec.Rdates, cut)
	ev.Recurrence = &rec
	ev.Sequence++
	ev.UpdatedVia = viaOf(id)
	old.Ev = ev
	old.UpdatedByID = id.EffectiveUserID()
	old.SeriesUntil = seriesUntil(ev)
	return old, nil
}

// ---- delete --------------------------------------------------------------

// DeleteEvent removes an event, one instance (an EXDATE), or the instance
// and every later one.
func (s *EventService) DeleteEvent(ctx context.Context, req csil.DeleteEventRequest) (csil.EmptyResponse, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	row, c, acc, _, err := r.loadEvent(ctx, string(req.EventId))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	csilrpc.SetAuditTarget(ctx, "event:"+row.ID())
	if err := requireWritable(c, acc); err != nil {
		return csil.EmptyResponse{}, err
	}
	scope := scopeOf(req.Scope)
	master := row
	var rid *time.Time
	if req.RecurrenceId != nil {
		t, err := parseTS("recurrence_id", *req.RecurrenceId)
		if err != nil {
			return csil.EmptyResponse{}, err
		}
		rid = &t
	}
	if row.MasterID() != "" {
		// An override: "all" removes the series; otherwise this instance.
		m, err := s.Store.GetEvent(ctx, row.MasterID())
		if err != nil {
			return csil.EmptyResponse{}, internal(err, "load master")
		}
		master = m
		if rid == nil {
			t, _ := store.ParseTime(string(*row.Ev.RecurrenceId))
			rid = &t
		}
		if scope == "all" {
			return s.deleteRow(ctx, master.ID())
		}
	} else if row.Ev.Recurrence == nil || scope == "all" {
		return s.deleteRow(ctx, row.ID())
	}
	if rid == nil {
		return csil.EmptyResponse{}, badReq("recurrence_id is required for scope " + scope)
	}
	mstart, _, err := store.Bounds(master.Ev)
	if err != nil {
		return csil.EmptyResponse{}, internal(err, "master bounds")
	}
	switch scope {
	case "this":
		m := master
		ev := m.Ev
		rec := *ev.Recurrence
		rec.Exdates, _ = normalizeTimes("exdates", append(append([]csil.Timestamp{}, rec.Exdates...), ts(*rid)))
		ev.Recurrence = &rec
		ev.Sequence++
		ev.UpdatedVia = viaOf(id)
		m.Ev, m.UpdatedByID = ev, id.EffectiveUserID()
		writes := []store.EventWrite{{Update: &m}}
		if ov, err := s.Store.FindOverride(ctx, master.ID(), *rid); err == nil {
			writes = append(writes, store.EventWrite{DeleteID: ov.ID()})
		}
		if err := s.Store.ApplyEventWrites(ctx, writes); err != nil {
			return csil.EmptyResponse{}, internal(err, "delete instance")
		}
	case "this_and_future":
		if !rid.After(mstart) {
			return s.deleteRow(ctx, master.ID())
		}
		old, err := truncatedMaster(id, master, *rid)
		if err != nil {
			return csil.EmptyResponse{}, err
		}
		if err := s.Store.ApplyEventWrites(ctx, []store.EventWrite{
			{Update: &old}, {DeleteOverridesOf: master.ID(), From: *rid},
		}); err != nil {
			return csil.EmptyResponse{}, internal(err, "truncate series")
		}
	default:
		return csil.EmptyResponse{}, badReq("unknown scope " + scope)
	}
	return csil.EmptyResponse{}, nil
}

func (s *EventService) deleteRow(ctx context.Context, id string) (csil.EmptyResponse, error) {
	if err := s.Store.DeleteEvent(ctx, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		return csil.EmptyResponse{}, internal(err, "delete event")
	}
	return csil.EmptyResponse{}, nil
}

// ---- move ----------------------------------------------------------------

// MoveEvent moves an event (with its overrides) to another calendar. The
// caller needs write access on both.
func (s *EventService) MoveEvent(ctx context.Context, req csil.MoveEventRequest) (csil.Event, error) {
	_, r, err := s.begin(ctx)
	if err != nil {
		return csil.Event{}, err
	}
	row, c, acc, _, err := r.loadEvent(ctx, string(req.EventId))
	if err != nil {
		return csil.Event{}, err
	}
	csilrpc.SetAuditTarget(ctx, "event:"+row.ID())
	if err := requireWritable(c, acc); err != nil {
		return csil.Event{}, err
	}
	if row.MasterID() != "" {
		return csil.Event{}, badReq("move the series, not one instance")
	}
	tc, tacc, err := r.loadCalendar(ctx, string(req.CalendarId))
	if err != nil {
		return csil.Event{}, err
	}
	if err := requireWritable(tc, tacc); err != nil {
		return csil.Event{}, err
	}
	if tc.ID != c.ID {
		if err := s.Store.MoveEvent(ctx, row.ID(), tc.ID); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return csil.Event{}, csilrpc.Conflict("the target calendar already has an event with this uid")
			}
			return csil.Event{}, internal(err, "move event")
		}
	}
	return s.reload(ctx, row.ID())
}

// ---- invitations ----------------------------------------------------------

// Respond records the effective user's answer to an invitation. A
// delegate needs events.respond and is recorded as responded_by.
func (s *EventService) Respond(ctx context.Context, req csil.RespondRequest) (csil.Event, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.Event{}, err
	}
	if !validPartstat[string(req.Status)] {
		return csil.Event{}, badReq("unknown status " + string(req.Status))
	}
	row, _, _, _, err := r.loadEvent(ctx, string(req.EventId))
	if err != nil {
		return csil.Event{}, err
	}
	csilrpc.SetAuditTarget(ctx, "event:"+row.ID())
	if !r.isAttendee(row.Ev) {
		return csil.Event{}, csilrpc.Forbidden("you are not an attendee of this event")
	}
	if id.IsDelegated() && !id.HasScope(access.ScopeEventsRespond) {
		return csil.Event{}, csilrpc.Forbidden("this bearer cannot answer invitations (needs events.respond)")
	}
	respondedBy := ""
	if id.Via == auth.ViaDelegation {
		respondedBy = id.UserID
	}
	target := row
	if req.RecurrenceId != nil && row.MasterID() == "" && row.Ev.Recurrence != nil {
		rid, err := parseTS("recurrence_id", *req.RecurrenceId)
		if err != nil {
			return csil.Event{}, err
		}
		ov, err := s.Store.FindOverride(ctx, row.ID(), rid)
		switch {
		case err == nil:
			target = ov
		case errors.Is(err, store.ErrNotFound):
			content, err := instanceContent(row.Ev, rid)
			if err != nil {
				return csil.Event{}, internal(err, "instance content")
			}
			nr := row
			nr.Ev = content
			nr.Ev.EventId = ""
			nr.Ev.MasterEventId = ptr(row.Ev.EventId)
			nr.Ev.RecurrenceId = ptr(ts(rid))
			nr.SeriesUntil = nil
			if err := s.Store.CreateEvent(ctx, &nr); err != nil {
				return csil.Event{}, internal(err, "create override for answer")
			}
			target = nr
		default:
			return csil.Event{}, internal(err, "find override")
		}
	}
	if err := s.Store.SetAttendeeStatus(ctx, target.ID(), r.effDomain, r.effSubject, string(req.Status), respondedBy); err != nil {
		return csil.Event{}, internal(err, "record answer")
	}
	return s.reload(ctx, target.ID())
}

// ListInvitations returns events on other people's calendars that name the
// effective user as an attendee.
func (s *EventService) ListInvitations(ctx context.Context, _ csil.EmptyRequest) (csil.EventList, error) {
	id, r, err := s.begin(ctx)
	if err != nil {
		return csil.EventList{}, err
	}
	rows, err := s.Store.InvitationRows(ctx, r.effDomain, r.effSubject, id.EffectiveUserID())
	if err != nil {
		return csil.EventList{}, internal(err, "list invitations")
	}
	pp := s.people()
	out := csil.EventList{Events: []csil.Event{}}
	for _, e := range rows {
		if e.MasterID() != "" {
			continue
		}
		_, _, view, err := r.eventAccess(ctx, e)
		if err != nil {
			continue // not visible to this bearer
		}
		ev, err := pp.viewOut(ctx, e, view)
		if err != nil {
			return csil.EventList{}, err
		}
		out.Events = append(out.Events, ev)
	}
	return out, nil
}

// SearchEvents finds events by title, location, and description on
// calendars where the caller reads details, and in invitations. Events the
// caller would see as busy only never match.
func (s *EventService) SearchEvents(ctx context.Context, req csil.SearchEventsRequest) (csil.EventList, error) {
	_, r, err := s.begin(ctx)
	if err != nil {
		return csil.EventList{}, err
	}
	q := strings.TrimSpace(req.Query)
	if q == "" || len(q) > 256 {
		return csil.EventList{}, badReq("query must have 1 to 256 bytes")
	}
	limit := 50
	if req.Limit != nil && *req.Limit > 0 {
		limit = int(min(*req.Limit, 200))
	}
	cals, accs, err := r.visibleCalendars(ctx)
	if err != nil {
		return csil.EventList{}, err
	}
	var ids []string
	for i, c := range cals {
		if accs[i].CanReadDetails() {
			ids = append(ids, c.ID)
		}
	}
	rows, err := s.Store.SearchEvents(ctx, ids, r.effDomain, r.effSubject, q, limit*2)
	if err != nil {
		return csil.EventList{}, internal(err, "search events")
	}
	pp := s.people()
	out := csil.EventList{Events: []csil.Event{}}
	for _, e := range rows {
		if len(out.Events) >= limit {
			break
		}
		_, _, view, err := r.eventAccess(ctx, e)
		if err != nil || view.BusyOnly {
			continue
		}
		ev, err := pp.eventOut(ctx, e)
		if err != nil {
			return csil.EventList{}, err
		}
		out.Events = append(out.Events, ev)
	}
	return out, nil
}
