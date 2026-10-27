// Package subscription fetches remote iCalendar feeds for subscription
// calendars.
//
// When private addresses are not allowed, the check runs in the dialer's
// Control hook, on the address that is actually dialed. A check at parse
// time only would let DNS rebinding (a name that resolves to a public
// address first and to 127.0.0.1 later) reach internal services.
package subscription

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// MaxFeedBytes caps the size of a fetched feed.
const MaxFeedBytes = 10 << 20

// ErrForbiddenAddress is returned for a loopback, private, or link-local
// destination when private addresses are not allowed.
var ErrForbiddenAddress = errors.New("subscription URL resolves to a private, loopback, or link-local address")

// Fetcher downloads feeds.
type Fetcher struct {
	AllowPrivate bool
	Timeout      time.Duration
	// Client overrides the HTTP client (tests).
	Client *http.Client
}

// NormalizeURL checks a subscription URL and maps webcal:// to https://.
// It refuses literal private addresses early, for a clear error; the dial
// check still runs for every connection.
func (f *Fetcher) NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("source_url is not a URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "webcal", "webcals":
		u.Scheme = "https"
	case "http", "https":
		u.Scheme = strings.ToLower(u.Scheme)
	default:
		return "", errors.New("source_url must use http, https, or webcal")
	}
	if u.Host == "" {
		return "", errors.New("source_url needs a host")
	}
	if u.User != nil {
		return "", errors.New("source_url must not contain credentials")
	}
	if !f.AllowPrivate {
		host := u.Hostname()
		if ip := net.ParseIP(host); ip != nil && Forbidden(ip) {
			return "", ErrForbiddenAddress
		}
		if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
			return "", ErrForbiddenAddress
		}
	}
	return u.String(), nil
}

var cgnat = mustCIDR("100.64.0.0/10")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// Forbidden reports whether ip is loopback, private, link-local,
// unspecified, multicast, or carrier-grade NAT.
func Forbidden(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || cgnat.Contains(ip)
}

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if !f.AllowPrivate {
		dialer.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || Forbidden(ip) {
				return ErrForbiddenAddress
			}
			return nil
		}
	}
	transport := &http.Transport{
		// No proxy: a proxy would make the dial check meaningless.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect to a non-http URL")
			}
			return nil
		},
	}
}

// Fetch downloads a feed.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	req.Header.Set("User-Agent", "catalystcal-subscription/1")
	resp, err := f.client().Do(req)
	if err != nil {
		if errors.Is(err, ErrForbiddenAddress) {
			return nil, ErrForbiddenAddress
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxFeedBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxFeedBytes {
		return nil, fmt.Errorf("feed is larger than %d bytes", MaxFeedBytes)
	}
	return body, nil
}
