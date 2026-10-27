// Package access holds the one effective-access function of catalystcal. It
// implements docs/access-model.md. Every service asks it; no service repeats
// the rules.
//
// The function is pure. The caller collects the facts (the calendar owner,
// the share for the effective user, the delegation calendar limit) and
// passes them in.
package access

import (
	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// Role is an access level, in increasing order.
type Role int

const (
	None Role = iota
	FreeBusy
	Read
	Write
	Manage
	Owner
)

// CSIL returns the wire name.
func (r Role) CSIL() csil.AccessRole {
	switch r {
	case FreeBusy:
		return "free_busy"
	case Read:
		return "read"
	case Write:
		return "write"
	case Manage:
		return "manage"
	case Owner:
		return "owner"
	}
	return "none"
}

func (r Role) String() string { return string(r.CSIL()) }

// FromShareRole maps a ShareRole word. An unknown word is None.
func FromShareRole(s string) Role {
	switch s {
	case "free_busy":
		return FreeBusy
	case "read":
		return Read
	case "write":
		return Write
	case "manage":
		return Manage
	}
	return None
}

// Scope words. They match the CSIL Scope enumeration.
const (
	ScopeCalendarsRead   = "calendars.read"
	ScopeFreeBusyRead    = "freebusy.read"
	ScopeEventsWrite     = "events.write"
	ScopeEventsRespond   = "events.respond"
	ScopeCalendarsManage = "calendars.manage"
)

// AllScopes is every scope word, in the CSIL order.
var AllScopes = []string{ScopeCalendarsRead, ScopeFreeBusyRead, ScopeEventsWrite, ScopeEventsRespond, ScopeCalendarsManage}

// ValidScope reports whether s is a scope word.
func ValidScope(s string) bool {
	for _, v := range AllScopes {
		if v == s {
			return true
		}
	}
	return false
}

// Facts are what the store knows about one calendar for one caller.
type Facts struct {
	CalendarID string
	OwnerID    string
	// ShareRole is the role of the share on this calendar for the
	// effective user's identity (the principal for a delegated bearer).
	ShareRole Role
	// DelegationCalendars is the calendar limit of the bearer's delegation.
	// Nil or empty means every calendar of the principal. Ignored for a
	// self bearer. An act-as grant has no limit.
	DelegationCalendars []string
}

// Access is the effective access of a caller on one calendar.
type Access struct {
	Role Role
	Path csil.AccessPath
	// SeePrivate is true when private and confidential events show their
	// details (the owner, or a delegate with calendars.read).
	SeePrivate bool
	// Manage allows calendar settings, the feed, and shares.
	Manage bool
	// DeleteCalendar allows deleting the calendar. Only the owner with a
	// self bearer has it.
	DeleteCalendar bool
}

// CanSee is true for any access above None.
func (a Access) CanSee() bool { return a.Role > None }

// CanReadDetails is true when events show more than busy time.
func (a Access) CanReadDetails() bool { return a.Role >= Read }

// CanWrite is true when the caller can create, change, and delete events.
func (a Access) CanWrite() bool { return a.Role >= Write }

func has(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// Effective is THE effective-access function (docs/access-model.md,
// "Effective access, in one function").
//
//  1. The effective user owns the calendar:
//     - a self bearer is owner;
//     - a delegated bearer on a calendar inside its delegation maps its
//     scopes: calendars.manage + events.write is owner without
//     delete-calendar, events.write is write, calendars.read is read with
//     private events visible, freebusy.read is free_busy, else none.
//     A calendar outside the delegation is none.
//  2. Else a share for the effective user's identity gives its role. A
//     delegated bearer uses the principal's share only as far as its
//     scopes allow (calendars.read for read, events.write for write,
//     events.write + calendars.manage for manage, freebusy.read for
//     free_busy), and a delegation limited to a calendar list never
//     reaches other people's calendars.
//  3. Else none.
func Effective(id *auth.Identity, f Facts) Access {
	if id == nil {
		return Access{Role: None}
	}
	delegated := id.IsDelegated()
	if id.EffectiveUserID() == f.OwnerID {
		if !delegated {
			return Access{Role: Owner, Path: "owner", SeePrivate: true, Manage: true, DeleteCalendar: true}
		}
		if !inDelegation(f) {
			return Access{Role: None}
		}
		read := has(id.Scopes, ScopeCalendarsRead)
		write := has(id.Scopes, ScopeEventsWrite)
		manage := has(id.Scopes, ScopeCalendarsManage)
		fb := has(id.Scopes, ScopeFreeBusyRead) || read
		a := Access{Path: "delegation", SeePrivate: read}
		switch {
		case manage && write:
			a.Role = Owner
		case write:
			a.Role = Write
		case read:
			a.Role = Read
		case fb:
			a.Role = FreeBusy
		default:
			return Access{Role: None}
		}
		a.Manage = manage
		return a
	}

	if f.ShareRole == None {
		return Access{Role: None}
	}
	role := f.ShareRole
	if delegated {
		if len(f.DelegationCalendars) > 0 {
			return Access{Role: None}
		}
		role = min(role, delegatedShareCap(id.Scopes))
		if role == None {
			return Access{Role: None}
		}
	}
	return Access{Role: role, Path: "share", Manage: role >= Manage}
}

func inDelegation(f Facts) bool {
	if len(f.DelegationCalendars) == 0 {
		return true
	}
	for _, c := range f.DelegationCalendars {
		if c == f.CalendarID {
			return true
		}
	}
	return false
}

// delegatedShareCap is the highest share role a delegated bearer may use
// on someone else's calendar.
func delegatedShareCap(scopes []string) Role {
	switch {
	case has(scopes, ScopeEventsWrite) && has(scopes, ScopeCalendarsManage):
		return Manage
	case has(scopes, ScopeEventsWrite):
		return Write
	case has(scopes, ScopeCalendarsRead):
		return Read
	case has(scopes, ScopeFreeBusyRead):
		return FreeBusy
	}
	return None
}

// EventView is what a caller may see of one event.
type EventView struct {
	Role Role
	Path csil.AccessPath
	// BusyOnly hides every detail but the time.
	BusyOnly bool
	// Invited is true when the caller sees the event only as an attendee.
	Invited bool
}

// CanSee is true for any view above None.
func (v EventView) CanSee() bool { return v.Role > None }

// ViewEvent combines the calendar access with the event's visibility and
// the caller's attendance.
//
//   - free_busy access shows busy time only.
//   - A private or confidential event shows busy time only, unless the
//     caller sees private events (the owner, or a delegate with
//     calendars.read) or is an attendee.
//   - An attendee who has no other access reads the one event. A delegated
//     bearer reads the principal's invitations only with calendars.read or
//     events.respond.
func ViewEvent(id *auth.Identity, cal Access, private, attendee bool) EventView {
	attendeeOK := attendee && id != nil &&
		(!id.IsDelegated() || has(id.Scopes, ScopeCalendarsRead) || has(id.Scopes, ScopeEventsRespond))
	switch {
	case cal.Role >= Read:
		return EventView{Role: cal.Role, Path: cal.Path, BusyOnly: private && !cal.SeePrivate && !attendeeOK}
	case attendeeOK:
		return EventView{Role: Read, Path: "attendee", Invited: true}
	case cal.Role == FreeBusy:
		return EventView{Role: FreeBusy, Path: cal.Path, BusyOnly: true}
	}
	return EventView{Role: None}
}

// IsPrivate reports whether a visibility hides details from shares.
func IsPrivate(v *csil.EventVisibility) bool {
	return v != nil && (*v == "private" || *v == "confidential")
}
