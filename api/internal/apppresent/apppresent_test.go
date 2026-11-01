package apppresent

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	api "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

// These tests drive the verifier with a fake key resolver. The fake gives
// the key status that the SDK's VerifyApplicationKeySet would give; the
// SDK's own conformance tests prove that classification. These tests prove
// that the verifier refuses every key that KeyForUse refuses, and every
// presentation rule that catalystcal owns.

var (
	own    = Ref{SubjectUserID: "cal-uuid", SubjectDomain: "cal.example", ApplicationID: "catalystcal"}
	signer = Ref{SubjectUserID: "lh-uuid", SubjectDomain: "longhouse.example", ApplicationID: "longhouse"}
	now    = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
)

const instance = "lh-1"

type fakeResolver struct {
	keys      []regularrp.VerifiedApplicationKey
	revoked   map[string]bool
	freshness regularrp.Freshness
	err       error
	asked     regularrp.InstanceRef
}

func (f *fakeResolver) Resolve(_ context.Context, ref regularrp.InstanceRef, _ *int64) (regularrp.ResolveResult, error) {
	f.asked = ref
	if f.err != nil {
		return regularrp.ResolveResult{}, f.err
	}
	return regularrp.ResolveResult{
		Keys:      regularrp.VerifiedApplicationKeySet{Keys: f.keys, RevokedKeyIDs: f.revoked},
		Freshness: f.freshness,
	}, nil
}

type testKey struct {
	id   string
	priv ed25519.PrivateKey
}

func newKey(t *testing.T, id string, seedByte byte) testKey {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = seedByte
	}
	return testKey{id: id, priv: ed25519.NewKeyFromSeed(seed)}
}

func attested(k testKey, usage, alg string, kind regularrp.KeyStatusKind) regularrp.VerifiedApplicationKey {
	return regularrp.VerifiedApplicationKey{
		Attestation: api.ApplicationKeyAttestation{
			SubjectUserId: signer.SubjectUserID, SubjectDomain: signer.SubjectDomain, ApplicationId: signer.ApplicationID,
			InstanceId: instance, KeyId: k.id, KeyUsage: usage, Algorithm: alg,
			PublicKey: k.priv.Public().(ed25519.PublicKey),
		},
		Status: regularrp.KeyStatus{Kind: kind},
	}
}

func presentation(mod func(*csil.ApplicationPresentation)) csil.ApplicationPresentation {
	p := csil.ApplicationPresentation{
		Signer:           signer.CSIL(),
		SignerInstanceId: instance,
		Audience:         own.CSIL(),
		RequestDigest:    RequestDigest(),
		PresentedAt:      csil.Timestamp(now.Add(-10 * time.Second).Format("2006-01-02T15:04:05Z")),
		Nonce:            []byte("0123456789abcdef"),
	}
	if mod != nil {
		mod(&p)
	}
	return p
}

func sign(k testKey, p csil.ApplicationPresentation) csil.SignedApplicationPresentation {
	raw := csil.EncodeApplicationPresentation(p)
	return csil.SignedApplicationPresentation{
		Presentation:  raw,
		SignedByKeyId: k.id,
		Signature:     ed25519.Sign(k.priv, SignatureInput(raw)),
	}
}

func verifier(r KeyResolver) *Verifier {
	return &Verifier{Own: own, Resolver: r, Allowed: []Ref{signer}, MaxAge: 300 * time.Second, Skew: 60 * time.Second,
		Now: func() time.Time { return now }}
}

func TestVerifyAccepts(t *testing.T) {
	k := newKey(t, "k1", 1)
	r := &fakeResolver{keys: []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed25519", regularrp.KeyStatusUsable)}}
	got, err := verifier(r).Verify(context.Background(), sign(k, presentation(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Signer != signer || got.InstanceID != instance || got.Nonce != hex.EncodeToString([]byte("0123456789abcdef")) {
		t.Fatalf("verified: %+v", got)
	}
	want := regularrp.InstanceRef{SubjectUserID: "lh-uuid", SubjectDomain: "longhouse.example", ApplicationID: "longhouse", InstanceID: instance}
	if r.asked != want {
		t.Fatalf("resolved %+v, want %+v", r.asked, want)
	}
	if got.NonceKey() != "app/lh-uuid@longhouse.example/longhouse/"+got.Nonce {
		t.Fatalf("nonce key %q", got.NonceKey())
	}
}

func TestVerifyRefuses(t *testing.T) {
	k := newKey(t, "k1", 1)
	other := newKey(t, "k2", 2)
	usable := []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed25519", regularrp.KeyStatusUsable)}

	cases := []struct {
		name   string
		keys   []regularrp.VerifiedApplicationKey
		revoke map[string]bool
		signed func() csil.SignedApplicationPresentation
		want   error
		reason string
	}{
		{name: "wrong audience", keys: usable, want: ErrRefused, reason: "audience",
			signed: func() csil.SignedApplicationPresentation {
				return sign(k, presentation(func(p *csil.ApplicationPresentation) {
					p.Audience = Ref{SubjectUserID: "cal-uuid", SubjectDomain: "cal.example", ApplicationID: "other"}.CSIL()
				}))
			}},
		{name: "wrong digest", keys: usable, want: ErrRefused, reason: "request_digest",
			signed: func() csil.SignedApplicationPresentation {
				return sign(k, presentation(func(p *csil.ApplicationPresentation) { p.RequestDigest = []byte("x") }))
			}},
		{name: "too old", keys: usable, want: ErrRefused, reason: "too old",
			signed: func() csil.SignedApplicationPresentation {
				return sign(k, presentation(func(p *csil.ApplicationPresentation) {
					p.PresentedAt = csil.Timestamp(now.Add(-361 * time.Second).Format("2006-01-02T15:04:05Z"))
				}))
			}},
		{name: "in the future", keys: usable, want: ErrRefused, reason: "future",
			signed: func() csil.SignedApplicationPresentation {
				return sign(k, presentation(func(p *csil.ApplicationPresentation) {
					p.PresentedAt = csil.Timestamp(now.Add(61 * time.Second).Format("2006-01-02T15:04:05Z"))
				}))
			}},
		{name: "bad time layout", keys: usable, want: ErrRefused, reason: "presented_at",
			signed: func() csil.SignedApplicationPresentation {
				return sign(k, presentation(func(p *csil.ApplicationPresentation) { p.PresentedAt = "2026-11-01T11:59:50+00:00" }))
			}},
		{name: "short nonce", keys: usable, want: ErrRefused, reason: "nonce",
			signed: func() csil.SignedApplicationPresentation {
				return sign(k, presentation(func(p *csil.ApplicationPresentation) { p.Nonce = []byte("short") }))
			}},
		{name: "act-as tag", keys: usable, want: ErrRefused, reason: "signature",
			signed: func() csil.SignedApplicationPresentation {
				s := sign(k, presentation(nil))
				in := []byte{0x82, 0x78, byte(len("linkkeys-act-as-presentation-v1alpha"))}
				in = append(in, "linkkeys-act-as-presentation-v1alpha"...)
				in = append(in, cborHead(2, uint64(len(s.Presentation)))...)
				in = append(in, s.Presentation...)
				s.Signature = ed25519.Sign(k.priv, in)
				return s
			}},
		{name: "unsigned payload", keys: usable, want: ErrRefused, reason: "signature",
			signed: func() csil.SignedApplicationPresentation {
				s := sign(k, presentation(nil))
				s.Signature = ed25519.Sign(k.priv, s.Presentation)
				return s
			}},
		{name: "tampered bytes", keys: usable, want: ErrRefused, reason: "signature",
			signed: func() csil.SignedApplicationPresentation {
				s := sign(k, presentation(nil))
				s.Presentation = csil.EncodeApplicationPresentation(presentation(func(p *csil.ApplicationPresentation) {
					p.Nonce = []byte("fedcba9876543210")
				}))
				return s
			}},
		{name: "bad signature", keys: usable, want: ErrRefused, reason: "signature",
			signed: func() csil.SignedApplicationPresentation {
				s := sign(other, presentation(nil))
				s.SignedByKeyId = "k1"
				return s
			}},
		{name: "unknown key", keys: usable, want: ErrRefused, reason: "not usable",
			signed: func() csil.SignedApplicationPresentation { return sign(other, presentation(nil)) }},
		{name: "agree key", keys: []regularrp.VerifiedApplicationKey{attested(k, "agree", "x25519", regularrp.KeyStatusUsable)},
			want: ErrRefused, reason: "not usable",
			signed: func() csil.SignedApplicationPresentation { return sign(k, presentation(nil)) }},
		{name: "wrong algorithm", keys: []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed448", regularrp.KeyStatusUsable)},
			want: ErrRefused, reason: "not usable",
			signed: func() csil.SignedApplicationPresentation { return sign(k, presentation(nil)) }},
		{name: "revoked key", keys: []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed25519", regularrp.KeyStatusRevoked)},
			want: ErrRefused, reason: "not usable",
			signed: func() csil.SignedApplicationPresentation { return sign(k, presentation(nil)) }},
		{name: "revocation list", keys: usable, revoke: map[string]bool{"k1": true}, want: ErrRefused, reason: "not usable",
			signed: func() csil.SignedApplicationPresentation { return sign(k, presentation(nil)) }},
		{name: "expired attestation", keys: []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed25519", regularrp.KeyStatusAttestationExpired)},
			want: ErrRefused, reason: "not usable",
			signed: func() csil.SignedApplicationPresentation { return sign(k, presentation(nil)) }},
		{name: "expired key", keys: []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed25519", regularrp.KeyStatusKeyExpired)},
			want: ErrRefused, reason: "not usable",
			signed: func() csil.SignedApplicationPresentation { return sign(k, presentation(nil)) }},
		{name: "not decodable", keys: usable, want: ErrRefused, reason: "decode",
			signed: func() csil.SignedApplicationPresentation {
				s := sign(k, presentation(nil))
				s.Presentation = []byte{0xff}
				return s
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeResolver{keys: tc.keys, revoked: tc.revoke}
			_, err := verifier(r).Verify(context.Background(), tc.signed())
			if !errors.Is(err, tc.want) {
				t.Fatalf("err %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("err %q does not name %q", err, tc.reason)
			}
		})
	}
}

func TestVerifyStaleAndUnresolved(t *testing.T) {
	k := newKey(t, "k1", 1)
	usable := []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed25519", regularrp.KeyStatusUsable)}
	r := &fakeResolver{keys: usable, freshness: regularrp.FreshnessStale}
	if _, err := verifier(r).Verify(context.Background(), sign(k, presentation(nil))); !errors.Is(err, ErrStale) {
		t.Fatalf("stale: %v", err)
	}
	r = &fakeResolver{err: errors.New("rp down")}
	if _, err := verifier(r).Verify(context.Background(), sign(k, presentation(nil))); !errors.Is(err, ErrRefused) {
		t.Fatalf("unresolved: %v", err)
	}
	if _, err := verifier(nil).Verify(context.Background(), sign(k, presentation(nil))); !errors.Is(err, ErrStale) {
		t.Fatalf("no resolver: %v", err)
	}
}

// The allow list needs the full identity: the same application id on
// another account is refused.
func TestVerifyNotAllowed(t *testing.T) {
	k := newKey(t, "k1", 1)
	r := &fakeResolver{keys: []regularrp.VerifiedApplicationKey{attested(k, "sign", "ed25519", regularrp.KeyStatusUsable)}}
	v := verifier(r)
	v.Allowed = []Ref{{SubjectUserID: "someone-else", SubjectDomain: "longhouse.example", ApplicationID: "longhouse"}}
	if _, err := v.Verify(context.Background(), sign(k, presentation(nil))); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("err %v", err)
	}
	// The refusal comes before the key lookup.
	if r.asked != (regularrp.InstanceRef{}) {
		t.Fatalf("resolved %+v for a signer that is not allowed", r.asked)
	}
}

func TestParseRef(t *testing.T) {
	r, err := ParseRef(" 01a1-uuid@Longhouse.Example/longhouse ")
	if err != nil || r != (Ref{SubjectUserID: "01a1-uuid", SubjectDomain: "longhouse.example", ApplicationID: "longhouse"}) {
		t.Fatalf("%+v %v", r, err)
	}
	for _, bad := range []string{"", "longhouse", "uuid@domain", "@domain/app", "uuid@/app", "uuid@domain/"} {
		if _, err := ParseRef(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestMaxAgeCeilingAndHold(t *testing.T) {
	v := &Verifier{MaxAge: time.Hour, Skew: 30 * time.Second}
	if v.MaxAgeOrDefault() != MaxAgeCeiling {
		t.Fatalf("max age %v", v.MaxAgeOrDefault())
	}
	if v.NonceHold() != MaxAgeCeiling+time.Minute {
		t.Fatalf("hold %v", v.NonceHold())
	}
}

// A fixed vector: these bytes are what a signer must sign. The Go client
// SDK test checks the same value.
func TestSignatureInputVector(t *testing.T) {
	got := hex.EncodeToString(SignatureInput([]byte{0xa0}))
	want := "8278" + "27" + hex.EncodeToString([]byte(Tag)) + "41a0"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
