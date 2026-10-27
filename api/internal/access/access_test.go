package access

import (
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/auth"
)

const (
	alice = "user-alice" // owner / principal
	bob   = "user-bob"   // delegate or grantee
	carol = "user-carol" // a third party
)

func self(user string) *auth.Identity {
	return &auth.Identity{UserID: user, Via: auth.ViaSelf}
}

// delegated is bob acting as alice.
func delegated(scopes ...string) *auth.Identity {
	return &auth.Identity{UserID: bob, Via: auth.ViaDelegation, PrincipalID: alice, DelegationID: "d1", Scopes: scopes}
}

func grant(scopes ...string) *auth.Identity {
	return &auth.Identity{UserID: alice, Via: auth.ViaActAsGrant, PrincipalID: alice, GrantID: "g1", Scopes: scopes}
}

func TestEffective(t *testing.T) {
	aliceCal := Facts{CalendarID: "cal-a", OwnerID: alice}
	aliceCalLimited := Facts{CalendarID: "cal-a", OwnerID: alice, DelegationCalendars: []string{"cal-a"}}
	aliceOtherLimited := Facts{CalendarID: "cal-a2", OwnerID: alice, DelegationCalendars: []string{"cal-a"}}
	carolCal := func(share Role, limit ...string) Facts {
		return Facts{CalendarID: "cal-c", OwnerID: carol, ShareRole: share, DelegationCalendars: limit}
	}

	cases := []struct {
		name   string
		id     *auth.Identity
		facts  Facts
		role   Role
		path   string
		manage bool
		del    bool
		priv   bool
	}{
		// Owner.
		{"owner self", self(alice), aliceCal, Owner, "owner", true, true, true},
		{"nil identity", nil, aliceCal, None, "", false, false, false},
		{"stranger no share", self(bob), aliceCal, None, "", false, false, false},

		// Every share role, as themselves.
		{"share free_busy", self(bob), Facts{CalendarID: "cal-a", OwnerID: alice, ShareRole: FreeBusy}, FreeBusy, "share", false, false, false},
		{"share read", self(bob), Facts{CalendarID: "cal-a", OwnerID: alice, ShareRole: Read}, Read, "share", false, false, false},
		{"share write", self(bob), Facts{CalendarID: "cal-a", OwnerID: alice, ShareRole: Write}, Write, "share", false, false, false},
		{"share manage", self(bob), Facts{CalendarID: "cal-a", OwnerID: alice, ShareRole: Manage}, Manage, "share", true, false, false},

		// Every delegation scope on the principal's own calendar.
		{"delegation calendars.read", delegated(ScopeCalendarsRead), aliceCal, Read, "delegation", false, false, true},
		{"delegation freebusy.read", delegated(ScopeFreeBusyRead), aliceCal, FreeBusy, "delegation", false, false, false},
		{"delegation events.write", delegated(ScopeEventsWrite), aliceCal, Write, "delegation", false, false, false},
		{"delegation events.write+calendars.read", delegated(ScopeEventsWrite, ScopeCalendarsRead), aliceCal, Write, "delegation", false, false, true},
		{"delegation events.respond only", delegated(ScopeEventsRespond), aliceCal, None, "", false, false, false},
		{"delegation calendars.manage only", delegated(ScopeCalendarsManage), aliceCal, None, "", false, false, false},
		{"delegation manage+write is owner without delete", delegated(ScopeCalendarsManage, ScopeEventsWrite), aliceCal, Owner, "delegation", true, false, false},
		{"delegation manage+read", delegated(ScopeCalendarsManage, ScopeCalendarsRead), aliceCal, Read, "delegation", true, false, true},
		{"delegation no scopes", delegated(), aliceCal, None, "", false, false, false},

		// Calendar-limited delegation.
		{"limited delegation inside", delegated(ScopeEventsWrite), aliceCalLimited, Write, "delegation", false, false, false},
		{"limited delegation outside", delegated(ScopeEventsWrite, ScopeCalendarsRead), aliceOtherLimited, None, "", false, false, false},

		// The delegate's own calendars are not the principal's: a delegated
		// bearer acts as alice, so bob's calendar is a stranger's.
		{"delegated bearer on delegate's own calendar", delegated(ScopeEventsWrite), Facts{CalendarID: "cal-b", OwnerID: bob}, None, "", false, false, false},

		// Delegated bearer on a third party's calendar shared with the principal.
		{"delegated on shared write, scope write", delegated(ScopeEventsWrite), carolCal(Write), Write, "share", false, false, false},
		{"delegated on shared manage, scope write", delegated(ScopeEventsWrite), carolCal(Manage), Write, "share", false, false, false},
		{"delegated on shared manage, scope write+manage", delegated(ScopeEventsWrite, ScopeCalendarsManage), carolCal(Manage), Manage, "share", true, false, false},
		{"delegated on shared write, scope read", delegated(ScopeCalendarsRead), carolCal(Write), Read, "share", false, false, false},
		{"delegated on shared read, scope write", delegated(ScopeEventsWrite), carolCal(Read), Read, "share", false, false, false},
		{"delegated on shared read, scope freebusy", delegated(ScopeFreeBusyRead), carolCal(Read), FreeBusy, "share", false, false, false},
		{"delegated on shared read, scope respond", delegated(ScopeEventsRespond), carolCal(Read), None, "", false, false, false},
		{"delegated on shared, calendar-limited delegation", delegated(ScopeEventsWrite), carolCal(Write, "cal-a"), None, "", false, false, false},
		{"delegated, no share", delegated(ScopeEventsWrite, ScopeCalendarsRead), carolCal(None), None, "", false, false, false},

		// Act-as grant: same scope words, no calendar limit.
		{"grant calendars.read", grant(ScopeCalendarsRead), aliceCal, Read, "delegation", false, false, true},
		{"grant events.write", grant(ScopeEventsWrite), aliceCal, Write, "delegation", false, false, false},
		{"grant on shared read", grant(ScopeCalendarsRead), carolCal(Write), Read, "share", false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Effective(c.id, c.facts)
			if got.Role != c.role {
				t.Fatalf("role = %v, want %v", got.Role, c.role)
			}
			if c.role == None {
				return
			}
			if string(got.Path) != c.path {
				t.Errorf("path = %q, want %q", got.Path, c.path)
			}
			if got.Manage != c.manage {
				t.Errorf("manage = %v, want %v", got.Manage, c.manage)
			}
			if got.DeleteCalendar != c.del {
				t.Errorf("delete = %v, want %v", got.DeleteCalendar, c.del)
			}
			if got.SeePrivate != c.priv {
				t.Errorf("see private = %v, want %v", got.SeePrivate, c.priv)
			}
		})
	}
}

func TestViewEvent(t *testing.T) {
	owner := Effective(self(alice), Facts{CalendarID: "c", OwnerID: alice})
	shareRead := Effective(self(bob), Facts{CalendarID: "c", OwnerID: alice, ShareRole: Read})
	shareFB := Effective(self(bob), Facts{CalendarID: "c", OwnerID: alice, ShareRole: FreeBusy})
	delegRead := Effective(delegated(ScopeCalendarsRead), Facts{CalendarID: "c", OwnerID: alice})
	delegWrite := Effective(delegated(ScopeEventsWrite), Facts{CalendarID: "c", OwnerID: alice})
	none := Access{Role: None}

	cases := []struct {
		name     string
		id       *auth.Identity
		cal      Access
		private  bool
		attendee bool
		role     Role
		busy     bool
		invited  bool
	}{
		{"owner private", self(alice), owner, true, false, Owner, false, false},
		{"share read public", self(bob), shareRead, false, false, Read, false, false},
		{"share read private is busy only", self(bob), shareRead, true, false, Read, true, false},
		{"share read private but attendee", self(bob), shareRead, true, true, Read, false, false},
		{"share free_busy", self(bob), shareFB, false, false, FreeBusy, true, false},
		{"share free_busy, attendee reads", self(bob), shareFB, false, true, Read, false, true},
		{"delegate calendars.read sees private", delegated(ScopeCalendarsRead), delegRead, true, false, Read, false, false},
		{"delegate events.write private is busy only", delegated(ScopeEventsWrite), delegWrite, true, false, Write, true, false},
		{"no access", self(carol), none, false, false, None, false, false},
		{"attendee only", self(carol), none, true, true, Read, false, true},
		{"delegated attendee with respond", delegated(ScopeEventsRespond), none, false, true, Read, false, true},
		{"delegated attendee with write only", delegated(ScopeEventsWrite), none, false, true, None, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := ViewEvent(c.id, c.cal, c.private, c.attendee)
			if v.Role != c.role || v.BusyOnly != c.busy || v.Invited != c.invited {
				t.Fatalf("got role=%v busy=%v invited=%v, want role=%v busy=%v invited=%v",
					v.Role, v.BusyOnly, v.Invited, c.role, c.busy, c.invited)
			}
		})
	}
}
