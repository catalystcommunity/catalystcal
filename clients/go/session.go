// Hand-written. Source: clients/overlays/go/ (copied in by clients/regenerate.sh).

package catalystcalclient

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

// PresentationTag is the domain-separation tag of an application
// presentation signature. The signature covers CBOR([tag, presentation]).
const PresentationTag = "catalystcal-application-presentation-v1"

// PresentationDigestInput is the text whose SHA-256 is the request digest
// of AuthService.exchange-application-presentation.
const PresentationDigestInput = "catalystcal/AuthService/exchange-application-presentation"

// ActAsDigestInput is the text whose SHA-256 is the presentation request
// digest of AuthService.exchange-act-as-credential. A grantee builds its
// LinkKeys act-as presentation with this digest.
const ActAsDigestInput = "catalystcal/AuthService/exchange-act-as-credential"

// Digest returns SHA-256 of a digest input.
func Digest(input string) []byte {
	sum := sha256.Sum256([]byte(input))
	return sum[:]
}

// PresentationSignatureInput returns CBOR([PresentationTag, presentation]),
// the bytes the application key signs.
func PresentationSignatureInput(presentation []byte) []byte {
	out := ovlHead(4, 2)
	out = append(out, ovlHead(3, uint64(len(PresentationTag)))...)
	out = append(out, PresentationTag...)
	out = append(out, ovlHead(2, uint64(len(presentation)))...)
	return append(out, presentation...)
}

func ovlHead(major byte, n uint64) []byte {
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

// AppSigner signs application presentations with one Ed25519 application
// key of an enrolled LinkKeys application instance. The key must have a
// current attestation at the home domain; catalystcal checks it through its
// RP.
type AppSigner struct {
	// Application is the signer: the LinkKeys account, its domain, and the
	// application id.
	Application ApplicationRef
	// InstanceID is the enrolled application instance.
	InstanceID string
	// KeyID is the application key id at the home domain.
	KeyID string
	// Key is the Ed25519 private key.
	Key ed25519.PrivateKey
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

// Sign makes a signed presentation for audience.
func (s *AppSigner) Sign(audience ApplicationRef) (SignedApplicationPresentation, error) {
	if len(s.Key) != ed25519.PrivateKeySize || s.KeyID == "" || s.InstanceID == "" {
		return SignedApplicationPresentation{}, errors.New("catalystcal: AppSigner needs Key, KeyID, and InstanceID")
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return SignedApplicationPresentation{}, err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	raw := EncodeApplicationPresentation(ApplicationPresentation{
		Signer:           s.Application,
		SignerInstanceId: s.InstanceID,
		Audience:         audience,
		RequestDigest:    Digest(PresentationDigestInput),
		PresentedAt:      Timestamp(now().UTC().Format("2006-01-02T15:04:05Z")),
		Nonce:            nonce,
	})
	return SignedApplicationPresentation{
		Presentation:  raw,
		SignedByKeyId: s.KeyID,
		Signature:     ed25519.Sign(s.Key, PresentationSignatureInput(raw)),
	}, nil
}

// refreshBefore is how long before expiry a session gets a new bearer.
const refreshBefore = time.Minute

// bearerCache holds one bearer and makes a new one when it is near its
// expiry or after Invalidate. It is safe for concurrent use.
type bearerCache struct {
	mu      sync.Mutex
	token   string
	expires time.Time
	mint    func(ctx context.Context) (LoginResponse, error)
	now     func() time.Time
}

func (c *bearerCache) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	if c.token != "" && now().Add(refreshBefore).Before(c.expires) {
		return c.token, nil
	}
	resp, err := c.mint(ctx)
	if err != nil {
		return "", err
	}
	exp, err := time.Parse(time.RFC3339, string(resp.ExpiresAt))
	if err != nil {
		return "", fmt.Errorf("catalystcal: bearer expires_at: %w", err)
	}
	c.token, c.expires = resp.Token, exp
	return c.token, nil
}

func (c *bearerCache) Invalidate(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == token {
		c.token = ""
	}
}

// AppSession is the bearer of an application principal. It reads
// catalystcal's audience once, signs a fresh presentation, and exchanges
// it for a short bearer when the old one is near its expiry.
//
//	session := catalystcalclient.NewAppSession("https://cal.example.com", signer)
//	t := catalystcalclient.NewHTTPTransport("https://cal.example.com", session)
//	cal, err := catalystcalclient.NewCalendarClient(t).CreateCalendar(ctx, ...)
type AppSession struct {
	bearerCache
	signer *AppSigner
	auth   *AuthClient

	audMu    sync.Mutex
	audience *ApplicationRef
}

var _ TokenSource = (*AppSession)(nil)

// NewAppSession makes the session for one server.
func NewAppSession(baseURL string, signer *AppSigner) *AppSession {
	s := &AppSession{signer: signer, auth: NewAuthClient(NewHTTPTransport(baseURL, nil))}
	s.mint = s.exchange
	s.now = signer.Now
	return s
}

func (s *AppSession) audienceRef(ctx context.Context) (ApplicationRef, error) {
	s.audMu.Lock()
	defer s.audMu.Unlock()
	if s.audience != nil {
		return *s.audience, nil
	}
	id, err := s.auth.GetApplicationIdentity(ctx, EmptyRequest{})
	if err != nil {
		return ApplicationRef{}, err
	}
	s.audience = &id.Audience
	return id.Audience, nil
}

func (s *AppSession) exchange(ctx context.Context) (LoginResponse, error) {
	aud, err := s.audienceRef(ctx)
	if err != nil {
		return LoginResponse{}, err
	}
	signed, err := s.signer.Sign(aud)
	if err != nil {
		return LoginResponse{}, err
	}
	return s.auth.ExchangeApplicationPresentation(ctx, ExchangeApplicationPresentationRequest{Presentation: signed})
}

// ActAsSession is the bearer of one user through a LinkKeys act-as grant.
// Credential makes a fresh CBOR(ActAsCredential) for each exchange: the
// grant plus a new presentation (audience catalystcal, request digest
// Digest(ActAsDigestInput)). The LinkKeys regular-RP SDK builds it.
type ActAsSession struct {
	bearerCache
	auth *AuthClient
}

var _ TokenSource = (*ActAsSession)(nil)

// NewActAsSession makes the session for one user at one server.
func NewActAsSession(baseURL string, credential func(ctx context.Context) ([]byte, error)) *ActAsSession {
	s := &ActAsSession{auth: NewAuthClient(NewHTTPTransport(baseURL, nil))}
	s.mint = func(ctx context.Context) (LoginResponse, error) {
		cred, err := credential(ctx)
		if err != nil {
			return LoginResponse{}, err
		}
		return s.auth.ExchangeActAsCredential(ctx, ExchangeActAsCredentialRequest{Credential: cred})
	}
	return s
}

// StaticToken is a TokenSource with one fixed bearer, for tests and tools.
type StaticToken string

func (t StaticToken) Token(context.Context) (string, error) { return string(t), nil }
func (StaticToken) Invalidate(string)                       {}
