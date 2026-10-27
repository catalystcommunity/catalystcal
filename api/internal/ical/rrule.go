package ical

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"
)

// Notes on rrule-go (github.com/teambition/rrule-go). Read these before you
// change this file.
//
//   - rrule-go makes each instance with time.Date in the location of
//     DTSTART. Give it DTSTART in the event zone, and the local wall time
//     stays the same across a daylight-saving change.
//   - rrule-go skips a month or a year that does not have the day
//     (BYMONTHDAY=31 in April, Feb 30). This is the RFC 5545 behavior.
//   - rrule-go does not stop when a rule has no instance. It searches until
//     the year 9999. A daily rule can take seconds before it gives up. Thus
//     this package tests a rule with possible() before it iterates.
//   - rrule-go yields only instances that match the rule. RFC 5545 says
//     that DTSTART is always the first instance. This package adds DTSTART
//     itself (see series.startsBetween).
//   - UNTIL with no COUNT does not stop the search for a rule with no
//     instance. UNTIL applies only when rrule-go finds a candidate.
//   - StrToROption reads COUNT=0 as "no COUNT". This package reads COUNT
//     itself and refuses COUNT=0.

// ruleSpec is a parsed RRULE value. opt holds FREQ, INTERVAL, WKST, and
// the BY parts. COUNT and UNTIL are kept apart, because their meaning
// depends on DTSTART.
type ruleSpec struct {
	opt      rrule.ROption
	count    int
	untilRaw string
	parts    []rulePart
}

type rulePart struct{ key, val string }

// parseRuleSpec reads an RRULE value. A "RRULE:" prefix is permitted.
// Rule parts that start with "X-" are dropped.
func parseRuleSpec(s string) (*ruleSpec, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 6 && strings.EqualFold(s[:6], "RRULE:") {
		s = s[6:]
	}
	if s == "" {
		return nil, errors.New("ical: empty RRULE")
	}
	spec := &ruleSpec{}
	seen := map[string]bool{}
	var core []string
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		k = strings.ToUpper(strings.TrimSpace(k))
		v = strings.ToUpper(strings.TrimSpace(v))
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("ical: RRULE part %q is not KEY=VALUE", part)
		}
		if strings.HasPrefix(k, "X-") {
			continue
		}
		if seen[k] {
			return nil, fmt.Errorf("ical: RRULE part %s occurs more than one time", k)
		}
		seen[k] = true
		spec.parts = append(spec.parts, rulePart{k, v})
		switch k {
		case "COUNT":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("ical: RRULE COUNT must be 1 or more, got %q", v)
			}
			spec.count = n
		case "UNTIL":
			if _, _, err := parseUntilText(v); err != nil {
				return nil, err
			}
			spec.untilRaw = v
		case "INTERVAL":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("ical: RRULE INTERVAL must be 1 or more, got %q", v)
			}
			core = append(core, k+"="+v)
		case "FREQ", "WKST", "BYSETPOS", "BYMONTH", "BYMONTHDAY", "BYYEARDAY",
			"BYWEEKNO", "BYDAY", "BYHOUR", "BYMINUTE", "BYSECOND":
			core = append(core, k+"="+v)
		default:
			return nil, fmt.Errorf("ical: unknown RRULE part %q", k)
		}
	}
	if !seen["FREQ"] {
		return nil, errors.New("ical: RRULE has no FREQ")
	}
	opt, err := rrule.StrToROption(strings.Join(core, ";"))
	if err != nil {
		return nil, fmt.Errorf("ical: bad RRULE: %w", err)
	}
	probe := *opt
	probe.Dtstart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := rrule.NewRRule(probe); err != nil {
		return nil, fmt.Errorf("ical: bad RRULE: %w", err)
	}
	spec.opt = *opt
	return spec, nil
}

// parseUntilText reads an UNTIL value. kind is "date", "utc", or "floating".
func parseUntilText(v string) (time.Time, string, error) {
	switch {
	case len(v) == 8:
		t, err := time.Parse("20060102", v)
		if err == nil {
			return t, "date", nil
		}
	case len(v) == 16 && v[15] == 'Z':
		t, err := time.Parse("20060102T150405Z", v)
		if err == nil {
			return t, "utc", nil
		}
	case len(v) == 15:
		t, err := time.Parse("20060102T150405", v)
		if err == nil {
			return t, "floating", nil
		}
	}
	return time.Time{}, "", fmt.Errorf("ical: bad RRULE UNTIL %q", v)
}

// until resolves UNTIL for a series.
//
// A DATE UNTIL on an all-day series is that date. A DATE UNTIL on a timed
// series (not valid RFC 5545, but seen in real files) is the end of that
// day in the series zone. A floating UNTIL is in the series zone.
func (s *ruleSpec) until(loc *time.Location, allDay bool) time.Time {
	if s.untilRaw == "" {
		return time.Time{}
	}
	t, kind, _ := parseUntilText(s.untilRaw)
	switch kind {
	case "date":
		if allDay {
			return t
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc)
	case "floating":
		if allDay {
			return t
		}
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
	}
	return t
}

// options returns the full rrule options for a DTSTART.
func (s *ruleSpec) options(dtstart time.Time, allDay bool) rrule.ROption {
	o := s.opt
	o.Dtstart = dtstart
	o.Count = s.count
	o.Until = s.until(dtstart.Location(), allDay)
	pinDefaults(&o)
	return o
}

var rruleWeekdays = map[time.Weekday]rrule.Weekday{
	time.Monday: rrule.MO, time.Tuesday: rrule.TU, time.Wednesday: rrule.WE,
	time.Thursday: rrule.TH, time.Friday: rrule.FR, time.Saturday: rrule.SA,
	time.Sunday: rrule.SU,
}

// pinDefaults writes the BY parts that rrule-go would take from DTSTART.
// After this, a change of DTSTART (see shiftStart) does not change the
// rule. It also keeps the wall time when a shifted DTSTART falls in a
// daylight-saving gap.
func pinDefaults(o *rrule.ROption) {
	d := o.Dtstart
	if len(o.Byweekno) == 0 && len(o.Byyearday) == 0 && len(o.Bymonthday) == 0 &&
		len(o.Byweekday) == 0 && len(o.Byeaster) == 0 {
		switch o.Freq {
		case rrule.YEARLY:
			if len(o.Bymonth) == 0 {
				o.Bymonth = []int{int(d.Month())}
			}
			o.Bymonthday = []int{d.Day()}
		case rrule.MONTHLY:
			o.Bymonthday = []int{d.Day()}
		case rrule.WEEKLY:
			o.Byweekday = []rrule.Weekday{rruleWeekdays[d.Weekday()]}
		}
	}
	if o.Freq < rrule.HOURLY && len(o.Byhour) == 0 {
		o.Byhour = []int{d.Hour()}
	}
	if o.Freq < rrule.MINUTELY && len(o.Byminute) == 0 {
		o.Byminute = []int{d.Minute()}
	}
	if o.Freq < rrule.SECONDLY && len(o.Bysecond) == 0 {
		o.Bysecond = []int{d.Second()}
	}
}

// possible reports if a rule (with pinned defaults) can make an instance.
// It ignores COUNT, UNTIL, and INTERVAL. It runs the rule from 9600-01-01
// in UTC. rrule-go stops at the year 9999, so a rule with no instance
// costs at most 400 years of search. 400 years is one full Gregorian
// cycle, thus a rule that can occur in some year occurs in that span.
func possible(o rrule.ROption) bool {
	p := o
	p.Count = 0
	p.Until = time.Time{}
	p.Interval = 1
	d := o.Dtstart
	p.Dtstart = time.Date(9600, 1, 1, d.Hour(), d.Minute(), d.Second(), 0, time.UTC)
	r, err := rrule.NewRRule(p)
	if err != nil {
		return false
	}
	_, ok := r.Iterator()()
	return ok
}

// shiftStart returns a later DTSTART that gives the same instances on and
// after from. It moves DTSTART by whole periods (INTERVAL times the FREQ
// unit), so the rule stays aligned. It stops two periods before from, so
// the first period (which can lose instances before the new DTSTART) is
// fully before from. Use it only for a rule with no COUNT.
func shiftStart(o rrule.ROption, from time.Time) time.Time {
	d := o.Dtstart
	loc := d.Location()
	f := from.In(loc)
	if !f.After(d) {
		return d
	}
	iv := o.Interval
	if iv < 1 {
		iv = 1
	}
	h, mi, s := d.Clock()
	switch o.Freq {
	case rrule.DAILY, rrule.HOURLY:
		k := civilDays(d, f)/iv - 2
		if k > 0 {
			return time.Date(d.Year(), d.Month(), d.Day()+k*iv, h, mi, s, 0, loc)
		}
	case rrule.WEEKLY:
		k := civilDays(d, f)/(7*iv) - 2
		if k > 0 {
			return time.Date(d.Year(), d.Month(), d.Day()+k*iv*7, h, mi, s, 0, loc)
		}
	case rrule.MONTHLY:
		months := (f.Year()-d.Year())*12 + int(f.Month()) - int(d.Month())
		k := months/iv - 2
		if k > 0 {
			return time.Date(d.Year(), d.Month()+time.Month(k*iv), 1, h, mi, s, 0, loc)
		}
	case rrule.YEARLY:
		k := (f.Year()-d.Year())/iv - 2
		if k > 0 {
			return time.Date(d.Year()+k*iv, time.January, 1, h, mi, s, 0, loc)
		}
	}
	return d
}

// civilDays is the count of calendar days from the date of a to the date
// of b, in their own locations.
func civilDays(a, b time.Time) int {
	ad := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	bd := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(bd.Sub(ad).Hours() / 24)
}

// ValidateRRule checks an RRULE value (no "RRULE:" prefix). It refuses
// rules that cannot produce an instance and FREQ=SECONDLY/MINUTELY.
//
// It also refuses COUNT=0, INTERVAL=0, unknown rule parts, values out of
// the RFC 5545 bounds, and COUNT together with UNTIL. The check for "no
// instance" uses the BY parts only. It cannot see DTSTART, so a rule that
// fails only because of INTERVAL and DTSTART passes.
func ValidateRRule(rule string) error {
	spec, err := parseRuleSpec(rule)
	if err != nil {
		return err
	}
	switch spec.opt.Freq {
	case rrule.SECONDLY, rrule.MINUTELY:
		return fmt.Errorf("ical: RRULE FREQ=%v is not supported", spec.opt.Freq)
	}
	if spec.count > 0 && spec.untilRaw != "" {
		return errors.New("ical: RRULE must not have both COUNT and UNTIL")
	}
	o := spec.opt
	o.Dtstart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	pinDefaults(&o)
	if !possible(o) {
		return errors.New("ical: RRULE cannot produce an instance")
	}
	return nil
}

// ErrCutBeforeStart is returned by TruncateRRule when the cut is at or
// before DTSTART. The truncated series would have no instance.
var ErrCutBeforeStart = errors.New("ical: cut is at or before the series start")

// TruncateRRule returns rrule changed to end before cut (an UNTIL one
// second before cut, with COUNT removed). The services use it to split a
// series for "this and future" edits.
//
// Rules:
//   - When the series already ends before cut (by COUNT or UNTIL), the
//     rule comes back unchanged.
//   - The new UNTIL is UTC DATE-TIME (cut minus one second). For an
//     all-day series it is a DATE (the day before cut). This function
//     sees an all-day series as tz "" with dtstart and cut at 00:00:00Z.
//     The two forms give the same instances, and Encode writes UNTIL in
//     the form that matches DTSTART.
//   - RDATE values after cut are not changed here. The caller removes
//     them.
func TruncateRRule(rule string, dtstart time.Time, tz string, cut time.Time) (string, error) {
	spec, err := parseRuleSpec(rule)
	if err != nil {
		return "", err
	}
	loc, err := loadZone(tz)
	if err != nil {
		return "", err
	}
	if !cut.After(dtstart) {
		return "", ErrCutBeforeStart
	}
	allDay := tz == "" && isMidnightUTC(dtstart) && isMidnightUTC(cut)
	start := dtstart.In(loc)
	if allDay {
		start = dtstart.UTC()
	}
	o, err := ruleOptions(spec, start, allDay)
	if err != nil {
		return "", err
	}
	if o == nil || (!o.Until.IsZero() && o.Until.Before(cut)) {
		return strings.TrimSpace(rule), nil
	}
	if o.Count > 0 {
		last, ok := lastCountInstance(*o)
		if !ok || last.Before(cut) {
			return strings.TrimSpace(rule), nil
		}
	}
	var until string
	if allDay {
		until = cut.UTC().AddDate(0, 0, -1).Format("20060102")
	} else {
		until = cut.UTC().Add(-time.Second).Format("20060102T150405Z")
	}
	return spec.render(map[string]string{"COUNT": "", "UNTIL": until}), nil
}

// lastCountInstance returns the last instance of a COUNT rule.
func lastCountInstance(o rrule.ROption) (time.Time, bool) {
	if !possible(o) {
		return time.Time{}, false
	}
	r, err := rrule.NewRRule(o)
	if err != nil {
		return time.Time{}, false
	}
	next := r.Iterator()
	var last time.Time
	found := false
	for {
		t, ok := next()
		if !ok {
			break
		}
		last, found = t, true
	}
	return last, found
}

// render writes the rule parts in their first order. set changes a part
// value; an empty value removes the part; a new key goes at the end.
func (s *ruleSpec) render(set map[string]string) string {
	var out []string
	done := map[string]bool{}
	for _, p := range s.parts {
		v := p.val
		if nv, ok := set[p.key]; ok {
			done[p.key] = true
			v = nv
		}
		if v != "" {
			out = append(out, p.key+"="+v)
		}
	}
	for _, k := range []string{"COUNT", "UNTIL"} {
		if v, ok := set[k]; ok && !done[k] && v != "" {
			out = append(out, k+"="+v)
		}
	}
	return strings.Join(out, ";")
}

// normalizeRRuleForWrite returns the rule with UNTIL in the form RFC 5545
// requires for the DTSTART type: DATE for an all-day series, UTC
// DATE-TIME for a timed series. Other parts stay as they are.
func normalizeRRuleForWrite(rule string, allDay bool, loc *time.Location) (string, error) {
	spec, err := parseRuleSpec(rule)
	if err != nil {
		return "", err
	}
	rule = strings.TrimSpace(rule)
	if len(rule) >= 6 && strings.EqualFold(rule[:6], "RRULE:") {
		rule = rule[6:]
	}
	if spec.untilRaw == "" {
		return rule, nil
	}
	_, kind, _ := parseUntilText(spec.untilRaw)
	u := spec.until(loc, allDay)
	switch {
	case allDay && kind != "date":
		return spec.render(map[string]string{"UNTIL": u.UTC().Format("20060102")}), nil
	case !allDay && kind != "utc":
		return spec.render(map[string]string{"UNTIL": u.UTC().Format("20060102T150405Z")}), nil
	}
	return rule, nil
}

func isMidnightUTC(t time.Time) bool {
	t = t.UTC()
	return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

// loadZone loads an IANA zone. An empty name is UTC.
func loadZone(name string) (*time.Location, error) {
	if name == "" {
		return time.UTC, nil
	}
	if strings.EqualFold(name, "local") {
		return nil, fmt.Errorf("ical: zone %q is not an IANA zone", name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("ical: unknown time zone %q: %w", name, err)
	}
	return loc, nil
}
