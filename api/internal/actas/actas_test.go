package actas

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	api "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

// staticKeys is a KeySource with fixed material, as a vector supplies it.
type staticKeys struct {
	own, grantee []regularrp.ApplicationKeyRef
	domain       []api.DomainPublicKey
	revoked      []string
}

func (s staticKeys) OwnScopeSetKeys(context.Context, api.ApplicationRef, string) ([]regularrp.ApplicationKeyRef, error) {
	return s.own, nil
}
func (s staticKeys) GranteeInstanceKeys(context.Context, regularrp.InstanceRef) ([]regularrp.ApplicationKeyRef, error) {
	return s.grantee, nil
}
func (s staticKeys) DomainKeys(context.Context, string) ([]api.DomainPublicKey, error) {
	return s.domain, nil
}
func (s staticKeys) RevokedGrants(context.Context, string, []string) ([]string, error) {
	return s.revoked, nil
}

type keyVec struct {
	KeyID        string  `json:"key_id"`
	KeyUsage     string  `json:"key_usage"`
	Algorithm    string  `json:"algorithm"`
	PublicKeyHex string  `json:"public_key_hex"`
	Fingerprint  string  `json:"fingerprint"`
	CreatedAt    string  `json:"created_at"`
	ExpiresAt    string  `json:"expires_at"`
	RevokedAt    *string `json:"revoked_at"`
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex: %v", err)
	}
	return b
}

// TestConformanceCredentialThroughAdapter replays the regular-RP SDK's
// act_as_credential.json vectors through Verifier, so the adapter (key
// lookup, digest, clock, revocation plumbing, scope mapping) agrees with
// the Rust reference.
func TestConformanceCredentialThroughAdapter(t *testing.T) {
	raw, err := os.ReadFile("testdata/act_as_credential.json")
	if err != nil {
		t.Fatal(err)
	}
	type vec map[string]json.RawMessage
	var file struct {
		Context       vec   `json:"context"`
		Cases         []vec `json:"cases"`
		NegativeCases []vec `json:"negative_cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	cases := append(append([]vec{}, file.Cases...), file.NegativeCases...)
	if len(cases) < 10 {
		t.Fatalf("only %d cases", len(cases))
	}
	ran := 0
	for _, c := range cases {
		field := func(name string, out any) {
			t.Helper()
			r, ok := c[name]
			if !ok {
				r, ok = file.Context[name]
			}
			if !ok {
				t.Fatalf("missing field %s", name)
			}
			if err := json.Unmarshal(r, out); err != nil {
				t.Fatalf("field %s: %v", name, err)
			}
		}
		var name, credHex, digestHex, now string
		var valid bool
		var own struct {
			SubjectUserID string `json:"subject_user_id"`
			SubjectDomain string `json:"subject_domain"`
			ApplicationID string `json:"application_id"`
		}
		var ownKeys, granteeKeys, issuer []keyVec
		var revoked []string
		var maxAge, skew int64
		var policy *string
		field("name", &name)
		field("expected_valid", &valid)
		field("credential_cbor_hex", &credHex)
		field("own_application", &own)
		field("own_scope_set_keys", &ownKeys)
		field("issuer_domain_keys", &issuer)
		field("grantee_instance_keys", &granteeKeys)
		field("expected_request_digest_hex", &digestHex)
		field("revoked_grant_ids", &revoked)
		field("max_presentation_age_seconds", &maxAge)
		field("now", &now)
		field("skew_seconds", &skew)
		field("revoked_key_policy", &policy)
		if policy != nil && *policy != "accept_before_revocation" {
			continue // catalystcal always uses AcceptBeforeRevocation
		}
		ran++
		t.Run(name, func(t *testing.T) {
			nowT, err := time.Parse(time.RFC3339, now)
			if err != nil {
				t.Fatal(err)
			}
			v := &Verifier{
				Own: api.ApplicationRef{SubjectUserId: own.SubjectUserID, SubjectDomain: own.SubjectDomain, ApplicationId: own.ApplicationID},
				Keys: staticKeys{
					own:     appRefs(t, ownKeys),
					grantee: appRefs(t, granteeKeys),
					domain:  domainRefs(t, issuer),
					revoked: revoked,
				},
				Digest:      mustHex(t, digestHex),
				MaxAge:      time.Duration(maxAge) * time.Second,
				SkewSeconds: skew,
				Now:         func() time.Time { return nowT },
			}
			got, err := v.VerifyCredential(context.Background(), mustHex(t, credHex), nil)
			if valid != (err == nil) {
				t.Fatalf("valid = %v, want %v (err %v)", err == nil, valid, err)
			}
			if err != nil {
				return
			}
			var want struct {
				GrantID       string   `json:"grant_id"`
				UserID        string   `json:"user_id"`
				ApprovedScope []string `json:"approved_scope"`
				NonceHex      string   `json:"nonce_hex"`
			}
			if r, ok := c["expected"]; ok {
				if err := json.Unmarshal(r, &want); err != nil {
					t.Fatal(err)
				}
				if got.GrantID != want.GrantID {
					t.Errorf("grant id %q, want %q", got.GrantID, want.GrantID)
				}
				if got.SubjectUserID+"@"+got.SubjectDomain != want.UserID {
					t.Errorf("user %q@%q, want %q", got.SubjectUserID, got.SubjectDomain, want.UserID)
				}
				if got.Nonce != want.NonceHex {
					t.Errorf("nonce %q, want %q", got.Nonce, want.NonceHex)
				}
				// The vector approves non-catalystcal scope words; catalystcal drops them.
				if len(got.Scopes) != len(MapScopes(want.ApprovedScope)) {
					t.Errorf("scopes %v from %v", got.Scopes, want.ApprovedScope)
				}
			}
		})
	}
	if ran < 10 {
		t.Fatalf("ran only %d cases", ran)
	}
}

func appRefs(t *testing.T, in []keyVec) []regularrp.ApplicationKeyRef {
	out := make([]regularrp.ApplicationKeyRef, 0, len(in))
	for _, k := range in {
		out = append(out, regularrp.ApplicationKeyRef{
			KeyID: k.KeyID, KeyUsage: k.KeyUsage, Algorithm: k.Algorithm, PublicKey: mustHex(t, k.PublicKeyHex),
			Fingerprint: k.Fingerprint, CreatedAt: k.CreatedAt, ExpiresAt: k.ExpiresAt, RevokedAt: k.RevokedAt,
		})
	}
	return out
}

func domainRefs(t *testing.T, in []keyVec) []api.DomainPublicKey {
	out := make([]api.DomainPublicKey, 0, len(in))
	for _, k := range in {
		out = append(out, api.DomainPublicKey{
			KeyId: k.KeyID, KeyUsage: k.KeyUsage, Algorithm: k.Algorithm, PublicKey: mustHex(t, k.PublicKeyHex),
			Fingerprint: k.Fingerprint, CreatedAt: k.CreatedAt, ExpiresAt: k.ExpiresAt, RevokedAt: k.RevokedAt,
		})
	}
	return out
}

func TestSignScopeSetVerifies(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	own := api.ApplicationRef{SubjectUserId: "catalystcal-account", SubjectDomain: "catalystcal.example", ApplicationId: "catalystcal"}
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	v := &Verifier{
		Own:        own,
		InstanceID: "catalystcal-1",
		Signers:    []regularrp.ApplicationSigner{{KeyID: "k1", Algorithm: "ed25519", PrivateKeyBytes: seed}},
		Now:        func() time.Time { return now },
	}
	raw, err := v.SignScopeSet(context.Background(), ScopeSetRequest{
		GranteeDomain: "Grantee.Example", GranteeUserID: "acct", GranteeApplicationID: "helper",
	})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := api.DecodeSignedActAsScopeSet(raw)
	if err != nil {
		t.Fatal(err)
	}
	if signed.SignerInstanceId != "catalystcal-1" {
		t.Fatalf("signer instance %q", signed.SignerInstanceId)
	}
	keys := []regularrp.ApplicationKeyRef{{
		KeyID: "k1", KeyUsage: "sign", Algorithm: "ed25519", PublicKey: pub,
		Fingerprint: regularrp.Fingerprint(pub), CreatedAt: "2026-01-01T00:00:00Z", ExpiresAt: "2027-01-01T00:00:00Z",
	}}
	set, err := regularrp.VerifyScopeSet(signed, own, keys, regularrp.AcceptBeforeRevocation)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(set.Entries) != len(ScopeOrder) {
		t.Fatalf("entries %d", len(set.Entries))
	}
	if set.Grantee.Application == nil || set.Grantee.Application.SubjectDomain != "grantee.example" {
		t.Fatalf("grantee %+v", set.Grantee)
	}
}

func TestMapScopes(t *testing.T) {
	got := MapScopes([]string{"events.write", "read", "calendars.read", "events.write", "admin"})
	if len(got) != 2 || got[0] != "calendars.read" || got[1] != "events.write" {
		t.Fatalf("got %v", got)
	}
}

func TestRequestDigest(t *testing.T) {
	if hex.EncodeToString(RequestDigest()) == "" || len(RequestDigest()) != 32 {
		t.Fatal("digest")
	}
}
