package identity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
	if err := rejectDuplicateJSONFields(data); err != nil {
		return nil, fmt.Errorf("load signing key set: malformed or duplicate configuration fields")
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
	derived := ed25519.NewKeyFromSeed(private[:ed25519.SeedSize])
	if subtle.ConstantTimeCompare(private, derived) != 1 || subtle.ConstantTimeCompare(derived[ed25519.SeedSize:], public) != 1 {
		return nil, fmt.Errorf("load signing key set: current public key does not match private key")
	}
	current := SigningKey{KID: cfg.Current.KID, Algorithm: cfg.Current.Algorithm, Private: private, Public: public}
	set, err := NewKeySet(clock, current)
	if err != nil { // Keep this boundary safe if validation changes later.
		return nil, fmt.Errorf("load signing key set: invalid current EdDSA key")
	}
	for _, retired := range cfg.Retired {
		if retired.KID == "" || retired.KID == current.KID || retired.Algorithm != "EdDSA" || len(retired.Public) != ed25519.PublicKeySize || retired.RetiredAt.IsZero() || retired.RetiredAt.After(clock.Now().Add(defaultMaxTokenAge+maxClockSkew)) {
			return nil, fmt.Errorf("load signing key set: invalid or reused retired key id")
		}
		if _, exists := set.keys[retired.KID]; exists {
			return nil, fmt.Errorf("load signing key set: duplicate key id")
		}
		set.keys[retired.KID] = SigningKey{KID: retired.KID, Algorithm: retired.Algorithm, Public: append(ed25519.PublicKey(nil), retired.Public...), NotAfter: retired.RetiredAt}
	}
	return set, nil
}

// rejectDuplicateJSONFields walks every object before encoding/json's struct
// decoder can apply its last-value-wins behavior. Parser details are discarded
// by the caller so malformed input cannot leak supplied key material.
func rejectDuplicateJSONFields(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func consumeJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("invalid object key")
			}
			if strings.ToLower(key) != key {
				return fmt.Errorf("noncanonical object key")
			}
			for prior := range seen {
				if strings.EqualFold(prior, key) {
					return fmt.Errorf("duplicate object key")
				}
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("invalid object close")
		}
	case '[':
		for dec.More() {
			if err := consumeJSONValue(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("invalid array close")
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	return nil
}
