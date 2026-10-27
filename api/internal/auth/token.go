// Package auth issues and checks catalystcal bearer tokens.
//
// The token is not JWT. It has three base64url parts, `header.payload.sig`.
// The header and the payload are CBOR. The signature is HMAC-SHA256 over
// `header.payload`.
//
// A token always names the real caller (the actor). A delegated token also
// names the principal whose authority it uses, the delegation or act-as
// grant behind it, and the scopes. The dispatcher re-checks a delegated
// token against the store on every call, so a revoked delegation stops at
// once and not at token expiry.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
)

const (
	// DefaultTTL is the life of a self bearer.
	DefaultTTL = 12 * time.Hour
	// DelegatedTTL is the longest life of a delegated bearer.
	DelegatedTTL = time.Hour
)

// Via values. They match the CSIL ActingVia enumeration.
const (
	ViaSelf       = "self"
	ViaDelegation = "delegation"
	ViaActAsGrant = "act_as_grant"
)

var (
	ErrTokenMalformed = errors.New("auth: token is malformed")
	ErrBadSignature   = errors.New("auth: token signature is invalid")
	ErrTokenExpired   = errors.New("auth: token has expired")
)

// Identity is the CBOR payload of a bearer token.
type Identity struct {
	// UserID is the catalystcal user id of the real caller. For an act-as grant it
	// is the principal, because the grantee is an application, not a user.
	UserID      string `cbor:"uid"`
	Domain      string `cbor:"domain"`
	Subject     string `cbor:"sub"`
	DisplayName string `cbor:"name,omitempty"`

	// Via is self, delegation, or act_as_grant.
	Via string `cbor:"via"`
	// PrincipalID is the user whose authority this bearer uses. Empty for
	// a self bearer.
	PrincipalID  string   `cbor:"principal,omitempty"`
	DelegationID string   `cbor:"delegation,omitempty"`
	GrantID      string   `cbor:"grant,omitempty"`
	Application  string   `cbor:"app,omitempty"`
	Scopes       []string `cbor:"scopes,omitempty"`

	// SessionID names the browser session that minted a self bearer.
	SessionID string `cbor:"session,omitempty"`

	IssuedAt  int64 `cbor:"iat"`
	ExpiresAt int64 `cbor:"exp"`
}

// EffectiveUserID is the user whose resources the call acts on.
func (id *Identity) EffectiveUserID() string {
	if id.PrincipalID != "" {
		return id.PrincipalID
	}
	return id.UserID
}

// IsDelegated is true for a delegation or act-as grant bearer.
func (id *Identity) IsDelegated() bool {
	return id.Via == ViaDelegation || id.Via == ViaActAsGrant
}

// HasScope is true for a self bearer (the owner has every scope), or when a
// delegated bearer carries the scope.
func (id *Identity) HasScope(scope string) bool {
	if !id.IsDelegated() {
		return true
	}
	for _, s := range id.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

type tokenHeader struct {
	Alg string `cbor:"alg"`
	Typ string `cbor:"typ"`
}

// Mint signs a new token. A zero ttl uses DefaultTTL. A negative ttl is kept,
// so tests can mint an expired token.
func Mint(secret []byte, id Identity, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	if id.Via == "" {
		id.Via = ViaSelf
	}
	now := time.Now().UTC()
	id.IssuedAt = now.Unix()
	id.ExpiresAt = now.Add(ttl).Unix()

	header, err := cbor.Marshal(tokenHeader{Alg: "HS256", Typ: "catalystcal-id"})
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	payload, err := cbor.Marshal(id)
	if err != nil {
		return "", fmt.Errorf("marshal identity: %w", err)
	}
	signing := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	return signing + "." + sign(secret, signing), nil
}

// Verify checks the signature and the expiry and returns the identity.
func Verify(secret []byte, token string) (*Identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}
	signing := parts[0] + "." + parts[1]
	if subtle.ConstantTimeCompare([]byte(sign(secret, signing)), []byte(parts[2])) != 1 {
		return nil, ErrBadSignature
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenMalformed
	}
	var id Identity
	if err := cbor.Unmarshal(payload, &id); err != nil {
		return nil, ErrTokenMalformed
	}
	if id.UserID == "" {
		return nil, ErrTokenMalformed
	}
	if id.ExpiresAt > 0 && time.Now().UTC().Unix() >= id.ExpiresAt {
		return nil, ErrTokenExpired
	}
	return &id, nil
}

func sign(secret []byte, signing string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("catalystcal-bearer\x00"))
	mac.Write([]byte(signing))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type identityKey struct{}

// WithIdentity attaches the caller's identity to ctx.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFromContext returns the identity the dispatcher attached, or nil
// on a public op.
func IdentityFromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKey{}).(*Identity)
	return id
}
