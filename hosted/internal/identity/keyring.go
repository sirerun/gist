package identity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// KeySetConfig is the operator supplied JSON format consumed by LoadKeySet.
// Ed25519 key byte fields use standard padded base64 as defined by encoding/json.
type KeySetConfig struct {
	Current KeyConfig          `json:"current"`
	Retired []RetiredKeyConfig `json:"retired,omitempty"`
}

type KeyConfig struct {
	KID       string `json:"kid"`
	Algorithm string `json:"algorithm"`
	Private   []byte `json:"private_key"`
	Public    []byte `json:"public_key"`
}

type RetiredKeyConfig struct {
	KID       string    `json:"kid"`
	Algorithm string    `json:"algorithm"`
	Public    []byte    `json:"public_key"`
	RetiredAt time.Time `json:"retired_at"`
}

// LoadKeySet strictly decodes explicit EdDSA key material. It never generates
// keys, and errors deliberately omit parser and key material details.
func LoadKeySet(data []byte, clock Clock) (*KeySet, error) {
	if clock == nil {
		return nil, fmt.Errorf("load signing key set: clock is required")
	}
	var cfg KeySetConfig
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&cfg) != nil {
		return nil, fmt.Errorf("load signing key set: malformed or unsupported configuration")
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("load signing key set: trailing JSON value")
	} else if err != io.EOF {
		return nil, fmt.Errorf("load signing key set: malformed trailing data")
	}
	if cfg.Current.Algorithm != "EdDSA" || cfg.Current.KID == "" || len(cfg.Current.Private) != ed25519.PrivateKeySize || len(cfg.Current.Public) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("load signing key set: invalid current EdDSA key")
	}
	private := ed25519.PrivateKey(append([]byte(nil), cfg.Current.Private...))
	public := ed25519.PublicKey(append([]byte(nil), cfg.Current.Public...))
	if subtle.ConstantTimeCompare(private.Public().(ed25519.PublicKey), public) != 1 {
		return nil, fmt.Errorf("load signing key set: current public key does not match private key")
	}
	current := SigningKey{KID: cfg.Current.KID, Algorithm: cfg.Current.Algorithm, Private: private, Public: public}
	set, err := NewKeySet(clock, current)
	if err != nil { // Keep this boundary safe if validation changes later.
		return nil, fmt.Errorf("load signing key set: invalid current EdDSA key")
	}
	for _, retired := range cfg.Retired {
		if retired.KID == "" || retired.KID == current.KID || retired.Algorithm != "EdDSA" || len(retired.Public) != ed25519.PublicKeySize || retired.RetiredAt.IsZero() {
			return nil, fmt.Errorf("load signing key set: invalid or reused retired key id")
		}
		if _, exists := set.keys[retired.KID]; exists {
			return nil, fmt.Errorf("load signing key set: duplicate key id")
		}
		set.keys[retired.KID] = SigningKey{KID: retired.KID, Algorithm: retired.Algorithm, Public: append(ed25519.PublicKey(nil), retired.Public...), NotAfter: retired.RetiredAt}
	}
	return set, nil
}
