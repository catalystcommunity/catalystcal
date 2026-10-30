package linkkeys

import (
	"errors"
	"net/url"
	"testing"
)

func TestParseIdentity(t *testing.T) {
	cases := []struct {
		in, user, domain string
		ok               bool
	}{
		{"tod@todandlorna.com", "tod", "todandlorna.com", true},
		{"  Tod@TodAndLorna.COM ", "Tod", "todandlorna.com", true},
		{"squizzlezig.com", "", "squizzlezig.com", true},
		{"a.b@c@example.com", "a.b@c", "", false},
		{"tod@localhost", "", "", false},
		{"tod@exa_mple.com", "", "", false},
		{"", "", "", false},
		{"to d@example.com", "", "", false},
	}
	for _, c := range cases {
		u, d, err := ParseIdentity(c.in)
		if (err == nil) != c.ok {
			t.Fatalf("ParseIdentity(%q) err = %v, want ok=%v", c.in, err, c.ok)
		}
		if c.ok && (u != c.user || d != c.domain) {
			t.Fatalf("ParseIdentity(%q) = %q, %q", c.in, u, d)
		}
	}
}

func TestResolveBrowserBase(t *testing.T) {
	records := map[string][]string{
		"_linkkeys_apis.plain.example":   {"v=lk1 tcp=lk.plain.example https=lk.plain.example"},
		"_linkkeys_apis.prefix.example":  {"other", "v=lk1 https=login.prefix.example/lk/"},
		"_linkkeys_apis.scheme.example":  {"v=lk1 https=https://lk.scheme.example"},
		"_linkkeys_apis.bad.example":     {"v=lk1 https=http://lk.bad.example"},
		"_linkkeys_apis.nohttps.example": {"v=lk1 tcp=lk.nohttps.example"},
	}
	old := LookupTXT
	t.Cleanup(func() { LookupTXT = old })
	LookupTXT = func(name string) ([]string, error) {
		if r, ok := records[name]; ok {
			return r, nil
		}
		return nil, errors.New("no such host")
	}
	for domain, want := range map[string]string{
		"plain.example":   "https://lk.plain.example",
		"prefix.example":  "https://login.prefix.example/lk",
		"scheme.example":  "https://lk.scheme.example",
		"bad.example":     "https://bad.example",
		"nohttps.example": "https://nohttps.example",
		"none.example":    "https://none.example",
	} {
		if got := ResolveBrowserBase(domain); got != want {
			t.Fatalf("ResolveBrowserBase(%q) = %q, want %q", domain, got, want)
		}
	}
}

func TestAuthorizeURL(t *testing.T) {
	got, err := AuthorizeURL("https://login.example/lk", "sig+/=", "tod")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if u.Host != "login.example" || u.Path != "/lk/auth/authorize" {
		t.Fatalf("url = %s", got)
	}
	if u.Query().Get("signed_request") != "sig+/=" || u.Query().Get("username") != "tod" {
		t.Fatalf("query = %s", u.RawQuery)
	}
	got, _ = AuthorizeURL("https://login.example", "s", "")
	if u, _ := url.Parse(got); u.Query().Has("username") {
		t.Fatalf("empty username was sent: %s", got)
	}
}
