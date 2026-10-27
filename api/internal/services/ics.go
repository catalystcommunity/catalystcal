package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/catalystcommunity/catalystcal/api/internal/access"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/ical"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// IcsService implements csil.IcsService.
type IcsService struct{ *Deps }

var _ csil.IcsService = (*IcsService)(nil)

// withoutAttendee drops the attendee with a LinkKeys identity. ical.Parse
// lists the ORGANIZER as a chair attendee; the importing person is not an
// attendee of their own copy.
func withoutAttendee(atts []csil.Attendee, domain, subject string) []csil.Attendee {
	out := atts[:0:0]
	for _, a := range atts {
		if a.Identity != nil && a.Identity.UserId != nil &&
			strings.EqualFold(a.Identity.Domain, domain) && *a.Identity.UserId == subject {
			continue
		}
		out = append(out, a)
	}
	return out
}

// importedRow turns a parsed event into a row for calendarID. actorID is
// the creator ("" for a subscription sync).
func (d *Deps) importedRow(ev csil.Event, calendarID, actorID string) (store.Event, error) {
	rid := ev.RecurrenceId
	seq := ev.Sequence
	clearReceiveOnly(&ev)
	if strings.TrimSpace(ev.Title) == "" {
		ev.Title = "(no title)"
	}
	if len(ev.Title) > 1024 {
		ev.Title = ev.Title[:1024]
	}
	for i := range ev.Attendees {
		a := &ev.Attendees[i]
		if a.Identity != nil && (a.Identity.UserId == nil || *a.Identity.UserId == "") {
			a.Identity = nil
		}
	}
	if err := validateContent(&ev); err != nil {
		return store.Event{}, err
	}
	if rid != nil {
		t, err := store.ParseTime(string(*rid))
		if err != nil {
			return store.Event{}, err
		}
		ev.RecurrenceId = ptr(ts(t))
		ev.Recurrence = nil
	}
	if ev.Uid == nil || strings.TrimSpace(*ev.Uid) == "" {
		ev.Uid = ptr(store.NewID() + "@catalystcal")
	}
	ev.CalendarId = csil.CalendarID(calendarID)
	ev.Sequence = seq
	ev.UpdatedVia = ptr(csil.ActingVia("self"))
	return store.Event{Ev: ev, CreatedByID: actorID, UpdatedByID: actorID, SeriesUntil: seriesUntil(ev)}, nil
}

// ImportIcs adds the events of an iCalendar object to a writable local
// calendar. An event whose UID is already on the calendar is updated.
func (s *IcsService) ImportIcs(ctx context.Context, req csil.ImportIcsRequest) (csil.ImportResult, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.ImportResult{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.ImportResult{}, err
	}
	c, acc, err := r.loadCalendar(ctx, string(req.CalendarId))
	if err != nil {
		return csil.ImportResult{}, err
	}
	csilrpc.SetAuditTarget(ctx, "calendar:"+c.ID)
	if err := requireWritable(c, acc); err != nil {
		return csil.ImportResult{}, err
	}
	if strings.TrimSpace(req.Ics) == "" {
		return csil.ImportResult{}, badReq("ics is empty")
	}
	parsed, err := ical.Parse([]byte(req.Ics))
	if err != nil {
		return csil.ImportResult{}, badReq("ics does not parse: " + err.Error())
	}
	res := csil.ImportResult{Warnings: append([]string{}, parsed.Warnings...)}
	eff := id.EffectiveUserID()
	var writes []store.EventWrite
	masterIDs := map[string]string{} // uid -> event id
	// Masters and single events first, so overrides find them.
	for _, pe := range parsed.Events {
		if pe.RecurrenceId != nil {
			continue
		}
		pe.Attendees = withoutAttendee(pe.Attendees, r.effDomain, r.effSubject)
		row, err := s.importedRow(pe, c.ID, eff)
		if err != nil {
			res.Skipped++
			res.Warnings = append(res.Warnings, fmt.Sprintf("skipped %q: %v", pe.Title, err))
			continue
		}
		uid := *row.Ev.Uid
		if _, dup := masterIDs[uid]; dup {
			res.Skipped++
			continue
		}
		prev, err := s.Store.FindMasterByUID(ctx, c.ID, uid)
		switch {
		case err == nil:
			next := nextContent(id, r, prev, row.Ev)
			writes = append(writes, store.EventWrite{Update: &next})
			masterIDs[uid] = prev.ID()
			res.Updated++
		case errors.Is(err, store.ErrNotFound):
			row.Ev.EventId = csil.EventID(store.NewID())
			row.OrganizerID = ""
			writes = append(writes, store.EventWrite{Insert: &row})
			masterIDs[uid] = row.ID()
			res.Created++
		default:
			return csil.ImportResult{}, internal(err, "find event by uid")
		}
	}
	for _, pe := range parsed.Events {
		if pe.RecurrenceId == nil {
			continue
		}
		pe.Attendees = withoutAttendee(pe.Attendees, r.effDomain, r.effSubject)
		row, err := s.importedRow(pe, c.ID, eff)
		if err != nil {
			res.Skipped++
			continue
		}
		mid, ok := masterIDs[*row.Ev.Uid]
		if !ok {
			if m, err := s.Store.FindMasterByUID(ctx, c.ID, *row.Ev.Uid); err == nil {
				mid = m.ID()
			} else {
				res.Skipped++
				res.Warnings = append(res.Warnings, "skipped an override whose series is not on the calendar: "+*row.Ev.Uid)
				continue
			}
		}
		row.Ev.MasterEventId = ptr(csil.EventID(mid))
		rid, _ := store.ParseTime(string(*row.Ev.RecurrenceId))
		if prev, err := s.Store.FindOverride(ctx, mid, rid); err == nil {
			next := nextContent(id, r, prev, row.Ev)
			writes = append(writes, store.EventWrite{Update: &next})
			res.Updated++
		} else {
			row.Ev.EventId = csil.EventID(store.NewID())
			writes = append(writes, store.EventWrite{Insert: &row})
			res.Created++
		}
	}
	if err := s.Store.ApplyEventWrites(ctx, writes); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return csil.ImportResult{}, csilrpc.Conflict("the import collides with an existing event; try again")
		}
		return csil.ImportResult{}, internal(err, "import events")
	}
	if len(res.Warnings) == 0 {
		res.Warnings = nil
	}
	return res, nil
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func filename(base string) string {
	n := strings.Trim(unsafeName.ReplaceAllString(base, "-"), "-.")
	if n == "" {
		n = "calendar"
	}
	if len(n) > 80 {
		n = n[:80]
	}
	return n + ".ics"
}

// calendarDocument encodes the rows of a calendar. Rows that the view
// shows as busy only are stripped first.
func (d *Deps) calendarDocument(ctx context.Context, rows []store.Event, name string, view func(store.Event) access.EventView) ([]byte, error) {
	pp := d.people()
	evs := make([]csil.Event, 0, len(rows))
	for _, e := range rows {
		v := view(e)
		if !v.CanSee() {
			continue
		}
		ev, err := pp.viewOut(ctx, e, v)
		if err != nil {
			return nil, err
		}
		evs = append(evs, ev)
	}
	return ical.Encode(evs, ical.EncodeOptions{CalendarName: name, Method: "PUBLISH", Now: d.now()})
}

// ExportCalendar returns a calendar as iCalendar text.
func (s *IcsService) ExportCalendar(ctx context.Context, req csil.ExportCalendarRequest) (csil.IcsDocument, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.IcsDocument{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.IcsDocument{}, err
	}
	c, acc, err := r.loadCalendar(ctx, string(req.CalendarId))
	if err != nil {
		return csil.IcsDocument{}, err
	}
	rows, err := s.Store.ListCalendarEvents(ctx, c.ID)
	if err != nil {
		return csil.IcsDocument{}, internal(err, "list events")
	}
	doc, err := s.calendarDocument(ctx, rows, c.Name, func(e store.Event) access.EventView {
		return access.ViewEvent(id, acc, access.IsPrivate(e.Ev.Visibility), r.isAttendee(e.Ev))
	})
	if err != nil {
		return csil.IcsDocument{}, internal(err, "encode calendar")
	}
	return csil.IcsDocument{Ics: string(doc), Filename: filename(c.Name)}, nil
}

// ExportEvent returns one event (a series with its overrides) as
// iCalendar text. method "request" makes an iTIP REQUEST.
func (s *IcsService) ExportEvent(ctx context.Context, req csil.ExportEventRequest) (csil.IcsDocument, error) {
	id, err := requireIdentity(ctx)
	if err != nil {
		return csil.IcsDocument{}, err
	}
	r, err := s.resolver(ctx, id)
	if err != nil {
		return csil.IcsDocument{}, err
	}
	e, _, _, view, err := r.loadEvent(ctx, string(req.EventId))
	if err != nil {
		return csil.IcsDocument{}, err
	}
	method := "PUBLISH"
	switch strings.ToLower(deref(req.Method)) {
	case "", "publish":
	case "request":
		method = "REQUEST"
		if view.BusyOnly || view.Invited {
			return csil.IcsDocument{}, csilrpc.Forbidden("only someone who can see the details can send an invitation")
		}
	default:
		return csil.IcsDocument{}, badReq("method must be publish or request")
	}
	rows := []store.Event{e}
	if e.MasterID() == "" && e.Ev.Recurrence != nil {
		ovs, err := s.Store.ListOverrides(ctx, e.ID())
		if err != nil {
			return csil.IcsDocument{}, internal(err, "list overrides")
		}
		rows = append(rows, ovs...)
	}
	pp := s.people()
	evs := make([]csil.Event, 0, len(rows))
	for _, row := range rows {
		ev, err := pp.viewOut(ctx, row, view)
		if err != nil {
			return csil.IcsDocument{}, err
		}
		evs = append(evs, ev)
	}
	doc, err := ical.Encode(evs, ical.EncodeOptions{Method: method, Now: s.now()})
	if err != nil {
		return csil.IcsDocument{}, internal(err, "encode event")
	}
	return csil.IcsDocument{Ics: string(doc), Filename: filename(deref(e.Ev.Uid))}, nil
}

// ErrFeedNotFound is returned by Feed for an unknown token.
var ErrFeedNotFound = errors.New("feed not found")

// Feed renders the public feed of a calendar by its secret token.
// Private and confidential events are busy only.
func (d *Deps) Feed(ctx context.Context, token string) ([]byte, error) {
	token = strings.TrimSuffix(token, ".ics")
	c, err := d.Store.GetCalendarByFeedToken(ctx, token)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrFeedNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := d.Store.ListCalendarEvents(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	return d.calendarDocument(ctx, rows, c.Name, func(e store.Event) access.EventView {
		return access.EventView{Role: access.Read, BusyOnly: access.IsPrivate(e.Ev.Visibility)}
	})
}
