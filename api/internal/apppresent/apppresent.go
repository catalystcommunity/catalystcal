// Package apppresent verifies the application presentations that
// AuthService.exchange-application-presentation accepts. An application
// presentation lets a LinkKeys application call catalystcal as itself, with
// no user. catalystcal defines the type (csil/catalystcal.csil); LinkKeys
// gives only the application keys and the RP key lookup.
//
// The signature covers CBOR([Tag, presentation bytes]). The verifier checks
// the bytes as they came, so the signer's CBOR encoder does not have to
// match catalystcal's.
package apppresent

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
)

// Tag is the domain-separation tag of the signature input.
const Tag = "catalystcal-application-presentation-v1"

// RequestDigestInput is the text whose SHA-256 is the request digest of
// exchange-application-presentation.
const RequestDigestInput = "catalystcal/AuthService/exchange-application-presentation"

// RequestDigest returns SHA-256(RequestDigestInput).
func RequestDigest() []byte {
	sum := sha256.Sum256([]byte(RequestDigestInput))
	return sum[:]
}

// Nonce length limits, in bytes.
const (
	MinNonceBytes = 16
	MaxNonceBytes = 64
)

// MaxAgeCeiling is the longest presentation age a server can accept. The
// nonce store holds each nonce for the age plus two times the skew.
const MaxAgeCeiling = 900 * time.Second

// Errors. Refused wraps the reason a presentation is not accepted.
var (
	// ErrRefused: the presentation is not valid proof.
	ErrRefused = errors.New("application presentation refused")
	// ErrNotAllowed: the proof is valid, but this server does not accept
	// the signer.
	ErrNotAllowed = errors.New("this catalystcal server does not accept this application")
	// ErrStale: the RP could not give current key material. Try again.
	ErrStale = errors.New("application keys are not current; try again later")
)

func refused(reason string) error { return fmt.Errorf("%w: %s", ErrRefused, reason) }

// Ref is a LinkKeys application identity.
type Ref struct {
	SubjectUserID string
	SubjectDomain string
	ApplicationID string
}

// FromCSIL converts and lower-cases the domain.
func FromCSIL(r csil.ApplicationRef) Ref {
	return Ref{SubjectUserID: r.SubjectUserId, SubjectDomain: strings.ToLower(r.SubjectDomain), ApplicationID: r.ApplicationId}
}

// CSIL converts back.
func (r Ref) CSIL() csil.ApplicationRef {
	return csil.ApplicationRef{SubjectUserId: r.SubjectUserID, SubjectDomain: r.SubjectDomain, ApplicationId: r.ApplicationID}
}

// String is "<subject_user_id>@<domain>/<application_id>", the form of
// CATALYSTCAL_ALLOWED_APPLICATIONS.
func (r Ref) String() string { return r.SubjectUserID + "@" + r.SubjectDomain + "/" + r.ApplicationID }

// ParseRef reads "<subject_user_id>@<domain>/<application_id>".
func ParseRef(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	slash := strings.LastIndex(s, "/")
	at := strings.LastIndex(s, "@")
	if at <= 0 || slash < at+2 || slash == len(s)-1 {
		return Ref{}, fmt.Errorf("%q is not <subject_user_id>@<domain>/<application_id>", s)
	}
	return Ref{SubjectUserID: s[:at], SubjectDomain: strings.ToLower(s[at+1 : slash]), ApplicationID: s[slash+1:]}, nil
}

// KeyResolver resolves an application instance's keys through catalystcal's
// RP. *regularrp.CachedResolver implements it; tests use a fake.
type KeyResolver interface {
	Resolve(ctx context.Context, instance regularrp.InstanceRef, maxCacheAgeSeconds *int64) (regularrp.ResolveResult, error)
}

var _ KeyResolver = (*regularrp.CachedResolver)(nil)

// Verified is an accepted presentation.
type Verified struct {
	Signer     Ref
	InstanceID string
	// Nonce is the presentation nonce in hex. The caller records it.
	Nonce       string
	PresentedAt time.Time
}

// NonceKey is the replay-store key of a verified presentation. The "app/"
// prefix keeps it apart from act-as nonces in the same store.
func (v Verified) NonceKey() string { return "app/" + v.Signer.String() + "/" + v.Nonce }

// Verifier checks presentations for one audience: catalystcal itself.
type Verifier struct {
	// Own is catalystcal's application identity, from its key file. It is
	// never read from a request.
	Own      Ref
	Resolver KeyResolver
	// Allowed lists the applications that may sign in, by full identity.
	// Anyone can enroll an application with any id, so the application id
	// alone proves nothing.
	Allowed []Ref
	// MaxAge is the oldest presentation accepted. It is capped at
	// MaxAgeCeiling.
	MaxAge time.Duration
	// Skew is the permitted clock difference. Default 60 s.
	Skew time.Duration
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now().UTC()
	}
	return time.Now().UTC()
}

// SkewOrDefault is the permitted clock difference.
func (v *Verifier) SkewOrDefault() time.Duration {
	if v.Skew > 0 {
		return v.Skew
	}
	return 60 * time.Second
}

// MaxAgeOrDefault is the accepted presentation age.
func (v *Verifier) MaxAgeOrDefault() time.Duration {
	if v.MaxAge <= 0 {
		return 300 * time.Second
	}
	return min(v.MaxAge, MaxAgeCeiling)
}

// NonceHold is how long a nonce must stay in the replay store.
func (v *Verifier) NonceHold() time.Duration { return v.MaxAgeOrDefault() + 2*v.SkewOrDefault() }

// Verify checks a signed presentation in this order: decode, audience,
// digest, age, allowed signer, key (through the RP; a stale answer is
// refused), signature. The allow list comes before the key lookup, so an
// anonymous caller cannot make the RP fetch the keys of any instance. The
// caller records the nonce after Verify succeeds.
func (v *Verifier) Verify(ctx context.Context, signed csil.SignedApplicationPresentation) (Verified, error) {
	p, err := csil.DecodeApplicationPresentation(signed.Presentation)
	if err != nil {
		return Verified{}, refused("presentation does not decode")
	}
	signer := FromCSIL(p.Signer)
	if signer.SubjectUserID == "" || signer.SubjectDomain == "" || signer.ApplicationID == "" || p.SignerInstanceId == "" {
		return Verified{}, refused("signer and signer_instance_id are required")
	}
	if FromCSIL(p.Audience) != v.Own {
		return Verified{}, refused("audience is not this catalystcal server")
	}
	if !equalBytes(p.RequestDigest, RequestDigest()) {
		return Verified{}, refused("request_digest does not match exchange-application-presentation")
	}
	at, err := time.Parse("2006-01-02T15:04:05Z", string(p.PresentedAt))
	if err != nil {
		return Verified{}, refused("presented_at must be a UTC time like 2026-10-15T17:00:00Z")
	}
	now, skew := v.now(), v.SkewOrDefault()
	if at.After(now.Add(skew)) {
		return Verified{}, refused("presented_at is in the future")
	}
	if now.Sub(at) > v.MaxAgeOrDefault()+skew {
		return Verified{}, refused("presentation is too old")
	}
	if len(p.Nonce) < MinNonceBytes || len(p.Nonce) > MaxNonceBytes {
		return Verified{}, refused(fmt.Sprintf("nonce must have %d to %d bytes", MinNonceBytes, MaxNonceBytes))
	}
	if signed.SignedByKeyId == "" || len(signed.Signature) != ed25519.SignatureSize {
		return Verified{}, refused("signed_by_key_id and a 64-byte signature are required")
	}
	if !v.allowed(signer) {
		return Verified{}, ErrNotAllowed
	}
	if v.Resolver == nil {
		return Verified{}, ErrStale
	}
	res, err := v.Resolver.Resolve(ctx, regularrp.InstanceRef{
		SubjectUserID: signer.SubjectUserID,
		SubjectDomain: signer.SubjectDomain,
		ApplicationID: signer.ApplicationID,
		InstanceID:    p.SignerInstanceId,
	}, nil)
	if err != nil {
		return Verified{}, refused("signer keys do not resolve")
	}
	// A stale answer is the last verified material, not current proof. A
	// bearer is not minted on it.
	if res.Freshness == regularrp.FreshnessStale {
		return Verified{}, ErrStale
	}
	key, err := res.Keys.KeyForUse(signed.SignedByKeyId, regularrp.KeyUsageSign, "ed25519")
	if err != nil {
		return Verified{}, refused("signing key is not usable: " + err.Error())
	}
	if len(key.PublicKey) != ed25519.PublicKeySize ||
		!ed25519.Verify(ed25519.PublicKey(key.PublicKey), SignatureInput(signed.Presentation), signed.Signature) {
		return Verified{}, refused("signature does not verify")
	}
	return Verified{Signer: signer, InstanceID: p.SignerInstanceId, Nonce: hex.EncodeToString(p.Nonce), PresentedAt: at}, nil
}

// Allows reports whether r is an allowed signer. The dispatcher also asks
// it on each call of an application bearer, so removing an application
// from the list stops it at once.
func (v *Verifier) Allows(r Ref) bool { return v.allowed(r) }

func (v *Verifier) allowed(r Ref) bool {
	for _, a := range v.Allowed {
		if a == r {
			return true
		}
	}
	return false
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var d byte
	for i := range a {
		d |= a[i] ^ b[i]
	}
	return d == 0
}

// SignatureInput is CBOR([Tag, presentation]): a definite two-element
// array of a text string and a byte string, in shortest form.
func SignatureInput(presentation []byte) []byte {
	out := cborHead(4, 2)
	out = append(out, cborHead(3, uint64(len(Tag)))...)
	out = append(out, Tag...)
	out = append(out, cborHead(2, uint64(len(presentation)))...)
	return append(out, presentation...)
}

func cborHead(major byte, n uint64) []byte {
	m := major << 5
	switch {
	case n < 24:
		return []byte{m | byte(n)}
	case n <= 0xff:
		return []byte{m | 24, byte(n)}
	case n <= 0xffff:
		b := []byte{m | 25, 0, 0}
		binary.BigEndian.PutUint16(b[1:], uint16(n))
		return b
	case n <= 0xffffffff:
		b := []byte{m | 26, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(b[1:], uint32(n))
		return b
	default:
		b := make([]byte, 9)
		b[0] = m | 27
		binary.BigEndian.PutUint64(b[1:], n)
		return b
	}
}
