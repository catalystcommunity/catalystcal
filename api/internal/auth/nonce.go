package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

// Login nonces are self-verifying: they carry their own expiry and an HMAC,
// so the server keeps NO state between /auth/start and /auth/complete. We
// mint a nonce, the RP binds it into the signed request, the IDP echoes it
// back inside the assertion, and we re-check the HMAC + expiry on return.
//
// This proves "we issued this login recently" without a cookie or store —
// the property the no-cookie, any-client design needs. It does NOT enforce
// single-use within the window; that protection leans on the assertion's own
// short expiry and linkkeys' single-use of the sealed token.

const NonceTTL = 10 * time.Minute

const (
	nonceRandLen = 16
	nonceExpLen  = 8 // unix seconds, big-endian
	nonceMACLen  = sha256.Size
	nonceRawLen  = nonceRandLen + nonceExpLen + nonceMACLen
)

var (
	ErrNonceInvalid = errors.New("auth: login nonce is invalid")
	ErrNonceExpired = errors.New("auth: login nonce has expired")
)

// MintNonce returns a fresh self-verifying nonce valid for NonceTTL.
func MintNonce(secret []byte) string {
	body := make([]byte, nonceRandLen+nonceExpLen)
	_, _ = rand.Read(body[:nonceRandLen])
	exp := time.Now().UTC().Add(NonceTTL).Unix()
	binary.BigEndian.PutUint64(body[nonceRandLen:], uint64(exp))
	mac := nonceMAC(secret, body)
	return base64.RawURLEncoding.EncodeToString(append(body, mac...))
}

// VerifyNonce checks a nonce's HMAC and expiry. Returns nil when the nonce
// is one we minted and is still fresh.
func VerifyNonce(secret []byte, nonce string) error {
	raw, err := base64.RawURLEncoding.DecodeString(nonce)
	if err != nil || len(raw) != nonceRawLen {
		return ErrNonceInvalid
	}
	body, mac := raw[:nonceRandLen+nonceExpLen], raw[nonceRandLen+nonceExpLen:]
	if !hmac.Equal(nonceMAC(secret, body), mac) {
		return ErrNonceInvalid
	}
	exp := int64(binary.BigEndian.Uint64(body[nonceRandLen:]))
	if time.Now().UTC().Unix() >= exp {
		return ErrNonceExpired
	}
	return nil
}

func nonceMAC(secret, body []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("catalystcal-login-nonce\x00")) // domain separation from token sig
	m.Write(body)
	return m.Sum(nil)
}
