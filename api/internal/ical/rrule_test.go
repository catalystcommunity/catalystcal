package ical

import (
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func TestValidateRRule(t *testing.T) {
	tests := []struct {
		rule string
		ok   bool
	}{
		{"FREQ=DAILY", true},
		{"FREQ=WEEKLY;BYDAY=MO,WE,FR", true},
		{"FREQ=WEEKLY;INTERVAL=2;BYDAY=TU;UNTIL=20271231T235959Z", true},
		{"FREQ=MONTHLY;BYDAY=FR;BYSETPOS=-1", true},
		{"FREQ=MONTHLY;BYDAY=-1FR", true},
		{"FREQ=MONTHLY;BYMONTHDAY=31", true},
		{"FREQ=MONTHLY;BYMONTHDAY=-1", true},
		{"FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=29", true},
		{"FREQ=YEARLY;COUNT=10", true},
		{"FREQ=YEARLY;UNTIL=20300101", true},
		{"FREQ=HOURLY;INTERVAL=4", true},
		{"freq=weekly;byday=mo", true},
		{"FREQ=DAILY;X-NAME=FOO", true},
		{"", false},
		{"garbage", false},
		{"FREQ=SOMETIMES", false},
		{"BYDAY=MO", false},
		{"FREQ=DAILY;FREQ=WEEKLY", false},
		{"FREQ=DAILY;BYDAY=XX", false},
		{"FREQ=DAILY;FOO=BAR", false},
		{"FREQ=SECONDLY", false},
		{"FREQ=MINUTELY;INTERVAL=15", false},
		{"FREQ=DAILY;COUNT=0", false},
		{"FREQ=DAILY;COUNT=-1", false},
		{"FREQ=DAILY;INTERVAL=0", false},
		{"FREQ=DAILY;COUNT=2;UNTIL=20270101T000000Z", false},
		{"FREQ=DAILY;UNTIL=2027-01-01", false},
		{"FREQ=MONTHLY;BYMONTHDAY=32", false},
		{"FREQ=YEARLY;BYMONTH=13", false},
		{"FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30", false},
		{"FREQ=DAILY;BYMONTH=2;BYMONTHDAY=30", false},
		{"FREQ=MONTHLY;BYMONTH=4,6,9,11;BYMONTHDAY=31", false},
		{"FREQ=MONTHLY;BYMONTH=2;BYDAY=5MO;BYMONTHDAY=1", false},
	}
	for _, tc := range tests {
		t.Run(tc.rule, func(t *testing.T) {
			begin := time.Now()
			err := ValidateRRule(tc.rule)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidateRRule(%q) = %v, want ok=%v", tc.rule, err, tc.ok)
			}
			if d := time.Since(begin); d > time.Second {
				t.Fatalf("took %v", d)
			}
		})
	}
}

func TestTruncateRRule(t *testing.T) {
	tests := []struct {
		name    string
		rule    string
		dtstart string
		tz      string
		cut     string
		want    string
		wantErr error
	}{
		{"open timed", "FREQ=WEEKLY;BYDAY=MO", "2026-01-05T16:00:00Z", "America/Denver", "2026-03-02T16:00:00Z",
			"FREQ=WEEKLY;BYDAY=MO;UNTIL=20260302T155959Z", nil},
		{"count replaced", "FREQ=DAILY;COUNT=10;INTERVAL=1", "2026-01-01T10:00:00Z", "", "2026-01-05T10:00:00Z",
			"FREQ=DAILY;INTERVAL=1;UNTIL=20260105T095959Z", nil},
		{"count already ends before cut", "FREQ=DAILY;COUNT=3", "2026-01-01T10:00:00Z", "", "2026-01-05T10:00:00Z",
			"FREQ=DAILY;COUNT=3", nil},
		{"until replaced", "FREQ=DAILY;UNTIL=20261231T235959Z", "2026-01-01T10:00:00Z", "", "2026-02-01T10:00:00Z",
			"FREQ=DAILY;UNTIL=20260201T095959Z", nil},
		{"until already before cut", "FREQ=DAILY;UNTIL=20260110T000000Z", "2026-01-01T10:00:00Z", "", "2026-02-01T10:00:00Z",
			"FREQ=DAILY;UNTIL=20260110T000000Z", nil},
		{"all-day open", "FREQ=YEARLY", "2026-04-15T00:00:00Z", "", "2029-04-15T00:00:00Z",
			"FREQ=YEARLY;UNTIL=20290414", nil},
		{"all-day count", "FREQ=WEEKLY;COUNT=20", "2026-01-03T00:00:00Z", "", "2026-02-07T00:00:00Z",
			"FREQ=WEEKLY;UNTIL=20260206", nil},
		{"all-day until", "FREQ=DAILY;UNTIL=20261231", "2026-01-01T00:00:00Z", "", "2026-01-10T00:00:00Z",
			"FREQ=DAILY;UNTIL=20260109", nil},
		{"cut at start", "FREQ=DAILY", "2026-01-01T10:00:00Z", "", "2026-01-01T10:00:00Z", "", ErrCutBeforeStart},
		{"bad rule", "FREQ=NEVER", "2026-01-01T10:00:00Z", "", "2026-01-05T10:00:00Z", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TruncateRRule(tc.rule, mustTime(t, tc.dtstart), tc.tz, mustTime(t, tc.cut))
			if tc.want == "" {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if err := ValidateRRule(got); err != nil {
				t.Fatalf("result does not validate: %v", err)
			}
		})
	}
}

// TestTruncateRRuleSplitsSeries checks that the truncated rule plus a new
// series from cut give the same instances as the old rule.
func TestTruncateRRuleSplitsSeries(t *testing.T) {
	rs, re := mustTime(t, "2026-01-01T00:00:00Z"), mustTime(t, "2027-01-01T00:00:00Z")
	cases := []struct {
		name string
		ev   csil.Event
		cut  string
	}{
		{"timed dst", timedEvent("2026-10-05T15:00:00Z", "2026-10-05T16:00:00Z", "America/Denver", "FREQ=WEEKLY;BYDAY=MO,WE;COUNT=12"), "2026-11-02T16:00:00Z"},
		{"all-day", allDayEvent("2026-01-03", "2026-01-04", "FREQ=WEEKLY;COUNT=10"), "2026-02-07T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			all, _, err := Expand(tc.ev, nil, rs, re, 0)
			if err != nil {
				t.Fatal(err)
			}
			cut := mustTime(t, tc.cut)
			tz := ""
			if tc.ev.Timezone != nil {
				tz = *tc.ev.Timezone
			}
			start, _ := parseInstant(string(*tc.ev.Start))
			rule, err := TruncateRRule(tc.ev.Recurrence.Rrule, start, tz, cut)
			if err != nil {
				t.Fatal(err)
			}
			head := tc.ev
			head.Recurrence = &csil.Recurrence{Rrule: rule}
			got, _, err := Expand(head, nil, rs, re, 0)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, in := range all {
				if in.Start.Before(cut) {
					want = append(want, in.Start.Format(TimeLayout))
				}
			}
			if gs := startsOf(got); len(gs) != len(want) || (len(want) > 0 && gs[len(gs)-1] != want[len(want)-1]) {
				t.Fatalf("head %v, want %v", gs, want)
			}
		})
	}
}

func TestNormalizeRRuleForWrite(t *testing.T) {
	den, _ := time.LoadLocation("America/Denver")
	tests := []struct {
		rule   string
		allDay bool
		loc    *time.Location
		want   string
	}{
		{"FREQ=DAILY;UNTIL=20261231T235959Z", false, time.UTC, "FREQ=DAILY;UNTIL=20261231T235959Z"},
		{"RRULE:FREQ=DAILY", false, time.UTC, "FREQ=DAILY"},
		{"FREQ=DAILY;UNTIL=20261231", false, den, "FREQ=DAILY;UNTIL=20270101T065959Z"},
		{"FREQ=DAILY;UNTIL=20261231T090000", false, den, "FREQ=DAILY;UNTIL=20261231T160000Z"},
		{"FREQ=DAILY;UNTIL=20261231T235959Z;BYDAY=MO", true, time.UTC, "FREQ=DAILY;UNTIL=20261231;BYDAY=MO"},
		{"FREQ=YEARLY;UNTIL=20300414", true, time.UTC, "FREQ=YEARLY;UNTIL=20300414"},
	}
	for _, tc := range tests {
		got, err := normalizeRRuleForWrite(tc.rule, tc.allDay, tc.loc)
		if err != nil || got != tc.want {
			t.Errorf("normalize(%q) = %q, %v; want %q", tc.rule, got, err, tc.want)
		}
	}
}
