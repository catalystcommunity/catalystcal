package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// These tests use the ical package (Expand, SeriesUntil, ValidateRRule,
// TruncateRRule, Parse, Encode).

func scopeP(s string) *csil.RecurrenceScope { x := csil.RecurrenceScope(s); return &x }

func titles(occ csil.OccurrenceList) []string {
	var out []string
	for _, o := range occ.Occurrences {
		out = append(out, string(o.Start)[:10]+" "+o.Event.Title)
	}
	return out
}

func TestRecurrenceScopes(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	cal := defaultCalendar(t, alice)

	// Bad RRULE and bad zone are 400.
	bad := timedEvent(cal.CalendarId, "x", "2026-11-02T16:00:00Z", "2026-11-02T17:00:00Z")
	bad.Recurrence = &csil.Recurrence{Rrule: "FREQ=SECONDLY"}
	wantCode(t, createEvent(alice, bad), 400)
	bad.Recurrence = nil
	bad.Timezone = sp("Mars/Olympus")
	wantCode(t, createEvent(alice, bad), 400)
	bad.Timezone = nil
	bad.End = tsp("2026-11-02T15:00:00Z")
	wantCode(t, createEvent(alice, bad), 400)

	// Weekly on Mondays at 09:00 Denver, five times.
	ev := timedEvent(cal.CalendarId, "Weekly", "2026-11-02T16:00:00Z", "2026-11-02T17:00:00Z")
	ev.Timezone = sp("America/Denver")
	ev.Recurrence = &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=5"}
	master := must(t, createEvent(alice, ev))

	occ := must(t, listEvents(alice, "2026-11-01T00:00:00Z", "2026-12-31T00:00:00Z"))
	if len(occ.Occurrences) != 5 {
		t.Fatalf("occurrences: %v", titles(occ))
	}
	if occ.Occurrences[1].RecurrenceId == nil || string(occ.Occurrences[1].Start) != "2026-11-09T16:00:00Z" {
		t.Fatalf("second instance: %+v", occ.Occurrences[1])
	}

	// this: one instance changes.
	third := *occ.Occurrences[2].RecurrenceId
	upd := master
	upd.Title = "Moved"
	upd.Start, upd.End = tsp("2026-11-16T18:00:00Z"), tsp("2026-11-16T19:00:00Z")
	ov := must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: upd, Scope: scopeP("this"), RecurrenceId: &third},
		csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	if ov.MasterEventId == nil || *ov.MasterEventId != master.EventId || ov.RecurrenceId == nil {
		t.Fatalf("override: %+v", ov)
	}

	// this (delete): an EXDATE on the second instance.
	second := *occ.Occurrences[1].RecurrenceId
	must(t, call(alice, "EventService", "delete-event", csil.DeleteEventRequest{EventId: master.EventId, Scope: scopeP("this"), RecurrenceId: &second},
		csil.EncodeDeleteEventRequest, csil.DecodeEmptyResponse))
	occ = must(t, listEvents(alice, "2026-11-01T00:00:00Z", "2026-12-31T00:00:00Z"))
	got := strings.Join(titles(occ), ",")
	if got != "2026-11-02 Weekly,2026-11-16 Moved,2026-11-23 Weekly,2026-11-30 Weekly" {
		t.Fatalf("after this-edits: %s", got)
	}

	// this_and_future at the fourth instance: the master ends, a new series starts.
	fourth := csil.Timestamp("2026-11-23T16:00:00Z")
	split := master
	split.Title = "Renamed"
	split.Start, split.End = tsp("2026-11-23T16:00:00Z"), tsp("2026-11-23T17:00:00Z")
	split.Recurrence = &csil.Recurrence{Rrule: "FREQ=WEEKLY;COUNT=2"}
	next := must(t, call(alice, "EventService", "update-event", csil.UpdateEventRequest{Event: split, Scope: scopeP("this_and_future"), RecurrenceId: &fourth},
		csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	if next.EventId == master.EventId || *next.Uid == *master.Uid {
		t.Fatalf("split did not make a new series: %+v", next)
	}
	occ = must(t, listEvents(alice, "2026-11-01T00:00:00Z", "2026-12-31T00:00:00Z"))
	got = strings.Join(titles(occ), ",")
	if got != "2026-11-02 Weekly,2026-11-16 Moved,2026-11-23 Renamed,2026-11-30 Renamed" {
		t.Fatalf("after split: %s", got)
	}

	// this_and_future delete on the new series from its second instance.
	last := csil.Timestamp("2026-11-30T16:00:00Z")
	must(t, call(alice, "EventService", "delete-event", csil.DeleteEventRequest{EventId: next.EventId, Scope: scopeP("this_and_future"), RecurrenceId: &last},
		csil.EncodeDeleteEventRequest, csil.DecodeEmptyResponse))
	occ = must(t, listEvents(alice, "2026-11-01T00:00:00Z", "2026-12-31T00:00:00Z"))
	if len(occ.Occurrences) != 3 {
		t.Fatalf("after truncating delete: %v", titles(occ))
	}

	// all: the whole first series goes, with its override.
	must(t, call(alice, "EventService", "delete-event", csil.DeleteEventRequest{EventId: master.EventId},
		csil.EncodeDeleteEventRequest, csil.DecodeEmptyResponse))
	occ = must(t, listEvents(alice, "2026-11-01T00:00:00Z", "2026-12-31T00:00:00Z"))
	if len(occ.Occurrences) != 1 {
		t.Fatalf("after delete all: %v", titles(occ))
	}
	wantCode(t, call(alice, "EventService", "get-event", ov.EventId, csil.EncodeEventGetEventRequest, csil.DecodeEvent), 404)

	// The range limit.
	wantCode(t, listEvents(alice, "2026-01-01T00:00:00Z", "2027-03-01T00:00:00Z"), 400)
}

const sampleICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nX-WR-CALNAME:Holidays\r\n" +
	"BEGIN:VEVENT\r\nUID:h1@example\r\nDTSTAMP:20260101T000000Z\r\nDTSTART;VALUE=DATE:20261126\r\nDTEND;VALUE=DATE:20261127\r\nSUMMARY:Thanksgiving\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:h2@example\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261110T150000Z\r\nDTEND:20261110T160000Z\r\nSUMMARY:Meetup\r\nRRULE:FREQ=DAILY;COUNT=3\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestImportExportAndSubscription(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	cal := defaultCalendar(t, alice)

	res := must(t, call(alice, "IcsService", "import-ics", csil.ImportIcsRequest{CalendarId: cal.CalendarId, Ics: sampleICS},
		csil.EncodeImportIcsRequest, csil.DecodeImportResult))
	if res.Created != 2 || res.Updated != 0 {
		t.Fatalf("import: %+v", res)
	}
	res = must(t, call(alice, "IcsService", "import-ics", csil.ImportIcsRequest{CalendarId: cal.CalendarId, Ics: sampleICS},
		csil.EncodeImportIcsRequest, csil.DecodeImportResult))
	if res.Created != 0 || res.Updated != 2 {
		t.Fatalf("re-import by uid: %+v", res)
	}
	occ := must(t, listEvents(alice, "2026-11-01T00:00:00Z", "2026-12-01T00:00:00Z"))
	if len(occ.Occurrences) != 4 {
		t.Fatalf("imported occurrences: %v", titles(occ))
	}
	doc := must(t, call(alice, "IcsService", "export-calendar", csil.ExportCalendarRequest{CalendarId: cal.CalendarId},
		csil.EncodeExportCalendarRequest, csil.DecodeIcsDocument))
	if !strings.Contains(doc.Ics, "Thanksgiving") || !strings.HasSuffix(doc.Filename, ".ics") {
		t.Fatalf("export: %s %s", doc.Filename, doc.Ics)
	}
	invite := must(t, call(alice, "IcsService", "export-event", csil.ExportEventRequest{EventId: occ.Occurrences[0].Event.EventId, Method: sp("request")},
		csil.EncodeExportEventRequest, csil.DecodeIcsDocument))
	if !strings.Contains(invite.Ics, "METHOD:REQUEST") {
		t.Fatalf("iTIP request:\n%s", invite.Ics)
	}

	// Subscription: imported at once, read-only, replaced on refresh.
	body := sampleICS
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = w.Write([]byte(body))
	}))
	defer feed.Close()
	sub := must(t, call(alice, "CalendarService", "subscribe-calendar", csil.SubscribeCalendarRequest{Name: "Holidays", SourceUrl: feed.URL + "/h.ics"},
		csil.EncodeSubscribeCalendarRequest, csil.DecodeCalendar))
	if sub.Kind != "subscription" || sub.LastSyncedAt == nil || sub.SyncError != nil {
		t.Fatalf("subscription: %+v", sub)
	}
	subOcc := must(t, call(alice, "EventService", "list-events", csil.ListEventsRequest{RangeStart: "2026-11-01T00:00:00Z", RangeEnd: "2026-12-01T00:00:00Z", CalendarIds: []csil.CalendarID{sub.CalendarId}},
		csil.EncodeListEventsRequest, csil.DecodeOccurrenceList))
	if len(subOcc.Occurrences) != 4 {
		t.Fatalf("subscription occurrences: %v", titles(subOcc))
	}
	wantCode(t, createEvent(alice, timedEvent(sub.CalendarId, "x", "2026-11-02T16:00:00Z", "2026-11-02T17:00:00Z")), 403)
	wantCode(t, call(alice, "IcsService", "import-ics", csil.ImportIcsRequest{CalendarId: sub.CalendarId, Ics: sampleICS},
		csil.EncodeImportIcsRequest, csil.DecodeImportResult), 403)

	body = strings.Replace(sampleICS, "COUNT=3", "COUNT=1", 1)
	must(t, call(alice, "CalendarService", "refresh-subscription", sub.CalendarId, csil.EncodeCalendarRefreshSubscriptionRequest, csil.DecodeCalendar))
	subOcc = must(t, call(alice, "EventService", "list-events", csil.ListEventsRequest{RangeStart: "2026-11-01T00:00:00Z", RangeEnd: "2026-12-01T00:00:00Z", CalendarIds: []csil.CalendarID{sub.CalendarId}},
		csil.EncodeListEventsRequest, csil.DecodeOccurrenceList))
	if len(subOcc.Occurrences) != 2 {
		t.Fatalf("after refresh: %v", titles(subOcc))
	}

	// A loopback feed is refused when private addresses are not allowed.
	e.deps.Fetcher.AllowPrivate = false
	wantCode(t, call(alice, "CalendarService", "subscribe-calendar", csil.SubscribeCalendarRequest{Name: "x", SourceUrl: feed.URL},
		csil.EncodeSubscribeCalendarRequest, csil.DecodeCalendar), 400)
	refreshed := must(t, call(alice, "CalendarService", "refresh-subscription", sub.CalendarId, csil.EncodeCalendarRefreshSubscriptionRequest, csil.DecodeCalendar))
	if refreshed.SyncError == nil {
		t.Fatal("refresh of a loopback feed did not record a sync error")
	}

	// The default calendar cannot be deleted; another one can.
	wantCode(t, call(alice, "CalendarService", "delete-calendar", cal.CalendarId, csil.EncodeCalendarDeleteCalendarRequest, csil.DecodeEmptyResponse), 403)
	must(t, call(alice, "CalendarService", "delete-calendar", sub.CalendarId, csil.EncodeCalendarDeleteCalendarRequest, csil.DecodeEmptyResponse))
	wantCode(t, call(alice, "CalendarService", "get-calendar", sub.CalendarId, csil.EncodeCalendarGetCalendarRequest, csil.DecodeCalendar), 404)
}
