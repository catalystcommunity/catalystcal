package actas

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	regularrp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	api "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

// KeyLifetime is the key life that enrollment asks for. The home domain
// can attest a shorter one (APPLICATION_KEY_MAX_LIFETIME_SECONDS).
const KeyLifetime = 365 * 24 * time.Hour

// RenewInterval is how often the server renews its attestations. The home
// domain gives a new attestation only when less than half of its life
// (24 hours by default) is left, so a renewal every 6 hours is a read.
const RenewInterval = 6 * time.Hour

// GenerateAppKeys makes a key file with three new Ed25519 signing keys. An
// instance needs two keys for each later key operation, and three keys
// let two revoke the third.
func GenerateAppKeys(domain, userID, applicationID, instanceID string) (AppKeys, error) {
	k := AppKeys{Domain: strings.ToLower(domain), UserID: userID, ApplicationID: applicationID, InstanceID: instanceID}
	if domain == "" || userID == "" || applicationID == "" || instanceID == "" {
		return AppKeys{}, errors.New("domain, user id, application id, and instance id are required")
	}
	stamp := time.Now().UTC().Format("2006-01")
	for i := range 3 {
		_, seed, _, err := regularrp.NewSigningKeyPair()
		if err != nil {
			return AppKeys{}, err
		}
		k.SigningKeys = append(k.SigningKeys, SigningKey{KeyID: fmt.Sprintf("k-%s-%d", stamp, i+1), SeedHex: hex.EncodeToString(seed[:])})
	}
	return k, nil
}

// WriteAppKeys writes a new key file with mode 0600. It does not replace a
// file that exists.
func WriteAppKeys(path string, k AppKeys) error {
	b, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

type keyProof struct {
	signer regularrp.ApplicationSigner
	pub    ed25519.PublicKey
	id     string
}

func (k AppKeys) proofs() ([]keyProof, error) {
	signers, err := k.Signers()
	if err != nil {
		return nil, err
	}
	out := make([]keyProof, 0, len(signers))
	for _, s := range signers {
		pub := ed25519.NewKeyFromSeed(s.PrivateKeyBytes).Public().(ed25519.PublicKey)
		out = append(out, keyProof{signer: s, pub: pub, id: s.KeyID})
	}
	return out, nil
}

func (k AppKeys) challenge(ctx context.Context, c *api.ApplicationKeysClient, purpose string, pub ed25519.PublicKey) (api.StartApplicationKeyChallengeResponse, error) {
	ch, err := c.StartKeyChallenge(ctx, api.StartApplicationKeyChallengeRequest{
		SubjectUserId: k.UserID, ApplicationId: k.ApplicationID, InstanceId: k.InstanceID,
		Purpose: purpose, KeyUsage: "sign", Algorithm: "ed25519", PublicKey: pub,
	})
	if err != nil {
		return ch, fmt.Errorf("start %s challenge: %w", purpose, err)
	}
	if ch.Challenge == nil {
		return ch, errors.New("the home domain gave no signing challenge")
	}
	return ch, nil
}

// Enroll does the initial enrollment of the instance at the home domain:
// every key proves possession, and the account approves the instance. t
// must carry the API key of the account (k.UserID); the home domain takes
// the subject from that key.
func Enroll(ctx context.Context, t api.Transport, k AppKeys) error {
	proofs, err := k.proofs()
	if err != nil {
		return err
	}
	if len(proofs) < 2 {
		return errors.New("enrollment needs at least two signing keys")
	}
	c := api.NewApplicationKeysClient(t)
	signed := make([]api.SignedApplicationKeyAddition, 0, len(proofs))
	for _, p := range proofs {
		ch, err := k.challenge(ctx, c, "add", p.pub)
		if err != nil {
			return err
		}
		add, err := regularrp.SignAddition(api.ApplicationKeyAddition{
			SubjectUserId: k.UserID, SubjectDomain: k.Ref().SubjectDomain, ApplicationId: k.ApplicationID, InstanceId: k.InstanceID,
			KeyId: p.id, KeyUsage: "sign", Algorithm: "ed25519", PublicKey: p.pub, Fingerprint: regularrp.Fingerprint(p.pub),
			RequestedKeyLifetimeSeconds: int64(KeyLifetime / time.Second), ChallengeId: ch.ChallengeId, Challenge: *ch.Challenge,
			RequestedAt: time.Now().UTC().Format(time.RFC3339), ExpiresAt: ch.ExpiresAt,
		}, nil, &p.signer)
		if err != nil {
			return err
		}
		signed = append(signed, add)
	}
	resp, err := api.NewAccountClient(t).EnrollApplicationInstance(ctx, api.EnrollApplicationInstanceRequest{
		ApplicationId: k.ApplicationID, InstanceId: k.InstanceID, Keys: signed,
	})
	if err != nil {
		return fmt.Errorf("enroll: %w", err)
	}
	if resp.SubjectUserId != k.UserID || resp.ApplicationId != k.ApplicationID || resp.InstanceId != k.InstanceID ||
		resp.SubjectDomain != k.Ref().SubjectDomain || len(resp.Attestations) != len(signed) {
		return fmt.Errorf("the home domain enrolled %s@%s/%s instance %s with %d attestations; the key file names another identity",
			resp.SubjectUserId, resp.SubjectDomain, resp.ApplicationId, resp.InstanceId, len(resp.Attestations))
	}
	return nil
}

// RenewAttestations renews the attestation of every signing key with a
// proof of possession. No API key is needed. It tries every key and
// returns the first error.
func RenewAttestations(ctx context.Context, t api.Transport, k AppKeys) error {
	proofs, err := k.proofs()
	if err != nil {
		return err
	}
	c := api.NewApplicationKeysClient(t)
	var first error
	for _, p := range proofs {
		if err := k.renewOne(ctx, c, p); err != nil && first == nil {
			first = fmt.Errorf("renew %s: %w", p.id, err)
		}
	}
	return first
}

func (k AppKeys) renewOne(ctx context.Context, c *api.ApplicationKeysClient, p keyProof) error {
	ch, err := k.challenge(ctx, c, "renew", p.pub)
	if err != nil {
		return err
	}
	req, err := regularrp.SignRenewal(api.ApplicationKeyRenewal{
		SubjectUserId: k.UserID, SubjectDomain: k.Ref().SubjectDomain, ApplicationId: k.ApplicationID, InstanceId: k.InstanceID,
		KeyId: p.id, ChallengeId: ch.ChallengeId, Challenge: *ch.Challenge,
		RequestedAt: time.Now().UTC().Format(time.RFC3339), ExpiresAt: ch.ExpiresAt,
	}, nil, &p.signer)
	if err != nil {
		return err
	}
	_, err = c.RenewAttestation(ctx, api.RenewApplicationKeyAttestationRequest{Request: req})
	return err
}
