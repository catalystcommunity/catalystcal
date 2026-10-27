package ical

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	goical "github.com/emersion/go-ical"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// Notes on go-ical (github.com/emersion/go-ical). Read these before you
// change this file.
//
//   - The decoder unfolds lines and splits parameters. It accepts CRLF and
//     LF line ends. Property and parameter names come back in upper case.
//   - Prop.Text returns only the part before the first unescaped comma,
//     and it fails on an unknown escape. Real files have both. This
//     package unescapes text itself (unescapeText).
//   - The decoder can panic on a malformed parameter. Parse recovers and
//     returns an error.

// maxTitle is the longest title the csil Event type permits.
const maxTitle = 1024

// UntitledTitle is the title Parse gives an event with no SUMMARY. The
// csil Event type requires a title.
const UntitledTitle = "(no title)"

type zoneResult struct {
	loc  *time.Location
	name string // IANA name; "" when the zone is not known
}

type parser struct {
	defLoc   *time.Location
	defZone  string
	vtz      map[string]*goical.Component
	zones    map[string]zoneResult
	warnings []string
}

func (p *parser) warn(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

// Parse reads an iCalendar object. It understands VEVENT with DTSTART
// (DATE, DATE-TIME UTC, floating, or with TZID resolved through the IANA
// database or the object's own VTIMEZONE names), DTEND or DURATION,
// SUMMARY, DESCRIPTION, LOCATION, URL, UID, STATUS, TRANSP, CLASS, RRULE,
// RDATE, EXDATE, RECURRENCE-ID, CATEGORIES, COLOR, ATTENDEE (mailto: and
// the catalystcal identity form, see AttendeeURI), ORGANIZER (as an attendee with
// role chair when it is not the importer), VALARM (display and email,
// relative triggers), SEQUENCE, and X-GOOGLE-CONFERENCE / CONFERENCE. A
// Windows zone name in TZID maps to IANA where it can.
//
// Interpretations:
//   - Floating time (no TZID, no "Z") is in the X-WR-TIMEZONE zone of the
//     calendar, and Timezone is that zone. With no X-WR-TIMEZONE it is UTC.
//   - A UTC DTSTART gives no Timezone, so a series expands in UTC
//     (RFC 5545).
//   - TZID resolves in this order: IANA name, Windows name, Outlook
//     display name, the last path parts of a prefixed TZID, and the
//     X-LIC-LOCATION of the VTIMEZONE. When all fail, the time uses the
//     fixed offset of the VTIMEZONE STANDARD part, Timezone is empty, and
//     a warning names the TZID.
//   - Parse does not know the importer. It always adds the ORGANIZER as an
//     attendee with role "chair" (or sets role "chair" on the attendee
//     with the same address). The caller removes the importer.
//   - The RRULE value is kept as written. EXDATE, RDATE, and RECURRENCE-ID
//     become UTC instants. A DATE value on a timed event takes the clock
//     time of DTSTART.
//   - STATUS, TRANSP, and CLASS are always set (defaults "confirmed",
//     "opaque", "default"). AllDay is always set.
//   - An attendee with RSVP absent gets Rsvp false (the RFC 5545 default).
//   - An alarm that fires after the start, or has ACTION other than
//     DISPLAY, AUDIO, or EMAIL, is dropped.
func Parse(data []byte) (parsed Parsed, err error) {
	defer func() {
		if r := recover(); r != nil {
			parsed = Parsed{}
			err = fmt.Errorf("ical: malformed iCalendar data: %v", r)
		}
	}()
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		data = bytes.ToValidUTF8(data, []byte("�"))
	}
	cal, err := goical.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return Parsed{}, fmt.Errorf("ical: %w", err)
	}
	p := &parser{defLoc: time.UTC, vtz: map[string]*goical.Component{}, zones: map[string]zoneResult{}}
	if v := cal.Props.Get("X-WR-CALNAME"); v != nil {
		parsed.CalendarName = unescapeText(v.Value)
	}
	for _, c := range cal.Children {
		if c.Name == goical.CompTimezone {
			if id := c.Props.Get(goical.PropTimezoneID); id != nil {
				p.vtz[strings.TrimSpace(id.Value)] = c
			}
		}
	}
	if v := cal.Props.Get("X-WR-TIMEZONE"); v != nil {
		if z := p.zone(strings.TrimSpace(v.Value)); z.name != "" {
			p.defLoc, p.defZone = z.loc, z.name
		}
	}
	for _, c := range cal.Children {
		switch c.Name {
		case goical.CompEvent:
			if ev, ok := p.event(c); ok {
				parsed.Events = append(parsed.Events, ev)
			}
		case goical.CompTimezone:
		default:
			uid := ""
			if u := c.Props.Get(goical.PropUID); u != nil {
				uid = u.Value
			}
			p.warn("skipped %s %q: catalystcal imports only VEVENT", c.Name, uid)
		}
	}
	parsed.Warnings = p.warnings
	return parsed, nil
}

// zone resolves a TZID.
func (p *parser) zone(tzid string) zoneResult {
	if z, ok := p.zones[tzid]; ok {
		return z
	}
	z := zoneResult{}
	name := resolveZoneName(tzid)
	vtz := p.vtz[tzid]
	if name == "" && vtz != nil {
		if l := vtz.Props.Get("X-LIC-LOCATION"); l != nil {
			name = resolveZoneName(l.Value)
		}
	}
	if name != "" {
		loc, err := time.LoadLocation(name)
		if err == nil {
			z = zoneResult{loc: loc, name: name}
			if utcZone(name) {
				z.name = ""
			}
		}
	}
	if z.loc == nil {
		off := 0
		if vtz != nil {
			off = vtimezoneOffset(vtz)
		}
		z.loc = time.FixedZone(tzid, off)
		p.warn("unknown time zone %q: used fixed offset %s", tzid, formatOffset(off))
	}
	p.zones[tzid] = z
	return z
}

// vtimezoneOffset returns the TZOFFSETTO of the STANDARD part with the
// latest DTSTART, or of any part when there is no STANDARD.
func vtimezoneOffset(c *goical.Component) int {
	best, bestStart, found := 0, "", false
	for _, sub := range c.Children {
		to := sub.Props.Get(goical.PropTimezoneOffsetTo)
		if to == nil {
			continue
		}
		off, ok := parseOffset(to.Value)
		if !ok {
			continue
		}
		start := ""
		if s := sub.Props.Get(goical.PropDateTimeStart); s != nil {
			start = s.Value
		}
		std := sub.Name == goical.CompTimezoneStandard
		if !found || (std && start >= bestStart) {
			best, bestStart, found = off, start, true
		}
	}
	return best
}

func parseOffset(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 5 && len(s) != 7 {
		return 0, false
	}
	sign := 1
	switch s[0] {
	case '-':
		sign = -1
	case '+':
	default:
		return 0, false
	}
	h, err1 := strconv.Atoi(s[1:3])
	m, err2 := strconv.Atoi(s[3:5])
	sec := 0
	var err3 error
	if len(s) == 7 {
		sec, err3 = strconv.Atoi(s[5:7])
	}
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return sign * (h*3600 + m*60 + sec), true
}

// dtValue is one parsed DATE or DATE-TIME.
type dtValue struct {
	t    time.Time
	date bool
	// zone is the IANA name for a TZID or floating value; "" for UTC.
	zone string
}

func (p *parser) parseDT(v string, params goical.Params) (dtValue, error) {
	v = strings.TrimSpace(v)
	if strings.EqualFold(params.Get("VALUE"), "DATE") || len(v) == 8 {
		t, err := time.Parse("20060102", v)
		if err != nil {
			return dtValue{}, fmt.Errorf("bad date %q", v)
		}
		return dtValue{t: t, date: true}, nil
	}
	if strings.HasSuffix(v, "Z") || strings.HasSuffix(v, "z") {
		t, err := time.Parse("20060102T150405", v[:len(v)-1])
		if err != nil {
			return dtValue{}, fmt.Errorf("bad date-time %q", v)
		}
		return dtValue{t: t.UTC()}, nil
	}
	loc, zone := p.defLoc, p.defZone
	if tzid := params.Get("TZID"); tzid != "" {
		z := p.zone(strings.TrimSpace(tzid))
		loc, zone = z.loc, z.name
	}
	t, err := time.ParseInLocation("20060102T150405", v, loc)
	if err != nil {
		return dtValue{}, fmt.Errorf("bad date-time %q", v)
	}
	return dtValue{t: t, zone: zone}, nil
}

// unescapeText reads a TEXT value. Unlike go-ical Prop.Text, it keeps
// the text after an unescaped comma, and it keeps an unknown escape as
// the plain character.
func unescapeText(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			sb.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n', 'N':
			sb.WriteByte('\n')
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// splitTextList splits a TEXT list at unescaped commas and unescapes each
// part.
func splitTextList(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case ',':
			out = append(out, unescapeText(s[start:i]))
			start = i + 1
		}
	}
	return append(out, unescapeText(s[start:]))
}

func textProp(c *goical.Component, name string) (string, bool) {
	v := c.Props.Get(name)
	if v == nil {
		return "", false
	}
	return unescapeText(v.Value), true
}

func ptr[T any](v T) *T { return &v }

// icalDuration is a parsed RFC 5545 duration. Days are calendar days.
type icalDuration struct {
	neg  bool
	days int
	secs int64
}

func parseDuration(s string) (icalDuration, error) {
	var d icalDuration
	s = strings.ToUpper(strings.TrimSpace(s))
	if strings.HasPrefix(s, "-") {
		d.neg = true
		s = s[1:]
	} else {
		s = strings.TrimPrefix(s, "+")
	}
	if !strings.HasPrefix(s, "P") || len(s) < 3 {
		return d, fmt.Errorf("bad duration %q", s)
	}
	s = s[1:]
	inTime := false
	num := ""
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			num += string(r)
		case r == 'T':
			inTime = true
		default:
			n, err := strconv.Atoi(num)
			if err != nil {
				return d, fmt.Errorf("bad duration %q", s)
			}
			num = ""
			switch {
			case r == 'W' && !inTime:
				d.days += 7 * n
			case r == 'D' && !inTime:
				d.days += n
			case r == 'H' && inTime:
				d.secs += int64(n) * 3600
			case r == 'M' && inTime:
				d.secs += int64(n) * 60
			case r == 'S' && inTime:
				d.secs += int64(n)
			default:
				return d, fmt.Errorf("bad duration %q", s)
			}
		}
	}
	if num != "" {
		return d, fmt.Errorf("bad duration %q", s)
	}
	return d, nil
}

// addTo adds the duration to t. Days are calendar days in the zone of t.
func (d icalDuration) addTo(t time.Time) time.Time {
	if d.neg {
		return t.AddDate(0, 0, -d.days).Add(-time.Duration(d.secs) * time.Second)
	}
	return t.AddDate(0, 0, d.days).Add(time.Duration(d.secs) * time.Second)
}

// approx is the duration as a fixed length (a day is 24 hours).
func (d icalDuration) approx() time.Duration {
	x := time.Duration(d.days)*24*time.Hour + time.Duration(d.secs)*time.Second
	if d.neg {
		return -x
	}
	return x
}

func (p *parser) event(c *goical.Component) (csil.Event, bool) {
	var ev csil.Event
	uid := ""
	if u := c.Props.Get(goical.PropUID); u != nil {
		uid = strings.TrimSpace(u.Value)
		if uid != "" {
			ev.Uid = ptr(uid)
		}
	}
	ds := c.Props.Get(goical.PropDateTimeStart)
	if ds == nil {
		p.warn("skipped VEVENT %q: no DTSTART", uid)
		return ev, false
	}
	start, err := p.parseDT(ds.Value, ds.Params)
	if err != nil {
		p.warn("skipped VEVENT %q: DTSTART: %v", uid, err)
		return ev, false
	}

	// End: DTEND, else DURATION, else the RFC 5545 default.
	var end dtValue
	haveEnd := false
	if de := c.Props.Get(goical.PropDateTimeEnd); de != nil {
		if end, err = p.parseDT(de.Value, de.Params); err == nil {
			haveEnd = true
		} else {
			p.warn("VEVENT %q: ignored DTEND: %v", uid, err)
		}
	} else if du := c.Props.Get(goical.PropDuration); du != nil {
		if d, err := parseDuration(du.Value); err == nil && !d.neg {
			end, haveEnd = dtValue{t: d.addTo(start.t), date: start.date, zone: start.zone}, true
		} else {
			p.warn("VEVENT %q: ignored DURATION %q", uid, du.Value)
		}
	}

	allDay := start.date
	ev.AllDay = ptr(allDay)
	var tm timing
	if allDay {
		sd := midnight(start.t)
		ed := sd.AddDate(0, 0, 1)
		if haveEnd {
			e := end.t
			if !end.date {
				e = end.t.In(start.t.Location())
			}
			if e2 := midnight(e); e2.After(sd) {
				ed = e2
			}
		}
		ev.StartDate = ptr(csil.DateText(sd.Format(DateLayout)))
		ev.EndDate = ptr(csil.DateText(ed.Format(DateLayout)))
		ev.Start = ptr(csil.Timestamp(sd.Format(TimeLayout)))
		ev.End = ptr(csil.Timestamp(ed.Format(TimeLayout)))
		tm = timing{allDay: true, loc: time.UTC, start: sd, end: ed}
	} else {
		st := start.t.Truncate(time.Second)
		en := st
		if haveEnd {
			en = end.t
			if end.date {
				en = time.Date(end.t.Year(), end.t.Month(), end.t.Day(), 0, 0, 0, 0, st.Location())
			}
			if en.Before(st) {
				en = st
			}
		}
		ev.Start = ptr(csil.Timestamp(st.UTC().Format(TimeLayout)))
		ev.End = ptr(csil.Timestamp(en.UTC().Format(TimeLayout)))
		if start.zone != "" {
			ev.Timezone = ptr(start.zone)
		}
		tm = timing{loc: st.Location(), start: st, end: en}
	}

	// instant converts an EXDATE, RDATE, or RECURRENCE-ID value to the
	// instance start it names.
	instant := func(v dtValue) time.Time {
		if allDay {
			return midnight(v.t)
		}
		if v.date {
			h, m, s := tm.start.Clock()
			return time.Date(v.t.Year(), v.t.Month(), v.t.Day(), h, m, s, 0, tm.loc).UTC()
		}
		return v.t.UTC()
	}
	dateProps := func(name string) []csil.Timestamp {
		var out []csil.Timestamp
		for _, pr := range c.Props.Values(name) {
			for _, part := range strings.Split(pr.Value, ",") {
				part = strings.TrimSpace(part)
				if i := strings.Index(part, "/"); i >= 0 {
					part = part[:i] // RDATE PERIOD: keep the start
				}
				if part == "" {
					continue
				}
				params := pr.Params
				if strings.EqualFold(params.Get("VALUE"), "PERIOD") {
					params = goical.Params{"TZID": params.Values("TZID")}
				}
				v, err := p.parseDT(part, params)
				if err != nil {
					p.warn("VEVENT %q: ignored %s: %v", uid, name, err)
					continue
				}
				out = append(out, csil.Timestamp(instant(v).Format(TimeLayout)))
			}
		}
		return out
	}

	if rid := c.Props.Get(goical.PropRecurrenceID); rid != nil {
		v, err := p.parseDT(rid.Value, rid.Params)
		if err != nil {
			p.warn("skipped VEVENT %q: RECURRENCE-ID: %v", uid, err)
			return ev, false
		}
		if strings.EqualFold(rid.Params.Get("RANGE"), "THISANDFUTURE") {
			p.warn("VEVENT %q: RECURRENCE-ID RANGE=THISANDFUTURE read as one instance", uid)
		}
		ev.RecurrenceId = ptr(csil.Timestamp(instant(v).Format(TimeLayout)))
	} else {
		rules := c.Props.Values(goical.PropRecurrenceRule)
		rdates := dateProps(goical.PropRecurrenceDates)
		if len(rules) > 0 || len(rdates) > 0 {
			rec := &csil.Recurrence{Rdates: rdates, Exdates: dateProps(goical.PropExceptionDates)}
			if len(rules) > 0 {
				rule := strings.TrimSpace(rules[0].Value)
				if _, err := parseRuleSpec(rule); err != nil {
					p.warn("VEVENT %q: ignored RRULE: %v", uid, err)
				} else {
					rec.Rrule = rule
				}
				if len(rules) > 1 {
					p.warn("VEVENT %q: used the first of %d RRULEs", uid, len(rules))
				}
			}
			if rec.Rrule != "" || len(rec.Rdates) > 0 {
				ev.Recurrence = rec
			}
		}
	}

	title, _ := textProp(c, goical.PropSummary)
	title = strings.TrimSpace(title)
	if title == "" {
		title = UntitledTitle
	}
	if utf8.RuneCountInString(title) > maxTitle {
		title = string([]rune(title)[:maxTitle])
	}
	ev.Title = title
	if v, ok := textProp(c, goical.PropDescription); ok && v != "" {
		ev.Description = ptr(v)
	}
	if v, ok := textProp(c, goical.PropLocation); ok && v != "" {
		ev.Location = ptr(v)
	}
	if v := c.Props.Get(goical.PropURL); v != nil && strings.TrimSpace(v.Value) != "" {
		ev.Url = ptr(strings.TrimSpace(v.Value))
	}
	if v, ok := textProp(c, goical.PropColor); ok && v != "" {
		ev.Color = ptr(v)
	}
	for _, pr := range c.Props.Values(goical.PropCategories) {
		for _, cat := range splitTextList(pr.Value) {
			if cat = strings.TrimSpace(cat); cat != "" {
				ev.Categories = append(ev.Categories, cat)
			}
		}
	}
	if v := c.Props.Get(goical.PropSequence); v != nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(v.Value), 10, 64); err == nil && n >= 0 {
			ev.Sequence = n
		}
	}

	status := csil.EventStatus("confirmed")
	if v, ok := textProp(c, goical.PropStatus); ok {
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "TENTATIVE":
			status = "tentative"
		case "CANCELLED":
			status = "cancelled"
		}
	}
	ev.Status = &status
	transp := csil.EventTransparency("opaque")
	if v, ok := textProp(c, goical.PropTransparency); ok && strings.EqualFold(strings.TrimSpace(v), "TRANSPARENT") {
		transp = "transparent"
	}
	ev.Transparency = &transp
	vis := csil.EventVisibility("default")
	if v, ok := textProp(c, goical.PropClass); ok {
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "PUBLIC":
			vis = "public"
		case "PRIVATE":
			vis = "private"
		case "CONFIDENTIAL":
			vis = "confidential"
		}
	}
	ev.Visibility = &vis

	ev.ConferenceUrl = conferenceURL(c)
	ev.Attendees = p.attendees(c, uid)
	ev.Reminders = p.reminders(c, uid, tm)
	return ev, true
}

// conferenceURL picks CONFERENCE (a VIDEO one first), then
// X-GOOGLE-CONFERENCE, then the Microsoft Teams URL.
func conferenceURL(c *goical.Component) *string {
	confs := c.Props.Values("CONFERENCE")
	for _, pr := range confs {
		if strings.Contains(strings.ToUpper(strings.Join(pr.Params.Values("FEATURE"), ",")), "VIDEO") {
			return ptr(strings.TrimSpace(pr.Value))
		}
	}
	if len(confs) > 0 {
		return ptr(strings.TrimSpace(confs[0].Value))
	}
	for _, name := range []string{"X-GOOGLE-CONFERENCE", "X-MICROSOFT-SKYPETEAMSMEETINGURL"} {
		if v := c.Props.Get(name); v != nil && strings.TrimSpace(v.Value) != "" {
			return ptr(strings.TrimSpace(v.Value))
		}
	}
	return nil
}

// parseAddress reads a calendar-user address: "mailto:", the catalystcal
// identity form (see AttendeeURI), or a bare email. The EMAIL parameter
// (RFC 7986) and X-CATALYSTCAL-IDENTITY add to it.
func parseAddress(v string, params goical.Params) (email string, ident *csil.IdentityRef) {
	v = strings.TrimSpace(v)
	low := strings.ToLower(v)
	readIdentity := func(s string) *csil.IdentityRef {
		rest := s[len("linkkeys:"):]
		i := strings.LastIndex(rest, "@")
		if i <= 0 || i == len(rest)-1 {
			return nil
		}
		return &csil.IdentityRef{Domain: rest[i+1:], UserId: ptr(rest[:i])}
	}
	switch {
	case strings.HasPrefix(low, "mailto:"):
		email = strings.TrimSpace(v[len("mailto:"):])
	case strings.HasPrefix(low, "linkkeys:"):
		ident = readIdentity(v)
	case strings.Contains(v, "@") && !strings.Contains(v, ":"):
		email = v
	}
	if email == "" {
		email = strings.TrimSpace(params.Get("EMAIL"))
	}
	if x := strings.TrimSpace(params.Get("X-CATALYSTCAL-IDENTITY")); ident == nil && strings.HasPrefix(strings.ToLower(x), "linkkeys:") {
		ident = readIdentity(x)
	}
	return email, ident
}

func sameAddress(a csil.Attendee, email string, ident *csil.IdentityRef) bool {
	if email != "" && a.Email != nil && strings.EqualFold(*a.Email, email) {
		return true
	}
	if ident != nil && a.Identity != nil && a.Identity.UserId != nil && ident.UserId != nil &&
		strings.EqualFold(a.Identity.Domain, ident.Domain) && *a.Identity.UserId == *ident.UserId {
		return true
	}
	return false
}

func (p *parser) attendees(c *goical.Component, uid string) []csil.Attendee {
	var out []csil.Attendee
	for _, pr := range c.Props.Values(goical.PropAttendee) {
		email, ident := parseAddress(pr.Value, pr.Params)
		if email == "" && ident == nil {
			p.warn("VEVENT %q: skipped ATTENDEE %q: no email or identity", uid, pr.Value)
			continue
		}
		a := csil.Attendee{Identity: ident}
		if email != "" {
			a.Email = ptr(email)
		}
		if cn := strings.TrimSpace(pr.Params.Get("CN")); cn != "" {
			a.Name = ptr(cn)
		}
		role := csil.AttendeeRole("required")
		switch strings.ToUpper(pr.Params.Get("ROLE")) {
		case "CHAIR":
			role = "chair"
		case "OPT-PARTICIPANT":
			role = "optional"
		case "NON-PARTICIPANT":
			role = "non_participant"
		}
		a.Role = &role
		st := csil.ParticipationStatus("needs_action")
		switch strings.ToUpper(pr.Params.Get("PARTSTAT")) {
		case "ACCEPTED":
			st = "accepted"
		case "DECLINED":
			st = "declined"
		case "TENTATIVE":
			st = "tentative"
		case "DELEGATED":
			st = "delegated"
		}
		a.Status = &st
		a.Rsvp = ptr(strings.EqualFold(pr.Params.Get("RSVP"), "TRUE"))
		out = append(out, a)
	}
	if pr := c.Props.Get(goical.PropOrganizer); pr != nil {
		email, ident := parseAddress(pr.Value, pr.Params)
		if email != "" || ident != nil {
			chair := csil.AttendeeRole("chair")
			found := false
			for i := range out {
				if sameAddress(out[i], email, ident) {
					out[i].Role = &chair
					found = true
				}
			}
			if !found {
				a := csil.Attendee{Identity: ident, Role: &chair,
					Status: ptr(csil.ParticipationStatus("accepted")), Rsvp: ptr(false)}
				if email != "" {
					a.Email = ptr(email)
				}
				if cn := strings.TrimSpace(pr.Params.Get("CN")); cn != "" {
					a.Name = ptr(cn)
				}
				out = append(out, a)
			}
		}
	}
	return out
}

func (p *parser) reminders(c *goical.Component, uid string, tm timing) []csil.Reminder {
	var out []csil.Reminder
	seen := map[string]bool{}
	for _, al := range c.Children {
		if al.Name != goical.CompAlarm {
			continue
		}
		action, _ := textProp(al, goical.PropAction)
		var method csil.ReminderMethod
		switch strings.ToUpper(strings.TrimSpace(action)) {
		case "DISPLAY", "AUDIO":
			method = "display"
		case "EMAIL":
			method = "email"
		default:
			continue
		}
		tr := al.Props.Get(goical.PropTrigger)
		if tr == nil {
			continue
		}
		var before time.Duration
		val := strings.TrimSpace(tr.Value)
		if strings.EqualFold(tr.Params.Get("VALUE"), "DATE-TIME") || (len(val) > 0 && val[0] >= '0' && val[0] <= '9') {
			v, err := p.parseDT(val, tr.Params)
			if err != nil {
				p.warn("VEVENT %q: skipped VALARM: %v", uid, err)
				continue
			}
			before = tm.start.Sub(v.t)
		} else {
			d, err := parseDuration(val)
			if err != nil {
				p.warn("VEVENT %q: skipped VALARM: %v", uid, err)
				continue
			}
			off := d.approx()
			if strings.EqualFold(tr.Params.Get("RELATED"), "END") {
				off += tm.end.Sub(tm.start)
			}
			before = -off
		}
		if before < 0 {
			continue
		}
		mins := int64(before / time.Minute)
		key := fmt.Sprintf("%d/%s", mins, method)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, csil.Reminder{MinutesBefore: mins, Method: ptr(method)})
	}
	return out
}
