package services

import (
	"reflect"
	"strings"
	"testing"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
)

// generatedServices lists every generated service interface. A new CSIL
// service must be added here; the length check below catches a miss.
var generatedServices = []reflect.Type{
	reflect.TypeOf((*csil.AuthService)(nil)).Elem(),
	reflect.TypeOf((*csil.DevAuthService)(nil)).Elem(),
	reflect.TypeOf((*csil.UserService)(nil)).Elem(),
	reflect.TypeOf((*csil.CalendarService)(nil)).Elem(),
	reflect.TypeOf((*csil.ShareService)(nil)).Elem(),
	reflect.TypeOf((*csil.DelegationService)(nil)).Elem(),
	reflect.TypeOf((*csil.EventService)(nil)).Elem(),
	reflect.TypeOf((*csil.FreeBusyService)(nil)).Elem(),
	reflect.TypeOf((*csil.IcsService)(nil)).Elem(),
	reflect.TypeOf((*csil.AuditService)(nil)).Elem(),
}

// TestEveryGeneratedOpIsRegistered walks every method of every generated
// service interface and checks the dispatcher has it, so the generated
// client never hits "unknown service/op".
func TestEveryGeneratedOpIsRegistered(t *testing.T) {
	d := csilrpc.New([]byte("secret"))
	Register(d, &Deps{}, true)
	total := 0
	for _, it := range generatedServices {
		key := strings.ToLower(strings.TrimSuffix(it.Name(), "Service"))
		for i := 0; i < it.NumMethod(); i++ {
			m := it.Method(i).Name
			total++
			if !d.Registered(key, m) {
				t.Errorf("%s.%s is not registered (key %q)", it.Name(), m, key)
			}
		}
	}
	// 8 + 2 + 3 + 8 + 3 + 5 + 9 + 1 + 3 + 1 ops in csil/catalystcal.csil.
	if total != 43 {
		t.Fatalf("walked %d ops, want 43: a service interface is missing from generatedServices", total)
	}
}

func TestDevAuthAbsentWhenDisabled(t *testing.T) {
	d := csilrpc.New([]byte("secret"))
	Register(d, &Deps{}, false)
	if d.Registered("devauth", "DevLogin") || d.Registered("devauth", "ListDevUsers") {
		t.Fatal("dev auth registered while disabled")
	}
}

// The interface assertions in each service file are the compile-time half
// of this check; this keeps the service types honest at run time too.
func TestServiceTypesImplementInterfaces(t *testing.T) {
	deps := &Deps{}
	impls := []any{&AuthService{deps}, &DevAuthService{deps}, &UserService{deps}, &CalendarService{deps}, &ShareService{deps},
		&DelegationService{deps}, &EventService{deps}, &FreeBusyService{deps}, &IcsService{deps}, &AuditService{deps}}
	for i, it := range generatedServices {
		if !reflect.TypeOf(impls[i]).Implements(it) {
			t.Errorf("%T does not implement %s", impls[i], it.Name())
		}
	}
}
