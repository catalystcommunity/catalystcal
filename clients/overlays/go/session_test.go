// Hand-written. Source: clients/overlays/go/ (copied in by clients/regenerate.sh).

package catalystcalclient

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/clients/go/internal/transport"
)

// The server test api/internal/apppresent checks the same vector.
func TestPresentationSignatureInputVector(t *testing.T) {
	got := hex.EncodeToString(PresentationSignatureInput([]byte{0xa0}))
	want := "8278" + "27" + hex.EncodeToString([]byte(PresentationTag)) + "41a0"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestAppSignerSigns(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	key := ed25519.NewKeyFromSeed(seed)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	s := &AppSigner{
		Application: ApplicationRef{SubjectUserId: "u", SubjectDomain: "d.example", ApplicationId: "longhouse"},
		InstanceID:  "i1", KeyID: "k1", Key: key, Now: func() time.Time { return now },
	}
	aud := ApplicationRef{SubjectUserId: "c", SubjectDomain: "cal.example", ApplicationId: "catalystcal"}
	a, err := s.Sign(aud)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Sign(aud)
	if !ed25519.Verify(key.Public().(ed25519.PublicKey), PresentationSignatureInput(a.Presentation), a.Signature) {
		t.Fatal("signature does not verify")
	}
	p, err := DecodeApplicationPresentation(a.Presentation)
	if err != nil {
		t.Fatal(err)
	}
	if p.Audience != aud || p.SignerInstanceId != "i1" || p.PresentedAt != "2026-11-01T12:00:00Z" || len(p.Nonce) != 32 {
		t.Fatalf("presentation %+v", p)
	}
	if hex.EncodeToString(p.RequestDigest) != hex.EncodeToString(Digest(PresentationDigestInput)) {
		t.Fatal("digest")
	}
	q, _ := DecodeApplicationPresentation(b.Presentation)
	if string(q.Nonce) == string(p.Nonce) {
		t.Fatal("nonce repeats")
	}
	if _, err := (&AppSigner{}).Sign(aud); err == nil {
		t.Fatal("empty signer signed")
	}
}

// fakeServer answers CSIL-RPC envelopes with handler(service, op, auth).
func fakeServer(t *testing.T, handler func(service, op, auth string) transport.RpcResponse) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != RPCPath {
			t.Errorf("path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		req, err := transport.DecodeRpcRequest(body)
		if err != nil {
			t.Errorf("decode request: %v", err)
		}
		out, _ := handler(req.Service, req.Op, r.Header.Get("Authorization")).Encode()
		w.Write(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAppSessionExchangesAndRefreshes(t *testing.T) {
	var exchanges, refusals atomic.Int32
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	clock := now
	srv := fakeServer(t, func(service, op, auth string) transport.RpcResponse {
		switch op {
		case "get-application-identity":
			return transport.NewRpcResponseOk("ApplicationIdentityResponse", EncodeApplicationIdentityResponse(ApplicationIdentityResponse{
				Audience: ApplicationRef{SubjectUserId: "c", SubjectDomain: "cal.example", ApplicationId: "catalystcal"},
			}))
		case "exchange-application-presentation":
			n := exchanges.Add(1)
			return transport.NewRpcResponseOk("LoginResponse", EncodeLoginResponse(LoginResponse{
				Token:     "bearer-" + string(rune('0'+n)),
				ExpiresAt: Timestamp(clock.Add(15 * time.Minute).Format("2006-01-02T15:04:05Z")),
				Via:       "self",
			}))
		case "me":
			if auth == "Bearer bearer-2" && refusals.Load() == 0 {
				refusals.Add(1)
				return transport.NewRpcResponseTransportError(transport.StatusUnauthenticated, "bearer refused")
			}
			return transport.NewRpcResponseOk("MeResponse", EncodeMeResponse(MeResponse{User: User{UserId: UserID(auth)}, Via: "self", Principals: []PersonSummary{}}))
		}
		return transport.NewRpcResponseOk("ServiceError", EncodeServiceError(ServiceError{Code: 404, Message: "no " + op}))
	})
	signer := &AppSigner{
		Application: ApplicationRef{SubjectUserId: "u", SubjectDomain: "d.example", ApplicationId: "longhouse"},
		InstanceID:  "i1", KeyID: "k1", Key: ed25519.NewKeyFromSeed(make([]byte, 32)), Now: func() time.Time { return clock },
	}
	session := NewAppSession(srv.URL, signer)
	auth := NewAuthClient(NewHTTPTransport(srv.URL, session))
	ctx := context.Background()

	me, err := auth.Me(ctx, EmptyRequest{})
	if err != nil || me.User.UserId != "Bearer bearer-1" {
		t.Fatalf("me %+v %v", me.User, err)
	}
	// The cached bearer is reused.
	if _, err := auth.Me(ctx, EmptyRequest{}); err != nil || exchanges.Load() != 1 {
		t.Fatalf("exchanges %d %v", exchanges.Load(), err)
	}
	// Near expiry, a new bearer. The server refuses bearer-2 once: the
	// transport drops it, exchanges again, and retries.
	clock = now.Add(14*time.Minute + 30*time.Second)
	me, err = auth.Me(ctx, EmptyRequest{})
	if err != nil || exchanges.Load() != 3 || refusals.Load() != 1 || me.User.UserId != "Bearer bearer-3" {
		t.Fatalf("refresh: exchanges %d refusals %d me %q %v", exchanges.Load(), refusals.Load(), me.User.UserId, err)
	}
	// A ServiceError is a *ClientError with the code.
	_, err = NewCalendarClient(NewHTTPTransport(srv.URL, session)).ListCalendars(ctx, EmptyRequest{})
	ce, ok := err.(*ClientError)
	if !ok || ce.Code != 404 {
		t.Fatalf("service error: %v", err)
	}
}
