package httpapi

import (
	"strings"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func deleteEvent(c *client, id csil.EventID, scope string, rid *csil.Timestamp) reply[csil.DeleteEventResponse] {
	req := csil.DeleteEventRequest{EventId: id, RecurrenceId: rid}
	if scope != "" {
		s := csil.RecurrenceScope(scope)
		req.Scope = &s
	}
	return call(c, "EventService", "delete-event", req, csil.EncodeDeleteEventRequest, csil.DecodeDeleteEventResponse)
}

func listTrash(c *client, cal csil.CalendarID) reply[csil.TrashList] {
	return call(c, "EventService", "list-trash", csil.ListTrashRequest{CalendarId: cal}, csil.EncodeListTrashRequest, csil.DecodeTrashList)
}

func restore(c *client, id *csil.TrashID) reply[csil.Event] {
	return call(c, "EventService", "restore-event", csil.RestoreEventRequest{TrashId: *id}, csil.EncodeRestoreEventRequest, csil.DecodeEvent)
}

func importEvent(c *client, ev csil.Event, by csil.IdentityRef, at *csil.Timestamp) reply[csil.Event] {
	return call(c, "EventService", "import-event", csil.ImportEventRequest{Event: ev, CreatedBy: by, CreatedAt: at},
		csil.EncodeImportEventRequest, csil.DecodeEvent)
}

func weekly(cal csil.CalendarID, title string) csil.Event {
	ev := timedEvent(cal, title, "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	ev.Recurrence = &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=6"}
	return ev
}

// starts returns the occurrence starts of one event in November 2026.
func starts(t *testing.T, c *client, id csil.EventID) []string {
	t.Helper()
	var out []string
	for _, o := range must(t, listEvents(c, "2026-11-01T00:00:00Z", "2026-12-31T00:00:00Z")).Occurrences {
		if o.Event.EventId == id || (o.Event.MasterEventId != nil && *o.Event.MasterEventId == id) {
			out = append(out, string(o.Start))
		}
	}
	return out
}

func TestTrashWholeEvent(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	a1 := defaultCalendar(t, alice)

	ev := timedEvent(a1.CalendarId, "Dentist", "2026-11-05T15:00:00Z", "2026-11-05T16:00:00Z")
	ev.Attendees = []csil.Attendee{{Email: sp("pat@example.com")}}
	made := must(t, createEvent(alice, ev))
	del := must(t, deleteEvent(alice, made.EventId, "", nil))
	if del.TrashId == nil {
		t.Fatal("no trash id")
	}
	wantCode(t, call(alice, "EventService", "get-event", made.EventId, csil.EncodeEventGetEventRequest, csil.DecodeEvent), 404)

	trash := must(t, listTrash(alice, a1.CalendarId))
	if len(trash.Entries) != 1 {
		t.Fatalf("trash: %+v", trash)
	}
	en := trash.Entries[0]
	if en.TrashId != *del.TrashId || en.Scope != "all" || en.Event.Title != "Dentist" || en.DeletedBy == nil {
		t.Fatalf("entry: %+v", en)
	}
	if exp, _ := time.Parse(time.RFC3339, string(en.ExpiresAt)); exp.Sub(mustTime(t, en.DeletedAt)) != 30*24*time.Hour {
		t.Fatalf("expires %s deleted %s", en.ExpiresAt, en.DeletedAt)
	}

	back := must(t, restore(alice, del.TrashId))
	if back.EventId != made.EventId || back.CreatedAt != made.CreatedAt || len(back.Attendees) != 1 {
		t.Fatalf("restored %+v", back)
	}
	if n := len(must(t, listTrash(alice, a1.CalendarId)).Entries); n != 0 {
		t.Fatalf("trash after restore: %d", n)
	}
	wantCode(t, restore(alice, del.TrashId), 404)

	// A uid that is on the calendar again is a conflict.
	del = must(t, deleteEvent(alice, made.EventId, "", nil))
	again := timedEvent(a1.CalendarId, "Dentist again", "2026-11-06T15:00:00Z", "2026-11-06T16:00:00Z")
	again.Uid = made.Uid
	must(t, createEvent(alice, again))
	r := wantCode(t, restore(alice, del.TrashId), 409)
	if !strings.Contains(r.svcErr.Message, "same uid") {
		t.Fatalf("message %q", r.svcErr.Message)
	}

	// An expired entry is gone, and the purge removes it.
	now := time.Now().UTC().Add(31 * 24 * time.Hour)
	e.deps.Store.Now = func() time.Time { return now }
	wantCode(t, restore(alice, del.TrashId), 404)
	if n := len(must(t, listTrash(alice, a1.CalendarId)).Entries); n != 0 {
		t.Fatalf("expired entries listed: %d", n)
	}
	if n, err := e.deps.Store.PurgeTrash(t.Context()); err != nil || n != 1 {
		t.Fatalf("purge %d %v", n, err)
	}
}

func mustTime(t *testing.T, v csil.Timestamp) time.Time {
	t.Helper()
	x, err := time.Parse(time.RFC3339, string(v))
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestTrashSeriesScopes(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	a1 := defaultCalendar(t, alice)
	series := must(t, createEvent(alice, weekly(a1.CalendarId, "Standup")))
	all := []string{"2026-11-02T15:00:00Z", "2026-11-09T15:00:00Z", "2026-11-16T15:00:00Z",
		"2026-11-23T15:00:00Z", "2026-11-30T15:00:00Z", "2026-12-07T15:00:00Z"}
	if got := starts(t, alice, series.EventId); strings.Join(got, ",") != strings.Join(all, ",") {
		t.Fatalf("series %v", got)
	}

	// An override of 11-09, then delete that one instance.
	ov := series
	ov.Title = "Standup (long)"
	ov.Recurrence = nil
	ov.Start, ov.End = tsp("2026-11-09T15:00:00Z"), tsp("2026-11-09T17:00:00Z")
	this := csil.RecurrenceScope("this")
	must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: ov, Scope: &this, RecurrenceId: tsp(all[1])},
		csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	one := must(t, deleteEvent(alice, series.EventId, "this", tsp(all[1])))
	if got := starts(t, alice, series.EventId); len(got) != 5 {
		t.Fatalf("after this: %v", got)
	}
	en := must(t, listTrash(alice, a1.CalendarId)).Entries[0]
	if en.Scope != "this" || en.RecurrenceId == nil || *en.RecurrenceId != csil.Timestamp(all[1]) ||
		en.Instance == nil || en.Instance.Title != "Standup (long)" {
		t.Fatalf("this entry: %+v", en)
	}
	must(t, restore(alice, one.TrashId))
	if got := starts(t, alice, series.EventId); len(got) != 6 {
		t.Fatalf("after restore this: %v", got)
	}
	ovr := call(alice, "EventService", "list-events", csil.ListEventsRequest{RangeStart: csil.Timestamp(all[1]), RangeEnd: "2026-11-10T00:00:00Z"},
		csil.EncodeListEventsRequest, csil.DecodeOccurrenceList)
	if occ := must(t, ovr).Occurrences; len(occ) != 1 || occ[0].Event.Title != "Standup (long)" {
		t.Fatalf("override not restored: %+v", occ)
	}

	// This and future from 11-23: the master ends before it.
	future := must(t, deleteEvent(alice, series.EventId, "this_and_future", tsp(all[3])))
	if got := starts(t, alice, series.EventId); len(got) != 3 {
		t.Fatalf("after this_and_future: %v", got)
	}
	must(t, restore(alice, future.TrashId))
	if got := starts(t, alice, series.EventId); len(got) != 6 {
		t.Fatalf("after restore this_and_future: %v", got)
	}

	// A change after a this_and_future delete blocks its restore.
	future = must(t, deleteEvent(alice, series.EventId, "this_and_future", tsp(all[3])))
	cur := must(t, call(alice, "EventService", "get-event", series.EventId, csil.EncodeEventGetEventRequest, csil.DecodeEvent))
	cur.Title = "Standup, renamed"
	must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: cur}, csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	r := wantCode(t, restore(alice, future.TrashId), 409)
	if !strings.Contains(r.svcErr.Message, "changed after the delete") {
		t.Fatalf("message %q", r.svcErr.Message)
	}

	// An instance entry of a deleted series waits for the series.
	one = must(t, deleteEvent(alice, series.EventId, "this", tsp(all[0])))
	whole := must(t, deleteEvent(alice, series.EventId, "all", nil))
	wantCode(t, restore(alice, one.TrashId), 409)
	must(t, restore(alice, whole.TrashId))
	must(t, restore(alice, one.TrashId))
	if got := starts(t, alice, series.EventId); len(got) != 3 || got[0] != all[0] {
		t.Fatalf("after restoring both: %v", got)
	}
}

func TestTrashFollowsTheDeleteRule(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	bob, _ := e.login("bob")
	carol, _ := e.login("carol")
	dave, _ := e.login("dave")
	a1 := defaultCalendar(t, alice)
	for who, role := range map[string]string{"bob": "contribute", "carol": "read", "dave": "write"} {
		must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: devRefP(who), Role: csil.ShareRole(role)},
			csil.EncodeShare, csil.DecodeShare))
	}
	mine := must(t, createEvent(bob, timedEvent(a1.CalendarId, "Bob's", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
	theirs := must(t, createEvent(alice, timedEvent(a1.CalendarId, "Alice's", "2026-11-03T15:00:00Z", "2026-11-03T16:00:00Z")))
	bobDel := must(t, deleteEvent(bob, mine.EventId, "", nil))
	aliceDel := must(t, deleteEvent(alice, theirs.EventId, "", nil))

	// contribute sees and restores only its own entries.
	if en := must(t, listTrash(bob, a1.CalendarId)).Entries; len(en) != 1 || en[0].TrashId != *bobDel.TrashId {
		t.Fatalf("bob trash: %+v", en)
	}
	wantCode(t, restore(bob, aliceDel.TrashId), 403)
	// read sees no trash; restore does not tell that the entry exists.
	wantCode(t, listTrash(carol, a1.CalendarId), 403)
	wantCode(t, restore(carol, aliceDel.TrashId), 404)
	// No access at all: 404 for the calendar and the entry.
	erin, _ := e.login("erin")
	wantCode(t, listTrash(erin, a1.CalendarId), 404)
	wantCode(t, restore(erin, bobDel.TrashId), 404)
	// write sees and restores every entry.
	if en := must(t, listTrash(dave, a1.CalendarId)).Entries; len(en) != 2 || en[0].TrashId != *aliceDel.TrashId {
		t.Fatalf("dave trash (newest first): %+v", en)
	}
	must(t, restore(dave, aliceDel.TrashId))
	back := must(t, restore(bob, bobDel.TrashId))
	if back.CreatedBy == nil || back.CreatedBy.Handle == nil || *back.CreatedBy.Handle != "bob" {
		t.Fatalf("creator after restore: %+v", back.CreatedBy)
	}
	// Bob can still change his own event after the restore.
	back.Title = "Bob's, again"
	must(t, call(bob, "EventService", "update-event", csil.UpdateEventRequest{Event: back}, csil.EncodeUpdateEventRequest, csil.DecodeEvent))
}

func TestImportEventKeepsTheCreator(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	bob, bobLogin := e.login("bob")
	dave, _ := e.login("dave")
	a1 := defaultCalendar(t, alice)
	// frank has not signed in yet. He is in the group by account id.
	house := must(t, createGroup(alice, "House", memberRef("bob"), memberRef("frank")))
	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, GranteeGroupId: gid(house), Role: "contribute"},
		csil.EncodeShare, csil.DecodeShare))
	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: devRefP("dave"), Role: "write"},
		csil.EncodeShare, csil.DecodeShare))

	ev := timedEvent(a1.CalendarId, "Bob's old chore", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")
	ev.Uid = sp("longhouse-event-1")
	at := tsp("2025-03-04T05:06:07Z")
	got := must(t, importEvent(alice, ev, devRef("bob"), at))
	if got.CreatedBy == nil || got.CreatedBy.UserId != bobLogin.User.UserId || got.Organizer == nil ||
		got.Organizer.UserId != bobLogin.User.UserId || got.CreatedAt != *at {
		t.Fatalf("imported: created_by %+v organizer %+v created_at %s", got.CreatedBy, got.Organizer, got.CreatedAt)
	}
	if got.UpdatedBy == nil || got.UpdatedBy.Handle == nil || *got.UpdatedBy.Handle != "alice" {
		t.Fatalf("updated_by %+v", got.UpdatedBy)
	}
	// The same uid again changes nothing.
	wantCode(t, importEvent(alice, ev, devRef("bob"), at), 409)
	// contribute works for the imported event.
	got.Title = "Bob's chore, edited"
	must(t, call(bob, "EventService", "update-event", csil.UpdateEventRequest{Event: got}, csil.EncodeUpdateEventRequest, csil.DecodeEvent))

	// The creator sees the attribution in the audit list.
	audit := must(t, call(bob, "AuditService", "list-audit", csil.AuditQuery{DelegatedOnly: ptrBool(true)}, csil.EncodeAuditQuery, csil.DecodeAuditList))
	found := false
	for _, a := range audit.Entries {
		if a.Operation == "event.ImportEvent.created_by" && a.Target != nil && *a.Target == "event:"+string(got.EventId) &&
			a.Actor.Handle != nil && *a.Actor.Handle == "alice" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no attribution audit for bob: %+v", audit.Entries)
	}

	// A creator who has not signed in gets a user, and a default calendar
	// at the first sign-in.
	ev2 := timedEvent(a1.CalendarId, "Frank's", "2026-11-03T15:00:00Z", "2026-11-03T16:00:00Z")
	fe := must(t, importEvent(alice, ev2, devRef("frank"), nil))
	if fe.CreatedBy == nil || fe.CreatedBy.Handle != nil {
		t.Fatalf("frank: %+v", fe.CreatedBy)
	}
	frank, frankLogin := e.login("frank")
	if frankLogin.User.UserId != fe.CreatedBy.UserId {
		t.Fatalf("frank signed in as a new user: %s, want %s", frankLogin.User.UserId, fe.CreatedBy.UserId)
	}
	defaultCalendar(t, frank)
	fe.Title = "Frank's, edited"
	must(t, call(frank, "EventService", "update-event", csil.UpdateEventRequest{Event: fe}, csil.EncodeUpdateEventRequest, csil.DecodeEvent))

	// Refusals.
	r := wantCode(t, importEvent(alice, ev2, devRef("erin"), nil), 403)
	if !strings.Contains(r.svcErr.Message, "contribute access") {
		t.Fatalf("message %q", r.svcErr.Message)
	}
	wantCode(t, importEvent(dave, ev2, devRef("bob"), nil), 403)
	wantCode(t, importEvent(bob, ev2, devRef("bob"), nil), 403)
	future := tsp(time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02T15:04:05Z"))
	wantCode(t, importEvent(alice, ev2, devRef("bob"), future), 400)
	erin, _ := e.login("erin")
	wantCode(t, importEvent(erin, ev2, devRef("bob"), nil), 404)
	// The owner can import as the owner.
	must(t, importEvent(alice, ev2, devRef("alice"), nil))
}
