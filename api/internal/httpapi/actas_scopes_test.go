package httpapi

import (
	"crypto/ed25519"
	"testing"
	"time"

	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	api "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"

	"github.com/catalystcommunity/catalystcal/api/internal/actas"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
)

// TestScopeSetRequestedScopes signs real scope sets through the HTTP
// carrier and verifies them with the regular-RP SDK.
func TestScopeSetRequestedScopes(t *testing.T) {
	e := newEnv(t)
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 7)
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	own := api.ApplicationRef{SubjectUserId: "catalystcal-account", SubjectDomain: "catalystcal.example", ApplicationId: "catalystcal"}
	e.deps.ActAs = &actas.Verifier{
		Own: own, InstanceID: "catalystcal-1",
		Signers: []regularrp.ApplicationSigner{{KeyID: "k1", Algorithm: "ed25519", PrivateKeyBytes: seed}},
	}
	now := time.Now().UTC()
	keys := []regularrp.ApplicationKeyRef{{
		KeyID: "k1", KeyUsage: "sign", Algorithm: "ed25519", PublicKey: pub, Fingerprint: regularrp.Fingerprint(pub),
		CreatedAt: now.Add(-time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
	}}
	ask := func(scopes []string) reply[csil.ScopeSetResponse] {
		return call(e.client(), "AuthService", "get-act-as-scope-set", csil.ScopeSetRequest{
			GranteeDomain: "longhouse.example", GranteeUserId: "acct", GranteeApplicationId: "longhouse", Scopes: scopes,
		}, csil.EncodeScopeSetRequest, csil.DecodeScopeSetResponse)
	}
	entries := func(scopes []string) []string {
		t.Helper()
		resp := must(t, ask(scopes))
		signed, err := api.DecodeSignedActAsScopeSet(resp.SignedScopeSet)
		if err != nil {
			t.Fatal(err)
		}
		set, err := regularrp.VerifyScopeSet(signed, own, keys, regularrp.AcceptBeforeRevocation)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		var out []string
		for _, en := range set.Entries {
			out = append(out, en.Scope)
		}
		return out
	}
	same := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// The request of an old client, without scopes: every scope.
	if got := entries(nil); !same(got, actas.ScopeOrder) {
		t.Fatalf("no scopes: %v", got)
	}
	two := []string{"calendars.read", "events.write"}
	if got := entries(two); !same(got, two) {
		t.Fatalf("two scopes: %v", got)
	}
	// Reordered: the set uses the server's order.
	if got := entries([]string{"events.write", "calendars.read"}); !same(got, two) {
		t.Fatalf("reordered: %v", got)
	}
	for name, bad := range map[string][]string{
		"empty":     {},
		"unknown":   {"calendars.read", "calendars.delete"},
		"duplicate": {"events.write", "events.write"},
	} {
		if r := wantCode(t, ask(bad), 400); r.svcErr.Message == "" {
			t.Fatalf("%s: no message", name)
		}
	}
}
