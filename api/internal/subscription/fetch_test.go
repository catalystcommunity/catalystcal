package subscription

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForbidden(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "192.168.0.9", "172.16.5.5", "169.254.169.254", "::1", "fe80::1", "0.0.0.0", "100.64.1.1", "::ffff:127.0.0.1", "fd00::1"} {
		if !Forbidden(net.ParseIP(s)) {
			t.Errorf("%s should be forbidden", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "2606:4700::1111", "93.184.216.34"} {
		if Forbidden(net.ParseIP(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	f := &Fetcher{}
	got, err := f.NormalizeURL("webcal://example.com/cal.ics")
	if err != nil || got != "https://example.com/cal.ics" {
		t.Fatalf("got %q %v", got, err)
	}
	for _, bad := range []string{"ftp://example.com/x", "http://127.0.0.1/x", "http://localhost/x", "https://user:pw@example.com/", "nonsense"} {
		if _, err := f.NormalizeURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	allow := &Fetcher{AllowPrivate: true}
	if _, err := allow.NormalizeURL("http://127.0.0.1:8080/x.ics"); err != nil {
		t.Fatalf("allow private: %v", err)
	}
}

// The dial check refuses a loopback server even when the URL names it by
// a host name (the DNS-rebinding case: the check sees the dialed address).
func TestFetchRefusesLoopbackAtDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
	}))
	defer srv.Close()
	url := strings.Replace(srv.URL, "127.0.0.1", "localhost.", 1) // a name, not a literal
	_, err := (&Fetcher{}).Fetch(context.Background(), url)
	if !errors.Is(err, ErrForbiddenAddress) {
		t.Fatalf("err = %v, want ErrForbiddenAddress", err)
	}
	body, err := (&Fetcher{AllowPrivate: true}).Fetch(context.Background(), srv.URL)
	if err != nil || !strings.Contains(string(body), "VCALENDAR") {
		t.Fatalf("allowed fetch: %q %v", body, err)
	}
}
