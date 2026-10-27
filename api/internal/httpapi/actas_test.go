package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/actas"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// fakeActAs accepts credentials by their bytes: "good-<nonce>".
type fakeActAs struct {
	scopes  []string
	expires time.Time
	revoked map[string]bool
}

func (f *fakeActAs) SignScopeSet(_ context.Context, req actas.ScopeSetRequest) ([]byte, error) {
	return []byte("signed:" + req.GranteeApplicationID), nil
}

func (f *fakeActAs) VerifyCredential(_ context.Context, cred []byte, known []string) (actas.Verified, error) {
	if len(cred) < 5 || string(cred[:5]) != "good-" {
		return actas.Verified{}, errors.New("bad signature")
	}
	for _, k := range known {
		if k == "grant-1" {
			return actas.Verified{}, errors.New("grant revoked")
		}
	}
	return actas.Verified{
		GrantID: "grant-1", SubjectUserID: "acct-1", SubjectDomain: "home.example", Application: "helper@grantee.example",
		Scopes: f.scopes, ExpiresAt: f.expires, Nonce: string(cred[5:]),
	}, nil
}

func (f *fakeActAs) RevokedGrants(_ context.Context, _ string, ids []string) ([]string, error) {
	var out []string
	for _, id := range ids {
		if f.revoked[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func (f *fakeActAs) MaxPresentationAge() time.Duration { return 5 * time.Minute }

func exchange(c *client, cred string) reply[csil.LoginResponse] {
	return call(c, "AuthService", "exchange-act-as-credential", csil.ExchangeActAsCredentialRequest{Credential: []byte(cred)},
		csil.EncodeExchangeActAsCredentialRequest, csil.DecodeLoginResponse)
}

func TestActAsNotConfigured(t *testing.T) {
	e := newEnv(t)
	r := exchange(e.client(), "good-n1")
	if wantCode(t, r, 503).svcErr.Message != "act-as is not configured on this server" {
		t.Fatalf("message %q", r.r.svcErr.Message)
	}
	wantCode(t, call(e.client(), "AuthService", "get-act-as-scope-set", csil.ScopeSetRequest{GranteeDomain: "g", GranteeUserId: "u", GranteeApplicationId: "a"},
		csil.EncodeScopeSetRequest, csil.DecodeScopeSetResponse), 503)
}

func TestActAsExchange(t *testing.T) {
	e := newEnv(t)
	fake := &fakeActAs{
		scopes:  []string{"calendars.read"},
		expires: time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second),
		revoked: map[string]bool{},
	}
	e.deps.ActAs = fake

	set := must(t, call(e.client(), "AuthService", "get-act-as-scope-set", csil.ScopeSetRequest{GranteeDomain: "g", GranteeUserId: "u", GranteeApplicationId: "helper"},
		csil.EncodeScopeSetRequest, csil.DecodeScopeSetResponse))
	if string(set.SignedScopeSet) != "signed:helper" {
		t.Fatalf("scope set %q", set.SignedScopeSet)
	}

	wantCode(t, exchange(e.client(), "forged"), 401)
	login := must(t, exchange(e.client(), "good-n1"))
	if login.Via != "act_as_grant" || len(login.Scopes) != 1 || login.Scopes[0] != "calendars.read" {
		t.Fatalf("login: %+v", login)
	}
	if login.User.Domain != "home.example" || login.User.Subject != "acct-1" {
		t.Fatalf("user: %+v", login.User)
	}
	exp, _ := time.Parse(time.RFC3339, string(login.ExpiresAt))
	if exp.After(fake.expires) {
		t.Fatalf("bearer outlives the grant: %s > %s", exp, fake.expires)
	}
	// Replay of the same presentation is refused.
	wantCode(t, exchange(e.client(), "good-n1"), 401)

	app := e.client()
	app.token = login.Token
	// calendars.read: read yes, write no.
	list := must(t, call(app, "CalendarService", "list-calendars", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeCalendarList))
	if len(list.Calendars) != 1 || list.Calendars[0].Access != "read" {
		t.Fatalf("act-as calendars: %+v", list.Calendars)
	}
	wantCode(t, createEvent(app, timedEvent(list.Calendars[0].CalendarId, "x", "2026-11-02T16:00:00Z", "2026-11-02T17:00:00Z")), 403)
	wantCode(t, call(app, "AuthService", "release-principal", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeLoginResponse), 403)

	// The audit entry names the application.
	audit := must(t, call(app, "AuditService", "list-audit", csil.AuditQuery{}, csil.EncodeAuditQuery, csil.DecodeAuditList))
	found := false
	for _, a := range audit.Entries {
		if a.Via == "act_as_grant" && a.Application != nil && *a.Application == "helper@grantee.example" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no act-as audit entry: %+v", audit.Entries)
	}

	// Unknown scope words give no catalystcal scope: refused.
	fake.scopes = nil
	wantCode(t, exchange(e.client(), "good-n2"), 403)
	fake.scopes = []string{"calendars.read"}

	// The home domain revokes the grant: the worker records it and the
	// dispatcher refuses the bearer at once.
	fake.revoked["grant-1"] = true
	if err := e.deps.RefreshRevocations(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantUnauthenticated(t, call(app, "AuthService", "me", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeMeResponse))
	wantCode(t, exchange(e.client(), "good-n3"), 401)
}
