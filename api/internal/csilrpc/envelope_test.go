package csilrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/transport"
	"github.com/fxamacker/cbor/v2"
)

// serveRPC drives one envelope through ServeRPC and returns the decoded reply.
func serveRPC(t *testing.T, d *Dispatcher, req transport.RpcRequest, bearer string) transport.RpcResponse {
	t.Helper()
	frame, err := req.Encode()
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, RPCMountPath, strings.NewReader(string(frame)))
	r.Header.Set("Content-Type", "application/cbor")
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	d.ServeRPC(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status: got %d want 200 (envelope carries the real status)", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("Content-Type: got %q want application/cbor", ct)
	}
	resp, err := transport.DecodeRpcResponse(w.Body.Bytes())
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestServeRPC_SuccessRoundTrip(t *testing.T) {
	d := New(nil)
	// The dispatcher is payload-agnostic: a typed route hands it a variant +
	// pre-encoded bytes. We borrow the generated CalendarID/ShareList codecs
	// (from the trusted-domain ops) to exercise a real round-trip under an
	// arbitrary route name, echoing the request through to prove the payload
	// survives tag-24 wrapping.
	d.RegisterTypedPublic("widget", "GetWidget", Route(
		func(_ context.Context, in csil.CalendarID) (csil.ShareList, error) {
			return csil.ShareList{Shares: []csil.Share{{CalendarId: in, Role: "read"}}}, nil
		},
		csil.DecodeShareListSharesRequest,
		csil.EncodeShareList,
	))

	payload := csil.EncodeShareListSharesRequest(csil.CalendarID("h-1"))
	resp := serveRPC(t, d, transport.NewRpcRequest("widget", "get-widget", payload).WithID(7), "")

	if !resp.Status.IsOk() {
		t.Fatalf("status: got %v want ok", resp.Status.Name())
	}
	if resp.ID == nil || *resp.ID != 7 {
		t.Fatalf("id not echoed: got %v want 7", resp.ID)
	}
	if resp.Variant == nil || *resp.Variant != "ShareList" {
		t.Fatalf("variant: got %v want ShareList", resp.Variant)
	}
	out, err := csil.DecodeShareList(resp.Payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(out.Shares) != 1 || out.Shares[0].CalendarId != "h-1" {
		t.Fatalf("payload round-trip failed: got result=false")
	}
}

func TestServeRPC_AcceptsGeneratedServiceName(t *testing.T) {
	d := New(nil)
	d.RegisterTypedPublic("widget", "GetWidget", Route(
		func(_ context.Context, _ csil.EmptyRequest) (csil.EmptyResponse, error) {
			return csil.EmptyResponse{}, nil
		},
		csil.DecodeEmptyRequest,
		csil.EncodeEmptyResponse,
	))
	resp := serveRPC(t, d, transport.NewRpcRequest("WidgetService", "get-widget", csil.EncodeEmptyRequest(csil.EmptyRequest{})), "")
	if !resp.Status.IsOk() {
		t.Fatalf("status: got %v want ok", resp.Status.Name())
	}
}

func TestServeRPC_ApplicationErrorIsServiceErrorArm(t *testing.T) {
	d := New(nil)
	d.RegisterTypedPublic("widget", "GetWidget", Route(
		func(_ context.Context, _ csil.EmptyRequest) (csil.EmptyResponse, error) {
			return csil.EmptyResponse{}, NotFound("no such widget")
		},
		csil.DecodeEmptyRequest,
		csil.EncodeEmptyResponse,
	))

	payload := csil.EncodeEmptyRequest(csil.EmptyRequest{})
	resp := serveRPC(t, d, transport.NewRpcRequest("widget", "get-widget", payload), "")

	// Application errors ride at status 0 with the ServiceError variant — never a
	// transport status.
	if !resp.Status.IsOk() {
		t.Fatalf("status: got %v want ok (application error is not a transport status)", resp.Status.Name())
	}
	if resp.Variant == nil || *resp.Variant != "ServiceError" {
		t.Fatalf("variant: got %v want ServiceError", resp.Variant)
	}
	var se csil.ServiceError
	if err := cbor.Unmarshal(resp.Payload, &se); err != nil {
		t.Fatalf("decode ServiceError: %v", err)
	}
	if se.Code != uint64(http.StatusNotFound) || se.Message != "no such widget" {
		t.Fatalf("ServiceError: got {%d, %q} want {404, \"no such widget\"}", se.Code, se.Message)
	}
}

func TestServeRPC_UnknownServiceOrOp(t *testing.T) {
	d := New(nil)
	resp := serveRPC(t, d, transport.NewRpcRequest("nope", "do-thing", []byte{}), "")
	if resp.Status.Code() != transport.StatusUnknownServiceOrOp.Code() {
		t.Fatalf("status: got %v want unknown-service-or-op", resp.Status.Name())
	}
}

func TestServeRPC_MalformedEnvelope(t *testing.T) {
	d := New(nil)
	r := httptest.NewRequest(http.MethodPost, RPCMountPath, strings.NewReader("not cbor"))
	w := httptest.NewRecorder()
	d.ServeRPC(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status: got %d want 200", w.Code)
	}
	resp, err := transport.DecodeRpcResponse(w.Body.Bytes())
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status.Code() != transport.StatusMalformedEnvelope.Code() {
		t.Fatalf("status: got %v want malformed-envelope", resp.Status.Name())
	}
}

func TestServeRPC_UnauthenticatedWhenNoBearer(t *testing.T) {
	d := New([]byte("secret"))
	d.RegisterTyped("widget", "GetWidget", Route(
		func(_ context.Context, _ csil.EmptyRequest) (csil.EmptyResponse, error) {
			return csil.EmptyResponse{}, nil
		},
		csil.DecodeEmptyRequest,
		csil.EncodeEmptyResponse,
	))
	resp := serveRPC(t, d, transport.NewRpcRequest("widget", "get-widget", []byte{}), "")
	if resp.Status.Code() != transport.StatusUnauthenticated.Code() {
		t.Fatalf("status: got %v want unauthenticated", resp.Status.Name())
	}
}

func TestMethodFromOp(t *testing.T) {
	cases := map[string]string{
		"me":                        "Me",
		"login":                     "Login",
		"list-tasks":                "ListTasks",
		"dev-login":                 "DevLogin",
		"list-dev-users":            "ListDevUsers",
		"get-member-by-identity":    "GetMemberByIdentity",
		"set-project-task-position": "SetProjectTaskPosition",
	}
	for op, want := range cases {
		if got := methodFromOp(op); got != want {
			t.Errorf("methodFromOp(%q) = %q, want %q", op, got, want)
		}
	}
}
