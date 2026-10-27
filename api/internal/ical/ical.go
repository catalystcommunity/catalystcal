// Package ical converts between catalystcal events (the generated csil types) and
// iCalendar (RFC 5545), and expands recurring series.
//
// It is pure: no store, no network, no clock except the arguments. The
// services and the subscription worker call it.
//
// Conventions shared with the store and the services:
//
//   - A timed event has Start and End (RFC3339 UTC, whole seconds) and an
//     optional Timezone (IANA). Recurrence expands in that zone, so a weekly
//     09:00 meeting stays at 09:00 local across a daylight-saving change.
//     No Timezone means UTC.
//   - An all-day event has AllDay = true, StartDate and EndDate
//     (YYYY-MM-DD, end exclusive). Start and End are 00:00:00Z of those
//     dates. Expansion steps whole days.
//   - A series master has Recurrence set. An override of one instance has
//     RecurrenceId set (the original start of the instance, RFC3339 UTC) and
//     the same Uid as the master.
//   - EXDATE and RDATE values are RFC3339 UTC instants of instance starts.
package ical

import (
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// TimeLayout is the one text layout for instants: store, wire, and here.
const TimeLayout = "2006-01-02T15:04:05Z"

// DateLayout is the layout for all-day dates.
const DateLayout = "2006-01-02"

// Instance is one occurrence of an event in a time range.
type Instance struct {
	// Event is the master, or the override that replaces this instance.
	// For a master, it is a copy with Start and End (and StartDate and
	// EndDate for all-day) set to this instance.
	Event csil.Event
	Start time.Time
	End   time.Time
	// RecurrenceID is the original start of the instance, for a series.
	// Nil for a single event.
	RecurrenceID *time.Time
}

// Parsed is the result of Parse.
type Parsed struct {
	// CalendarName is X-WR-CALNAME, when present.
	CalendarName string
	// Events holds masters, single events, and overrides (RecurrenceId set).
	// EventId and CalendarId are empty; the caller sets CalendarId.
	Events []csil.Event
	// Warnings name parts that were skipped, for example a VTODO or an
	// event with no DTSTART.
	Warnings []string
}

// EncodeOptions controls Encode.
type EncodeOptions struct {
	// ProdID defaults to "-//catalystcal//catalystcal calendar//EN".
	ProdID string
	// CalendarName sets X-WR-CALNAME.
	CalendarName string
	// Method is empty, "PUBLISH", "REQUEST", "REPLY", or "CANCEL" (iTIP).
	Method string
	// BusyOnly writes each event as "Busy" with no details (free_busy
	// role and private events in a shared feed).
	BusyOnly bool
	// Now is the DTSTAMP. Zero means time.Now().
	Now time.Time
}

// AttendeeURI returns the calendar-user address catalystcal writes for a person
// with a LinkKeys identity and no email: "linkkeys:<subject>@<domain>".
// Parse reads it back into Attendee.Identity.
func AttendeeURI(domain, subject string) string {
	return "linkkeys:" + subject + "@" + domain
}
