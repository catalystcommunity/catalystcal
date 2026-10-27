// Package csilrpc serves CSIL services over the canonical CSIL-RPC v1 carrier:
// one endpoint takes a CBOR CsilRpcRequest envelope and returns a
// CsilRpcResponse envelope (see envelope.go). The envelope names the service
// and the op, so the HTTP path has no meaning.
//
// Each op goes through a typed route (RegisterTyped). The generated codec owns
// the payload bytes. The dispatcher adds bearer checks, the delegation re-check,
// audit, and the declared ServiceError arm.
package csilrpc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	log "github.com/sirupsen/logrus"
)

// IdentityChecker re-checks a verified bearer before each call. It returns an
// error when a delegated bearer's delegation is revoked or expired, or when
// the user no longer exists. A nil checker accepts every verified bearer.
type IdentityChecker func(ctx context.Context, id *auth.Identity) error

// AuditEntry is one authenticated call, written after the call returns.
type AuditEntry struct {
	Identity  *auth.Identity
	Service   string
	Method    string
	Target    string
	Outcome   string // ok, error, denied
	Mutation  bool
	Delegated bool
}

// AuditRecorder stores audit entries. The dispatcher records every mutation
// and every call made through a delegation or an act-as grant.
type AuditRecorder interface {
	RecordAudit(ctx context.Context, e AuditEntry) error
}

// Dispatcher routes (service, op) to a registered typed handler.
type Dispatcher struct {
	typedRegistry map[string]map[string]TypedHandler
	publicMethods map[string]bool
	jwtSecret     []byte
	checker       IdentityChecker
	audit         AuditRecorder
}

// New makes an empty Dispatcher. Without a secret, every non-public method
// fails closed.
func New(jwtSecret []byte) *Dispatcher {
	return &Dispatcher{
		typedRegistry: map[string]map[string]TypedHandler{},
		publicMethods: map[string]bool{},
		jwtSecret:     jwtSecret,
	}
}

// SetIdentityChecker installs the per-call bearer re-check.
func (d *Dispatcher) SetIdentityChecker(c IdentityChecker) { d.checker = c }

// SetAuditRecorder installs the audit sink.
func (d *Dispatcher) SetAuditRecorder(r AuditRecorder) { d.audit = r }

// RegisterTyped attaches a handler that needs a bearer. The service is the
// lower-case name without "Service" ("event"); the method is PascalCase
// ("ListEvents"). A duplicate registration panics at startup.
func (d *Dispatcher) RegisterTyped(service, method string, h TypedHandler) {
	d.registerTyped(service, method, h)
}

// RegisterTypedPublic attaches a handler that needs no bearer.
func (d *Dispatcher) RegisterTypedPublic(service, method string, h TypedHandler) {
	d.registerTyped(service, method, h)
	d.publicMethods[service+"."+method] = true
}

func (d *Dispatcher) registerTyped(service, method string, h TypedHandler) {
	svc := d.typedRegistry[service]
	if svc == nil {
		svc = map[string]TypedHandler{}
		d.typedRegistry[service] = svc
	}
	if _, exists := svc[method]; exists {
		panic("csilrpc: duplicate typed registration for " + service + "." + method)
	}
	svc[method] = h
}

// Registered reports whether service.method has a handler. Tests use it to
// prove that every generated op is wired.
func (d *Dispatcher) Registered(service, method string) bool {
	return d.typedRegistry[service][method] != nil
}

// readPrefixes name non-mutating methods. Everything else is a mutation.
var readPrefixes = []string{"List", "Get", "Me", "Lookup", "Search", "Export", "Query"}

func isMutation(method string) bool {
	for _, p := range readPrefixes {
		if strings.HasPrefix(method, p) {
			return false
		}
	}
	return true
}

func (d *Dispatcher) emitAudit(ctx context.Context, service, method string, callErr error) {
	if d.audit == nil {
		return
	}
	id := auth.IdentityFromContext(ctx)
	if id == nil {
		return
	}
	mutation := isMutation(method)
	if !mutation && !id.IsDelegated() {
		return
	}
	outcome := "ok"
	if callErr != nil {
		outcome = "error"
		var serr *Error
		if errors.As(callErr, &serr) && (serr.Code == http.StatusUnauthorized || serr.Code == http.StatusForbidden) {
			outcome = "denied"
		}
	}
	entry := AuditEntry{
		Identity:  id,
		Service:   service,
		Method:    method,
		Target:    auditTarget(ctx),
		Outcome:   outcome,
		Mutation:  mutation,
		Delegated: id.IsDelegated(),
	}
	if err := d.audit.RecordAudit(ctx, entry); err != nil {
		log.WithFields(log.Fields{"service": service, "method": method}).
			WithError(err).Error("audit: record failed")
	}
}

type auditKey struct{}

type auditSlot struct {
	mu     sync.Mutex
	target string
}

func withAuditSlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, auditKey{}, &auditSlot{})
}

// SetAuditTarget names the resource a call touched, for example
// "event:01J...". The dispatcher writes it in the audit entry.
func SetAuditTarget(ctx context.Context, target string) {
	if s, ok := ctx.Value(auditKey{}).(*auditSlot); ok {
		s.mu.Lock()
		s.target = target
		s.mu.Unlock()
	}
}

func auditTarget(ctx context.Context) string {
	if s, ok := ctx.Value(auditKey{}).(*auditSlot); ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.target
	}
	return ""
}

// maxRequestBytes caps the request body. An iCalendar import is the largest
// payload, so the cap is 8 MiB.
const maxRequestBytes = 8 << 20
