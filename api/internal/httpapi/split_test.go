package httpapi

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func splitOnce(c *client, key string, ev csil.Event, rid string, maps ...string) reply[csil.SplitSeriesReceipt] {
	return call(c, "EventService", "split-series-once", csil.SplitSeriesOnceRequest{IdempotencyKey: key, Event: ev, RecurrenceId: csil.Timestamp(rid), MapRecurrenceIds: tsList(maps)},
		csil.EncodeSplitSeriesOnceRequest, csil.DecodeSplitSeriesReceipt)
}

func getSplit(c *client, series csil.EventID, key string, maps ...string) reply[csil.SplitSeriesReceipt] {
	return call(c, "EventService", "get-split-receipt", csil.SplitReceiptRequest{SeriesId: series, IdempotencyKey: key, MapRecurrenceIds: tsList(maps)},
		csil.EncodeSplitReceiptRequest, csil.DecodeSplitSeriesReceipt)
}

func tsList(in []string) []csil.Timestamp {
	out := make([]csil.Timestamp, len(in))
	for i, v := range in {
		out[i] = csil.Timestamp(v)
	}
	return out
}

// wantMaps compares the mappings with "result event rid" entries; the
// event is S (series) or N (successor).
func wantMaps(t *testing.T, r csil.SplitSeriesReceipt, want ...string) {
	t.Helper()
	if len(r.Mappings) != len(want) {
		t.Fatalf("%d mappings, want %d", len(r.Mappings), len(want))
	}
	for i, m := range r.Mappings {
		got := string(m.Result)
		if m.EventId != nil {
			who := "?"
			switch *m.EventId {
			case r.SeriesId:
				who = "S"
			case r.SuccessorId:
				who = "N"
			}
			got += " " + who + " " + string(*m.MappedRecurrenceId)
		}
		if got != want[i] {
			t.Fatalf("mapping %d (%s): %q, want %q", i, m.RecurrenceId, got, want[i])
		}
	}
}

func at(ev csil.Event, start, end string) csil.Event {
	ev.Start, ev.End = tsp(start), tsp(end)
	return ev
}

func TestSplitSeriesOnce(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	a1 := defaultCalendar(t, alice)
	series := must(t, createEvent(alice, weekly(a1.CalendarId, "Standup"))) // Mondays 15:00Z, 11-02 .. 12-07
	must(t, deleteEvent(alice, series.EventId, "this", tsp("2026-11-23T15:00:00Z")))

	// The successor starts a day and an hour later, with a shorter rule.
	next := at(series, "2026-11-17T16:00:00Z", "2026-11-17T17:00:00Z")
	next.Title = "Standup v2"
	next.Recurrence = &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=3"}
	maps := []string{"2026-11-09T15:00:00Z", "2026-11-10T15:00:00Z", "2026-11-16T15:00:00Z", "2026-11-23T15:00:00Z",
		"2026-11-30T15:00:00Z", "2026-12-07T15:00:00Z", "2026-12-14T15:00:00Z"}
	wantAll := []string{"original S 2026-11-09T15:00:00Z", "none", "successor N 2026-11-17T16:00:00Z", "none",
		"successor N 2026-12-01T16:00:00Z", "none", "none"}
	first := must(t, splitOnce(alice, "s1", next, "2026-11-16T15:00:00Z", maps...))
	if !first.Created || first.InPlace || first.SuccessorId == series.EventId || first.SplitAt != "2026-11-16T15:00:00Z" ||
		first.SeriesState != "live" || first.SuccessorState != "live" || first.Successor.Title != "Standup v2" {
		t.Fatalf("first: %+v", first)
	}
	wantMaps(t, first, wantAll...)
	succ := first.SuccessorId
	if got := strings.Join(starts(t, alice, series.EventId), ","); got != "2026-11-02T15:00:00Z,2026-11-09T15:00:00Z" {
		t.Fatalf("series after split: %s", got)
	}
	wantOcc(t, must(t, getOccurrence(alice, succ, tsp("2026-12-01T16:00:00Z"))), "2026-12-01T16:00:00Z", "2026-12-01T16:00:00Z", "2026-12-01T17:00:00Z", "Standup v2")

	// The response was lost: the retry finds the same split and splits
	// nothing.
	again := must(t, splitOnce(alice, "s1", next, "2026-11-16T15:00:00Z", maps...))
	if again.Created || again.SuccessorId != succ {
		t.Fatalf("retry: %+v", again)
	}
	wantMaps(t, again, wantAll...)
	wantMaps(t, must(t, getSplit(alice, series.EventId, "s1")))
	if n := len(starts(t, alice, succ)); n != 3 {
		t.Fatalf("successor occurrences: %d", n)
	}
	// Other content or another recurrence id with the same key: 409.
	changed := next
	changed.Title = "Standup v3"
	wantCode(t, splitOnce(alice, "s1", changed, "2026-11-16T15:00:00Z"), 409)
	wantCode(t, splitOnce(alice, "s1", next, "2026-11-09T15:00:00Z"), 409)
	wantCode(t, getSplit(alice, series.EventId, "nope"), 404)

	// The receipt follows the series: edit, move, delete, purge. The
	// mapping does not change.
	edited := *again.Successor
	edited.Title = "Standup v4"
	must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: edited}, csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	if r := must(t, splitOnce(alice, "s1", next, "2026-11-16T15:00:00Z", maps...)); r.Successor.Title != "Standup v4" {
		t.Fatalf("after edit: %+v", r.Successor)
	} else {
		wantMaps(t, r, wantAll...)
	}
	a2 := must(t, call(alice, "CalendarService", "create-calendar", csil.CreateCalendarRequest{Name: "Work"},
		csil.EncodeCreateCalendarRequest, csil.DecodeCalendar))
	must(t, call(alice, "EventService", "move-event", csil.MoveEventRequest{EventId: succ, CalendarId: a2.CalendarId},
		csil.EncodeMoveEventRequest, csil.DecodeEvent))
	if r := must(t, getSplit(alice, series.EventId, "s1")); r.SuccessorState != "live" || r.Successor.CalendarId != a2.CalendarId || r.CalendarId != a1.CalendarId {
		t.Fatalf("after move: %+v", r)
	}
	del := must(t, deleteEvent(alice, succ, "", nil))
	r := must(t, splitOnce(alice, "s1", next, "2026-11-16T15:00:00Z", maps...))
	if r.SuccessorState != "trashed" || r.SuccessorTrashId == nil || *r.SuccessorTrashId != *del.TrashId || r.SeriesState != "live" || r.Created {
		t.Fatalf("after delete: %+v", r)
	}
	wantMaps(t, r, wantAll...)
	// The retry did not split again or restore.
	if got := strings.Join(starts(t, alice, series.EventId), ","); got != "2026-11-02T15:00:00Z,2026-11-09T15:00:00Z" {
		t.Fatalf("series after retry: %s", got)
	}
	wantCode(t, call(alice, "EventService", "get-event", succ, csil.EncodeEventGetEventRequest, csil.DecodeEvent), 404)
	must(t, deleteEvent(alice, series.EventId, "", nil))
	now := time.Now().UTC().Add(31 * 24 * time.Hour)
	e.deps.Store.Now = func() time.Time { return now }
	if _, err := e.deps.Store.PurgeTrash(t.Context()); err != nil {
		t.Fatal(err)
	}
	e.restart()
	r = must(t, getSplit(alice, series.EventId, "s1", maps...))
	if r.SeriesState != "deleted" || r.SuccessorState != "deleted" || r.SuccessorId != succ {
		t.Fatalf("after purge and restart: %+v", r)
	}
	wantMaps(t, r, wantAll...)

	// Bad requests.
	single := must(t, createEvent(alice, timedEvent(a1.CalendarId, "Once", "2026-11-04T15:00:00Z", "2026-11-04T16:00:00Z")))
	wantCode(t, splitOnce(alice, "x", single, "2026-11-04T15:00:00Z"), 400)
	ser2 := must(t, createEvent(alice, weekly(a1.CalendarId, "Other")))
	ov := ser2
	ov.Title = "Other (moved)"
	ov.Recurrence = nil
	moved := must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: ov, Scope: scopeP("this"), RecurrenceId: tsp("2026-11-09T15:00:00Z")},
		csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	wantCode(t, splitOnce(alice, "x", moved, "2026-11-09T15:00:00Z"), 400)
	wantCode(t, splitOnce(alice, "", ser2, "2026-11-09T15:00:00Z"), 400)
	wantCode(t, splitOnce(alice, "x", ser2, "2026-11-09T15:00:00Z", make([]string, 501)...), 400)
}

func TestSplitSeriesOnceMapping(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	a1 := defaultCalendar(t, alice)

	// Denver, 08:00 local, across the change to daylight time (2027-03-14).
	den := timedEvent(a1.CalendarId, "Denver", "2027-03-01T15:00:00Z", "2027-03-01T16:00:00Z")
	den.Timezone = sp("America/Denver")
	den.Recurrence = &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=4"}
	ser := must(t, createEvent(alice, den))
	// From 03-08, at 09:00 local.
	next := at(ser, "2027-03-08T16:00:00Z", "2027-03-08T17:00:00Z")
	r := must(t, splitOnce(alice, "dst", next, "2027-03-08T15:00:00Z", "2027-03-01T15:00:00Z", "2027-03-15T14:00:00Z", "2027-03-22T14:00:00Z"))
	wantMaps(t, r, "original S 2027-03-01T15:00:00Z", "successor N 2027-03-15T15:00:00Z", "successor N 2027-03-22T15:00:00Z")

	// At the first instance: the series changes in place.
	ser = must(t, createEvent(alice, den))
	next = at(ser, "2027-03-01T16:00:00Z", "2027-03-01T17:00:00Z")
	r = must(t, splitOnce(alice, "first", next, "2027-03-01T15:00:00Z", "2027-03-01T15:00:00Z", "2027-03-22T14:00:00Z"))
	if !r.InPlace || r.SuccessorId != ser.EventId || r.SplitAt != "2027-03-01T15:00:00Z" {
		t.Fatalf("in place: %+v", r)
	}
	wantMaps(t, r, "successor S 2027-03-01T16:00:00Z", "successor S 2027-03-22T15:00:00Z") // in place: S is N
	if r.Mappings[0].EventId == nil || *r.Mappings[0].EventId != ser.EventId {
		t.Fatalf("in place mapping: %+v", r.Mappings[0])
	}
	if must(t, splitOnce(alice, "first", next, "2027-03-01T15:00:00Z")).Created {
		t.Fatal("in place retry made a change")
	}

	// An all-day series: dates move by days.
	day := csil.Event{CalendarId: a1.CalendarId, Title: "Trash day", AllDay: ptrBool(true),
		StartDate: ptr(csil.DateText("2026-11-06")), EndDate: ptr(csil.DateText("2026-11-07")),
		Recurrence: &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=4"}}
	days := must(t, createEvent(alice, day))
	nd := days
	nd.StartDate, nd.EndDate = ptr(csil.DateText("2026-11-14")), ptr(csil.DateText("2026-11-15"))
	r = must(t, splitOnce(alice, "day", nd, "2026-11-13T00:00:00Z", "2026-11-06T00:00:00Z", "2026-11-20T00:00:00Z", "2026-11-21T00:00:00Z"))
	wantMaps(t, r, "original S 2026-11-06T00:00:00Z", "successor N 2026-11-21T00:00:00Z", "none")
}

func TestSplitSeriesOnceConcurrent(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	a1 := defaultCalendar(t, alice)
	series := must(t, createEvent(alice, weekly(a1.CalendarId, "Race")))
	next := at(series, "2026-11-16T16:00:00Z", "2026-11-16T17:00:00Z")
	const n = 8
	got := make([]reply[csil.SplitSeriesReceipt], n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = splitOnce(alice, "race", next, "2026-11-16T15:00:00Z")
		}()
	}
	wg.Wait()
	made := 0
	for _, g := range got {
		r := must(t, g)
		if r.SuccessorId != must(t, got[0]).SuccessorId {
			t.Fatalf("two successors for one key")
		}
		if r.Created {
			made++
		}
	}
	if made != 1 {
		t.Fatalf("%d calls report created", made)
	}
	occ := must(t, call(alice, "EventService", "list-events", csil.ListEventsRequest{CalendarIds: []csil.CalendarID{a1.CalendarId},
		RangeStart: "2026-11-01T00:00:00Z", RangeEnd: "2027-01-01T00:00:00Z"}, csil.EncodeListEventsRequest, csil.DecodeOccurrenceList))
	// Two before the split, six in the successor (COUNT=6). A second split
	// would add six more.
	if len(occ.Occurrences) != 8 {
		t.Fatalf("%d occurrences after the split, want 8", len(occ.Occurrences))
	}
}

func TestSplitSeriesOnceAccess(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	bob, bobLogin := e.login("bob")
	a1 := defaultCalendar(t, alice)
	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: devRefP("bob"), Role: "contribute"},
		csil.EncodeShare, csil.DecodeShare))
	aliceSeries := must(t, createEvent(alice, weekly(a1.CalendarId, "Alice")))
	// Contribute access changes only the caller's own series.
	wantCode(t, splitOnce(bob, "k", at(aliceSeries, "2026-11-16T15:00:00Z", "2026-11-16T16:00:00Z"), "2026-11-16T15:00:00Z"), 403)
	must(t, splitOnce(alice, "k", at(aliceSeries, "2026-11-16T15:00:00Z", "2026-11-16T16:00:00Z"), "2026-11-16T15:00:00Z"))
	// Another caller does not see alice's receipt.
	wantCode(t, getSplit(bob, aliceSeries.EventId, "k"), 404)

	bobSeries := must(t, createEvent(bob, weekly(a1.CalendarId, "Bob")))
	next := at(bobSeries, "2026-11-16T15:00:00Z", "2026-11-16T16:00:00Z")
	made := must(t, splitOnce(bob, "k", next, "2026-11-16T15:00:00Z"))
	// Access is checked now: after the share is removed, bob gets nothing.
	must(t, call(alice, "ShareService", "delete-share", mustShareFor(t, alice, a1.CalendarId, bobLogin.User.Subject),
		csil.EncodeShareDeleteShareRequest, csil.DecodeEmptyResponse))
	wantCode(t, splitOnce(bob, "k", next, "2026-11-16T15:00:00Z"), 404)
	wantCode(t, getSplit(bob, bobSeries.EventId, "k"), 404)
	// The owner's own key is separate from bob's.
	wantCode(t, getSplit(alice, bobSeries.EventId, "k"), 404)
	if r := must(t, getSplit(alice, aliceSeries.EventId, "k")); r.SuccessorId == made.SuccessorId {
		t.Fatalf("keys mixed: %+v", r)
	}
}
