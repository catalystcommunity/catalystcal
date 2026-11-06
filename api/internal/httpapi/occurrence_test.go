package httpapi

import (
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func getOccurrence(c *client, id csil.EventID, rid *csil.Timestamp) reply[csil.Occurrence] {
	return call(c, "EventService", "get-occurrence", csil.GetOccurrenceRequest{EventId: id, RecurrenceId: rid},
		csil.EncodeGetOccurrenceRequest, csil.DecodeOccurrence)
}

func wantOcc(t *testing.T, o csil.Occurrence, rid, start, end, title string) {
	t.Helper()
	gotRid := ""
	if o.RecurrenceId != nil {
		gotRid = string(*o.RecurrenceId)
	}
	if gotRid != rid || string(o.Start) != start || string(o.End) != end || o.Event.Title != title {
		t.Fatalf("occurrence rid %q %s..%s %q, want %q %s..%s %q", gotRid, o.Start, o.End, o.Event.Title, rid, start, end, title)
	}
}

func TestGetOccurrence(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	bob, bobLogin := e.login("bob")
	a1 := defaultCalendar(t, alice)
	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: devRefP("bob"), Role: "read"},
		csil.EncodeShare, csil.DecodeShare))

	// A single event needs no recurrence id; with one it has no occurrence.
	single := must(t, createEvent(alice, timedEvent(a1.CalendarId, "Once", "2026-11-04T15:00:00Z", "2026-11-04T16:00:00Z")))
	wantOcc(t, must(t, getOccurrence(bob, single.EventId, nil)), "", "2026-11-04T15:00:00Z", "2026-11-04T16:00:00Z", "Once")
	wantCode(t, getOccurrence(bob, single.EventId, tsp("2026-11-04T15:00:00Z")), 404)

	// A weekly series in Denver, 08:00 local. It expands in local time:
	// across the change to daylight time (2027-03-14) the instant moves.
	dst := timedEvent(a1.CalendarId, "Denver", "2027-03-08T15:00:00Z", "2027-03-08T16:00:00Z")
	dst.Timezone = sp("America/Denver")
	dst.Recurrence = &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=3"}
	dser := must(t, createEvent(alice, dst))
	wantOcc(t, must(t, getOccurrence(bob, dser.EventId, tsp("2027-03-15T14:00:00Z"))), "2027-03-15T14:00:00Z", "2027-03-15T14:00:00Z", "2027-03-15T15:00:00Z", "Denver")
	wantCode(t, getOccurrence(bob, dser.EventId, tsp("2027-03-15T15:00:00Z")), 404)

	ser := weekly(a1.CalendarId, "Standup")
	ser.Timezone = sp("America/Denver")
	series := must(t, createEvent(alice, ser))
	wantCode(t, getOccurrence(bob, series.EventId, nil), 400)
	wantCode(t, getOccurrence(bob, series.EventId, tsp("2026-11-03T15:00:00Z")), 404) // not an instance
	wantCode(t, getOccurrence(bob, series.EventId, tsp("2026-12-14T15:00:00Z")), 404) // after COUNT
	o := must(t, getOccurrence(bob, series.EventId, tsp("2026-11-16T15:00:00Z")))
	wantOcc(t, o, "2026-11-16T15:00:00Z", "2026-11-16T15:00:00Z", "2026-11-16T16:00:00Z", "Standup")
	if o.Event.EventId != series.EventId || o.Access != "read" {
		t.Fatalf("instance: %+v", o)
	}

	// Move one instance to another date and far out of the series range.
	ov := series
	ov.Title = "Standup (moved)"
	ov.Recurrence = nil
	ov.Start, ov.End = tsp("2027-03-01T22:30:00Z"), tsp("2027-03-01T23:00:00Z")
	moved := must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: ov, Scope: scopeP("this"), RecurrenceId: tsp("2026-11-09T15:00:00Z")},
		csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	o = must(t, getOccurrence(bob, series.EventId, tsp("2026-11-09T15:00:00Z")))
	wantOcc(t, o, "2026-11-09T15:00:00Z", "2027-03-01T22:30:00Z", "2027-03-01T23:00:00Z", "Standup (moved)")
	if o.Event.EventId != moved.EventId {
		t.Fatalf("override id %s, want %s", o.Event.EventId, moved.EventId)
	}
	// The override id also works.
	wantOcc(t, must(t, getOccurrence(bob, moved.EventId, nil)), "2026-11-09T15:00:00Z", "2027-03-01T22:30:00Z", "2027-03-01T23:00:00Z", "Standup (moved)")

	// An excluded instance is gone; restore brings it back.
	del := must(t, deleteEvent(alice, series.EventId, "this", tsp("2026-11-23T15:00:00Z")))
	wantCode(t, getOccurrence(bob, series.EventId, tsp("2026-11-23T15:00:00Z")), 404)
	must(t, restore(alice, del.TrashId))
	must(t, getOccurrence(bob, series.EventId, tsp("2026-11-23T15:00:00Z")))

	// The whole series deleted, then restored.
	del = must(t, deleteEvent(alice, series.EventId, "", nil))
	wantCode(t, getOccurrence(bob, series.EventId, tsp("2026-11-16T15:00:00Z")), 404)
	must(t, restore(alice, del.TrashId))
	wantOcc(t, must(t, getOccurrence(bob, series.EventId, tsp("2026-11-09T15:00:00Z"))), "2026-11-09T15:00:00Z", "2027-03-01T22:30:00Z", "2027-03-01T23:00:00Z", "Standup (moved)")

	// A split: the later instances belong to the new series.
	cut := series
	cut.Title = "Standup v2"
	split := must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: cut, Scope: scopeP("this_and_future"), RecurrenceId: tsp("2026-11-30T15:00:00Z")},
		csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	wantCode(t, getOccurrence(bob, series.EventId, tsp("2026-12-07T15:00:00Z")), 404)
	wantOcc(t, must(t, getOccurrence(bob, split.EventId, tsp("2026-12-07T15:00:00Z"))), "2026-12-07T15:00:00Z", "2026-12-07T15:00:00Z", "2026-12-07T16:00:00Z", "Standup v2")
	must(t, getOccurrence(bob, series.EventId, tsp("2026-11-16T15:00:00Z")))

	// An all-day series: the date counts.
	day := csil.Event{CalendarId: a1.CalendarId, Title: "Trash day", AllDay: ptrBool(true),
		StartDate: ptr(csil.DateText("2026-11-06")), EndDate: ptr(csil.DateText("2026-11-07")),
		Recurrence: &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=4"}}
	days := must(t, createEvent(alice, day))
	o = must(t, getOccurrence(bob, days.EventId, tsp("2026-11-13T00:00:00Z")))
	wantOcc(t, o, "2026-11-13T00:00:00Z", "2026-11-13T00:00:00Z", "2026-11-14T00:00:00Z", "Trash day")
	if o.Event.StartDate == nil || *o.Event.StartDate != "2026-11-13" {
		t.Fatalf("start date %v", o.Event.StartDate)
	}
	wantCode(t, getOccurrence(bob, days.EventId, tsp("2026-11-14T00:00:00Z")), 404)

	// Access loss: the share is removed.
	must(t, call(alice, "ShareService", "delete-share", mustShareFor(t, alice, a1.CalendarId, bobLogin.User.Subject),
		csil.EncodeShareDeleteShareRequest, csil.DecodeEmptyResponse))
	wantCode(t, getOccurrence(bob, series.EventId, tsp("2026-11-16T15:00:00Z")), 404)
	wantCode(t, getOccurrence(bob, single.EventId, nil), 404)
}

func ptr[T any](v T) *T { return &v }
