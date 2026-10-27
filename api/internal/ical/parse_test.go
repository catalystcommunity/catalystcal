package ical

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func parseFixture(t *testing.T, name string) Parsed {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return p
}

func findEvent(t *testing.T, p Parsed, uid string, override bool) csil.Event {
	t.Helper()
	for _, ev := range p.Events {
		if ev.Uid != nil && *ev.Uid == uid && (ev.RecurrenceId != nil) == override {
			return ev
		}
	}
	t.Fatalf("no event %q (override=%v)", uid, override)
	return csil.Event{}
}

func str[T ~string](p *T) string {
	if p == nil {
		return "<nil>"
	}
	return string(*p)
}

func check(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func TestParseGoogle(t *testing.T) {
	p := parseFixture(t, "google.ics")
	check(t, "CalendarName", p.CalendarName, "Team Calendar")
	if len(p.Events) != 4 {
		t.Fatalf("got %d events", len(p.Events))
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings: %v", p.Warnings)
	}

	m := findEvent(t, p, "standup123@google.com", false)
	check(t, "Title", m.Title, "Standup")
	check(t, "Start", str(m.Start), "2026-10-05T15:00:00Z")
	check(t, "End", str(m.End), "2026-10-05T15:30:00Z")
	check(t, "Timezone", str(m.Timezone), "America/Denver")
	check(t, "Rrule", m.Recurrence.Rrule, "FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20261231T235959Z")
	if !reflect.DeepEqual(m.Recurrence.Exdates, []csil.Timestamp{"2026-10-12T15:00:00Z"}) {
		t.Errorf("Exdates = %v", m.Recurrence.Exdates)
	}
	if m.Description == nil || !strings.HasPrefix(*m.Description, "Daily standup, keep it short.\nJoin with Google Meet: https://meet.google.com/abc-defg-hij\n\n-::~") {
		t.Errorf("Description = %q", str(m.Description))
	}
	if m.Location != nil {
		t.Errorf("empty LOCATION should be nil, got %q", *m.Location)
	}
	check(t, "ConferenceUrl", str(m.ConferenceUrl), "https://meet.google.com/abc-defg-hij")
	check(t, "Status", str(m.Status), "confirmed")
	check(t, "Transparency", str(m.Transparency), "opaque")
	check(t, "Visibility", str(m.Visibility), "default")
	if m.Sequence != 2 {
		t.Errorf("Sequence = %d", m.Sequence)
	}
	if len(m.Attendees) != 3 {
		t.Fatalf("attendees: %+v", m.Attendees)
	}
	alice := m.Attendees[0]
	check(t, "alice email", str(alice.Email), "alice@example.com")
	check(t, "alice name", str(alice.Name), "Alice Example")
	check(t, "alice role (organizer)", str(alice.Role), "chair")
	check(t, "alice status", str(alice.Status), "accepted")
	bob := m.Attendees[1]
	check(t, "bob role", str(bob.Role), "required")
	check(t, "bob status", str(bob.Status), "needs_action")
	if bob.Rsvp == nil || !*bob.Rsvp {
		t.Errorf("bob rsvp = %v", bob.Rsvp)
	}
	check(t, "carol role", str(m.Attendees[2].Role), "optional")
	check(t, "carol status", str(m.Attendees[2].Status), "tentative")
	wantRem := []csil.Reminder{{MinutesBefore: 15, Method: ptr(csil.ReminderMethod("display"))}, {MinutesBefore: 1440, Method: ptr(csil.ReminderMethod("email"))}}
	if !reflect.DeepEqual(m.Reminders, wantRem) {
		t.Errorf("Reminders = %+v", m.Reminders)
	}

	o := findEvent(t, p, "standup123@google.com", true)
	check(t, "override RecurrenceId", str(o.RecurrenceId), "2026-10-07T15:00:00Z")
	check(t, "override Start", str(o.Start), "2026-10-07T16:00:00Z")
	check(t, "override Title", o.Title, "Standup (moved)")
	if o.Recurrence != nil {
		t.Errorf("override has recurrence")
	}

	l := findEvent(t, p, "lunch@google.com", false)
	check(t, "lunch Start", str(l.Start), "2026-10-20T17:00:00Z")
	if l.Timezone != nil {
		t.Errorf("UTC event Timezone = %q", *l.Timezone)
	}
	check(t, "lunch Visibility", str(l.Visibility), "private")
	check(t, "lunch Transparency", str(l.Transparency), "transparent")
	check(t, "lunch ConferenceUrl", str(l.ConferenceUrl), "https://meet.google.com/xyz-abcd-efg")

	x := findEvent(t, p, "xmas@google.com", false)
	if x.AllDay == nil || !*x.AllDay {
		t.Fatalf("xmas not all-day")
	}
	check(t, "xmas StartDate", str(x.StartDate), "2026-12-25")
	check(t, "xmas EndDate", str(x.EndDate), "2026-12-26")
	check(t, "xmas Start", str(x.Start), "2026-12-25T00:00:00Z")
	check(t, "xmas Visibility", str(x.Visibility), "public")

	// The parsed master and override expand as Google shows them.
	got, _, err := Expand(m, []csil.Event{o}, mustTime(t, "2026-10-05T00:00:00Z"), mustTime(t, "2026-10-19T00:00:00Z"), 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-10-05T15:00:00Z", "2026-10-07T16:00:00Z", "2026-10-14T15:00:00Z"}
	if gs := startsOf(got); !reflect.DeepEqual(gs, want) {
		t.Errorf("expanded %v, want %v", gs, want)
	}
}

func TestParseApple(t *testing.T) {
	p := parseFixture(t, "apple.ics")
	check(t, "CalendarName", p.CalendarName, "Home")
	if len(p.Events) != 3 || len(p.Warnings) != 0 {
		t.Fatalf("events %d warnings %v", len(p.Events), p.Warnings)
	}
	v := findEvent(t, p, "6C1A9F3E-7A55-4B39-9D1C-1F0E3B8C9A01", false)
	check(t, "vacation StartDate", str(v.StartDate), "2026-10-30")
	check(t, "vacation EndDate", str(v.EndDate), "2026-11-03")
	if v.Timezone != nil {
		t.Errorf("all-day Timezone = %q", *v.Timezone)
	}
	if !reflect.DeepEqual(v.Categories, []string{"Personal", "Travel"}) {
		t.Errorf("Categories = %v", v.Categories)
	}
	check(t, "vacation Transparency", str(v.Transparency), "transparent")

	d := findEvent(t, p, "4B7D2C10-5E3F-4F6A-8B9C-2D1E0F3A4B5C", false)
	check(t, "dentist Start", str(d.Start), "2026-11-10T09:30:00Z")
	check(t, "dentist End", str(d.End), "2026-11-10T10:30:00Z")
	check(t, "dentist Timezone", str(d.Timezone), "Europe/Berlin")
	check(t, "dentist Location", str(d.Location), "Dr. Zahn\nHauptstr. 1, Berlin")
	check(t, "dentist Url", str(d.Url), "https://example.com/booking?id=42&lang=de")
	if d.Sequence != 1 {
		t.Errorf("Sequence = %d", d.Sequence)
	}
	if !reflect.DeepEqual(d.Reminders, []csil.Reminder{{MinutesBefore: 30, Method: ptr(csil.ReminderMethod("display"))}}) {
		t.Errorf("Reminders = %+v (ACTION:NONE must be dropped)", d.Reminders)
	}

	b := findEvent(t, p, "7E8F9A0B-1C2D-3E4F-5A6B-7C8D9E0F1A2B", false)
	check(t, "birthday Rrule", b.Recurrence.Rrule, "FREQ=YEARLY;UNTIL=20300414")
	check(t, "birthday Title", b.Title, "Mum's birthday")
	got, _, err := Expand(b, nil, mustTime(t, "2026-01-01T00:00:00Z"), mustTime(t, "2035-01-01T00:00:00Z"), 0)
	if err != nil {
		t.Fatal(err)
	}
	// UNTIL=20300414 ends the series before the 2030 birthday.
	if len(got) != 4 {
		t.Errorf("birthday instances %v", startsOf(got))
	}
	if !reflect.DeepEqual(b.Reminders, []csil.Reminder{{MinutesBefore: 900, Method: ptr(csil.ReminderMethod("display"))}}) {
		t.Errorf("birthday Reminders = %+v", b.Reminders)
	}
}

func TestParseOutlook(t *testing.T) {
	p := parseFixture(t, "outlook.ics")
	if len(p.Events) != 4 {
		t.Fatalf("got %d events", len(p.Events))
	}
	if len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0], "Custom Zone Nobody Knows") {
		t.Errorf("warnings: %v", p.Warnings)
	}
	m := p.Events[0]
	check(t, "Title", m.Title, "Monthly review")
	check(t, "Uid", str(m.Uid), "040000008200E00074C5B7101A82E00800000000A0B1C2D3E4F5A601000000000000000010000000F00E1D2C3B4A59687")
	check(t, "Timezone", str(m.Timezone), "America/Denver")
	check(t, "Start", str(m.Start), "2026-10-30T15:00:00Z")
	check(t, "Rrule", m.Recurrence.Rrule, "FREQ=MONTHLY;UNTIL=20270331T150000Z;INTERVAL=1;BYDAY=-1FR")
	check(t, "Description", str(m.Description), "Agenda:\n1. Numbers\n2. Risks; blockers\n3. Next steps, owners and dates. This line is long on purpose so that Outlook folds it over more than one physical line in the file.\n")
	check(t, "Location", str(m.Location), "Conf Room 4")
	check(t, "Visibility", str(m.Visibility), "public")
	check(t, "ConferenceUrl", str(m.ConferenceUrl), "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc")
	if len(m.Attendees) != 2 {
		t.Fatalf("attendees %+v", m.Attendees)
	}
	check(t, "sam email", str(m.Attendees[0].Email), "sam@contoso.com")
	org := m.Attendees[1]
	check(t, "organizer email", str(org.Email), "jane@contoso.com")
	check(t, "organizer name", str(org.Name), "Doe, Jane")
	check(t, "organizer role", str(org.Role), "chair")
	check(t, "organizer status", str(org.Status), "accepted")
	if !reflect.DeepEqual(m.Reminders, []csil.Reminder{{MinutesBefore: 15, Method: ptr(csil.ReminderMethod("display"))}}) {
		t.Errorf("Reminders = %+v", m.Reminders)
	}
	// Last Friday of each month, 09:00 Denver, across the DST change.
	got, _, err := Expand(m, nil, mustTime(t, "2026-10-01T00:00:00Z"), mustTime(t, "2027-01-01T00:00:00Z"), 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-10-30T15:00:00Z", "2026-11-27T16:00:00Z", "2026-12-25T16:00:00Z"}
	if gs := startsOf(got); !reflect.DeepEqual(gs, want) {
		t.Errorf("expanded %v, want %v", gs, want)
	}

	pst := findEvent(t, p, "pst-single-1", false)
	check(t, "pst Timezone", str(pst.Timezone), "America/Los_Angeles")
	check(t, "pst Start", str(pst.Start), "2026-11-02T21:00:00Z")
	check(t, "pst End (DURATION)", str(pst.End), "2026-11-02T21:45:00Z")
	check(t, "pst Status", str(pst.Status), "tentative")

	cz := findEvent(t, p, "custom-zone-1", false)
	if cz.Timezone != nil {
		t.Errorf("unknown zone Timezone = %q", *cz.Timezone)
	}
	check(t, "custom zone Start (fixed +0530)", str(cz.Start), "2026-11-02T07:30:00Z")

	dz := findEvent(t, p, "display-name-zone-1", false)
	check(t, "display zone", str(dz.Timezone), "America/Denver")
	check(t, "display zone Start", str(dz.Start), "2026-11-02T15:00:00Z")
}

func TestParseMisc(t *testing.T) {
	p := parseFixture(t, "misc.ics")
	check(t, "CalendarName", p.CalendarName, "Misc, things")
	if len(p.Events) != 4 {
		t.Fatalf("got %d events", len(p.Events))
	}
	var todo, nostart bool
	for _, w := range p.Warnings {
		todo = todo || strings.Contains(w, "VTODO") && strings.Contains(w, "todo-1")
		nostart = nostart || strings.Contains(w, "no-start-1") && strings.Contains(w, "DTSTART")
	}
	if !todo || !nostart {
		t.Errorf("warnings: %v", p.Warnings)
	}

	f := findEvent(t, p, "floating-1", false)
	check(t, "floating Timezone (X-WR-TIMEZONE)", str(f.Timezone), "America/New_York")
	check(t, "floating Start", str(f.Start), "2026-11-16T19:00:00Z")
	check(t, "floating End (DURATION)", str(f.End), "2026-11-16T20:30:00Z")
	check(t, "Description", str(f.Description), "Line one\nLine two, with comma; and semicolon\\backslash. This text is folded across lines and has UTF-8: café crème brûlée — ok.")
	check(t, "Location", str(f.Location), "Room 1, Floor 2")
	if !reflect.DeepEqual(f.Categories, []string{"Work", "Planning, long-term", "Extra"}) {
		t.Errorf("Categories = %q", f.Categories)
	}
	check(t, "Color", str(f.Color), "turquoise")

	a := findEvent(t, p, "allday-duration-1", false)
	check(t, "all-day DURATION EndDate", str(a.EndDate), "2026-12-04")
	check(t, "Status", str(a.Status), "cancelled")
	check(t, "Visibility", str(a.Visibility), "confidential")

	id := findEvent(t, p, "identity-1", false)
	if len(id.Attendees) != 3 {
		t.Fatalf("attendees %+v", id.Attendees)
	}
	dana := id.Attendees[0]
	if dana.Identity == nil || dana.Identity.Domain != "id.example.org" || str(dana.Identity.UserId) != "dana123" || dana.Email != nil {
		t.Errorf("dana = %+v", dana)
	}
	eve := id.Attendees[1]
	if str(eve.Email) != "eve@example.net" || eve.Identity == nil || str(eve.Identity.UserId) != "eve9" {
		t.Errorf("eve = %+v", eve)
	}
	if eve.Rsvp == nil || *eve.Rsvp {
		t.Errorf("eve rsvp = %v", eve.Rsvp)
	}
	check(t, "eve status", str(eve.Status), "declined")
	owner := id.Attendees[2]
	if owner.Identity == nil || str(owner.Identity.UserId) != "owner42" || str(owner.Role) != "chair" {
		t.Errorf("owner = %+v", owner)
	}
	var skipped bool
	for _, w := range p.Warnings {
		skipped = skipped || strings.Contains(w, "urn:uuid:1234")
	}
	if !skipped {
		t.Errorf("no warning for urn attendee: %v", p.Warnings)
	}
	// RELATED=END -PT90M on a one-hour event is 30 minutes before start.
	// PT5M after start is dropped.
	if !reflect.DeepEqual(id.Reminders, []csil.Reminder{{MinutesBefore: 30, Method: ptr(csil.ReminderMethod("display"))}}) {
		t.Errorf("Reminders = %+v", id.Reminders)
	}

	ns := findEvent(t, p, "no-summary-1", false)
	check(t, "no summary Title", ns.Title, UntitledTitle)
	if ns.End == nil || *ns.End != *ns.Start {
		t.Errorf("no DTEND: End = %v", str(ns.End))
	}
}

func TestParseFloatingWithoutCalendarZone(t *testing.T) {
	p := parseFixture(t, "floating_utc.ics")
	ev := p.Events[0]
	if ev.Timezone != nil {
		t.Errorf("Timezone = %q", *ev.Timezone)
	}
	check(t, "Start", str(ev.Start), "2026-11-16T14:00:00Z")
	if !reflect.DeepEqual(ev.Recurrence.Exdates, []csil.Timestamp{"2026-11-17T14:00:00Z"}) {
		t.Errorf("Exdates = %v", ev.Recurrence.Exdates)
	}
}

func TestParseErrors(t *testing.T) {
	bad := map[string]string{
		"empty":        "",
		"not calendar": "BEGIN:VEVENT\r\nEND:VEVENT\r\n",
		"no colon":     "BEGIN:VCALENDAR\r\nGARBAGE\r\nEND:VCALENDAR\r\n",
		"unterminated": "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n",
		"bad param":    "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nATTENDEE;CN=\"x\"y:mailto:a@b\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
	}
	for name, data := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(data)); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestParseRDatePeriodAndDateExdate(t *testing.T) {
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:x\r\nBEGIN:VEVENT\r\nUID:p1\r\n" +
		"DTSTART;TZID=America/Denver:20260105T090000\r\nDTEND;TZID=America/Denver:20260105T100000\r\n" +
		"RRULE:FREQ=DAILY;COUNT=3\r\n" +
		"RDATE;VALUE=PERIOD:20260110T170000Z/20260110T180000Z,20260111T170000Z/PT1H\r\n" +
		"EXDATE;VALUE=DATE:20260106\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	p, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	ev := p.Events[0]
	if !reflect.DeepEqual(ev.Recurrence.Rdates, []csil.Timestamp{"2026-01-10T17:00:00Z", "2026-01-11T17:00:00Z"}) {
		t.Errorf("Rdates = %v", ev.Recurrence.Rdates)
	}
	// A DATE EXDATE on a timed series takes the DTSTART clock time.
	if !reflect.DeepEqual(ev.Recurrence.Exdates, []csil.Timestamp{"2026-01-06T16:00:00Z"}) {
		t.Errorf("Exdates = %v", ev.Recurrence.Exdates)
	}
}

func TestResolveZoneName(t *testing.T) {
	tests := map[string]string{
		"America/Denver":                           "America/Denver",
		"Mountain Standard Time":                   "America/Denver",
		"pacific standard time":                    "America/Los_Angeles",
		"W. Europe Standard Time":                  "Europe/Berlin",
		"(UTC-07:00) Mountain Time (US & Canada)":  "America/Denver",
		"/mozilla.org/20050126_1/America/New_York": "America/New_York",
		"/citadel.org/20190914_1/Europe/Paris":     "Europe/Paris",
		"UTC":                                      "UTC",
		"Z":                                        "UTC",
		"Local":                                    "",
		"Nowhere Standard Time":                    "",
	}
	for in, want := range tests {
		if got := resolveZoneName(in); got != want {
			t.Errorf("resolveZoneName(%q) = %q, want %q", in, got, want)
		}
	}
	// Every mapped IANA name must load.
	for k, v := range windowsZones {
		if resolveZoneName(v) == "" {
			t.Errorf("windows zone %q maps to %q, which does not load", k, v)
		}
	}
	for k, v := range windowsDisplayZones {
		if resolveZoneName(v) == "" {
			t.Errorf("display zone %q maps to %q, which does not load", k, v)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := map[string]int64{ // minutes
		"-PT15M":       -15,
		"-P0DT0H15M0S": -15,
		"-P1D":         -1440,
		"-P1W":         -10080,
		"PT0S":         0,
		"P1DT2H":       1560,
		"+PT1H":        60,
	}
	for in, want := range tests {
		d, err := parseDuration(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := int64(d.approx().Minutes()); got != want {
			t.Errorf("%q = %d min, want %d", in, got, want)
		}
	}
	for _, in := range []string{"", "P", "PT", "15M", "PT15", "P1H", "PTD"} {
		if _, err := parseDuration(in); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
}
