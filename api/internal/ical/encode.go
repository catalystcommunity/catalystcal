package ical

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// param is one property parameter. Encode writes parameters in the
// order given.
type param struct{ name, value string }

// lineWriter writes RFC 5545 content lines: CRLF line ends, and lines
// folded at 75 octets. It does not cut a UTF-8 character.
//
// This package does not use the go-ical encoder. That encoder does not
// fold long lines and sorts properties by name.
type lineWriter struct {
	b bytes.Buffer
}

func (w *lineWriter) line(name string, params []param, value string) {
	var sb strings.Builder
	sb.WriteString(name)
	for _, p := range params {
		sb.WriteByte(';')
		sb.WriteString(p.name)
		sb.WriteByte('=')
		sb.WriteString(quoteParam(p.value))
	}
	sb.WriteByte(':')
	sb.WriteString(stripCRLF(value))
	w.fold(sb.String())
}

func (w *lineWriter) fold(s string) {
	limit := 75
	for len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		w.b.WriteString(s[:cut])
		w.b.WriteString("\r\n ")
		s = s[cut:]
		limit = 74
	}
	w.b.WriteString(s)
	w.b.WriteString("\r\n")
}

func stripCRLF(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	return strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(s)
}

// quoteParam makes a parameter value safe. RFC 5545 has no escape for a
// double quote in a parameter value, so it becomes a single quote.
func quoteParam(v string) string {
	v = strings.ReplaceAll(stripCRLF(v), `"`, "'")
	if strings.ContainsAny(v, ";:,") {
		return `"` + v + `"`
	}
	return v
}

// escapeText escapes a TEXT value (RFC 5545 3.3.11).
func escapeText(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', ';', ',':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			sb.WriteString(`\n`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// utcZone reports if an IANA name means UTC. Encode writes such an event
// with "Z" times and no VTIMEZONE.
func utcZone(name string) bool {
	switch strings.ToUpper(name) {
	case "", "UTC", "ETC/UTC", "GMT", "ETC/GMT", "Z":
		return true
	}
	return false
}

// eventZone returns the TZID Encode writes for a timed event, or "".
func eventZone(ev csil.Event) string {
	if isAllDay(ev) || ev.Timezone == nil || utcZone(*ev.Timezone) {
		return ""
	}
	return *ev.Timezone
}

// Encode writes an iCalendar object with one VEVENT per event (masters,
// single events, and overrides) and one VTIMEZONE for each zone used. The
// output parses back with Parse to equal events (round-trip test).
//
// Rules:
//   - A timed event with a Timezone gets TZID-qualified DTSTART, DTEND,
//     RECURRENCE-ID, EXDATE, and RDATE. A timed event with no Timezone
//     (or "UTC") gets UTC ("Z") values. An all-day event gets DATE values.
//   - RRULE is written as stored. Only UNTIL changes, when its form does
//     not match DTSTART (see normalizeRRuleForWrite).
//   - The VTIMEZONE comes from the Go tz database (see writeVTimezone).
//     It covers the years of the events, one year before, and one year
//     after the later of the last event and Now.
//   - UID is Event.Uid. With no Uid it is made from EventId, else from a
//     hash of the title and start.
//   - ORGANIZER comes from Event.Organizer (AttendeeURI of its domain and
//     subject). With no Organizer, the first attendee with role "chair"
//     is the organizer.
//   - An attendee with an email gets a "mailto:" address. When it also
//     has an identity, the parameter X-CATALYSTCAL-IDENTITY holds the
//     AttendeeURI form, so Parse gets both back. An attendee with only an
//     identity gets the AttendeeURI form as its address.
//   - BusyOnly writes SUMMARY "Busy" and keeps only UID, times,
//     recurrence, STATUS, and TRANSP. It drops description, location,
//     URL, conference, categories, color, class, organizer, attendees,
//     and alarms.
//   - An EMAIL alarm has no ATTENDEE unless there is an organizer.
func Encode(events []csil.Event, opts EncodeOptions) ([]byte, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC().Truncate(time.Second)
	prodID := opts.ProdID
	if prodID == "" {
		prodID = "-//catalystcal//catalystcal calendar//EN"
	}

	type zoneSpan struct {
		loc        *time.Location
		minY, maxY int
	}
	zones := map[string]*zoneSpan{}
	timings := make([]timing, len(events))
	for i, ev := range events {
		t, err := eventTiming(ev)
		if err != nil {
			return nil, fmt.Errorf("ical: event %d (%q): %w", i, ev.Title, err)
		}
		timings[i] = t
		if z := eventZone(ev); z != "" {
			zs := zones[z]
			if zs == nil {
				zs = &zoneSpan{loc: t.loc, minY: t.start.Year(), maxY: t.start.Year()}
				zones[z] = zs
			}
			zs.minY = min(zs.minY, t.start.Year())
			zs.maxY = max(zs.maxY, t.end.Year())
		}
	}

	w := &lineWriter{}
	w.line("BEGIN", nil, "VCALENDAR")
	w.line("VERSION", nil, "2.0")
	w.line("PRODID", nil, prodID)
	w.line("CALSCALE", nil, "GREGORIAN")
	if opts.Method != "" {
		m := strings.ToUpper(opts.Method)
		switch m {
		case "PUBLISH", "REQUEST", "REPLY", "CANCEL":
		default:
			return nil, fmt.Errorf("ical: unsupported METHOD %q", opts.Method)
		}
		w.line("METHOD", nil, m)
	}
	if opts.CalendarName != "" {
		w.line("X-WR-CALNAME", nil, escapeText(opts.CalendarName))
	}
	names := make([]string, 0, len(zones))
	for n := range zones {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		zs := zones[n]
		writeVTimezone(w, n, zs.loc, zs.minY-1, max(zs.maxY, now.Year())+1)
	}
	for i, ev := range events {
		if err := writeEvent(w, ev, timings[i], opts, now); err != nil {
			return nil, fmt.Errorf("ical: event %d (%q): %w", i, ev.Title, err)
		}
	}
	w.line("END", nil, "VCALENDAR")
	return w.b.Bytes(), nil
}

// dateValue writes an instant in the form that matches the event DTSTART.
func dateValue(t timing, zone string, at time.Time) ([]param, string) {
	switch {
	case t.allDay:
		return []param{{"VALUE", "DATE"}}, at.UTC().Format("20060102")
	case zone != "":
		return []param{{"TZID", zone}}, at.In(t.loc).Format("20060102T150405")
	default:
		return nil, at.UTC().Format("20060102T150405Z")
	}
}

func dateList(t timing, zone string, vals []csil.Timestamp) ([]param, string, error) {
	var ps []param
	var out []string
	for _, v := range vals {
		at, err := parseInstant(string(v))
		if err != nil {
			return nil, "", err
		}
		p, s := dateValue(t, zone, at)
		ps = p
		out = append(out, s)
	}
	return ps, strings.Join(out, ","), nil
}

func eventUID(ev csil.Event) string {
	if ev.Uid != nil && *ev.Uid != "" {
		return *ev.Uid
	}
	if ev.EventId != "" {
		return string(ev.EventId) + "@catalystcal"
	}
	start := ""
	if ev.Start != nil {
		start = string(*ev.Start)
	}
	h := sha1.Sum([]byte(ev.Title + "\x00" + start))
	return hex.EncodeToString(h[:]) + "@catalystcal"
}

func writeEvent(w *lineWriter, ev csil.Event, t timing, opts EncodeOptions, now time.Time) error {
	busy := opts.BusyOnly
	zone := eventZone(ev)
	w.line("BEGIN", nil, "VEVENT")
	w.line("UID", nil, eventUID(ev))
	w.line("DTSTAMP", nil, now.Format("20060102T150405Z"))
	w.line("SEQUENCE", nil, strconv.FormatInt(ev.Sequence, 10))
	if ts, err := parseInstant(string(ev.CreatedAt)); err == nil && ev.CreatedAt != "" {
		w.line("CREATED", nil, ts.Format("20060102T150405Z"))
	}
	if ts, err := parseInstant(string(ev.UpdatedAt)); err == nil && ev.UpdatedAt != "" {
		w.line("LAST-MODIFIED", nil, ts.Format("20060102T150405Z"))
	}
	p, v := dateValue(t, zone, t.start)
	w.line("DTSTART", p, v)
	p, v = dateValue(t, zone, t.end)
	w.line("DTEND", p, v)
	if ev.RecurrenceId != nil {
		rid, err := parseInstant(string(*ev.RecurrenceId))
		if err != nil {
			return err
		}
		p, v = dateValue(t, zone, rid)
		w.line("RECURRENCE-ID", p, v)
	}
	if rec := ev.Recurrence; rec != nil && ev.RecurrenceId == nil {
		if rec.Rrule != "" {
			rule, err := normalizeRRuleForWrite(rec.Rrule, t.allDay, t.loc)
			if err != nil {
				return err
			}
			w.line("RRULE", nil, rule)
		}
		if len(rec.Exdates) > 0 {
			p, v, err := dateList(t, zone, rec.Exdates)
			if err != nil {
				return err
			}
			w.line("EXDATE", p, v)
		}
		if len(rec.Rdates) > 0 {
			p, v, err := dateList(t, zone, rec.Rdates)
			if err != nil {
				return err
			}
			w.line("RDATE", p, v)
		}
	}
	if busy {
		w.line("SUMMARY", nil, "Busy")
	} else {
		w.line("SUMMARY", nil, escapeText(ev.Title))
		if ev.Description != nil && *ev.Description != "" {
			w.line("DESCRIPTION", nil, escapeText(*ev.Description))
		}
		if ev.Location != nil && *ev.Location != "" {
			w.line("LOCATION", nil, escapeText(*ev.Location))
		}
		if ev.Url != nil && *ev.Url != "" {
			w.line("URL", []param{{"VALUE", "URI"}}, *ev.Url)
		}
		if ev.ConferenceUrl != nil && *ev.ConferenceUrl != "" {
			w.line("CONFERENCE", []param{{"VALUE", "URI"}, {"FEATURE", "VIDEO"}}, *ev.ConferenceUrl)
		}
		if len(ev.Categories) > 0 {
			cs := make([]string, len(ev.Categories))
			for i, c := range ev.Categories {
				cs[i] = escapeText(c)
			}
			w.line("CATEGORIES", nil, strings.Join(cs, ","))
		}
		if ev.Color != nil && *ev.Color != "" {
			w.line("COLOR", nil, escapeText(*ev.Color))
		}
	}
	status := "CONFIRMED"
	if ev.Status != nil {
		switch *ev.Status {
		case "tentative":
			status = "TENTATIVE"
		case "cancelled":
			status = "CANCELLED"
		}
	}
	w.line("STATUS", nil, status)
	transp := "OPAQUE"
	if ev.Transparency != nil && *ev.Transparency == "transparent" {
		transp = "TRANSPARENT"
	}
	w.line("TRANSP", nil, transp)
	if busy {
		w.line("END", nil, "VEVENT")
		return nil
	}
	if ev.Visibility != nil {
		switch *ev.Visibility {
		case "public":
			w.line("CLASS", nil, "PUBLIC")
		case "private":
			w.line("CLASS", nil, "PRIVATE")
		case "confidential":
			w.line("CLASS", nil, "CONFIDENTIAL")
		}
	}

	orgAddr := ""
	switch {
	case ev.Organizer != nil && ev.Organizer.Domain != "" && ev.Organizer.Subject != "":
		orgAddr = AttendeeURI(ev.Organizer.Domain, ev.Organizer.Subject)
		var ps []param
		if ev.Organizer.DisplayName != nil && *ev.Organizer.DisplayName != "" {
			ps = append(ps, param{"CN", *ev.Organizer.DisplayName})
		}
		w.line("ORGANIZER", ps, orgAddr)
	default:
		for _, a := range ev.Attendees {
			if a.Role != nil && *a.Role == "chair" {
				addr, _ := attendeeAddress(a)
				if addr == "" {
					continue
				}
				orgAddr = addr
				var ps []param
				if a.Name != nil && *a.Name != "" {
					ps = append(ps, param{"CN", *a.Name})
				}
				w.line("ORGANIZER", ps, addr)
				break
			}
		}
	}
	for _, a := range ev.Attendees {
		addr, idParam := attendeeAddress(a)
		if addr == "" {
			continue
		}
		var ps []param
		if a.Name != nil && *a.Name != "" {
			ps = append(ps, param{"CN", *a.Name})
		}
		ps = append(ps, param{"CUTYPE", "INDIVIDUAL"})
		ps = append(ps, param{"ROLE", roleOut(a.Role)})
		ps = append(ps, param{"PARTSTAT", partstatOut(a.Status)})
		rsvp := "TRUE"
		if a.Rsvp != nil && !*a.Rsvp {
			rsvp = "FALSE"
		}
		ps = append(ps, param{"RSVP", rsvp})
		if idParam != "" {
			ps = append(ps, param{"X-CATALYSTCAL-IDENTITY", idParam})
		}
		w.line("ATTENDEE", ps, addr)
	}
	for _, r := range ev.Reminders {
		if r.MinutesBefore < 0 {
			continue
		}
		email := r.Method != nil && *r.Method == "email"
		w.line("BEGIN", nil, "VALARM")
		if email {
			w.line("ACTION", nil, "EMAIL")
			w.line("SUMMARY", nil, escapeText("Reminder: "+ev.Title))
		} else {
			w.line("ACTION", nil, "DISPLAY")
		}
		w.line("DESCRIPTION", nil, escapeText(ev.Title))
		w.line("TRIGGER", nil, formatTrigger(r.MinutesBefore))
		if email && orgAddr != "" {
			w.line("ATTENDEE", nil, orgAddr)
		}
		w.line("END", nil, "VALARM")
	}
	w.line("END", nil, "VEVENT")
	return nil
}

// attendeeAddress returns the calendar-user address of an attendee, and
// the X-CATALYSTCAL-IDENTITY value when the address is an email and the
// attendee also has an identity.
func attendeeAddress(a csil.Attendee) (addr, identity string) {
	if a.Identity != nil && a.Identity.Domain != "" && a.Identity.UserId != nil && *a.Identity.UserId != "" {
		identity = AttendeeURI(a.Identity.Domain, *a.Identity.UserId)
	}
	if a.Email != nil && *a.Email != "" {
		return "mailto:" + *a.Email, identity
	}
	return identity, ""
}

func roleOut(r *csil.AttendeeRole) string {
	if r != nil {
		switch *r {
		case "chair":
			return "CHAIR"
		case "optional":
			return "OPT-PARTICIPANT"
		case "non_participant":
			return "NON-PARTICIPANT"
		}
	}
	return "REQ-PARTICIPANT"
}

func partstatOut(s *csil.ParticipationStatus) string {
	if s != nil {
		switch *s {
		case "accepted":
			return "ACCEPTED"
		case "declined":
			return "DECLINED"
		case "tentative":
			return "TENTATIVE"
		case "delegated":
			return "DELEGATED"
		}
	}
	return "NEEDS-ACTION"
}

// formatTrigger writes minutes before start as a negative duration.
func formatTrigger(min int64) string {
	switch {
	case min == 0:
		return "PT0S"
	case min%(7*1440) == 0:
		return fmt.Sprintf("-P%dW", min/(7*1440))
	case min%1440 == 0:
		return fmt.Sprintf("-P%dD", min/1440)
	case min%60 == 0:
		return fmt.Sprintf("-PT%dH", min/60)
	}
	return fmt.Sprintf("-PT%dM", min)
}
