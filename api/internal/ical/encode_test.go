package ical

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	goical "github.com/emersion/go-ical"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

var fixedNow = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func richEvents() []csil.Event {
	status := func(s string) *csil.EventStatus { v := csil.EventStatus(s); return &v }
	transp := func(s string) *csil.EventTransparency { v := csil.EventTransparency(s); return &v }
	vis := func(s string) *csil.EventVisibility { v := csil.EventVisibility(s); return &v }
	role := func(s string) *csil.AttendeeRole { v := csil.AttendeeRole(s); return &v }
	part := func(s string) *csil.ParticipationStatus { v := csil.ParticipationStatus(s); return &v }
	method := func(s string) *csil.ReminderMethod { v := csil.ReminderMethod(s); return &v }
	longText := strings.Repeat("Long description with commas, semicolons; backslash \\ and newline\nand UTF-8 é ü 日本語 ", 6)

	master := csil.Event{
		Uid: ptr("series-1@catalystcal"), Title: "Planning, weekly; team", Description: ptr(longText),
		Location: ptr("Room 4, Floor 2"), Url: ptr("https://example.com/plan?a=1,2;b=3"),
		ConferenceUrl: ptr("https://meet.example.com/abc"),
		Start:         tsp("2026-10-05T15:00:00Z"), End: tsp("2026-10-05T16:00:00Z"), AllDay: ptr(false),
		Timezone: ptr("America/Denver"), Status: status("confirmed"), Transparency: transp("opaque"), Visibility: vis("default"),
		Recurrence: &csil.Recurrence{
			Rrule:   "FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20261231T235959Z",
			Exdates: []csil.Timestamp{"2026-10-12T15:00:00Z", "2026-11-04T16:00:00Z"},
			Rdates:  []csil.Timestamp{"2026-10-10T16:00:00Z"},
		},
		Attendees: []csil.Attendee{
			{Email: ptr("alice@example.com"), Name: ptr("Alice \"Al\" Example"), Role: role("chair"), Status: part("accepted"), Rsvp: ptr(false)},
			{Identity: &csil.IdentityRef{Domain: "id.example.org", UserId: ptr("bob42")}, Name: ptr("Bob"), Role: role("required"), Status: part("needs_action"), Rsvp: ptr(true)},
			{Email: ptr("carol@example.com"), Identity: &csil.IdentityRef{Domain: "id.example.org", UserId: ptr("carol7")}, Role: role("optional"), Status: part("tentative"), Rsvp: ptr(true)},
			{Email: ptr("dan@example.com"), Role: role("non_participant"), Status: part("declined"), Rsvp: ptr(false)},
		},
		Reminders: []csil.Reminder{
			{MinutesBefore: 10, Method: method("display")},
			{MinutesBefore: 60, Method: method("email")},
			{MinutesBefore: 2880, Method: method("display")},
			{MinutesBefore: 0, Method: method("display")},
		},
		Categories: []string{"Work", "Planning, long-term"},
		Color:      ptr("crimson"),
		Sequence:   4,
	}
	moved := csil.Event{
		Uid: ptr("series-1@catalystcal"), Title: "Planning (moved)", RecurrenceId: tsp("2026-10-07T15:00:00Z"),
		Start: tsp("2026-10-07T17:00:00Z"), End: tsp("2026-10-07T18:00:00Z"), AllDay: ptr(false),
		Timezone: ptr("America/Denver"), Status: status("confirmed"), Transparency: transp("opaque"), Visibility: vis("default"),
		Sequence: 5,
	}
	cancelled := csil.Event{
		Uid: ptr("series-1@catalystcal"), Title: "Planning, weekly; team", RecurrenceId: tsp("2026-10-14T15:00:00Z"),
		Start: tsp("2026-10-14T15:00:00Z"), End: tsp("2026-10-14T16:00:00Z"), AllDay: ptr(false),
		Timezone: ptr("America/Denver"), Status: status("cancelled"), Transparency: transp("opaque"), Visibility: vis("default"),
		Sequence: 5,
	}
	allDay := csil.Event{
		Uid: ptr("holiday-1@catalystcal"), Title: "Holiday", AllDay: ptr(true),
		StartDate: dtp("2026-12-24"), EndDate: dtp("2026-12-26"),
		Start: tsp("2026-12-24T00:00:00Z"), End: tsp("2026-12-26T00:00:00Z"),
		Status: status("tentative"), Transparency: transp("transparent"), Visibility: vis("public"),
		Recurrence: &csil.Recurrence{Rrule: "FREQ=YEARLY;UNTIL=20301224", Exdates: []csil.Timestamp{"2027-12-24T00:00:00Z"}},
	}
	allDayOverride := csil.Event{
		Uid: ptr("holiday-1@catalystcal"), Title: "Holiday (short)", AllDay: ptr(true), RecurrenceId: tsp("2028-12-24T00:00:00Z"),
		StartDate: dtp("2028-12-24"), EndDate: dtp("2028-12-25"),
		Start: tsp("2028-12-24T00:00:00Z"), End: tsp("2028-12-25T00:00:00Z"),
		Status: status("confirmed"), Transparency: transp("transparent"), Visibility: vis("default"),
	}
	utc := csil.Event{
		Uid: ptr("utc-1@catalystcal"), Title: "UTC call", Start: tsp("2026-11-01T08:30:00Z"), End: tsp("2026-11-01T09:00:00Z"),
		AllDay: ptr(false), Status: status("confirmed"), Transparency: transp("opaque"), Visibility: vis("private"),
		Recurrence: &csil.Recurrence{Rrule: "FREQ=DAILY;COUNT=5;INTERVAL=2"},
	}
	berlin := csil.Event{
		Uid: ptr("berlin-1@catalystcal"), Title: "Berlin", Start: tsp("2026-03-29T08:00:00Z"), End: tsp("2026-03-29T09:00:00Z"),
		AllDay: ptr(false), Timezone: ptr("Europe/Berlin"), Status: status("confirmed"), Transparency: transp("opaque"),
		Visibility: vis("confidential"),
	}
	return []csil.Event{master, moved, cancelled, allDay, allDayOverride, utc, berlin}
}

func encodeRich(t *testing.T, opts EncodeOptions) []byte {
	t.Helper()
	if opts.Now.IsZero() {
		opts.Now = fixedNow
	}
	out, err := Encode(richEvents(), opts)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return out
}

func TestEncodeFormat(t *testing.T) {
	out := encodeRich(t, EncodeOptions{CalendarName: "Team, cal", Method: "publish"})
	s := string(out)
	if !strings.HasSuffix(s, "END:VCALENDAR\r\n") || !strings.HasPrefix(s, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//catalystcal//catalystcal calendar//EN\r\n") {
		t.Fatalf("bad envelope:\n%s", s)
	}
	for i, line := range strings.Split(strings.TrimSuffix(s, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line %d is %d octets: %q", i, len(line), line)
		}
		if strings.Contains(line, "\n") || strings.Contains(line, "\r") {
			t.Errorf("line %d has a bare CR or LF", i)
		}
	}
	// No folded line starts inside a UTF-8 character.
	for _, line := range strings.Split(s, "\r\n ") {
		if line != "" && !strings.HasPrefix(line, "BEGIN") && line[0]&0xC0 == 0x80 {
			t.Errorf("fold cut a UTF-8 character: %q", line[:10])
		}
	}
	unfolded := strings.ReplaceAll(s, "\r\n ", "")
	for _, want := range []string{
		"METHOD:PUBLISH\r\n",
		"X-WR-CALNAME:Team\\, cal\r\n",
		"DTSTAMP:20261015T120000Z\r\n",
		"UID:series-1@catalystcal\r\n",
		"SEQUENCE:4\r\n",
		"DTSTART;TZID=America/Denver:20261005T090000\r\n",
		"DTEND;TZID=America/Denver:20261005T100000\r\n",
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20261231T235959Z\r\n",
		"EXDATE;TZID=America/Denver:20261012T090000,20261104T090000\r\n",
		"RDATE;TZID=America/Denver:20261010T100000\r\n",
		"RECURRENCE-ID;TZID=America/Denver:20261007T090000\r\n",
		"SUMMARY:Planning\\, weekly\\; team\r\n",
		"LOCATION:Room 4\\, Floor 2\r\n",
		"URL;VALUE=URI:https://example.com/plan?a=1,2;b=3\r\n",
		"CONFERENCE;VALUE=URI;FEATURE=VIDEO:https://meet.example.com/abc\r\n",
		"CATEGORIES:Work,Planning\\, long-term\r\n",
		"ORGANIZER;CN=Alice 'Al' Example:mailto:alice@example.com\r\n",
		"ATTENDEE;CN=Bob;CUTYPE=INDIVIDUAL;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:linkkeys:bob42@id.example.org\r\n",
		"X-CATALYSTCAL-IDENTITY=\"linkkeys:carol7@id.example.org\":mailto:carol@example.com\r\n",
		"TRIGGER:-PT10M\r\n", "TRIGGER:-PT1H\r\n", "TRIGGER:-P2D\r\n", "TRIGGER:PT0S\r\n",
		"ACTION:EMAIL\r\n",
		"DTSTART;VALUE=DATE:20261224\r\n",
		"DTEND;VALUE=DATE:20261226\r\n",
		"RRULE:FREQ=YEARLY;UNTIL=20301224\r\n",
		"EXDATE;VALUE=DATE:20271224\r\n",
		"RECURRENCE-ID;VALUE=DATE:20281224\r\n",
		"DTSTART:20261101T083000Z\r\n",
		"CLASS:PRIVATE\r\n", "CLASS:CONFIDENTIAL\r\n", "CLASS:PUBLIC\r\n",
		"STATUS:CANCELLED\r\n", "STATUS:TENTATIVE\r\n", "TRANSP:TRANSPARENT\r\n",
		"DESCRIPTION:Long description with commas\\, semicolons\\; backslash \\\\ and newline\\nand UTF-8 é",
		"BEGIN:VTIMEZONE\r\nTZID:America/Denver\r\n",
		"BEGIN:VTIMEZONE\r\nTZID:Europe/Berlin\r\n",
		"RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU\r\n",
		"RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU\r\n",
		"RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU\r\n",
		"RRULE:FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU\r\n",
		"TZOFFSETFROM:-0700\r\nTZOFFSETTO:-0600\r\nTZNAME:MDT\r\n",
	} {
		if !strings.Contains(unfolded, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	if strings.Contains(unfolded, "TZID:UTC") {
		t.Errorf("UTC must not get a VTIMEZONE")
	}
	if n := strings.Count(unfolded, "BEGIN:VEVENT"); n != 7 {
		t.Errorf("%d VEVENTs", n)
	}
	// The go-ical decoder reads it, and its strict encoder checks accept it.
	cal, err := goical.NewDecoder(bytes.NewReader(out)).Decode()
	if err != nil {
		t.Fatalf("go-ical decode: %v", err)
	}
	var buf bytes.Buffer
	if err := goical.NewEncoder(&buf).Encode(cal); err != nil {
		t.Fatalf("go-ical validation: %v", err)
	}
}

func TestEncodeVTimezoneMatchesGo(t *testing.T) {
	// Check the generated VTIMEZONE: compute each change from its RRULE
	// and compare with the Go tz database.
	for _, zone := range []string{"America/Denver", "Europe/Berlin", "Australia/Sydney", "Asia/Tokyo", "America/Sao_Paulo"} {
		t.Run(zone, func(t *testing.T) {
			loc, _ := time.LoadLocation(zone)
			w := &lineWriter{}
			writeVTimezone(w, zone, loc, 2005, 2027)
			cal, err := goical.NewDecoder(strings.NewReader("BEGIN:VCALENDAR\r\n" + w.b.String() + "END:VCALENDAR\r\n")).Decode()
			if err != nil {
				t.Fatal(err)
			}
			vtz := cal.Children[0]
			if len(vtz.Children) == 0 {
				t.Fatal("no observances")
			}
			// Each real change from 2005 to 2030 must be an onset of an
			// observance with the right offsets.
			onsets := map[int64]int{}
			for _, sub := range vtz.Children {
				from, _ := parseOffset(sub.Props.Get("TZOFFSETFROM").Value)
				to, _ := parseOffset(sub.Props.Get("TZOFFSETTO").Value)
				start, _ := time.Parse("20060102T150405", sub.Props.Get("DTSTART").Value)
				add := func(local time.Time) { onsets[local.Add(-time.Duration(from)*time.Second).Unix()] = to }
				add(start)
				if rr := sub.Props.Get("RRULE"); rr != nil {
					spec, err := parseRuleSpec(rr.Value)
					if err != nil {
						t.Fatal(err)
					}
					o := spec.options(start, false)
					if !o.Until.IsZero() {
						o.Until = o.Until.Add(time.Duration(from) * time.Second)
					}
					s := &series{t: timing{loc: time.UTC, start: start, end: start}, opt: &o, ex: map[int64]bool{}}
					got, _ := s.startsBetween(start, time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC), true, 1000)
					for _, g := range got {
						add(g)
					}
				}
				if rd := sub.Props.Get("RDATE"); rd != nil {
					for _, v := range strings.Split(rd.Value, ",") {
						l, _ := time.Parse("20060102T150405", v)
						add(l)
					}
				}
			}
			for _, z := range zoneTransitions(loc, 2005, 2030) {
				to, ok := onsets[z.at.Unix()]
				if !ok || to != z.to {
					t.Errorf("change at %v to %d: VTIMEZONE has %v %d", z.at, z.to, ok, to)
				}
			}
		})
	}
}

func TestEncodeBusyOnly(t *testing.T) {
	out := encodeRich(t, EncodeOptions{BusyOnly: true})
	s := strings.ReplaceAll(string(out), "\r\n ", "")
	for _, bad := range []string{"Planning", "DESCRIPTION", "LOCATION", "ATTENDEE", "ORGANIZER", "VALARM", "CATEGORIES", "CONFERENCE", "URL", "COLOR", "Holiday", "CLASS"} {
		if strings.Contains(s, bad) {
			t.Errorf("busy output has %q", bad)
		}
	}
	for _, want := range []string{"SUMMARY:Busy\r\n", "RRULE:FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20261231T235959Z\r\n", "EXDATE;TZID=America/Denver", "RECURRENCE-ID", "DTSTART;TZID=America/Denver:20261005T090000"} {
		if !strings.Contains(s, want) {
			t.Errorf("busy output lacks %q", want)
		}
	}
	p, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Events) != 7 || p.Events[0].Title != "Busy" || p.Events[0].Start == nil {
		t.Errorf("busy events: %+v", p.Events)
	}
}

func TestEncodeErrors(t *testing.T) {
	if _, err := Encode([]csil.Event{{Title: "no start"}}, EncodeOptions{}); err == nil {
		t.Error("want error for event with no start")
	}
	ev := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "")
	if _, err := Encode([]csil.Event{ev}, EncodeOptions{Method: "DESTROY"}); err == nil {
		t.Error("want error for bad method")
	}
	ev.Timezone = ptr("Mars/Olympus")
	if _, err := Encode([]csil.Event{ev}, EncodeOptions{}); err == nil {
		t.Error("want error for bad zone")
	}
}

func TestEncodeDefaults(t *testing.T) {
	ev := csil.Event{Title: "x", EventId: "ev123", Start: tsp("2026-01-05T10:00:00Z"), End: tsp("2026-01-05T11:00:00Z"),
		Organizer: &csil.PersonSummary{Domain: "id.example.org", Subject: "owner1", DisplayName: ptr("Owner")}}
	before := time.Now().UTC().Add(-time.Second)
	out, err := Encode([]csil.Event{ev}, EncodeOptions{ProdID: "-//test//EN"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"PRODID:-//test//EN\r\n", "UID:ev123@catalystcal\r\n", "ORGANIZER;CN=Owner:linkkeys:owner1@id.example.org\r\n", "STATUS:CONFIRMED\r\n"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	i := strings.Index(s, "DTSTAMP:")
	stamp, err := time.Parse("20060102T150405Z", s[i+8:i+24])
	if err != nil || stamp.Before(before.Truncate(time.Second)) {
		t.Errorf("DTSTAMP %v not from the clock", stamp)
	}
	p, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	a := p.Events[0].Attendees
	if len(a) != 1 || str(a[0].Role) != "chair" || a[0].Identity == nil || str(a[0].Identity.UserId) != "owner1" {
		t.Errorf("organizer attendee: %+v", a)
	}
}

// comparable copies the fields that Encode and Parse both carry.
func comparable(ev csil.Event) csil.Event {
	return csil.Event{
		Uid: ev.Uid, Title: ev.Title, Description: ev.Description, Location: ev.Location, Url: ev.Url,
		ConferenceUrl: ev.ConferenceUrl, Start: ev.Start, End: ev.End, AllDay: ev.AllDay,
		StartDate: ev.StartDate, EndDate: ev.EndDate, Timezone: ev.Timezone, Status: ev.Status,
		Transparency: ev.Transparency, Visibility: ev.Visibility, Recurrence: ev.Recurrence,
		RecurrenceId: ev.RecurrenceId, Attendees: ev.Attendees, Reminders: ev.Reminders,
		Categories: ev.Categories, Color: ev.Color, Sequence: ev.Sequence,
	}
}

func TestEncodeParseRoundTrip(t *testing.T) {
	in := richEvents()
	out := encodeRich(t, EncodeOptions{CalendarName: "Round trip"})
	p, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings: %v", p.Warnings)
	}
	if p.CalendarName != "Round trip" {
		t.Errorf("CalendarName = %q", p.CalendarName)
	}
	if len(p.Events) != len(in) {
		t.Fatalf("got %d events, want %d", len(p.Events), len(in))
	}
	// RFC 5545 has no escape for '"' in a parameter, so CN loses it.
	in[0].Attendees[0].Name = ptr("Alice 'Al' Example")
	for i := range in {
		want, got := comparable(in[i]), comparable(p.Events[i])
		if !reflect.DeepEqual(want, got) {
			t.Errorf("event %d (%s) differs:\n got %s\nwant %s", i, in[i].Title, dump(got), dump(want))
		}
	}
	// A second encode of the parsed events is the same bytes.
	again, err := Encode(p.Events, EncodeOptions{CalendarName: "Round trip", Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, out) {
		t.Errorf("second encode differs")
	}
}

func dump(ev csil.Event) string {
	var sb strings.Builder
	v := reflect.ValueOf(ev)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.IsZero() {
			continue
		}
		sb.WriteString(v.Type().Field(i).Name + "=")
		sb.WriteString(deref(f))
		sb.WriteString(" ")
	}
	return sb.String()
}

func deref(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return "nil"
		}
		return "&" + deref(v.Elem())
	case reflect.Slice:
		parts := []string{}
		for i := 0; i < v.Len(); i++ {
			parts = append(parts, deref(v.Index(i)))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case reflect.Struct:
		parts := []string{}
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).IsZero() {
				parts = append(parts, v.Type().Field(i).Name+":"+deref(v.Field(i)))
			}
		}
		return "{" + strings.Join(parts, " ") + "}"
	}
	return strings.ReplaceAll(fmt.Sprint(v.Interface()), "\n", `\n`)
}
