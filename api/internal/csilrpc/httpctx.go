package csilrpc

// A CSIL op returns a typed value, not an HTTP response, so it cannot read or
// set a cookie by itself. ServeRPC puts a per-request exchange on the context:
// a handler reads request cookies through it and queues Set-Cookie headers on
// it. ServeRPC writes the queued cookies before the envelope. This keeps
// http.ResponseWriter out of the service layer.

import (
	"context"
	"net/http"
)

type httpExchangeKey struct{}

type httpExchange struct {
	req     *http.Request
	cookies []*http.Cookie
}

func withHTTPExchange(ctx context.Context, r *http.Request) (context.Context, *httpExchange) {
	x := &httpExchange{req: r}
	return context.WithValue(ctx, httpExchangeKey{}, x), x
}

func exchangeFrom(ctx context.Context) *httpExchange {
	x, _ := ctx.Value(httpExchangeKey{}).(*httpExchange)
	return x
}

// RequestCookie returns the named request cookie's value. It returns "" when
// the cookie is absent or the op was not called over HTTP (a test calls it
// directly).
func RequestCookie(ctx context.Context, name string) string {
	x := exchangeFrom(ctx)
	if x == nil || x.req == nil {
		return ""
	}
	c, err := x.req.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// RequestUserAgent returns the request's User-Agent header, or "".
func RequestUserAgent(ctx context.Context) string {
	x := exchangeFrom(ctx)
	if x == nil || x.req == nil {
		return ""
	}
	return x.req.UserAgent()
}

// SetCookie queues a Set-Cookie header for the response. A later cookie with
// the same name replaces an earlier one. Outside HTTP it does nothing.
func SetCookie(ctx context.Context, c *http.Cookie) {
	x := exchangeFrom(ctx)
	if x == nil {
		return
	}
	for i := range x.cookies {
		if x.cookies[i].Name == c.Name {
			x.cookies[i] = c
			return
		}
	}
	x.cookies = append(x.cookies, c)
}

func (x *httpExchange) writeCookies(w http.ResponseWriter) {
	for _, c := range x.cookies {
		http.SetCookie(w, c)
	}
}

// WithHTTPRequest gives a context the same exchange ServeRPC builds. Tests use
// it to call an op directly with request cookies.
func WithHTTPRequest(ctx context.Context, r *http.Request) context.Context {
	ctx, _ = withHTTPExchange(ctx, r)
	return ctx
}

// QueuedCookies returns the Set-Cookie values an op queued on the context.
func QueuedCookies(ctx context.Context) []*http.Cookie {
	x := exchangeFrom(ctx)
	if x == nil {
		return nil
	}
	return x.cookies
}
