package linkkeys

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// LookupTXT is the DNS TXT lookup. Tests replace it.
var LookupTXT = net.LookupTXT

// ParseIdentity splits a login name "[username@]domain" into its parts. The
// domain is the trust identity; it is lowercased. The username is passed to
// the IDP login page as is.
func ParseIdentity(identity string) (username, domain string, err error) {
	identity = strings.TrimSpace(identity)
	if i := strings.LastIndex(identity, "@"); i >= 0 {
		username, domain = identity[:i], identity[i+1:]
	} else {
		domain = identity
	}
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if !validDomain(domain) {
		return "", "", fmt.Errorf("%q is not a LinkKeys login name (use name@domain)", identity)
	}
	if strings.ContainsAny(username, " \t\r\n/@") {
		return "", "", fmt.Errorf("%q is not a LinkKeys login name (use name@domain)", identity)
	}
	return username, domain, nil
}

// validDomain accepts a DNS name with at least two labels.
func validDomain(d string) bool {
	if len(d) == 0 || len(d) > 253 || !strings.Contains(d, ".") {
		return false
	}
	for _, label := range strings.Split(d, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

// ResolveBrowserBase returns the https base of domain's LinkKeys browser and
// API routes. It reads the https= value of the _linkkeys_apis.<domain> TXT
// record. The value has no scheme and can have a path prefix. Without a
// usable record it returns https://<domain>. The base is a service location
// only: trust stays bound to the identity domain.
func ResolveBrowserBase(domain string) string {
	txts, err := LookupTXT("_linkkeys_apis." + domain)
	if err == nil {
		for _, txt := range txts {
			fields := parseLinkkeysTXT(txt)
			if fields == nil || fields["https"] == "" {
				continue
			}
			v := strings.TrimRight(fields["https"], "/")
			if !strings.Contains(v, "://") {
				v = "https://" + v
			}
			if u, err := url.Parse(v); err == nil && u.Scheme == "https" && u.Host != "" && u.RawQuery == "" && u.Fragment == "" {
				return v
			}
		}
	}
	return "https://" + domain
}

// AuthorizeURL builds the IDP browser URL: <base>/auth/authorize with the
// signed request and, when known, the username for the login form.
func AuthorizeURL(base, signedRequest, username string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u = u.JoinPath("/auth/authorize")
	q := url.Values{}
	q.Set("signed_request", signedRequest)
	if username != "" {
		q.Set("username", username)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
