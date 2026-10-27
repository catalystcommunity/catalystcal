package ical

import (
	"reflect"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func tsp(s string) *csil.Timestamp { t := csil.Timestamp(s); return &t }
func dtp(s string) *csil.DateText  { d := csil.DateText(s); return &d }

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad time %q: %v", s, err)
	}
	return v
}

func timedEvent(start, end, tz, rule string) csil.Event {
	ev := csil.Event{Uid: ptr("uid-1"), Title: "Meeting", Start: tsp(start), End: tsp(end)}
	if tz != "" {
		ev.Timezone = ptr(tz)
	}
	if rule != "" {
		ev.Recurrence = &csil.Recurrence{Rrule: rule}
	}
	return ev
}

func allDayEvent(sd, ed, rule string) csil.Event {
	ev := csil.Event{Uid: ptr("uid-ad"), Title: "Holiday", AllDay: ptr(true), StartDate: dtp(sd), EndDate: dtp(ed),
		Start: tsp(sd + "T00:00:00Z"), End: tsp(ed + "T00:00:00Z")}
	if rule != "" {
		ev.Recurrence = &csil.Recurrence{Rrule: rule}
	}
	return ev
}

func override(rid, start, end, title string) csil.Event {
	return csil.Event{Uid: ptr("uid-1"), Title: title, RecurrenceId: tsp(rid), Start: tsp(start), End: tsp(end)}
}

func startsOf(in []Instance) []string {
	out := []string{}
	for _, i := range in {
		out = append(out, i.Start.UTC().Format(TimeLayout))
	}
	return out
}

func TestExpand(t *testing.T) {
	withEx := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=5")
	withEx.Recurrence.Exdates = []csil.Timestamp{"2026-01-06T10:00:00Z", "2026-01-08T10:00:00Z"}
	withRd := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=2")
	withRd.Recurrence.Rdates = []csil.Timestamp{"2026-01-20T15:00:00Z", "2026-01-06T10:00:00Z"}
	onlyRd := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "")
	onlyRd.Recurrence = &csil.Recurrence{Rdates: []csil.Timestamp{"2026-01-07T10:00:00Z"}}

	tests := []struct {
		name      string
		ev        csil.Event
		overrides []csil.Event
		rs, re    string
		max       int
		want      []string
		trunc     bool
	}{
		{name: "single inside", ev: timedEvent("2026-10-15T15:00:00Z", "2026-10-15T16:00:00Z", "", ""),
			rs: "2026-10-15T00:00:00Z", re: "2026-10-16T00:00:00Z", want: []string{"2026-10-15T15:00:00Z"}},
		{name: "single outside", ev: timedEvent("2026-10-15T15:00:00Z", "2026-10-15T16:00:00Z", "", ""),
			rs: "2026-10-16T00:00:00Z", re: "2026-10-17T00:00:00Z", want: []string{}},
		{name: "single starts before range ends inside", ev: timedEvent("2026-10-15T09:00:00Z", "2026-10-15T11:00:00Z", "", ""),
			rs: "2026-10-15T10:00:00Z", re: "2026-10-15T12:00:00Z", want: []string{"2026-10-15T09:00:00Z"}},
		{name: "single ends at range start", ev: timedEvent("2026-10-15T09:00:00Z", "2026-10-15T10:00:00Z", "", ""),
			rs: "2026-10-15T10:00:00Z", re: "2026-10-15T12:00:00Z", want: []string{}},
		{name: "single starts at range end", ev: timedEvent("2026-10-15T12:00:00Z", "2026-10-15T13:00:00Z", "", ""),
			rs: "2026-10-15T10:00:00Z", re: "2026-10-15T12:00:00Z", want: []string{}},
		{name: "zero length at range start", ev: timedEvent("2026-10-15T10:00:00Z", "2026-10-15T10:00:00Z", "", ""),
			rs: "2026-10-15T10:00:00Z", re: "2026-10-15T12:00:00Z", want: []string{"2026-10-15T10:00:00Z"}},
		{name: "daily count", ev: timedEvent("2026-01-01T10:00:00Z", "2026-01-01T11:00:00Z", "", "FREQ=DAILY;COUNT=3"),
			rs: "2025-01-01T00:00:00Z", re: "2027-01-01T00:00:00Z",
			want: []string{"2026-01-01T10:00:00Z", "2026-01-02T10:00:00Z", "2026-01-03T10:00:00Z"}},
		{name: "daily interval", ev: timedEvent("2026-01-01T10:00:00Z", "2026-01-01T11:00:00Z", "", "FREQ=DAILY;INTERVAL=2;COUNT=3"),
			rs: "2025-01-01T00:00:00Z", re: "2027-01-01T00:00:00Z",
			want: []string{"2026-01-01T10:00:00Z", "2026-01-03T10:00:00Z", "2026-01-05T10:00:00Z"}},
		{name: "daily instance overlapping range start", ev: timedEvent("2026-01-01T23:00:00Z", "2026-01-02T01:00:00Z", "", "FREQ=DAILY"),
			rs: "2026-01-03T00:00:00Z", re: "2026-01-04T00:00:00Z",
			want: []string{"2026-01-02T23:00:00Z", "2026-01-03T23:00:00Z"}},
		{name: "weekly byday until", ev: timedEvent("2026-01-05T15:00:00Z", "2026-01-05T16:00:00Z", "", "FREQ=WEEKLY;BYDAY=MO,WE,FR;UNTIL=20260116T235959Z"),
			rs: "2026-01-01T00:00:00Z", re: "2026-03-01T00:00:00Z",
			want: []string{"2026-01-05T15:00:00Z", "2026-01-07T15:00:00Z", "2026-01-09T15:00:00Z",
				"2026-01-12T15:00:00Z", "2026-01-14T15:00:00Z", "2026-01-16T15:00:00Z"}},
		{name: "weekly interval 2", ev: timedEvent("2026-01-05T15:00:00Z", "2026-01-05T16:00:00Z", "", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO;COUNT=3"),
			rs: "2026-01-01T00:00:00Z", re: "2026-03-01T00:00:00Z",
			want: []string{"2026-01-05T15:00:00Z", "2026-01-19T15:00:00Z", "2026-02-02T15:00:00Z"}},
		{name: "monthly bymonthday", ev: timedEvent("2026-01-15T12:00:00Z", "2026-01-15T13:00:00Z", "", "FREQ=MONTHLY;BYMONTHDAY=15;COUNT=3"),
			rs: "2026-01-01T00:00:00Z", re: "2027-01-01T00:00:00Z",
			want: []string{"2026-01-15T12:00:00Z", "2026-02-15T12:00:00Z", "2026-03-15T12:00:00Z"}},
		{name: "monthly last friday bysetpos", ev: timedEvent("2026-01-30T12:00:00Z", "2026-01-30T13:00:00Z", "", "FREQ=MONTHLY;BYDAY=FR;BYSETPOS=-1;COUNT=3"),
			rs: "2026-01-01T00:00:00Z", re: "2027-01-01T00:00:00Z",
			want: []string{"2026-01-30T12:00:00Z", "2026-02-27T12:00:00Z", "2026-03-27T12:00:00Z"}},
		{name: "monthly last friday ordinal", ev: timedEvent("2026-01-30T12:00:00Z", "2026-01-30T13:00:00Z", "", "FREQ=MONTHLY;BYDAY=-1FR;COUNT=3"),
			rs: "2026-01-01T00:00:00Z", re: "2027-01-01T00:00:00Z",
			want: []string{"2026-01-30T12:00:00Z", "2026-02-27T12:00:00Z", "2026-03-27T12:00:00Z"}},
		{name: "monthly 31st skips short months", ev: timedEvent("2026-01-31T12:00:00Z", "2026-01-31T13:00:00Z", "", "FREQ=MONTHLY;COUNT=4"),
			rs: "2026-01-01T00:00:00Z", re: "2027-01-01T00:00:00Z",
			want: []string{"2026-01-31T12:00:00Z", "2026-03-31T12:00:00Z", "2026-05-31T12:00:00Z", "2026-07-31T12:00:00Z"}},
		{name: "yearly", ev: timedEvent("2026-03-10T12:00:00Z", "2026-03-10T13:00:00Z", "", "FREQ=YEARLY"),
			rs: "2026-01-01T00:00:00Z", re: "2029-01-01T00:00:00Z",
			want: []string{"2026-03-10T12:00:00Z", "2027-03-10T12:00:00Z", "2028-03-10T12:00:00Z"}},
		{name: "yearly feb 29 only leap years", ev: timedEvent("2024-02-29T12:00:00Z", "2024-02-29T13:00:00Z", "", "FREQ=YEARLY;COUNT=2"),
			rs: "2024-01-01T00:00:00Z", re: "2040-01-01T00:00:00Z",
			want: []string{"2024-02-29T12:00:00Z", "2028-02-29T12:00:00Z"}},
		{name: "feb 30 gives only dtstart", ev: timedEvent("2026-01-30T12:00:00Z", "2026-01-30T13:00:00Z", "", "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30"),
			rs: "2026-01-01T00:00:00Z", re: "2030-01-01T00:00:00Z",
			want: []string{"2026-01-30T12:00:00Z"}},
		{name: "exdate", ev: withEx, rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-05T10:00:00Z", "2026-01-07T10:00:00Z", "2026-01-09T10:00:00Z"}},
		{name: "rdate added and deduplicated", ev: withRd, rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-05T10:00:00Z", "2026-01-06T10:00:00Z", "2026-01-20T15:00:00Z"}},
		{name: "rdates without rrule", ev: onlyRd, rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-05T10:00:00Z", "2026-01-07T10:00:00Z"}},
		{name: "dtstart not matching rule is first instance",
			ev: timedEvent("2026-01-06T10:00:00Z", "2026-01-06T11:00:00Z", "", "FREQ=WEEKLY;BYDAY=MO;COUNT=3"),
			rs: "2026-01-01T00:00:00Z", re: "2026-03-01T00:00:00Z",
			want: []string{"2026-01-06T10:00:00Z", "2026-01-12T10:00:00Z", "2026-01-19T10:00:00Z"}},
		{name: "override moves and renames",
			ev:        timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=3"),
			overrides: []csil.Event{override("2026-01-06T10:00:00Z", "2026-01-06T14:00:00Z", "2026-01-06T15:00:00Z", "Moved")},
			rs:        "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-05T10:00:00Z", "2026-01-06T14:00:00Z", "2026-01-07T10:00:00Z"}},
		{name: "override moved into range",
			ev:        timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=5"),
			overrides: []csil.Event{override("2026-01-09T10:00:00Z", "2026-01-05T20:00:00Z", "2026-01-05T21:00:00Z", "Early")},
			rs:        "2026-01-05T00:00:00Z", re: "2026-01-06T00:00:00Z",
			want: []string{"2026-01-05T10:00:00Z", "2026-01-05T20:00:00Z"}},
		{name: "override moved out of range",
			ev:        timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=5"),
			overrides: []csil.Event{override("2026-01-05T10:00:00Z", "2026-01-09T20:00:00Z", "2026-01-09T21:00:00Z", "Late")},
			rs:        "2026-01-05T00:00:00Z", re: "2026-01-06T00:00:00Z",
			want: []string{}},
		{name: "override for exdate ignored",
			ev:        withEx,
			overrides: []csil.Event{override("2026-01-06T10:00:00Z", "2026-01-06T14:00:00Z", "2026-01-06T15:00:00Z", "Ghost")},
			rs:        "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-05T10:00:00Z", "2026-01-07T10:00:00Z", "2026-01-09T10:00:00Z"}},
		{name: "cancelled override removes instance",
			ev: timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=3"),
			overrides: []csil.Event{func() csil.Event {
				o := override("2026-01-06T10:00:00Z", "2026-01-06T10:00:00Z", "2026-01-06T11:00:00Z", "Meeting")
				s := csil.EventStatus("cancelled")
				o.Status = &s
				return o
			}()},
			rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-05T10:00:00Z", "2026-01-07T10:00:00Z"}},
		{name: "dst weekly denver keeps 09:00 local",
			ev: timedEvent("2026-10-19T15:00:00Z", "2026-10-19T16:00:00Z", "America/Denver", "FREQ=WEEKLY;BYDAY=MO"),
			rs: "2026-10-01T00:00:00Z", re: "2026-11-15T00:00:00Z",
			want: []string{"2026-10-19T15:00:00Z", "2026-10-26T15:00:00Z", "2026-11-02T16:00:00Z", "2026-11-09T16:00:00Z"}},
		{name: "dst daily denver open series shifted start",
			ev: timedEvent("2020-01-01T16:00:00Z", "2020-01-01T17:00:00Z", "America/Denver", "FREQ=DAILY"),
			rs: "2026-10-31T00:00:00Z", re: "2026-11-03T00:00:00Z",
			want: []string{"2026-10-31T15:00:00Z", "2026-11-01T16:00:00Z", "2026-11-02T16:00:00Z"}},
		{name: "monthly 31st open series shifted start",
			ev: timedEvent("2001-01-31T12:00:00Z", "2001-01-31T13:00:00Z", "", "FREQ=MONTHLY"),
			rs: "2026-02-01T00:00:00Z", re: "2026-06-01T00:00:00Z",
			want: []string{"2026-03-31T12:00:00Z", "2026-05-31T12:00:00Z"}},
		{name: "weekly interval open series shifted start keeps parity",
			ev: timedEvent("2026-01-05T15:00:00Z", "2026-01-05T16:00:00Z", "", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,TH"),
			rs: "2026-06-01T00:00:00Z", re: "2026-06-22T00:00:00Z",
			want: []string{"2026-06-08T15:00:00Z", "2026-06-11T15:00:00Z"}},
		{name: "yearly open series shifted start",
			ev: timedEvent("1990-07-04T18:00:00Z", "1990-07-04T19:00:00Z", "", "FREQ=YEARLY"),
			rs: "2026-01-01T00:00:00Z", re: "2028-01-01T00:00:00Z",
			want: []string{"2026-07-04T18:00:00Z", "2027-07-04T18:00:00Z"}},
		{name: "max cap truncates",
			ev: timedEvent("2026-01-01T10:00:00Z", "2026-01-01T11:00:00Z", "", "FREQ=DAILY"),
			rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z", max: 3,
			want: []string{"2026-01-01T10:00:00Z", "2026-01-02T10:00:00Z", "2026-01-03T10:00:00Z"}, trunc: true},
		{name: "max equal to count is not truncated",
			ev: timedEvent("2026-01-01T10:00:00Z", "2026-01-01T11:00:00Z", "", "FREQ=DAILY;COUNT=3"),
			rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z", max: 3,
			want: []string{"2026-01-01T10:00:00Z", "2026-01-02T10:00:00Z", "2026-01-03T10:00:00Z"}},
		{name: "all-day weekly two days",
			ev: allDayEvent("2026-01-03", "2026-01-05", "FREQ=WEEKLY;COUNT=3"),
			rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-03T00:00:00Z", "2026-01-10T00:00:00Z", "2026-01-17T00:00:00Z"}},
		{name: "all-day yearly until date inclusive",
			ev: allDayEvent("2026-04-15", "2026-04-16", "FREQ=YEARLY;UNTIL=20280415"),
			rs: "2026-01-01T00:00:00Z", re: "2035-01-01T00:00:00Z",
			want: []string{"2026-04-15T00:00:00Z", "2027-04-15T00:00:00Z", "2028-04-15T00:00:00Z"}},
		{name: "timed with date until is end of day local",
			ev: timedEvent("2026-01-05T16:00:00Z", "2026-01-05T17:00:00Z", "America/Denver", "FREQ=DAILY;UNTIL=20260107"),
			rs: "2026-01-01T00:00:00Z", re: "2026-02-01T00:00:00Z",
			want: []string{"2026-01-05T16:00:00Z", "2026-01-06T16:00:00Z", "2026-01-07T16:00:00Z"}},
		{name: "event with recurrence id is single", ev: override("2026-01-05T10:00:00Z", "2026-01-05T12:00:00Z", "2026-01-05T13:00:00Z", "x"),
			rs: "2026-01-05T00:00:00Z", re: "2026-01-06T00:00:00Z", want: []string{"2026-01-05T12:00:00Z"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, trunc, err := Expand(tc.ev, tc.overrides, mustTime(t, tc.rs), mustTime(t, tc.re), tc.max)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if gs := startsOf(got); !reflect.DeepEqual(gs, tc.want) {
				t.Fatalf("starts\n got %v\nwant %v", gs, tc.want)
			}
			if trunc != tc.trunc {
				t.Fatalf("truncated = %v, want %v", trunc, tc.trunc)
			}
		})
	}
}

func TestExpandDetails(t *testing.T) {
	t.Run("dst local hour stays 9", func(t *testing.T) {
		ev := timedEvent("2026-10-19T15:00:00Z", "2026-10-19T16:00:00Z", "America/Denver", "FREQ=WEEKLY;BYDAY=MO")
		got, _, err := Expand(ev, nil, mustTime(t, "2026-10-01T00:00:00Z"), mustTime(t, "2026-12-01T00:00:00Z"), 0)
		if err != nil {
			t.Fatal(err)
		}
		den, _ := time.LoadLocation("America/Denver")
		for _, in := range got {
			if h := in.Start.In(den).Hour(); h != 9 {
				t.Errorf("%v: local hour %d", in.Start, h)
			}
			if in.End.Sub(in.Start) != time.Hour {
				t.Errorf("%v: length %v", in.Start, in.End.Sub(in.Start))
			}
			if string(*in.Event.Start) != in.Start.Format(TimeLayout) {
				t.Errorf("event start %s != %s", *in.Event.Start, in.Start.Format(TimeLayout))
			}
			if in.RecurrenceID == nil || !in.RecurrenceID.Equal(in.Start) {
				t.Errorf("recurrence id %v", in.RecurrenceID)
			}
		}
	})
	t.Run("all-day dates per instance", func(t *testing.T) {
		ev := allDayEvent("2026-03-07", "2026-03-09", "FREQ=WEEKLY;COUNT=3")
		ev.Timezone = ptr("America/Denver") // ignored for all-day
		got, _, err := Expand(ev, nil, mustTime(t, "2026-03-01T00:00:00Z"), mustTime(t, "2026-04-01T00:00:00Z"), 0)
		if err != nil {
			t.Fatal(err)
		}
		want := [][2]string{{"2026-03-07", "2026-03-09"}, {"2026-03-14", "2026-03-16"}, {"2026-03-21", "2026-03-23"}}
		if len(got) != len(want) {
			t.Fatalf("got %d instances", len(got))
		}
		for i, in := range got {
			if string(*in.Event.StartDate) != want[i][0] || string(*in.Event.EndDate) != want[i][1] {
				t.Errorf("%d: dates %s..%s, want %v", i, *in.Event.StartDate, *in.Event.EndDate, want[i])
			}
			if in.Start.Format(DateLayout) != want[i][0] || in.End.Format(DateLayout) != want[i][1] || in.Start.Hour() != 0 {
				t.Errorf("%d: instance %v..%v", i, in.Start, in.End)
			}
			if string(*in.Event.Start) != want[i][0]+"T00:00:00Z" {
				t.Errorf("%d: start %s", i, *in.Event.Start)
			}
		}
	})
	t.Run("override event returned as is", func(t *testing.T) {
		ev := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=3")
		o := override("2026-01-06T10:00:00Z", "2026-01-06T14:00:00Z", "2026-01-06T15:00:00Z", "Moved")
		got, _, err := Expand(ev, []csil.Event{o}, mustTime(t, "2026-01-06T00:00:00Z"), mustTime(t, "2026-01-07T00:00:00Z"), 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Event.Title != "Moved" || got[0].RecurrenceID.Format(TimeLayout) != "2026-01-06T10:00:00Z" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("bad zone is an error", func(t *testing.T) {
		ev := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "Mars/Olympus", "FREQ=DAILY")
		if _, _, err := Expand(ev, nil, mustTime(t, "2026-01-01T00:00:00Z"), mustTime(t, "2026-02-01T00:00:00Z"), 0); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("bad rule is an error", func(t *testing.T) {
		ev := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=SOMETIMES")
		if _, _, err := Expand(ev, nil, mustTime(t, "2026-01-01T00:00:00Z"), mustTime(t, "2026-02-01T00:00:00Z"), 0); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestExpandLongSeriesIsFast(t *testing.T) {
	cases := []struct {
		name string
		ev   csil.Event
		want int
	}{
		{"daily from 2000", timedEvent("2000-01-01T09:00:00Z", "2000-01-01T10:00:00Z", "America/Denver", "FREQ=DAILY"), 7},
		{"hourly from 2000", timedEvent("2000-01-01T09:00:00Z", "2000-01-01T09:30:00Z", "", "FREQ=HOURLY;INTERVAL=5"), 34},
		{"weekly from 1970", timedEvent("1970-01-05T09:00:00Z", "1970-01-05T10:00:00Z", "", "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR"), 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			begin := time.Now()
			got, _, err := Expand(tc.ev, nil, mustTime(t, "2026-10-12T00:00:00Z"), mustTime(t, "2026-10-19T00:00:00Z"), 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d instances, want %d: %v", len(got), tc.want, startsOf(got))
			}
			if d := time.Since(begin); d > 50*time.Millisecond {
				t.Fatalf("took %v", d)
			}
		})
	}
	t.Run("far future range", func(t *testing.T) {
		ev := timedEvent("2000-01-01T09:00:00Z", "2000-01-01T10:00:00Z", "", "FREQ=DAILY")
		begin := time.Now()
		got, _, err := Expand(ev, nil, mustTime(t, "2400-01-01T00:00:00Z"), mustTime(t, "2400-01-03T00:00:00Z"), 0)
		if err != nil || len(got) != 2 {
			t.Fatalf("got %v %v", startsOf(got), err)
		}
		if d := time.Since(begin); d > 50*time.Millisecond {
			t.Fatalf("took %v", d)
		}
	})
}

func TestSeriesUntil(t *testing.T) {
	withRd := timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;COUNT=2")
	withRd.Recurrence.Rdates = []csil.Timestamp{"2026-03-01T10:00:00Z"}
	tests := []struct {
		name string
		ev   csil.Event
		want string // "" means nil
	}{
		{"single", timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", ""), "2026-01-05T11:00:00Z"},
		{"single all-day", allDayEvent("2026-01-05", "2026-01-07", ""), "2026-01-07T00:00:00Z"},
		{"count", timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=WEEKLY;COUNT=3"), "2026-01-19T11:00:00Z"},
		{"count dst", timedEvent("2026-10-26T15:00:00Z", "2026-10-26T16:00:00Z", "America/Denver", "FREQ=WEEKLY;COUNT=2"), "2026-11-02T17:00:00Z"},
		{"count with later rdate", withRd, "2026-03-01T11:00:00Z"},
		{"until", timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY;UNTIL=20260110T100000Z"), "2026-01-10T11:00:00Z"},
		{"until all-day", allDayEvent("2026-04-15", "2026-04-16", "FREQ=YEARLY;UNTIL=20300415"), "2030-04-16T00:00:00Z"},
		{"open", timedEvent("2026-01-05T10:00:00Z", "2026-01-05T11:00:00Z", "", "FREQ=DAILY"), ""},
		{"impossible rule is just dtstart", timedEvent("2026-01-30T10:00:00Z", "2026-01-30T11:00:00Z", "", "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30"), "2026-01-30T11:00:00Z"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SeriesUntil(tc.ev)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if got == nil || got.Format(TimeLayout) != tc.want {
				t.Fatalf("got %v, want %s", got, tc.want)
			}
		})
	}
}
