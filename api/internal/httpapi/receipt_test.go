package httpapi

import (
	"sync"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func createOnce(c *client, key string, ev csil.Event) reply[csil.CreateEventReceipt] {
	return call(c, "EventService", "create-event-once", csil.CreateEventOnceRequest{IdempotencyKey: key, Event: ev},
		csil.EncodeCreateEventOnceRequest, csil.DecodeCreateEventReceipt)
}

func getReceipt(c *client, cal csil.CalendarID, key string) reply[csil.CreateEventReceipt] {
	return call(c, "EventService", "get-create-receipt", csil.CreateReceiptRequest{CalendarId: cal, IdempotencyKey: key},
		csil.EncodeCreateReceiptRequest, csil.DecodeCreateEventReceipt)
}

func wantReceipt(t *testing.T, got csil.CreateEventReceipt, id csil.EventID, state string, created bool) {
	t.Helper()
	if got.EventId != id || string(got.State) != state || got.Created != created {
		t.Fatalf("receipt %s/%s/%v, want %s/%s/%v", got.EventId, got.State, got.Created, id, state, created)
	}
	if (state == "live") != (got.Event != nil) {
		t.Fatalf("state %s with event %v", state, got.Event != nil)
	}
}

func TestCreateEventOnce(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	a1 := defaultCalendar(t, alice)
	ev := timedEvent(a1.CalendarId, "Planning", "2026-11-10T15:00:00Z", "2026-11-10T16:00:00Z")

	first := must(t, createOnce(alice, "k1", ev))
	if !first.Created || first.State != "live" || first.Event == nil || first.Event.Title != "Planning" {
		t.Fatalf("first: %+v", first)
	}
	id := first.EventId
	// The response was lost: the retry finds the same event, and makes no
	// second one.
	wantReceipt(t, must(t, createOnce(alice, "k1", ev)), id, "live", false)
	wantReceipt(t, must(t, getReceipt(alice, a1.CalendarId, "k1")), id, "live", false)
	if n := len(starts(t, alice, id)); n != 1 {
		t.Fatalf("occurrences: %d", n)
	}
	// Other content with the same key is a conflict.
	changed := ev
	changed.Title = "Planning (moved)"
	wantCode(t, createOnce(alice, "k1", changed), 409)
	// No receipt for an unknown key; bad keys are refused.
	wantCode(t, getReceipt(alice, a1.CalendarId, "nope"), 404)
	wantCode(t, createOnce(alice, "", ev), 400)
	// The old create-event is unchanged.
	must(t, createEvent(alice, timedEvent(a1.CalendarId, "Plain", "2026-11-11T15:00:00Z", "2026-11-11T16:00:00Z")))

	// Another calendar has its own keys.
	a2 := must(t, call(alice, "CalendarService", "create-calendar", csil.CreateCalendarRequest{Name: "Work"},
		csil.EncodeCreateCalendarRequest, csil.DecodeCalendar))
	other := ev
	other.CalendarId = a2.CalendarId
	if r := must(t, createOnce(alice, "k1", other)); !r.Created || r.EventId == id {
		t.Fatalf("other calendar: %+v", r)
	}

	// The receipt follows the event: edit, move, trash, restore, purge.
	live := must(t, createOnce(alice, "k1", ev)).Event
	live.Title = "Planning v2"
	must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: *live}, csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	if r := must(t, createOnce(alice, "k1", ev)); r.Event.Title != "Planning v2" {
		t.Fatalf("after edit: %+v", r.Event)
	}
	must(t, call(alice, "EventService", "move-event", csil.MoveEventRequest{EventId: id, CalendarId: a2.CalendarId},
		csil.EncodeMoveEventRequest, csil.DecodeEvent))
	r := must(t, createOnce(alice, "k1", ev))
	wantReceipt(t, r, id, "live", false)
	if r.CalendarId != a1.CalendarId || r.Event.CalendarId != a2.CalendarId {
		t.Fatalf("after move: receipt %s, event %s", r.CalendarId, r.Event.CalendarId)
	}
	del := must(t, deleteEvent(alice, id, "", nil))
	r = must(t, createOnce(alice, "k1", ev))
	wantReceipt(t, r, id, "trashed", false)
	if r.TrashId == nil || *r.TrashId != *del.TrashId {
		t.Fatalf("trash id %v, want %s", r.TrashId, *del.TrashId)
	}
	// The retry did not restore or make the event again.
	wantCode(t, call(alice, "EventService", "get-event", id, csil.EncodeEventGetEventRequest, csil.DecodeEvent), 404)
	must(t, restore(alice, del.TrashId))
	wantReceipt(t, must(t, createOnce(alice, "k1", ev)), id, "live", false)
	must(t, deleteEvent(alice, id, "", nil))
	now := time.Now().UTC().Add(31 * 24 * time.Hour)
	e.deps.Store.Now = func() time.Time { return now }
	if _, err := e.deps.Store.PurgeTrash(t.Context()); err != nil {
		t.Fatal(err)
	}
	wantReceipt(t, must(t, createOnce(alice, "k1", ev)), id, "deleted", false)

	// A restart keeps the receipt.
	e.restart()
	wantReceipt(t, must(t, getReceipt(alice, a1.CalendarId, "k1")), id, "deleted", false)
	wantReceipt(t, must(t, createOnce(alice, "k1", ev)), id, "deleted", false)
}

func TestCreateEventOnceConcurrent(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	a1 := defaultCalendar(t, alice)
	ev := timedEvent(a1.CalendarId, "Race", "2026-11-12T15:00:00Z", "2026-11-12T16:00:00Z")
	const n = 8
	got := make([]reply[csil.CreateEventReceipt], n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = createOnce(alice, "race", ev)
		}()
	}
	wg.Wait()
	made := 0
	for _, g := range got {
		r := must(t, g)
		if r.EventId != must(t, got[0]).EventId {
			t.Fatalf("two events for one key: %s, %s", r.EventId, must(t, got[0]).EventId)
		}
		if r.Created {
			made++
		}
	}
	if made != 1 {
		t.Fatalf("%d calls report created", made)
	}
	if c := len(starts(t, alice, must(t, got[0]).EventId)); c != 1 {
		t.Fatalf("occurrences: %d", c)
	}
}

func TestCreateEventOnceScopeAndAccess(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	bob, bobLogin := e.login("bob")
	a1 := defaultCalendar(t, alice)
	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: devRefP("bob"), Role: "contribute"},
		csil.EncodeShare, csil.DecodeShare))
	ev := timedEvent(a1.CalendarId, "Shared", "2026-11-13T15:00:00Z", "2026-11-13T16:00:00Z")
	aliceMade := must(t, createOnce(alice, "k", ev))
	// Another creator with the same key: a separate receipt and event.
	bobMade := must(t, createOnce(bob, "k", ev))
	if !bobMade.Created || bobMade.EventId == aliceMade.EventId {
		t.Fatalf("bob: %+v", bobMade)
	}
	// Moved to a calendar that bob cannot see: hidden, no details.
	a2 := must(t, call(alice, "CalendarService", "create-calendar", csil.CreateCalendarRequest{Name: "Private"},
		csil.EncodeCreateCalendarRequest, csil.DecodeCalendar))
	must(t, call(alice, "EventService", "move-event", csil.MoveEventRequest{EventId: bobMade.EventId, CalendarId: a2.CalendarId},
		csil.EncodeMoveEventRequest, csil.DecodeEvent))
	wantReceipt(t, must(t, createOnce(bob, "k", ev)), bobMade.EventId, "hidden", false)
	// Access is checked now: after the share is removed, bob gets nothing.
	must(t, call(alice, "ShareService", "delete-share", mustShareFor(t, alice, a1.CalendarId, bobLogin.User.Subject),
		csil.EncodeShareDeleteShareRequest, csil.DecodeEmptyResponse))
	wantCode(t, createOnce(bob, "k", ev), 404)
	wantCode(t, getReceipt(bob, a1.CalendarId, "k"), 404)

	// Act-as: the key is bound to the grantee application.
	fake := &fakeActAs{scopes: []string{"calendars.read", "events.write"}, expires: time.Now().Add(time.Hour).UTC().Truncate(time.Second)}
	e.deps.ActAs = fake
	app1 := e.client()
	app1.token = must(t, exchange(e.client(), "good-a1")).Token
	var h1 csil.Calendar
	for _, c := range must(t, call(app1, "CalendarService", "list-calendars", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeCalendarList)).Calendars {
		if c.IsDefault {
			h1 = c
		}
	}
	hev := timedEvent(h1.CalendarId, "Via app", "2026-11-14T15:00:00Z", "2026-11-14T16:00:00Z")
	viaApp := must(t, createOnce(app1, "k", hev))
	app1b := e.client()
	app1b.token = must(t, exchange(e.client(), "good-a2")).Token
	wantReceipt(t, must(t, createOnce(app1b, "k", hev)), viaApp.EventId, "live", false)
	fake.grantee = "acct-g@grantee.example/other-app"
	app2 := e.client()
	app2.token = must(t, exchange(e.client(), "good-a3")).Token
	if r := must(t, createOnce(app2, "k", hev)); !r.Created || r.EventId == viaApp.EventId {
		t.Fatalf("other grantee: %+v", r)
	}
	wantCode(t, getReceipt(app2, h1.CalendarId, "missing"), 404)
}
