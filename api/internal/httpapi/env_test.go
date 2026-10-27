package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/config"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/services"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
	"github.com/catalystcommunity/catalystcal/api/internal/subscription"
	"github.com/catalystcommunity/catalystcal/api/internal/transport"
)

// env is one catalystcal server over a temporary SQLite file.
type env struct {
	t    *testing.T
	srv  *httptest.Server
	deps *services.Deps
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "catalystcal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(func(k string) string {
		return map[string]string{
			"CATALYSTCAL_ENV":              "dev",
			"CATALYSTCAL_JWT_SECRET":       "test-secret-test-secret-test-secret",
			"CATALYSTCAL_DEV_AUTH_ENABLED": "true",
			"CATALYSTCAL_PUBLIC_URL":       "http://catalystcal.test",
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	deps := &services.Deps{
		Store:   store.New(db),
		Cfg:     cfg,
		Secret:  []byte(cfg.JWTSecret),
		Fetcher: &subscription.Fetcher{AllowPrivate: true},
		Revoked: services.NewRevokedGrants(),
	}
	e := &env{t: t, deps: deps}
	e.srv = httptest.NewServer(Handler(deps, true))
	t.Cleanup(e.srv.Close)
	return e
}

// client is one browser/app: a bearer and a cookie jar.
type client struct {
	e     *env
	token string
	http  *http.Client
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar}}
}

// result of one call: ok payload, a ServiceError, or a transport status.
type result struct {
	status  transport.Status
	variant string
	payload []byte
	svcErr  *csil.ServiceError
}

func (c *client) raw(service, op string, payload []byte) result {
	c.e.t.Helper()
	frame, err := transport.NewRpcRequest(service, op, payload).Encode()
	if err != nil {
		c.e.t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, c.e.srv.URL+"/api/csil/v1/rpc", bytes.NewReader(frame))
	req.Header.Set("Content-Type", "application/cbor")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	r, err := transport.DecodeRpcResponse(body)
	if err != nil {
		c.e.t.Fatalf("decode envelope: %v", err)
	}
	out := result{status: r.Status, payload: r.Payload}
	if r.Variant != nil {
		out.variant = *r.Variant
	}
	if r.Status.IsOk() && out.variant == "ServiceError" {
		se, err := csil.DecodeServiceError(r.Payload)
		if err != nil {
			c.e.t.Fatal(err)
		}
		out.svcErr = &se
	}
	return out
}

// reply is a typed call result.
type reply[Resp any] struct {
	v Resp
	r result
}

// resulter is a call result of any type.
type resulter interface{ res() result }

func (r result) res() result      { return r }
func (r reply[Resp]) res() result { return r.r }

// call runs one op through the real HTTP carrier with the generated codecs.
func call[Req, Resp any](c *client, service, op string, req Req, enc func(Req) []byte, dec func([]byte) (Resp, error)) reply[Resp] {
	c.e.t.Helper()
	r := c.raw(service, op, enc(req))
	if !r.status.IsOk() || r.svcErr != nil {
		return reply[Resp]{r: r}
	}
	out, err := dec(r.payload)
	if err != nil {
		c.e.t.Fatalf("%s/%s decode: %v", service, op, err)
	}
	return reply[Resp]{v: out, r: r}
}

// must fails the test unless the call succeeded.
func must[Resp any](t *testing.T, rep reply[Resp]) Resp {
	t.Helper()
	out, r := rep.v, rep.r
	if !r.status.IsOk() {
		t.Fatalf("transport status %s", r.status.Name())
	}
	if r.svcErr != nil {
		t.Fatalf("service error %d: %s", r.svcErr.Code, r.svcErr.Message)
	}
	return out
}

// wantCode fails unless the call returned the ServiceError code.
func wantCode(t *testing.T, rr resulter, code uint64) result {
	t.Helper()
	r := rr.res()
	if !r.status.IsOk() {
		t.Fatalf("transport status %s, want ServiceError %d", r.status.Name(), code)
	}
	if r.svcErr == nil {
		t.Fatalf("call succeeded, want ServiceError %d", code)
	}
	if r.svcErr.Code != code {
		t.Fatalf("ServiceError %d (%s), want %d", r.svcErr.Code, r.svcErr.Message, code)
	}
	return r
}

func wantUnauthenticated(t *testing.T, rr resulter) {
	t.Helper()
	r := rr.res()
	if r.status.Code() != transport.StatusUnauthenticated.Code() {
		t.Fatalf("status %s (svcErr %+v), want unauthenticated", r.status.Name(), r.svcErr)
	}
}

// login signs in with dev-login and keeps the bearer.
func (e *env) login(handle string) (*client, csil.LoginResponse) {
	e.t.Helper()
	c := e.client()
	resp := must(e.t, call(c, "DevAuthService", "dev-login", csil.DevLoginRequest{Handle: &handle},
		csil.EncodeDevLoginRequest, csil.DecodeLoginResponse))
	c.token = resp.Token
	return c, resp
}

func (c *client) as(token string) *client {
	return &client{e: c.e, token: token, http: c.http}
}

func sp(s string) *string { return &s }

func tsp(s string) *csil.Timestamp { v := csil.Timestamp(s); return &v }

func defaultCalendar(t *testing.T, c *client) csil.Calendar {
	t.Helper()
	list := must(t, call(c, "CalendarService", "list-calendars", csil.EmptyRequest{}, csil.EncodeEmptyRequest, csil.DecodeCalendarList))
	for _, cal := range list.Calendars {
		if cal.IsDefault && cal.Access == "owner" {
			return cal
		}
	}
	t.Fatal("no default calendar")
	return csil.Calendar{}
}

func timedEvent(cal csil.CalendarID, title, start, end string) csil.Event {
	return csil.Event{CalendarId: cal, Title: title, Start: tsp(start), End: tsp(end)}
}

func createEvent(c *client, ev csil.Event) reply[csil.Event] {
	return call(c, "EventService", "create-event", ev, csil.EncodeEvent, csil.DecodeEvent)
}

func listEvents(c *client, start, end string) reply[csil.OccurrenceList] {
	return call(c, "EventService", "list-events", csil.ListEventsRequest{RangeStart: csil.Timestamp(start), RangeEnd: csil.Timestamp(end)},
		csil.EncodeListEventsRequest, csil.DecodeOccurrenceList)
}
