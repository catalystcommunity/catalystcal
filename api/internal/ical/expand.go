package ical

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/teambition/rrule-go"
)

// DefaultMaxInstances is the cap Expand uses when max is 0 or less.
const DefaultMaxInstances = 5000

// timing is the resolved time of one event.
type timing struct {
	allDay bool
	// loc is the event zone (UTC for an all-day event or no Timezone).
	loc *time.Location
	// start and end are in loc. For an all-day event they are 00:00 UTC.
	start, end time.Time
	// days is the length of an all-day event in days.
	days int
}

// parseInstant reads a Timestamp. It accepts TimeLayout and other RFC 3339
// forms, and drops fractions of a second.
func parseInstant(s string) (time.Time, error) {
	t, err := time.Parse(TimeLayout, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return time.Time{}, fmt.Errorf("ical: bad timestamp %q", s)
		}
	}
	return t.UTC().Truncate(time.Second), nil
}

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("ical: bad date %q", s)
	}
	return t, nil
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func isAllDay(ev csil.Event) bool { return ev.AllDay != nil && *ev.AllDay }

// eventTiming resolves the time of an event. An all-day event with no
// EndDate lasts one day. A timed event with no End has zero length.
func eventTiming(ev csil.Event) (timing, error) {
	if isAllDay(ev) {
		var sd, ed time.Time
		var err error
		switch {
		case ev.StartDate != nil && *ev.StartDate != "":
			sd, err = parseDate(string(*ev.StartDate))
		case ev.Start != nil && *ev.Start != "":
			sd, err = parseInstant(string(*ev.Start))
			sd = midnight(sd)
		default:
			err = errors.New("ical: all-day event has no start date")
		}
		if err != nil {
			return timing{}, err
		}
		switch {
		case ev.EndDate != nil && *ev.EndDate != "":
			ed, err = parseDate(string(*ev.EndDate))
		case ev.End != nil && *ev.End != "":
			ed, err = parseInstant(string(*ev.End))
			ed = midnight(ed)
		}
		if err != nil {
			return timing{}, err
		}
		if !ed.After(sd) {
			ed = sd.AddDate(0, 0, 1)
		}
		return timing{allDay: true, loc: time.UTC, start: sd, end: ed, days: civilDays(sd, ed)}, nil
	}
	if ev.Start == nil || *ev.Start == "" {
		return timing{}, errors.New("ical: event has no start")
	}
	st, err := parseInstant(string(*ev.Start))
	if err != nil {
		return timing{}, err
	}
	en := st
	if ev.End != nil && *ev.End != "" {
		if en, err = parseInstant(string(*ev.End)); err != nil {
			return timing{}, err
		}
	}
	if en.Before(st) {
		en = st
	}
	loc := time.UTC
	if ev.Timezone != nil {
		if loc, err = loadZone(*ev.Timezone); err != nil {
			return timing{}, err
		}
	}
	return timing{loc: loc, start: st.In(loc), end: en.In(loc)}, nil
}

// endFor returns the end of an instance that starts at start.
// A timed instance keeps the absolute length of the master. An all-day
// instance keeps the count of days.
func (t timing) endFor(start time.Time) time.Time {
	if t.allDay {
		return start.AddDate(0, 0, t.days)
	}
	return start.Add(t.end.Sub(t.start))
}

// ruleOptions returns the rrule options of a series with DTSTART start.
// It returns nil when the rule adds no instance to DTSTART.
//
// DTSTART is always an instance (RFC 5545 3.3.10). When DTSTART does not
// match the rule, it is an extra instance, and it uses one of the COUNT
// instances (as libical does).
func ruleOptions(spec *ruleSpec, start time.Time, allDay bool) (*rrule.ROption, error) {
	o := spec.options(start, allDay)
	if !possible(o) {
		return nil, nil
	}
	if o.Count > 0 {
		r, err := rrule.NewRRule(o)
		if err != nil {
			return nil, fmt.Errorf("ical: bad RRULE: %w", err)
		}
		if first, ok := r.Iterator()(); !ok || !first.Equal(start) {
			o.Count--
			if o.Count == 0 {
				return nil, nil
			}
		}
	}
	return &o, nil
}

// series is a recurring event, ready to expand.
type series struct {
	t timing
	// opt is nil when the series has only DTSTART and RDATEs, or when the
	// rule cannot produce an instance.
	opt    *rrule.ROption
	rdates []time.Time
	ex     map[int64]bool
}

func isRecurring(ev csil.Event) bool {
	if ev.RecurrenceId != nil || ev.Recurrence == nil {
		return false
	}
	return ev.Recurrence.Rrule != "" || len(ev.Recurrence.Rdates) > 0
}

// newSeries prepares a series.
func newSeries(ev csil.Event, t timing) (*series, error) {
	s := &series{t: t, ex: map[int64]bool{}}
	rec := ev.Recurrence
	if rec.Rrule != "" {
		spec, err := parseRuleSpec(rec.Rrule)
		if err != nil {
			return nil, err
		}
		if s.opt, err = ruleOptions(spec, t.start, t.allDay); err != nil {
			return nil, err
		}
	}
	for _, v := range rec.Rdates {
		ts, err := parseInstant(string(v))
		if err != nil {
			return nil, err
		}
		if t.allDay {
			ts = midnight(ts)
		}
		s.rdates = append(s.rdates, ts)
	}
	for _, v := range rec.Exdates {
		ts, err := parseInstant(string(v))
		if err != nil {
			return nil, err
		}
		if t.allDay {
			ts = midnight(ts)
		}
		s.ex[ts.Unix()] = true
	}
	return s, nil
}

// startsBetween returns instance starts (UTC) after from (inclusive when
// fromInc) and before to, sorted, at most limit of them. more is true
// when instances were left out because of limit.
func (s *series) startsBetween(from, to time.Time, fromInc bool, limit int) (out []time.Time, more bool) {
	seen := map[int64]bool{}
	in := func(t time.Time) bool {
		if !t.Before(to) {
			return false
		}
		if fromInc {
			return !t.Before(from)
		}
		return t.After(from)
	}
	add := func(t time.Time) bool {
		k := t.Unix()
		if seen[k] || s.ex[k] {
			return false
		}
		seen[k] = true
		out = append(out, t.UTC())
		return true
	}
	if in(s.t.start) {
		add(s.t.start)
	}
	for _, r := range s.rdates {
		if in(r) {
			add(r)
		}
	}
	if s.opt != nil {
		o := *s.opt
		if o.Count == 0 {
			o.Dtstart = shiftStart(o, from)
		}
		r, err := rrule.NewRRule(o)
		if err == nil {
			next := r.Iterator()
			n := 0
			for {
				t, ok := next()
				if !ok || !t.Before(to) {
					break
				}
				if !in(t) {
					continue
				}
				if n >= limit {
					more = true
					break
				}
				if add(t) {
					n++
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	if len(out) > limit {
		out = out[:limit]
		more = true
	}
	return out, more
}

// isInstance reports if the series has an instance that starts at t.
func (s *series) isInstance(t time.Time) bool {
	got, _ := s.startsBetween(t, t.Add(time.Second), true, 1)
	return len(got) == 1 && got[0].Equal(t)
}

// overlaps reports if [start, end) overlaps [rs, re). An instance with
// zero length overlaps when its start is in [rs, re).
func overlaps(start, end, rs, re time.Time) bool {
	if !start.Before(re) {
		return false
	}
	if end.Equal(start) {
		return !start.Before(rs)
	}
	return end.After(rs)
}

// withTimes returns a copy of ev with its times set to one instance.
func withTimes(ev csil.Event, t timing, start, end time.Time) csil.Event {
	s := csil.Timestamp(start.UTC().Format(TimeLayout))
	e := csil.Timestamp(end.UTC().Format(TimeLayout))
	ev.Start, ev.End = &s, &e
	if t.allDay {
		sd := csil.DateText(start.UTC().Format(DateLayout))
		ed := csil.DateText(end.UTC().Format(DateLayout))
		ev.StartDate, ev.EndDate = &sd, &ed
	}
	return ev
}

func isCancelled(ev csil.Event) bool {
	return ev.Status != nil && *ev.Status == "cancelled"
}

// Expand returns the instances of ev that overlap [rangeStart, rangeEnd),
// sorted by start. For a single event it returns zero or one instance.
// For a series it applies RRULE, RDATE, and EXDATE in ev.Timezone, then
// replaces each instance that has an override in overrides (matched by
// RecurrenceId). An override with Status "cancelled" removes the instance.
// An override moved into the range from outside it is included. At most
// max instances come back; truncated is true when the cap cut the list.
//
// More rules:
//   - max 0 or less means DefaultMaxInstances.
//   - Instance.Event of a series instance is a copy of the master with
//     Start and End (and StartDate and EndDate for all-day) set to that
//     instance. An override comes back as it is.
//   - A timed instance has the absolute length of the master. An all-day
//     instance has the same count of days.
//   - An event with RecurrenceId set is a single event here.
//   - An override for a start that is not an instance (for example an
//     EXDATE) is ignored.
//   - A rule that cannot produce an instance (BYMONTHDAY=30;BYMONTH=2)
//     gives only DTSTART and the RDATEs.
//   - For an open rule (no COUNT), expansion starts near rangeStart, not
//     at DTSTART. Thus a daily series from 2000 is fast to read in 2026.
func Expand(ev csil.Event, overrides []csil.Event, rangeStart, rangeEnd time.Time, max int) (instances []Instance, truncated bool, err error) {
	if max <= 0 {
		max = DefaultMaxInstances
	}
	t, err := eventTiming(ev)
	if err != nil {
		return nil, false, err
	}
	if !rangeEnd.After(rangeStart) {
		return nil, false, nil
	}
	if !isRecurring(ev) {
		if overlaps(t.start, t.end, rangeStart, rangeEnd) {
			return []Instance{{Event: ev, Start: t.start.UTC(), End: t.end.UTC()}}, false, nil
		}
		return nil, false, nil
	}
	s, err := newSeries(ev, t)
	if err != nil {
		return nil, false, err
	}

	ovs := map[int64]csil.Event{}
	for _, o := range overrides {
		if o.RecurrenceId == nil {
			continue
		}
		rid, err := parseInstant(string(*o.RecurrenceId))
		if err != nil {
			return nil, false, err
		}
		if t.allDay {
			rid = midnight(rid)
		}
		ovs[rid.Unix()] = o
	}

	length := t.end.Sub(t.start)
	from, fromInc := rangeStart.Add(-length), false
	if length == 0 {
		from, fromInc = rangeStart, true
	}
	starts, more := s.startsBetween(from, rangeEnd, fromInc, max+len(ovs)+1)

	used := map[int64]bool{}
	addOverride := func(o csil.Event, rid time.Time) error {
		if isCancelled(o) {
			return nil
		}
		ot, err := eventTiming(o)
		if err != nil {
			return err
		}
		if overlaps(ot.start, ot.end, rangeStart, rangeEnd) {
			r := rid
			instances = append(instances, Instance{Event: o, Start: ot.start.UTC(), End: ot.end.UTC(), RecurrenceID: &r})
		}
		return nil
	}
	for _, st := range starts {
		if o, ok := ovs[st.Unix()]; ok {
			used[st.Unix()] = true
			if err := addOverride(o, st); err != nil {
				return nil, false, err
			}
			continue
		}
		en := t.endFor(st)
		if overlaps(st, en, rangeStart, rangeEnd) {
			r := st
			instances = append(instances, Instance{Event: withTimes(ev, t, st, en), Start: st, End: en.UTC(), RecurrenceID: &r})
		}
	}
	for k, o := range ovs {
		if used[k] {
			continue
		}
		rid := time.Unix(k, 0).UTC()
		if !s.isInstance(rid) {
			continue
		}
		if err := addOverride(o, rid); err != nil {
			return nil, false, err
		}
	}
	sort.SliceStable(instances, func(i, j int) bool {
		a, b := instances[i], instances[j]
		if !a.Start.Equal(b.Start) {
			return a.Start.Before(b.Start)
		}
		return a.RecurrenceID.Before(*b.RecurrenceID)
	})
	truncated = more
	if len(instances) > max {
		instances = instances[:max]
		truncated = true
	}
	return instances, truncated, nil
}

// SeriesUntil returns the end of the last possible instance of a series
// (from COUNT or UNTIL), or nil for an open series. The store keeps it so a
// range query can skip a finished series. For a single event it returns
// the event end.
//
// For COUNT it is exact. For UNTIL it is an upper bound: UNTIL (or the
// last RDATE, when later) plus the event length. Overrides that move an
// instance later are not included; the caller must allow for them.
func SeriesUntil(ev csil.Event) (*time.Time, error) {
	t, err := eventTiming(ev)
	if err != nil {
		return nil, err
	}
	if !isRecurring(ev) {
		e := t.end.UTC()
		return &e, nil
	}
	s, err := newSeries(ev, t)
	if err != nil {
		return nil, err
	}
	last := t.start
	for _, r := range s.rdates {
		if r.After(last) {
			last = r
		}
	}
	if s.opt != nil {
		switch {
		case s.opt.Count > 0:
			if l, ok := lastCountInstance(*s.opt); ok && l.After(last) {
				last = l
			}
		case !s.opt.Until.IsZero():
			if s.opt.Until.After(last) {
				last = s.opt.Until
			}
		default:
			return nil, nil
		}
	}
	e := t.endFor(last).UTC()
	return &e, nil
}
