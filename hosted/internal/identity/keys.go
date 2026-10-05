package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrUnauthorized       = errors.New("unauthorized")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrExpired            = errors.New("token expired")
	ErrRevoked            = errors.New("principal revoked")
	ErrReplay             = errors.New("token replay rejected")
)

const (
	defaultMaxTokenAge = 5 * time.Minute
	maxClockSkew       = 30 * time.Second
)

type SigningKey struct {
	KID       string
	Algorithm string
	Private   ed25519.PrivateKey
	Public    ed25519.PublicKey
	NotAfter  time.Time
}

type KeySet struct {
	mu      sync.RWMutex
	current SigningKey
	keys    map[string]SigningKey
	clock   Clock
}

func NewKeySet(clock Clock, current SigningKey) (*KeySet, error) {
	if clock == nil {
		return nil, fmt.Errorf("key set clock is required")
	}
	if err := validateSigningKey(current, true); err != nil {
		return nil, err
	}
	return &KeySet{current: current, keys: map[string]SigningKey{current.KID: current}, clock: clock}, nil
}

func GenerateSigningKey(kid string, now time.Time) (SigningKey, error) {
	if kid == "" {
		return SigningKey{}, fmt.Errorf("key id is required")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return SigningKey{}, fmt.Errorf("generate workload signing key: %w", err)
	}
	// Key lifetime is controlled by rotation/revocation, not by the local
	// validation-cache freshness bound. The timestamp is retained for callers
	// that provision an explicit key retirement time.
	_ = now
	return SigningKey{KID: kid, Algorithm: "EdDSA", Private: private, Public: public}, nil
}

func (k *KeySet) Rotate(next SigningKey) error {
	if err := validateSigningKey(next, true); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if next.KID == k.current.KID && sameSigningKey(next, k.current) {
		return nil
	}
	if _, used := k.keys[next.KID]; used {
		return fmt.Errorf("signing key id has already been used")
	}
	if prev := k.current; prev.KID != next.KID {
		// The retired key only needs to verify tokens minted before rotation,
		// and those cannot outlive defaultMaxTokenAge plus clock skew.
		retireAt := k.clock.Now().Add(defaultMaxTokenAge + maxClockSkew)
		if prev.NotAfter.IsZero() || prev.NotAfter.After(retireAt) {
			prev.NotAfter = retireAt
			k.keys[prev.KID] = prev
		}
	}
	k.current = next
	k.keys[next.KID] = next
	return nil
}

func sameSigningKey(a, b SigningKey) bool {
	return a.KID == b.KID && a.Algorithm == b.Algorithm && a.NotAfter.Equal(b.NotAfter) &&
		subtle.ConstantTimeCompare(a.Private, b.Private) == 1 && subtle.ConstantTimeCompare(a.Public, b.Public) == 1
}

func (k *KeySet) currentKey() (SigningKey, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.current.Private == nil {
		return SigningKey{}, fmt.Errorf("signing key is unavailable")
	}
	return k.current, nil
}

func (k *KeySet) verifyKey(kid string, now time.Time) (SigningKey, error) {
	k.mu.RLock()
	key, ok := k.keys[kid]
	k.mu.RUnlock()
	if !ok || key.Algorithm != "EdDSA" || len(key.Public) != ed25519.PublicKeySize {
		return SigningKey{}, fmt.Errorf("unknown signing key")
	}
	if !key.NotAfter.IsZero() && !now.Before(key.NotAfter) {
		return SigningKey{}, fmt.Errorf("signing key freshness exceeded")
	}
	return key, nil
}

func validateSigningKey(key SigningKey, requirePrivate bool) error {
	if key.KID == "" || key.Algorithm != "EdDSA" || len(key.Public) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid EdDSA signing key")
	}
	if requirePrivate && len(key.Private) != ed25519.PrivateKeySize {
		return fmt.Errorf("private signing key is required")
	}
	return nil
}

// VerificationKey is the public half of a signing key, safe to publish.
type VerificationKey struct {
	KID       string
	Algorithm string
	Public    ed25519.PublicKey
	NotAfter  time.Time
}

// VerificationKeys returns every key that can still verify a token, current
// key first. It never exposes private key material, so a JWKS endpoint can
// publish the result directly.
func (k *KeySet) VerificationKeys() []VerificationKey {
	now := k.clock.Now()
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := []VerificationKey{{KID: k.current.KID, Algorithm: k.current.Algorithm, Public: append(ed25519.PublicKey(nil), k.current.Public...), NotAfter: k.current.NotAfter}}
	for kid, key := range k.keys {
		if kid == k.current.KID || (!key.NotAfter.IsZero() && !now.Before(key.NotAfter)) {
			continue
		}
		out = append(out, VerificationKey{KID: key.KID, Algorithm: key.Algorithm, Public: append(ed25519.PublicKey(nil), key.Public...), NotAfter: key.NotAfter})
	}
	return out
}
