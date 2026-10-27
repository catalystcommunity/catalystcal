package httpapi

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

func scopes(s ...string) []csil.Scope {
	out := make([]csil.Scope, len(s))
	for i, v := range s {
		out[i] = csil.Scope(v)
	}
	return out
}

func devRef(handle string) csil.IdentityRef {
	return csil.IdentityRef{Domain: "dev.local", UserId: sp(handle)}
}

func TestDelegationEndToEnd(t *testing.T) {
	e := newEnv(t)
	alice, aliceLogin := e.login("alice")
	bob, bobLogin := e.login("bob")
	if aliceLogin.Via != "self" || bobLogin.User.Domain != "dev.local" {
		t.Fatalf("login: %+v", aliceLogin)
	}

	a1 := defaultCalendar(t, alice)
	a2 := must(t, call(alice, "CalendarService", "create-calendar", csil.CreateCalendarRequest{Name: "Work"},
		csil.EncodeCreateCalendarRequest, csil.DecodeCalendar))

	// A delegates events.write to B, limited to a1.
	dl := must(t, call(alice, "DelegationService", "create-delegation", csil.Delegation{
		Delegate: devRef("bob"), Scopes: scopes("events.write"), CalendarIds: []csil.CalendarID{a1.CalendarId},
	}, csil.EncodeDelegation, csil.DecodeDelegation))
	if dl.DelegatePerson == nil || dl.DelegatePerson.UserId != bobLogin.User.UserId {
		t.Fatalf("delegate person: %+v", dl.DelegatePerson)
	}

	// Cannot delegate to yourself; scopes must not be empty.
	var r resulter = call(alice, "DelegationService", "create-delegation", csil.Delegation{Delegate: devRef("alice"), Scopes: scopes("events.write")},
		csil.EncodeDelegation, csil.DecodeDelegation)
	wantCode(t, r, 400)
	r = call(alice, "DelegationService", "create-delegation", csil.Delegation{Delegate: devRef("carol")},
		csil.EncodeDelegation, csil.DecodeDelegation)
	wantCode(t, r, 400)

	// B sees the principal in Me, then assumes A.
	me := must(t, call(bob, "AuthService", "me", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeMeResponse))
	if len(me.Principals) != 1 || me.Principals[0].UserId != aliceLogin.User.UserId {
		t.Fatalf("principals: %+v", me.Principals)
	}
	assumed := must(t, call(bob, "AuthService", "assume-principal", csil.AssumePrincipalRequest{PrincipalUserId: aliceLogin.User.UserId},
		csil.EncodeAssumePrincipalRequest, csil.DecodeLoginResponse))
	if assumed.Via != "delegation" || assumed.ActingAs == nil || assumed.ActingAs.UserId != aliceLogin.User.UserId {
		t.Fatalf("assumed: %+v", assumed)
	}
	bAsA := bob.as(assumed.Token)

	// B creates an event as A on a1.
	ev := must(t, createEvent(bAsA, timedEvent(a1.CalendarId, "Dentist", "2026-11-02T16:00:00Z", "2026-11-02T17:00:00Z")))
	if ev.Organizer == nil || ev.Organizer.UserId != aliceLogin.User.UserId {
		t.Fatalf("organizer = %+v, want alice", ev.Organizer)
	}
	if ev.UpdatedVia == nil || *ev.UpdatedVia != "delegation" {
		t.Fatalf("updated_via = %v", ev.UpdatedVia)
	}
	if ev.CreatedBy == nil || ev.CreatedBy.UserId != aliceLogin.User.UserId {
		t.Fatalf("created_by = %+v", ev.CreatedBy)
	}

	// The audit log shows the real actor.
	audit := must(t, call(alice, "AuditService", "list-audit", csil.AuditQuery{DelegatedOnly: ptrBool(true)},
		csil.EncodeAuditQuery, csil.DecodeAuditList))
	found := false
	for _, a := range audit.Entries {
		if a.Operation == "event.CreateEvent" && a.Actor.UserId == bobLogin.User.UserId &&
			a.Principal.UserId == aliceLogin.User.UserId && a.Via == "delegation" && a.Outcome == "ok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no delegated CreateEvent audit entry: %+v", audit.Entries)
	}

	// Refused on A's other calendar (outside the delegation: not found).
	r = createEvent(bAsA, timedEvent(a2.CalendarId, "Nope", "2026-11-02T16:00:00Z", "2026-11-02T17:00:00Z"))
	wantCode(t, r, 404)
	// Refused on DelegationService and on chained assume and profile edits.
	r = call(bAsA, "DelegationService", "list-granted", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeDelegationList)
	wantCode(t, r, 403)
	r = call(bAsA, "AuthService", "assume-principal", csil.AssumePrincipalRequest{PrincipalUserId: aliceLogin.User.UserId},
		csil.EncodeAssumePrincipalRequest, csil.DecodeLoginResponse)
	wantCode(t, r, 403)
	r = call(bAsA, "UserService", "update-profile", csil.UpdateProfileRequest{DisplayName: sp("x")},
		csil.EncodeUpdateProfileRequest, csil.DecodeUser)
	wantCode(t, r, 403)
	// The delegate cannot delete the principal's calendar.
	r = call(bAsA, "CalendarService", "delete-calendar", a1.CalendarId, csil.EncodeCalendarDeleteCalendarRequest, csil.DecodeEmptyResponse)
	wantCode(t, r, 403)

	// Release gives B's own bearer back.
	rel := must(t, call(bAsA, "AuthService", "release-principal", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeLoginResponse))
	if rel.Via != "self" || rel.User.UserId != bobLogin.User.UserId {
		t.Fatalf("release: %+v", rel)
	}

	// A revokes; B's delegated bearer is refused on the next call.
	must(t, call(alice, "DelegationService", "revoke-delegation", dl.DelegationId,
		csil.EncodeDelegationRevokeDelegationRequest, csil.DecodeEmptyResponse))
	r = call(bAsA, "AuthService", "me", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeMeResponse)
	wantUnauthenticated(t, r)
	r = createEvent(bAsA, timedEvent(a1.CalendarId, "After revoke", "2026-11-03T16:00:00Z", "2026-11-03T17:00:00Z"))
	wantUnauthenticated(t, r)
}

func ptrBool(b bool) *bool { return &b }

func TestSharesFreeBusyAndPrivacy(t *testing.T) {
	e := newEnv(t)
	alice, aliceLogin := e.login("alice")
	carol, carolLogin := e.login("carol")
	dave, _ := e.login("dave")
	erin, _ := e.login("erin")
	a1 := defaultCalendar(t, alice)

	must(t, createEvent(alice, timedEvent(a1.CalendarId, "Standup", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")))
	priv := timedEvent(a1.CalendarId, "Doctor", "2026-11-02T18:00:00Z", "2026-11-02T19:00:00Z")
	priv.Visibility = ptrVis("private")
	must(t, createEvent(alice, priv))
	free := timedEvent(a1.CalendarId, "Lunch maybe", "2026-11-02T15:15:00Z", "2026-11-02T16:00:00Z")
	free.Transparency = ptrTransp("transparent")
	must(t, createEvent(alice, free))

	// Before any share, carol cannot see a1 (404, not 403).
	var r resulter = call(carol, "CalendarService", "get-calendar", a1.CalendarId, csil.EncodeCalendarGetCalendarRequest, csil.DecodeCalendar)
	wantCode(t, r, 404)

	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: devRef("carol"), Role: "write"},
		csil.EncodeShare, csil.DecodeShare))
	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: csil.IdentityRef{Domain: "dev.local", Handle: sp("dave")}, Role: "free_busy"},
		csil.EncodeShare, csil.DecodeShare))
	// An unknown handle gives the required message.
	r = call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: csil.IdentityRef{Domain: "dev.local", Handle: sp("nobody")}, Role: "read"},
		csil.EncodeShare, csil.DecodeShare)
	if !strings.Contains(wantCode(t, r, 404).svcErr.Message, "has not signed in") {
		t.Fatalf("message %q", r.res().svcErr.Message)
	}

	// Carol writes as herself: she is the organizer.
	cal := must(t, call(carol, "CalendarService", "get-calendar", a1.CalendarId, csil.EncodeCalendarGetCalendarRequest, csil.DecodeCalendar))
	if cal.Access != "write" || cal.AccessPath != "share" || cal.FeedPath != nil {
		t.Fatalf("carol calendar: %+v", cal)
	}
	ev := must(t, createEvent(carol, timedEvent(a1.CalendarId, "Carol's sync", "2026-11-02T20:00:00Z", "2026-11-02T21:00:00Z")))
	if ev.Organizer == nil || ev.Organizer.UserId != carolLogin.User.UserId || ev.CreatedBy.UserId != carolLogin.User.UserId {
		t.Fatalf("share write organizer: %+v", ev.Organizer)
	}
	if *ev.UpdatedVia != "self" {
		t.Fatalf("updated_via %v", *ev.UpdatedVia)
	}
	// A write share cannot manage the calendar or its shares.
	r = call(carol, "ShareService", "list-shares", a1.CalendarId, csil.EncodeShareListSharesRequest, csil.DecodeShareList)
	wantCode(t, r, 403)

	// Carol sees the private event as busy only.
	occ := must(t, listEvents(carol, "2026-11-01T00:00:00Z", "2026-11-04T00:00:00Z"))
	var sawPrivate bool
	for _, o := range occ.Occurrences {
		if o.Event.EventId == "" {
			t.Fatal("occurrence without event id")
		}
		if o.Event.Visibility != nil && *o.Event.Visibility == "private" {
			sawPrivate = true
			if !o.BusyOnly || o.Event.Title != "Busy" {
				t.Fatalf("private event leaked to a write share: %+v", o)
			}
		}
	}
	if !sawPrivate || len(occ.Occurrences) != 4 {
		t.Fatalf("carol occurrences: %d", len(occ.Occurrences))
	}
	// Search never matches a busy-only event.
	srch := must(t, call(carol, "EventService", "search-events", csil.SearchEventsRequest{Query: "doctor"},
		csil.EncodeSearchEventsRequest, csil.DecodeEventList))
	if len(srch.Events) != 0 {
		t.Fatalf("search leaked a private event: %+v", srch.Events)
	}
	srch = must(t, call(alice, "EventService", "search-events", csil.SearchEventsRequest{Query: "DOCTOR"},
		csil.EncodeSearchEventsRequest, csil.DecodeEventList))
	if len(srch.Events) != 1 {
		t.Fatalf("owner search: %d", len(srch.Events))
	}

	// Dave (free_busy share) sees busy blocks with no details.
	occ = must(t, listEvents(dave, "2026-11-01T00:00:00Z", "2026-11-04T00:00:00Z"))
	for _, o := range occ.Occurrences {
		if !o.BusyOnly || o.Event.Title != "Busy" || o.Event.Description != nil || o.Access != "free_busy" {
			t.Fatalf("free_busy occurrence has details: %+v", o)
		}
	}
	r = createEvent(dave, timedEvent(a1.CalendarId, "x", "2026-11-02T20:00:00Z", "2026-11-02T21:00:00Z"))
	wantCode(t, r, 403)

	// Free/busy: opaque, non-cancelled instances merged; transparent left out.
	q := csil.FreeBusyRequest{People: []csil.IdentityRef{devRef("alice")}, RangeStart: "2026-11-02T00:00:00Z", RangeEnd: "2026-11-03T00:00:00Z"}
	fb := must(t, call(dave, "FreeBusyService", "query", q, csil.EncodeFreeBusyRequest, csil.DecodeFreeBusyResponse))
	p := fb.People[0]
	if !p.Visible || p.Person == nil || p.Person.UserId != aliceLogin.User.UserId {
		t.Fatalf("dave free/busy: %+v", p)
	}
	want := [][2]string{{"2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z"}, {"2026-11-02T18:00:00Z", "2026-11-02T19:00:00Z"}, {"2026-11-02T20:00:00Z", "2026-11-02T21:00:00Z"}}
	if len(p.Busy) != len(want) {
		t.Fatalf("busy = %+v", p.Busy)
	}
	for i, w := range want {
		if string(p.Busy[i].Start) != w[0] || string(p.Busy[i].End) != w[1] {
			t.Fatalf("busy[%d] = %+v, want %v", i, p.Busy[i], w)
		}
	}
	// Erin has no share: not visible. An unknown person: not visible.
	q.People = append(q.People, devRef("ghost"))
	fb = must(t, call(erin, "FreeBusyService", "query", q, csil.EncodeFreeBusyRequest, csil.DecodeFreeBusyResponse))
	if fb.People[0].Visible || len(fb.People[0].Busy) != 0 || fb.People[1].Visible {
		t.Fatalf("erin free/busy: %+v", fb.People)
	}

	// Feed: off for non-managers, then on for the owner; served as text/calendar.
	r = call(carol, "CalendarService", "set-feed", csil.SetFeedRequest{CalendarId: a1.CalendarId, Enabled: true},
		csil.EncodeSetFeedRequest, csil.DecodeCalendar)
	wantCode(t, r, 403)
	withFeed := must(t, call(alice, "CalendarService", "set-feed", csil.SetFeedRequest{CalendarId: a1.CalendarId, Enabled: true},
		csil.EncodeSetFeedRequest, csil.DecodeCalendar))
	if withFeed.FeedPath == nil || !strings.HasPrefix(*withFeed.FeedPath, "/ics/") || !strings.HasSuffix(*withFeed.FeedPath, ".ics") {
		t.Fatalf("feed path %v", withFeed.FeedPath)
	}
	for _, prefix := range []string{"", "/api"} {
		resp, err := http.Get(e.srv.URL + prefix + *withFeed.FeedPath)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/calendar; charset=utf-8" {
			t.Fatalf("feed %s: %d %q", prefix, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		text := string(body)
		if !strings.Contains(text, "BEGIN:VCALENDAR") || !strings.Contains(text, "Standup") || strings.Contains(text, "Doctor") {
			t.Fatalf("feed body:\n%s", text)
		}
	}
	resp, _ := http.Get(e.srv.URL + "/ics/not-a-token.ics")
	if resp.StatusCode != 404 {
		t.Fatalf("unknown feed: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func ptrVis(v string) *csil.EventVisibility      { x := csil.EventVisibility(v); return &x }
func ptrTransp(v string) *csil.EventTransparency { x := csil.EventTransparency(v); return &x }

func TestInvitationsAndRespondViaDelegation(t *testing.T) {
	e := newEnv(t)
	alice, aliceLogin := e.login("alice")
	bob, bobLogin := e.login("bob")
	carol, _ := e.login("carol")
	c1 := defaultCalendar(t, carol)

	inv := timedEvent(c1.CalendarId, "Planning", "2026-11-05T15:00:00Z", "2026-11-05T16:00:00Z")
	inv.Attendees = []csil.Attendee{{Identity: &csil.IdentityRef{Domain: "DEV.local", UserId: sp("alice")}, Status: ptrPS("accepted")}}
	created := must(t, createEvent(carol, inv))
	// The organizer cannot answer for alice.
	if *created.Attendees[0].Status != "needs_action" || created.Attendees[0].Person == nil {
		t.Fatalf("attendee: %+v", created.Attendees[0])
	}

	list := must(t, call(alice, "EventService", "list-invitations", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeEventList))
	if len(list.Events) != 1 || list.Events[0].Title != "Planning" {
		t.Fatalf("invitations: %+v", list.Events)
	}
	occ := must(t, listEvents(alice, "2026-11-01T00:00:00Z", "2026-11-10T00:00:00Z"))
	if len(occ.Occurrences) != 1 || !occ.Occurrences[0].Invited || occ.Occurrences[0].Access != "read" {
		t.Fatalf("alice occurrences: %+v", occ.Occurrences)
	}
	// Bob is no attendee and has no access: 404.
	var r resulter = call(bob, "EventService", "get-event", created.EventId, csil.EncodeEventGetEventRequest, csil.DecodeEvent)
	wantCode(t, r, 404)

	// A delegation with only events.write cannot answer.
	dl := must(t, call(alice, "DelegationService", "create-delegation", csil.Delegation{
		Delegate: devRef("bob"), Scopes: scopes("events.write"),
	}, csil.EncodeDelegation, csil.DecodeDelegation))
	assumed := must(t, call(bob, "AuthService", "assume-principal", csil.AssumePrincipalRequest{PrincipalUserId: aliceLogin.User.UserId},
		csil.EncodeAssumePrincipalRequest, csil.DecodeLoginResponse))
	r = call(bob.as(assumed.Token), "EventService", "respond", csil.RespondRequest{EventId: created.EventId, Status: "accepted"},
		csil.EncodeRespondRequest, csil.DecodeEvent)
	wantCode(t, r, 404) // the invitation is not visible without calendars.read or events.respond

	// Update to events.respond: the bearer's scopes follow at once.
	must(t, call(alice, "DelegationService", "update-delegation", csil.Delegation{DelegationId: dl.DelegationId, Scopes: scopes("events.respond")},
		csil.EncodeDelegation, csil.DecodeDelegation))
	assumed = must(t, call(bob, "AuthService", "assume-principal", csil.AssumePrincipalRequest{PrincipalUserId: aliceLogin.User.UserId},
		csil.EncodeAssumePrincipalRequest, csil.DecodeLoginResponse))
	answered := must(t, call(bob.as(assumed.Token), "EventService", "respond", csil.RespondRequest{EventId: created.EventId, Status: "accepted"},
		csil.EncodeRespondRequest, csil.DecodeEvent))
	a := answered.Attendees[0]
	if *a.Status != "accepted" || a.RespondedBy == nil || a.RespondedBy.UserId != bobLogin.User.UserId {
		t.Fatalf("answer: %+v", a)
	}
	// Carol, a stranger to alice's delegation, cannot answer for alice.
	r = call(carol, "EventService", "respond", csil.RespondRequest{EventId: created.EventId, Status: "declined"},
		csil.EncodeRespondRequest, csil.DecodeEvent)
	wantCode(t, r, 403)
	// Alice answers for herself; responded_by clears.
	self := must(t, call(alice, "EventService", "respond", csil.RespondRequest{EventId: created.EventId, Status: "tentative"},
		csil.EncodeRespondRequest, csil.DecodeEvent))
	if *self.Attendees[0].Status != "tentative" || self.Attendees[0].RespondedBy != nil {
		t.Fatalf("self answer: %+v", self.Attendees[0])
	}
}

func ptrPS(s string) *csil.ParticipationStatus { x := csil.ParticipationStatus(s); return &x }

func TestSessionCookieResumeAndEnd(t *testing.T) {
	e := newEnv(t)
	alice, login := e.login("alice")
	resumed := must(t, call(alice, "AuthService", "resume-session", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeLoginResponse))
	if resumed.User.UserId != login.User.UserId || resumed.Token == "" {
		t.Fatalf("resume: %+v", resumed)
	}
	// The cookie rotated; resuming again still works with the new cookie.
	must(t, call(alice, "AuthService", "resume-session", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeLoginResponse))
	must(t, call(alice, "AuthService", "end-session", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeEmptyResponse))
	var r resulter = call(alice, "AuthService", "resume-session", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeLoginResponse)
	wantCode(t, r, 401)
	// No cookie at all.
	r = call(e.client(), "AuthService", "resume-session", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeLoginResponse)
	wantCode(t, r, 401)
}

func TestNoBearerIsUnauthenticated(t *testing.T) {
	e := newEnv(t)
	var r resulter = call(e.client(), "CalendarService", "list-calendars", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeCalendarList)
	wantUnauthenticated(t, r)
	c := e.client()
	c.token = "garbage"
	r = call(c, "CalendarService", "list-calendars", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeCalendarList)
	wantUnauthenticated(t, r)
}

func TestHealthAndAuthStart(t *testing.T) {
	e := newEnv(t)
	resp, err := http.Get(e.srv.URL + "/api/health")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("health: %v %v", resp, err)
	}
	resp.Body.Close()
	resp, err = http.Get(e.srv.URL + "/api/v1/auth/start")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("auth/start without PKI: %d", resp.StatusCode)
	}
}
