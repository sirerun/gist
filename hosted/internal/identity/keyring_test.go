package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLoadKeySetIndependentInstancesAndRetiredPublicKey(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0).UTC()}
	current, err := GenerateSigningKey("current", clock.now)
	if err != nil {
		t.Fatal(err)
	}
	old, err := GenerateSigningKey("old", clock.now)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(KeySetConfig{
		Current: KeyConfig{KID: current.KID, Algorithm: current.Algorithm, Private: current.Private, Public: current.Public},
		Retired: []RetiredKeyConfig{{KID: old.KID, Algorithm: old.Algorithm, Public: old.Public, RetiredAt: clock.now.Add(time.Hour)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := LoadKeySet(encoded, clock)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadKeySet(encoded, clock)
	if err != nil {
		t.Fatal(err)
	}
	policy := &policyDouble{policy: Policy{Allowed: true, PolicyGeneration: 7, ParentSubject: "parent", ParentScopes: []string{"catalog:read"}}}
	issuer, err := NewWorkloadIssuer(Config{Issuer: "https://gist.example", Audience: "https://gist.example", Clock: clock, Keys: first, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewWorkloadIssuer(Config{Issuer: "https://gist.example", Audience: "https://gist.example", Clock: clock, Keys: second, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Mint(context.Background(), WorkloadRequest{Subject: "workload", WorkspaceID: "workspace", Scopes: []string{"catalog:read"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), token); err != nil {
		t.Fatalf("independent keyset could not verify token: %v", err)
	}
	if _, err := second.verifyKey("old", clock.Now()); err != nil {
		t.Fatalf("configured overlap key unavailable: %v", err)
	}
	clock.Advance(2 * time.Hour)
	if _, err := second.verifyKey("old", clock.Now()); err == nil {
		t.Fatal("retired key remained valid after retirement")
	}
}

func TestLoadKeySetRejectsUnsafeConfigWithoutLeakingKey(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0).UTC()}
	key, err := GenerateSigningKey("current", clock.now)
	if err != nil {
		t.Fatal(err)
	}
	base := KeySetConfig{Current: KeyConfig{KID: key.KID, Algorithm: key.Algorithm, Private: key.Private, Public: key.Public}}
	wrong, err := GenerateSigningKey("wrong", clock.now)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*KeySetConfig)
		raw    string
	}{
		{name: "duplicate retired id", mutate: func(c *KeySetConfig) {
			c.Retired = []RetiredKeyConfig{{KID: "old", Algorithm: "EdDSA", Public: wrong.Public, RetiredAt: clock.now.Add(time.Hour)}, {KID: "old", Algorithm: "EdDSA", Public: wrong.Public, RetiredAt: clock.now.Add(time.Hour)}}
		}},
		{name: "reused current id", mutate: func(c *KeySetConfig) {
			c.Retired = []RetiredKeyConfig{{KID: key.KID, Algorithm: "EdDSA", Public: wrong.Public, RetiredAt: clock.now.Add(time.Hour)}}
		}},
		{name: "mismatched private and public", mutate: func(c *KeySetConfig) { c.Current.Public = wrong.Public }},
		{name: "wrong algorithm", mutate: func(c *KeySetConfig) { c.Current.Algorithm = "RS256" }},
		{name: "malformed key length", mutate: func(c *KeySetConfig) { c.Current.Private = []byte("sensitive-material") }},
		{name: "unknown field", raw: `{"current":{"kid":"k","algorithm":"EdDSA","private_key":"","public_key":""},"surprise":true}`},
		{name: "malformed json", raw: `{"current":`},
		{name: "trailing malformed data", raw: `{"current":{"kid":"k","algorithm":"EdDSA","private_key":"","public_key":""}} garbage`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data []byte
			if tc.raw != "" {
				data = []byte(tc.raw)
			} else {
				cfg := base
				tc.mutate(&cfg)
				data, err = json.Marshal(cfg)
				if err != nil {
					t.Fatal(err)
				}
			}
			set, loadErr := LoadKeySet(data, clock)
			if loadErr == nil || set != nil {
				t.Fatal("unsafe config was accepted")
			}
			if strings.Contains(loadErr.Error(), string(key.Private)) || strings.Contains(loadErr.Error(), base64.StdEncoding.EncodeToString(key.Private)) {
				t.Fatal("error exposed key bytes")
			}
		})
	}
	if _, err := LoadKeySet([]byte(`{}`), nil); err == nil || errors.Is(err, ErrUnauthorized) {
		t.Fatal("missing clock must fail as a configuration error")
	}
}
