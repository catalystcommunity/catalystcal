// Hand-written. Source: clients/overlays/go/ (copied in by clients/regenerate.sh).

package catalystcalclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/clients/go/internal/transport"
)

// RPCPath is the CSIL-RPC mount of a catalystcal server, under its public URL.
const RPCPath = "/api/csil/v1/rpc"

// maxResponseBytes caps a reply. An iCalendar export is the largest.
const maxResponseBytes = 32 << 20

// TokenSource gives the bearer for each call. Invalidate drops a cached
// bearer after the server refused it, so the next Token makes a new one.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Invalidate(token string)
}

// HTTPTransport is the CSIL-RPC carrier over HTTPS. It implements Transport.
//
//	t := catalystcalclient.NewHTTPTransport("https://cal.example.com", tokens)
//	events := catalystcalclient.NewEventClient(t)
type HTTPTransport struct {
	// BaseURL is the public URL of the server, without a path.
	BaseURL string
	// Tokens gives the bearer. Nil sends no bearer (public ops only).
	Tokens TokenSource
	// HTTP is the client. Default: a client with a 30 s timeout.
	HTTP *http.Client
}

var _ Transport = (*HTTPTransport)(nil)

// NewHTTPTransport makes a transport for baseURL.
func NewHTTPTransport(baseURL string, tokens TokenSource) *HTTPTransport {
	return &HTTPTransport{BaseURL: strings.TrimRight(baseURL, "/"), Tokens: tokens}
}

func (t *HTTPTransport) client() *http.Client {
	if t.HTTP != nil {
		return t.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// Unauthenticated reports whether err is a refused or missing bearer.
func Unauthenticated(err error) bool {
	var ce *ClientError
	return errors.As(err, &ce) && ce.Code == http.StatusUnauthorized
}

// Call implements Transport. A ServiceError reply becomes a *ClientError
// with its Code and Message. A transport status becomes a *ClientError
// with the HTTP-shaped code (401 for unauthenticated). When the server
// refuses the bearer, Call invalidates it and tries once more with a new
// one.
func (t *HTTPTransport) Call(ctx context.Context, service, op string, req []byte) ([]byte, error) {
	out, token, err := t.call(ctx, service, op, req)
	if err != nil && token != "" && Unauthenticated(err) {
		t.Tokens.Invalidate(token)
		out, _, err = t.call(ctx, service, op, req)
	}
	return out, err
}

func (t *HTTPTransport) call(ctx context.Context, service, op string, req []byte) ([]byte, string, error) {
	var token string
	if t.Tokens != nil {
		var err error
		if token, err = t.Tokens.Token(ctx); err != nil {
			return nil, "", err
		}
	}
	frame, err := transport.NewRpcRequest(service, op, req).Encode()
	if err != nil {
		return nil, token, &ClientError{Err: err}
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.BaseURL+RPCPath, bytes.NewReader(frame))
	if err != nil {
		return nil, token, &ClientError{Err: err}
	}
	hreq.Header.Set("Content-Type", "application/cbor")
	hreq.Header.Set("Accept", "application/cbor")
	if token != "" {
		hreq.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := t.client().Do(hreq)
	if err != nil {
		return nil, token, &ClientError{Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, token, &ClientError{Err: err}
	}
	r, err := transport.DecodeRpcResponse(body)
	if err != nil {
		return nil, token, &ClientError{Err: fmt.Errorf("HTTP %d: no CSIL-RPC envelope: %w", resp.StatusCode, err)}
	}
	if !r.Status.IsOk() {
		msg := r.Status.Name()
		if r.Error != nil {
			msg = *r.Error
		}
		return nil, token, &ClientError{Code: statusCode(r.Status), Message: msg}
	}
	if r.Variant != nil && *r.Variant == "ServiceError" {
		se, err := DecodeServiceError(r.Payload)
		if err != nil {
			return nil, token, &ClientError{Err: fmt.Errorf("decode ServiceError: %w", err)}
		}
		return nil, token, &ClientError{Code: int64(se.Code), Message: se.Message}
	}
	return r.Payload, token, nil
}

// statusCode maps a transport status to an HTTP-shaped code.
func statusCode(s transport.Status) int64 {
	switch s.Code() {
	case transport.StatusMalformedEnvelope.Code():
		return http.StatusBadRequest
	case transport.StatusUnknownServiceOrOp.Code():
		return http.StatusNotFound
	case transport.StatusUnauthenticated.Code():
		return http.StatusUnauthorized
	case transport.StatusForbidden.Code():
		return http.StatusForbidden
	case transport.StatusUnavailable.Code():
		return http.StatusServiceUnavailable
	case transport.StatusDeadlineExceeded.Code():
		return http.StatusGatewayTimeout
	}
	return http.StatusInternalServerError
}
