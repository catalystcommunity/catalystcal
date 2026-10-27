package services

import (
	"context"
	"errors"
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

var (
	validStatus       = map[string]bool{"confirmed": true, "tentative": true, "cancelled": true}
	validTransparency = map[string]bool{"opaque": true, "transparent": true}
	validVisibility   = map[string]bool{"default": true, "public": true, "private": true, "confidential": true}
	validPartstat     = map[string]bool{"needs_action": true, "accepted": true, "declined": true, "tentative": true, "delegated": true}
	validAttendeeRole = map[string]bool{"chair": true, "required": true, "optional": true, "non_participant": true}
	validReminder     = map[string]bool{"display": true, "email": true}
)

// clearReceiveOnly drops every receive-only field a client sent.
func clearReceiveOnly(ev *csil.Event) {
	ev.EventId = ""
	ev.MasterEventId = nil
	ev.RecurrenceId = nil
	ev.Organizer, ev.CreatedBy, ev.UpdatedBy, ev.UpdatedVia = nil, nil, nil, nil
	ev.Sequence = 0
	ev.CreatedAt, ev.UpdatedAt = "", ""
	for i := range ev.Attendees {
		ev.Attendees[i].Person = nil
		ev.Attendees[i].RespondedBy = nil
	}
}

func badReq(msg string) error { return csilrpc.BadRequest(msg) }

// validateContent checks and normalizes the client-writable fields of an
// event: title, times, all-day dates, time zone, enums, recurrence,
// attendees, and reminders.
func validateContent(ev *csil.Event) error {
	ev.Title = strings.TrimSpace(ev.Title)
	if len(ev.Title) < 1 || len(ev.Title) > 1024 {
		return badReq("title must have 1 to 1024 bytes")
	}
	if ev.Timezone != nil && *ev.Timezone == "" {
		ev.Timezone = nil
	}
	if ev.Timezone != nil {
		if err := checkZone(*ev.Timezone); err != nil {
			return err
		}
	}
	if ev.AllDay != nil && *ev.AllDay {
		if ev.StartDate == nil || ev.EndDate == nil {
			return badReq("an all-day event needs start_date and end_date")
		}
		sd, err := time.Parse(store.DateLayout, string(*ev.StartDate))
		if err != nil {
			return badReq("start_date must be YYYY-MM-DD")
		}
		ed, err := time.Parse(store.DateLayout, string(*ev.EndDate))
		if err != nil {
			return badReq("end_date must be YYYY-MM-DD")
		}
		if !ed.After(sd) {
			return badReq("end_date must be after start_date (it is exclusive)")
		}
		ev.Start, ev.End = ptr(ts(sd)), ptr(ts(ed))
	} else {
		ev.AllDay = ptr(false)
		ev.StartDate, ev.EndDate = nil, nil
		if ev.Start == nil || ev.End == nil {
			return badReq("a timed event needs start and end")
		}
		st, err := parseTS("start", *ev.Start)
		if err != nil {
			return err
		}
		en, err := parseTS("end", *ev.End)
		if err != nil {
			return err
		}
		if !en.After(st) {
			return badReq("end must be after start")
		}
		ev.Start, ev.End = ptr(ts(st)), ptr(ts(en))
	}
	if ev.Status == nil || *ev.Status == "" {
		ev.Status = ptr(csil.EventStatus("confirmed"))
	} else if !validStatus[string(*ev.Status)] {
		return badReq("unknown status " + string(*ev.Status))
	}
	if ev.Transparency == nil || *ev.Transparency == "" {
		ev.Transparency = ptr(csil.EventTransparency("opaque"))
	} else if !validTransparency[string(*ev.Transparency)] {
		return badReq("unknown transparency " + string(*ev.Transparency))
	}
	if ev.Visibility == nil || *ev.Visibility == "" {
		ev.Visibility = ptr(csil.EventVisibility("default"))
	} else if !validVisibility[string(*ev.Visibility)] {
		return badReq("unknown visibility " + string(*ev.Visibility))
	}
	if ev.Recurrence != nil {
		rec := ev.Recurrence
		rec.Rrule = strings.TrimPrefix(strings.TrimSpace(rec.Rrule), "RRULE:")
		if rec.Rrule == "" && len(rec.Rdates) == 0 {
			ev.Recurrence = nil
		} else {
			if rec.Rrule != "" {
				if err := ical.ValidateRRule(rec.Rrule); err != nil {
					return badReq("bad rrule: " + err.Error())
				}
			}
			var err error
			if rec.Exdates, err = normalizeTimes("exdates", rec.Exdates); err != nil {
				return err
			}
			if rec.Rdates, err = normalizeTimes("rdates", rec.Rdates); err != nil {
				return err
			}
		}
	}
	for i := range ev.Attendees {
		a := &ev.Attendees[i]
		if a.Identity != nil {
			a.Identity.Domain = strings.ToLower(strings.TrimSpace(a.Identity.Domain))
			if a.Identity.Domain == "" || a.Identity.UserId == nil || *a.Identity.UserId == "" {
				return badReq("an attendee identity needs domain and user_id")
			}
			a.Identity.Handle = nil
		}
		if a.Email != nil {
			e := strings.TrimSpace(strings.TrimPrefix(*a.Email, "mailto:"))
			if e == "" {
				a.Email = nil
			} else if !strings.Contains(e, "@") {
				return badReq("attendee email " + e + " is not an address")
			} else {
				a.Email = &e
			}
		}
		if a.Identity == nil && a.Email == nil {
			return badReq("an attendee needs an identity or an email")
		}
		if a.Role == nil || *a.Role == "" {
			a.Role = ptr(csil.AttendeeRole("required"))
		} else if !validAttendeeRole[string(*a.Role)] {
			return badReq("unknown attendee role " + string(*a.Role))
		}
		if a.Status == nil || *a.Status == "" {
			a.Status = ptr(csil.ParticipationStatus("needs_action"))
		} else if !validPartstat[string(*a.Status)] {
			return badReq("unknown attendee status " + string(*a.Status))
		}
		if a.Rsvp == nil {
			a.Rsvp = ptr(true)
		}
	}
	for i := range ev.Reminders {
		r := &ev.Reminders[i]
		if r.MinutesBefore < 0 || r.MinutesBefore > 40320 {
			return badReq("reminder minutes_before must be 0 to 40320")
		}
		if r.Method == nil || *r.Method == "" {
			r.Method = ptr(csil.ReminderMethod("display"))
		} else if !validReminder[string(*r.Method)] {
			return badReq("unknown reminder method " + string(*r.Method))
		}
	}
	return nil
}

func normalizeTimes(field string, in []csil.Timestamp) ([]csil.Timestamp, error) {
	out := make([]csil.Timestamp, 0, len(in))
	seen := map[csil.Timestamp]bool{}
	for _, v := range in {
		t, err := parseTS(field, v)
		if err != nil {
			return nil, err
		}
		n := ts(t)
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// seriesUntil is the stored end of a series, or nil for a single event or
// an open series (nil is always safe: the range query then keeps it).
func seriesUntil(ev csil.Event) *time.Time {
	if ev.Recurrence == nil {
		return nil
	}
	t, err := ical.SeriesUntil(ev)
	if err != nil || t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func sameAttendee(a, b csil.Attendee) bool {
	if a.Identity != nil && b.Identity != nil && a.Identity.UserId != nil && b.Identity.UserId != nil {
		return strings.EqualFold(a.Identity.Domain, b.Identity.Domain) && *a.Identity.UserId == *b.Identity.UserId
	}
	if a.Email != nil && b.Email != nil {
		return strings.EqualFold(*a.Email, *b.Email)
	}
	return false
}

// mergeAnswers keeps each attendee's own answer: an organizer cannot set
// another person's participation status. A new attendee starts at
// needs_action; the effective user (the organizer) may set their own.
func mergeAnswers(next []csil.Attendee, prev []csil.Attendee, prevBy []string, effDomain, effSubject string) ([]csil.Attendee, []string) {
	by := make([]string, len(next))
	for i := range next {
		a := &next[i]
		self := a.Identity != nil && a.Identity.UserId != nil &&
			strings.EqualFold(a.Identity.Domain, effDomain) && *a.Identity.UserId == effSubject
		found := false
		for j, p := range prev {
			if sameAttendee(*a, p) {
				found = true
				if !self {
					a.Status = p.Status
				}
				if j < len(prevBy) && !self {
					by[i] = prevBy[j]
				}
				break
			}
		}
		if !found && !self {
			a.Status = ptr(csil.ParticipationStatus("needs_action"))
		}
	}
	return next, by
}

// busyOnly strips an event to its time.
func busyOnly(ev csil.Event) csil.Event {
	return csil.Event{
		EventId: ev.EventId, CalendarId: ev.CalendarId, Uid: ev.Uid, Title: "Busy",
		Start: ev.Start, End: ev.End, AllDay: ev.AllDay, StartDate: ev.StartDate, EndDate: ev.EndDate,
		Timezone: ev.Timezone, Status: ev.Status, Transparency: ev.Transparency, Visibility: ev.Visibility,
		Recurrence: ev.Recurrence, MasterEventId: ev.MasterEventId, RecurrenceId: ev.RecurrenceId,
		Sequence: ev.Sequence, CreatedAt: ev.CreatedAt, UpdatedAt: ev.UpdatedAt,
	}
}

// people caches person summaries for one call.
type people struct {
	d      *Deps
	byID   map[string]*csil.PersonSummary
	byIdty map[string]*csil.PersonSummary
}

func (d *Deps) people() *people {
	return &people{d: d, byID: map[string]*csil.PersonSummary{}, byIdty: map[string]*csil.PersonSummary{}}
}

func (p *people) id(ctx context.Context, id string) (*csil.PersonSummary, error) {
	if id == "" {
		return nil, nil
	}
	if v, ok := p.byID[id]; ok {
		return v, nil
	}
	v, err := p.d.personByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.byID[id] = v
	return v, nil
}

func (p *people) identity(ctx context.Context, domain, subject string) (*csil.PersonSummary, error) {
	key := strings.ToLower(domain) + "\x00" + subject
	if v, ok := p.byIdty[key]; ok {
		return v, nil
	}
	u, err := p.d.Store.GetUserByIdentity(ctx, domain, subject)
	var v *csil.PersonSummary
	if err == nil {
		s := person(u)
		v = &s
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	p.byIdty[key] = v
	return v, nil
}

// eventOut fills the person summaries of a stored row.
func (p *people) eventOut(ctx context.Context, e store.Event) (csil.Event, error) {
	ev := e.Ev
	var err error
	if ev.Organizer, err = p.id(ctx, e.OrganizerID); err != nil {
		return csil.Event{}, internal(err, "load organizer")
	}
	if ev.CreatedBy, err = p.id(ctx, e.CreatedByID); err != nil {
		return csil.Event{}, internal(err, "load creator")
	}
	if ev.UpdatedBy, err = p.id(ctx, e.UpdatedByID); err != nil {
		return csil.Event{}, internal(err, "load updater")
	}
	if len(ev.Attendees) > 0 {
		atts := make([]csil.Attendee, len(ev.Attendees))
		copy(atts, ev.Attendees)
		for i := range atts {
			a := &atts[i]
			if a.Identity != nil && a.Identity.UserId != nil {
				if a.Person, err = p.identity(ctx, a.Identity.Domain, *a.Identity.UserId); err != nil {
					return csil.Event{}, internal(err, "load attendee")
				}
			}
			if i < len(e.RespondedBy) && e.RespondedBy[i] != "" {
				if a.RespondedBy, err = p.id(ctx, e.RespondedBy[i]); err != nil {
					return csil.Event{}, internal(err, "load responder")
				}
			}
		}
		ev.Attendees = atts
	}
	return ev, nil
}

// viewOut applies an event view: busy-only strips details.
func (p *people) viewOut(ctx context.Context, e store.Event, v access.EventView) (csil.Event, error) {
	if v.BusyOnly {
		return busyOnly(e.Ev), nil
	}
	return p.eventOut(ctx, e)
}

// expandRow returns the instances of a master or single event that
// overlap [start, end). A single event is handled here; a series goes to
// ical.Expand.
func expandRow(master store.Event, overrides []store.Event, start, end time.Time, max int) ([]ical.Instance, bool, error) {
	if master.Ev.Recurrence == nil {
		s, e, err := store.Bounds(master.Ev)
		if err != nil {
			return nil, false, err
		}
		if s.Before(end) && e.After(start) && max > 0 {
			return []ical.Instance{{Event: master.Ev, Start: s, End: e}}, false, nil
		}
		return nil, false, nil
	}
	ovs := make([]csil.Event, len(overrides))
	for i, o := range overrides {
		ovs[i] = o.Ev
	}
	return ical.Expand(master.Ev, ovs, start, end, max)
}

// groupRows splits rows into masters (and singles) and overrides by master.
func groupRows(rows []store.Event) ([]store.Event, map[string][]store.Event) {
	var masters []store.Event
	ovs := map[string][]store.Event{}
	for _, r := range rows {
		if m := r.MasterID(); m != "" {
			ovs[m] = append(ovs[m], r)
		} else {
			masters = append(masters, r)
		}
	}
	return masters, ovs
}

func viaOf(id *auth.Identity) *csil.ActingVia {
	return ptr(csil.ActingVia(viaOrSelf(id.Via)))
}

// instanceBounds returns the content times of one instance of a series:
// the instance at recurrence id rid keeps the master's duration.
func instanceContent(master csil.Event, rid time.Time) (csil.Event, error) {
	s, e, err := store.Bounds(master)
	if err != nil {
		return csil.Event{}, err
	}
	ev := master
	ev.Recurrence = nil
	if master.AllDay != nil && *master.AllDay {
		days := int(e.Sub(s).Hours() / 24)
		sd := rid.UTC().Truncate(24 * time.Hour)
		ed := sd.AddDate(0, 0, days)
		ev.StartDate = ptr(csil.DateText(sd.Format(store.DateLayout)))
		ev.EndDate = ptr(csil.DateText(ed.Format(store.DateLayout)))
		ev.Start, ev.End = ptr(ts(sd)), ptr(ts(ed))
	} else {
		ev.Start, ev.End = ptr(ts(rid)), ptr(ts(rid.Add(e.Sub(s))))
	}
	return ev, nil
}
