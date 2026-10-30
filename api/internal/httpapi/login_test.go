package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/linkkeys"
)

// fakePKI is the RP. It records the sign-request and returns a fixed
// assertion from verify-assertion.
type fakePKI struct {
	callback, nonce string
	verifiedFor     string
	assertion       linkkeys.Assertion
}

func (f *fakePKI) SignRequest(callbackURL, nonce string) (string, error) {
	f.callback, f.nonce = callbackURL, nonce
	return "signed-req", nil
}

func (f *fakePKI) DecryptToken(string) (string, error) { return "signed-assertion", nil }

func (f *fakePKI) VerifyAssertion(_, expectedDomain string) (*linkkeys.Assertion, error) {
	f.verifiedFor = expectedDomain
	a := f.assertion
	a.Nonce = f.nonce
	return &a, nil
}

func loginEnv(t *testing.T) (*env, *fakePKI) {
	e := newEnv(t)
	pki := &fakePKI{}
	e.deps.PKI = pki
	e.deps.Cfg.CallbackURL = "https://cal.example/auth/callback"
	e.deps.Cfg.LinkkeysIDPDomain = "home.example"
	e.deps.Cfg.LinkkeysIDPURL = "https://lk.home.example"
	e.deps.Cfg.LinkkeysRPDomain = "home.example"
	e.deps.Cfg.AllowedDomains = []string{"home.example", "friend.example"}
	old := linkkeys.LookupTXT
	t.Cleanup(func() { linkkeys.LookupTXT = old })
	linkkeys.LookupTXT = func(name string) ([]string, error) {
		if name == "_linkkeys_apis.friend.example" {
			return []string{"v=lk1 tcp=idp.friend.example https=idp.friend.example"}, nil
		}
		return nil, errors.New("no such host")
	}
	return e, pki
}

func startLogin(t *testing.T, e *env, identity string) *url.URL {
	t.Helper()
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(e.srv.URL + "/api/v1/auth/start?identity=" + url.QueryEscape(identity))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("auth/start %q: %d", identity, resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestAuthStartGoesToTheIdentityDomain(t *testing.T) {
	e, pki := loginEnv(t)
	loc := startLogin(t, e, "Tod@Friend.Example")
	if loc.Host != "idp.friend.example" || loc.Path != "/auth/authorize" {
		t.Fatalf("redirect = %s", loc)
	}
	if loc.Query().Get("signed_request") != "signed-req" || loc.Query().Get("username") != "Tod" {
		t.Fatalf("query = %s", loc.RawQuery)
	}
	if pki.callback != "https://cal.example/auth/callback?domain=friend.example" {
		t.Fatalf("callback = %s", pki.callback)
	}

	// The configured IDP domain uses the configured URL, not DNS.
	loc = startLogin(t, e, "myuser@home.example")
	if loc.Host != "lk.home.example" {
		t.Fatalf("redirect = %s", loc)
	}
}

func TestAuthStartRefusesBadAndDisallowedNames(t *testing.T) {
	e, pki := loginEnv(t)
	for _, id := range []string{"tod@stranger.example", "not a login"} {
		loc := startLogin(t, e, id)
		if loc.Path != "/login" || loc.Query().Get("login_error") == "" || loc.Query().Get("identity") != id {
			t.Fatalf("%q: redirect = %s", id, loc)
		}
	}
	if pki.callback != "" {
		t.Fatalf("sign-request was made for a refused name: %s", pki.callback)
	}
}

func TestCompleteVerifiesAgainstTheCallbackDomain(t *testing.T) {
	e, pki := loginEnv(t)
	startLogin(t, e, "tod@friend.example")
	pki.assertion = linkkeys.Assertion{UserID: "u-tod", Domain: "friend.example", Audience: "home.example"}

	domain := "friend.example"
	resp := must(t, call(e.client(), "AuthService", "complete", csil.CompleteRequest{EncryptedToken: "tok", Domain: &domain},
		csil.EncodeCompleteRequest, csil.DecodeLoginResponse))
	if pki.verifiedFor != "friend.example" || resp.User.Domain != "friend.example" {
		t.Fatalf("verified for %q, user %+v", pki.verifiedFor, resp.User)
	}

	// An assertion from another domain than the callback names is refused.
	startLogin(t, e, "tod@friend.example")
	pki.assertion.Domain = "home.example"
	var r resulter = call(e.client(), "AuthService", "complete", csil.CompleteRequest{EncryptedToken: "tok", Domain: &domain},
		csil.EncodeCompleteRequest, csil.DecodeLoginResponse)
	wantCode(t, r, 401)

	// A domain that the server does not accept is refused before the RP call.
	pki.verifiedFor = ""
	other := "stranger.example"
	r = call(e.client(), "AuthService", "complete", csil.CompleteRequest{EncryptedToken: "tok", Domain: &other},
		csil.EncodeCompleteRequest, csil.DecodeLoginResponse)
	wantCode(t, r, 403)
	if pki.verifiedFor != "" {
		t.Fatalf("verify-assertion ran for a refused domain")
	}
}
