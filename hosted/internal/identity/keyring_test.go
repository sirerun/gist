package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	policy := &policyDouble{policy: Policy{Allowed: true, PolicyGeneration: 7, ParentSubject: "parent", ParentScopes: []string{"catalog:read"}}}
	oldConfig, err := json.Marshal(KeySetConfig{Current: KeyConfig{KID: old.KID, Algorithm: old.Algorithm, Private: old.Private, Public: old.Public}})
	if err != nil {
		t.Fatal(err)
	}
	oldKeys, err := LoadKeySet(oldConfig, clock)
	if err != nil {
		t.Fatal(err)
	}
	oldIssuer, err := NewWorkloadIssuer(Config{Issuer: "https://gist.example", Audience: "https://gist.example", Clock: clock, Keys: oldKeys, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	oldToken, err := oldIssuer.Mint(context.Background(), WorkloadRequest{Subject: "workload", WorkspaceID: "workspace", Scopes: []string{"catalog:read"}})
	if err != nil {
		t.Fatal(err)
	}
	retireAt := clock.Now().Add(defaultMaxTokenAge + maxClockSkew)
	rotatedConfig, err := json.Marshal(KeySetConfig{
		Current: KeyConfig{KID: current.KID, Algorithm: current.Algorithm, Private: current.Private, Public: current.Public},
		Retired: []RetiredKeyConfig{{KID: old.KID, Algorithm: old.Algorithm, Public: old.Public, RetiredAt: retireAt}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := LoadKeySet(rotatedConfig, clock)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadKeySet(rotatedConfig, clock)
	if err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("independent current keysets could not verify token: %v", err)
	}
	if _, err := verifier.Verify(context.Background(), oldToken); err != nil {
		t.Fatalf("reloaded keyset could not verify token minted before rotation: %v", err)
	}
	if _, err := second.verifyKey("old", clock.Now()); err != nil {
		t.Fatalf("configured overlap key unavailable: %v", err)
	}
	if _, err := second.verifyKey("unprovisioned", clock.Now()); err == nil {
		t.Fatal("unknown key id was accepted")
	}
	clock.now = retireAt
	if _, err := second.verifyKey("old", clock.Now()); err == nil {
		t.Fatal("retired key remained valid at its exact retirement instant")
	}
	for _, published := range second.VerificationKeys() {
		if published.KID == "old" {
			t.Fatal("retired key remained published at its exact retirement instant")
		}
	}
	clock.Advance(time.Nanosecond)
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
	corrupt := append([]byte(nil), key.Private...)
	corrupt[0] ^= 0x80 // Preserve the old public suffix and configured public key.
	privateJSON, err := json.Marshal(key.Private)
	if err != nil {
		t.Fatal(err)
	}
	publicJSON, err := json.Marshal(key.Public)
	if err != nil {
		t.Fatal(err)
	}
	duplicateKidJSON := fmt.Sprintf(`{"current":{"kid":"shadow","kid":"current","algorithm":"EdDSA","private_key":%s,"public_key":%s}}`, privateJSON, publicJSON)
	duplicateCaseJSON := fmt.Sprintf(`{"current":{"kid":"shadow","KID":"current","algorithm":"EdDSA","private_key":%s,"public_key":%s}}`, privateJSON, publicJSON)
	retiredPublicJSON, err := json.Marshal(wrong.Public)
	if err != nil {
		t.Fatal(err)
	}
	retireAtJSON, err := json.Marshal(clock.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	duplicateNestedKidJSON := fmt.Sprintf(`{"current":{"kid":"current","algorithm":"EdDSA","private_key":%s,"public_key":%s},"retired":[{"kid":"shadow","kid":"old","algorithm":"EdDSA","public_key":%s,"retired_at":%s}]}`, privateJSON, publicJSON, retiredPublicJSON, retireAtJSON)
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
		{name: "corrupt seed with unchanged suffix", mutate: func(c *KeySetConfig) { c.Current.Private = corrupt }},
		{name: "wrong algorithm", mutate: func(c *KeySetConfig) { c.Current.Algorithm = "RS256" }},
		{name: "malformed key length", mutate: func(c *KeySetConfig) { c.Current.Private = []byte("sensitive-material") }},
		{name: "unknown field", raw: `{"current":{"kid":"k","algorithm":"EdDSA","private_key":"","public_key":""},"surprise":true}`},
		{name: "duplicate current field", raw: duplicateKidJSON},
		{name: "case-variant duplicate current field", raw: duplicateCaseJSON},
		{name: "duplicate retired key field", raw: duplicateNestedKidJSON},
		{name: "duplicate nested field", raw: `{"current":{"kid":"first","algorithm":"EdDSA","private_key":"","public_key":""},"retired":[{"kid":"old","kid":"other","algorithm":"EdDSA","public_key":"","retired_at":"2026-10-05T00:00:00Z"}]}`},
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
	farFuture, err := json.Marshal(KeySetConfig{Current: base.Current, Retired: []RetiredKeyConfig{{KID: "old", Algorithm: "EdDSA", Public: wrong.Public, RetiredAt: clock.Now().Add(defaultMaxTokenAge + maxClockSkew + time.Nanosecond)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeySet(farFuture, clock); err == nil {
		t.Fatal("retirement beyond bounded overlap was accepted")
	}
	past, err := json.Marshal(KeySetConfig{Current: base.Current, Retired: []RetiredKeyConfig{{KID: "old", Algorithm: "EdDSA", Public: wrong.Public, RetiredAt: clock.Now().Add(-time.Second)}}})
	if err != nil {
		t.Fatal(err)
	}
	pastKeys, err := LoadKeySet(past, clock)
	if err != nil {
		t.Fatalf("already retired public key should load and be denied by verification: %v", err)
	}
	if _, err := pastKeys.verifyKey("old", clock.Now()); err == nil {
		t.Fatal("already retired key was accepted")
	}
}

func TestRotateRejectsRetiredKIDReuse(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0).UTC()}
	current, err := GenerateSigningKey("current", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	retired, err := GenerateSigningKey("retired", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	retireAt := clock.Now().Add(defaultMaxTokenAge + maxClockSkew)
	data, err := json.Marshal(KeySetConfig{
		Current: KeyConfig{KID: current.KID, Algorithm: current.Algorithm, Private: current.Private, Public: current.Public},
		Retired: []RetiredKeyConfig{{KID: retired.KID, Algorithm: retired.Algorithm, Public: retired.Public, RetiredAt: retireAt}},
	})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := LoadKeySet(data, clock)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := GenerateSigningKey(retired.KID, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Rotate(reused); err == nil {
		t.Fatal("rotation reused a retired kid")
	}
	clock.now = retireAt
	if _, err := keys.verifyKey(retired.KID, clock.Now()); err == nil {
		t.Fatal("reused retired kid remained verifiable at its retirement boundary")
	}
	for _, published := range keys.VerificationKeys() {
		if published.KID == retired.KID {
			t.Fatal("reused retired kid remained published")
		}
	}
}

func TestRotateSameCurrentKeyIsNoop(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0).UTC()}
	current, err := GenerateSigningKey("current", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	keys, err := NewKeySet(clock, current)
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Rotate(current); err != nil {
		t.Fatalf("identical current key should be an idempotent no-op: %v", err)
	}
}
