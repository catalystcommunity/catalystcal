package httpapi

import (
	"context"
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/apppresent"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	api "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

func gid(g csil.Group) *csil.GroupID { id := g.GroupId; return &id }

func memberRef(handle string) csil.GroupMember { return csil.GroupMember{Identity: devRefP(handle)} }

func createGroup(c *client, name string, members ...csil.GroupMember) reply[csil.Group] {
	return call(c, "GroupService", "create-group", csil.CreateGroupRequest{Name: name, Members: members},
		csil.EncodeCreateGroupRequest, csil.DecodeGroup)
}

func setMembers(c *client, g csil.Group, rev *uint64, members ...csil.GroupMember) reply[csil.Group] {
	if members == nil {
		members = []csil.GroupMember{}
	}
	return call(c, "GroupService", "set-members", csil.SetGroupMembersRequest{GroupId: g.GroupId, ExpectedRevision: rev, Members: members},
		csil.EncodeSetGroupMembersRequest, csil.DecodeGroup)
}

func getCalendar(c *client, id csil.CalendarID) reply[csil.Calendar] {
	return call(c, "CalendarService", "get-calendar", id, csil.EncodeCalendarGetCalendarRequest, csil.DecodeCalendar)
}

func TestGroupSharesAndContribute(t *testing.T) {
	e := newEnv(t)
	alice, _ := e.login("alice")
	bob, bobLogin := e.login("bob")
	carol, _ := e.login("carol")
	dave, _ := e.login("dave")
	erin, _ := e.login("erin")
	a1 := defaultCalendar(t, alice)

	admins := must(t, createGroup(alice, "Admins", memberRef("dave")))
	// Members by account id; bob also by handle (he signed in here).
	house := must(t, createGroup(alice, "House",
		memberRef("bob"), csil.GroupMember{Identity: &csil.IdentityRef{Domain: "dev.local", Handle: sp("carol")}},
		memberRef("bob"), csil.GroupMember{GroupId: gid(admins)}))
	if house.Revision != 1 || len(house.Members) != 3 {
		t.Fatalf("house: %+v", house)
	}
	if m := house.Members[0]; m.Person == nil || m.Person.UserId != bobLogin.User.UserId || m.Identity.Handle == nil || *m.Identity.Handle != "bob" {
		t.Fatalf("member view: %+v", m)
	}
	if m := house.Members[2]; m.GroupName == nil || *m.GroupName != "Admins" {
		t.Fatalf("nested member view: %+v", m)
	}
	wantCode(t, createGroup(alice, "House"), 409)

	// Before a share, nobody sees a1.
	wantCode(t, getCalendar(bob, a1.CalendarId), 404)

	share := must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, GranteeGroupId: gid(house), Role: "contribute"},
		csil.EncodeShare, csil.DecodeShare))
	if share.GranteeGroupName == nil || *share.GranteeGroupName != "House" || share.Grantee != nil {
		t.Fatalf("share: %+v", share)
	}
	wantCode(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, GranteeGroupId: gid(house), Grantee: devRefP("bob"), Role: "read"},
		csil.EncodeShare, csil.DecodeShare), 400)

	for _, c := range []*client{bob, carol, dave} {
		cal := must(t, getCalendar(c, a1.CalendarId))
		if cal.Access != "contribute" || cal.AccessPath != "share" {
			t.Fatalf("group member access: %+v", cal)
		}
	}
	wantCode(t, getCalendar(erin, a1.CalendarId), 404)
	// The calendar list shows it once.
	list := must(t, call(dave, "CalendarService", "list-calendars", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeCalendarList))
	n := 0
	for _, c := range list.Calendars {
		if c.CalendarId == a1.CalendarId {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("a1 listed %d times", n)
	}

	// contribute: create, and change only your own events.
	mine := must(t, createEvent(bob, timedEvent(a1.CalendarId, "Bob's chore", "2026-11-02T15:00:00Z", "2026-11-02T16:00:00Z")))
	theirs := must(t, createEvent(alice, timedEvent(a1.CalendarId, "Alice's party", "2026-11-03T15:00:00Z", "2026-11-03T16:00:00Z")))
	mine.Title = "Bob's chore, moved"
	must(t, call(bob, "EventService", "update-event", csil.UpdateEventRequest{Event: mine}, csil.EncodeUpdateEventRequest, csil.DecodeEvent))
	theirs.Title = "Bob was here"
	r := wantCode(t, call(bob, "EventService", "update-event", csil.UpdateEventRequest{Event: theirs}, csil.EncodeUpdateEventRequest, csil.DecodeEvent), 403)
	if !strings.Contains(r.svcErr.Message, "only the events that you created") {
		t.Fatalf("message %q", r.svcErr.Message)
	}
	wantCode(t, call(bob, "EventService", "delete-event", csil.DeleteEventRequest{EventId: theirs.EventId}, csil.EncodeDeleteEventRequest, csil.DecodeEmptyResponse), 403)
	wantCode(t, call(carol, "EventService", "delete-event", csil.DeleteEventRequest{EventId: mine.EventId}, csil.EncodeDeleteEventRequest, csil.DecodeEmptyResponse), 403)
	// contribute does not import or manage.
	wantCode(t, call(bob, "ShareService", "list-shares", a1.CalendarId, csil.EncodeShareListSharesRequest, csil.DecodeShareList), 403)

	// A direct share and a group share: the highest role wins.
	must(t, call(alice, "ShareService", "put-share", csil.Share{CalendarId: a1.CalendarId, Grantee: devRefP("dave"), Role: "write"},
		csil.EncodeShare, csil.DecodeShare))
	if cal := must(t, getCalendar(dave, a1.CalendarId)); cal.Access != "write" {
		t.Fatalf("dave access %s", cal.Access)
	}
	must(t, call(dave, "EventService", "delete-event", csil.DeleteEventRequest{EventId: mine.EventId}, csil.EncodeDeleteEventRequest, csil.DecodeEmptyResponse))

	// set-members with an old revision is a conflict.
	old := house.Revision
	house = must(t, setMembers(alice, house, &old, memberRef("bob"), memberRef("carol"), csil.GroupMember{GroupId: gid(admins)}))
	wantCode(t, setMembers(alice, house, &old, memberRef("carol")), 409)

	// A removed member loses access at the next call, with the same bearer.
	rev := house.Revision
	house = must(t, setMembers(alice, house, &rev, memberRef("carol"), csil.GroupMember{GroupId: gid(admins)}))
	wantCode(t, getCalendar(bob, a1.CalendarId), 404)
	must(t, getCalendar(carol, a1.CalendarId))

	// Cycles and foreign groups are refused.
	wantCode(t, setMembers(alice, admins, nil, csil.GroupMember{GroupId: gid(house)}), 400)
	wantCode(t, setMembers(alice, admins, nil, csil.GroupMember{GroupId: gid(admins)}), 400)
	erinGroup := must(t, createGroup(erin, "Erin's"))
	wantCode(t, setMembers(alice, admins, nil, csil.GroupMember{GroupId: gid(erinGroup)}), 404)

	// Only the owner sees a group, and only the owner's groups can be
	// share grantees.
	wantCode(t, call(erin, "GroupService", "get-group", house.GroupId, csil.EncodeGroupGetGroupRequest, csil.DecodeGroup), 404)
	e1 := defaultCalendar(t, erin)
	wantCode(t, call(erin, "ShareService", "put-share", csil.Share{CalendarId: e1.CalendarId, GranteeGroupId: gid(house), Role: "read"},
		csil.EncodeShare, csil.DecodeShare), 404)
	groups := must(t, call(alice, "GroupService", "list-groups", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeGroupList))
	if len(groups.Groups) != 2 {
		t.Fatalf("alice groups: %d", len(groups.Groups))
	}

	// Deleting the nested group removes dave's group path; his direct
	// share stays. Deleting the house group removes its share.
	must(t, call(alice, "ShareService", "delete-share", mustShareFor(t, alice, a1.CalendarId, "dave"), csil.EncodeShareDeleteShareRequest, csil.DecodeEmptyResponse))
	must(t, call(alice, "GroupService", "delete-group", admins.GroupId, csil.EncodeGroupDeleteGroupRequest, csil.DecodeEmptyResponse))
	wantCode(t, getCalendar(dave, a1.CalendarId), 404)
	house = must(t, call(alice, "GroupService", "get-group", house.GroupId, csil.EncodeGroupGetGroupRequest, csil.DecodeGroup))
	if len(house.Members) != 1 {
		t.Fatalf("house after nested delete: %+v", house.Members)
	}
	must(t, call(alice, "GroupService", "delete-group", house.GroupId, csil.EncodeGroupDeleteGroupRequest, csil.DecodeEmptyResponse))
	wantCode(t, getCalendar(carol, a1.CalendarId), 404)
	shares := must(t, call(alice, "ShareService", "list-shares", a1.CalendarId, csil.EncodeShareListSharesRequest, csil.DecodeShareList))
	if len(shares.Shares) != 0 {
		t.Fatalf("shares left: %+v", shares.Shares)
	}

	// A delegated bearer cannot use GroupService.
	must(t, call(alice, "DelegationService", "create-delegation", csil.Delegation{Delegate: devRef("bob"), Scopes: scopes("calendars.read")},
		csil.EncodeDelegation, csil.DecodeDelegation))
	assumed := must(t, call(bob, "AuthService", "assume-principal", csil.AssumePrincipalRequest{PrincipalUserId: mustMe(t, alice).User.UserId},
		csil.EncodeAssumePrincipalRequest, csil.DecodeLoginResponse))
	wantCode(t, call(bob.as(assumed.Token), "GroupService", "list-groups", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeGroupList), 403)
}

func mustMe(t *testing.T, c *client) csil.MeResponse {
	t.Helper()
	return must(t, call(c, "AuthService", "me", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeMeResponse))
}

func mustShareFor(t *testing.T, c *client, cal csil.CalendarID, subject string) csil.ShareID {
	t.Helper()
	list := must(t, call(c, "ShareService", "list-shares", cal, csil.EncodeShareListSharesRequest, csil.DecodeShareList))
	for _, s := range list.Shares {
		if s.Grantee != nil && s.Grantee.UserId != nil && *s.Grantee.UserId == subject {
			return s.ShareId
		}
	}
	t.Fatalf("no share for %s", subject)
	return ""
}

// ---- application principals ----------------------------------------------

type appKeys struct {
	resolver *staticResolver
	priv     ed25519.PrivateKey
	signer   apppresent.Ref
	own      apppresent.Ref
}

type staticResolver struct {
	keys []regularrp.VerifiedApplicationKey
}

func (s *staticResolver) Resolve(context.Context, regularrp.InstanceRef, *int64) (regularrp.ResolveResult, error) {
	return regularrp.ResolveResult{Keys: regularrp.VerifiedApplicationKeySet{Keys: s.keys}, Freshness: regularrp.FreshnessRefreshed}, nil
}

func enableApps(e *env) *appKeys {
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = 7
	priv := ed25519.NewKeyFromSeed(seed)
	k := &appKeys{
		priv:   priv,
		signer: apppresent.Ref{SubjectUserID: "lh-uuid", SubjectDomain: "dev.local", ApplicationID: "longhouse"},
		own:    apppresent.Ref{SubjectUserID: "cal-uuid", SubjectDomain: "catalystcal.test", ApplicationID: "catalystcal"},
	}
	k.resolver = &staticResolver{keys: []regularrp.VerifiedApplicationKey{{
		Attestation: api.ApplicationKeyAttestation{
			SubjectUserId: k.signer.SubjectUserID, SubjectDomain: k.signer.SubjectDomain, ApplicationId: k.signer.ApplicationID,
			InstanceId: "lh-1", KeyId: "k1", KeyUsage: "sign", Algorithm: "ed25519", PublicKey: priv.Public().(ed25519.PublicKey),
		},
		Status: regularrp.KeyStatus{Kind: regularrp.KeyStatusUsable},
	}}}
	e.deps.Apps = &apppresent.Verifier{Own: k.own, Resolver: k.resolver, Allowed: []apppresent.Ref{k.signer}, MaxAge: 300 * time.Second}
	return k
}

func (k *appKeys) present(nonce string) csil.ExchangeApplicationPresentationRequest {
	p := csil.EncodeApplicationPresentation(csil.ApplicationPresentation{
		Signer: k.signer.CSIL(), SignerInstanceId: "lh-1", Audience: k.own.CSIL(),
		RequestDigest: apppresent.RequestDigest(),
		PresentedAt:   csil.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z")),
		Nonce:         []byte(nonce),
	})
	return csil.ExchangeApplicationPresentationRequest{Presentation: csil.SignedApplicationPresentation{
		Presentation: p, SignedByKeyId: "k1", Signature: ed25519.Sign(k.priv, apppresent.SignatureInput(p)),
	}}
}

func exchangeApp(c *client, req csil.ExchangeApplicationPresentationRequest) reply[csil.LoginResponse] {
	return call(c, "AuthService", "exchange-application-presentation", req,
		csil.EncodeExchangeApplicationPresentationRequest, csil.DecodeLoginResponse)
}

func TestApplicationPrincipal(t *testing.T) {
	e := newEnv(t)
	anon := e.client()
	// Off until configured.
	wantCode(t, call(anon, "AuthService", "get-application-identity", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeApplicationIdentityResponse), 503)

	k := enableApps(e)
	ident := must(t, call(anon, "AuthService", "get-application-identity", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeApplicationIdentityResponse))
	if apppresent.FromCSIL(ident.Audience) != k.own {
		t.Fatalf("audience %+v", ident.Audience)
	}

	req := k.present("nonce-0123456789")
	login := must(t, exchangeApp(anon, req))
	if login.Via != "self" || login.User.ApplicationId == nil || *login.User.ApplicationId != "longhouse" || login.User.Subject != "lh-uuid" {
		t.Fatalf("app login: %+v", login)
	}
	// The same presentation again is a replay.
	wantCode(t, exchangeApp(anon, req), 401)

	app := anon.as(login.Token)
	me := mustMe(t, app)
	if me.User.ApplicationId == nil || *me.User.ApplicationId != "longhouse" {
		t.Fatalf("me: %+v", me.User)
	}
	// An application principal has no default calendar.
	if list := must(t, call(app, "CalendarService", "list-calendars", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeCalendarList)); len(list.Calendars) != 0 {
		t.Fatalf("app calendars: %+v", list.Calendars)
	}

	// The application keeps a house calendar for a group of people who
	// have never signed in here.
	cal := must(t, call(app, "CalendarService", "create-calendar", csil.CreateCalendarRequest{Name: "House"},
		csil.EncodeCreateCalendarRequest, csil.DecodeCalendar))
	members := must(t, createGroup(app, "longhouse:h1:members", memberRef("alice"), memberRef("bob")))
	if members.Members[0].Person != nil {
		t.Fatalf("person before sign-in: %+v", members.Members[0])
	}
	must(t, call(app, "ShareService", "put-share", csil.Share{CalendarId: cal.CalendarId, GranteeGroupId: gid(members), Role: "contribute"},
		csil.EncodeShare, csil.DecodeShare))

	alice, _ := e.login("alice")
	if got := must(t, getCalendar(alice, cal.CalendarId)); got.Access != "contribute" || got.Owner.ApplicationId == nil {
		t.Fatalf("alice sees: %+v", got)
	}
	// The person account with the same id as the application is a
	// different principal.
	personTwin, _ := e.login("lh-uuid")
	wantCode(t, getCalendar(personTwin, cal.CalendarId), 404)

	// A presentation for another audience is refused.
	k.own.ApplicationID = "elsewhere"
	bad := k.present("nonce-abcdefghijk")
	wantCode(t, exchangeApp(anon, bad), 401)

	// Removing the application from the allow list stops its bearer at
	// the next call.
	e.deps.Apps.Allowed = nil
	wantUnauthenticated(t, call(app, "AuthService", "me", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeMeResponse))
}
