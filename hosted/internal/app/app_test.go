package app

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"github.com/sirerun/gist/hosted/internal/identity"
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{PublicOrigin: "https://registry.example.invalid", ResourceAudience: "https://registry.example.invalid", DatabaseURL: "postgres://registry@127.0.0.1:5432/registry", ObjectStoreRoot: "/tmp/registry-objects", RequestTimeout: 30 * time.Second, MaxPackageBytes: 10 << 20, MaxExpandedBytes: 50 << 20, MaxRequestBytes: 1 << 20, MaxResponseBytes: 2 << 20, MaxCatalogEntries: 100000, MaxConcurrentRequests: 80, MaxDiscoveryResults: 50, OAuthConsentSecret: []byte(strings.Repeat("s", MinOAuthConsentSecretBytes)), SigningKeyConfig: fixtureSigningConfig(), EventMaintenanceTargets: []MaintenanceTarget{{WorkspaceID: "fixture-workspace", Subject: "fixture-maintainer"}}}
}

func TestConfigRequiresExactOriginAndExplicitLimits(t *testing.T) {
	cases := []struct{ name, origin, audience, want string }{
		{"path", "https://registry.example.invalid/path", "https://registry.example.invalid/path", "origin"},
		{"trailing slash", "https://registry.example.invalid/", "https://registry.example.invalid/", "origin"},
		{"audience mismatch", "https://registry.example.invalid", "https://other.example.invalid", "audience"},
		{"missing request limit", "https://registry.example.invalid", "https://registry.example.invalid", "limits"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			c.PublicOrigin, c.ResourceAudience = tc.origin, tc.audience
			if tc.name == "missing request limit" {
				c.MaxRequestBytes = 0
			}
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want substring %q", err, tc.want)
			}
		})
	}
}

// Every replica must share the consent secret, so startup refuses a missing
// or short one instead of generating a per-process key.
func TestConfigRequiresOAuthConsentSecret(t *testing.T) {
	for _, secret := range [][]byte{nil, []byte(strings.Repeat("s", MinOAuthConsentSecretBytes-1))} {
		c := validConfig()
		c.OAuthConsentSecret = secret
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "GIST_OAUTH_CONSENT_SECRET") {
			t.Fatalf("secret of %d bytes: error=%v", len(secret), err)
		}
	}
}

func TestConfigAcceptsDeploymentShape(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNewFailsWhenPostgresFixtureIsAbsent(t *testing.T) {
	c := validConfig()
	c.DatabaseURL = "postgres://registry@127.0.0.1:1/registry"
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := New(ctx, c); err == nil {
		t.Fatal("expected missing PostgreSQL fixture to fail")
	}
}

// This deterministic key is test-only; deployment supplies independently held keys.
func fixtureSigningConfig() []byte {
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	raw, err := json.Marshal(identity.KeySetConfig{Current: identity.KeyConfig{KID: "fixture-signing", Algorithm: "EdDSA", Private: key, Public: key.Public().(ed25519.PublicKey)}})
	if err != nil {
		panic(err)
	}
	return raw
}
func TestConfigRequiresExplicitPersistentSigningKeys(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`{"current":{"private_key":"private-marker"}}`)} {
		c := validConfig()
		c.SigningKeyConfig = raw
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "GIST_SIGNING_KEY_CONFIG") || strings.Contains(err.Error(), "private-marker") {
			t.Fatalf("unsafe key configuration accepted or leaked: %v", err)
		}
	}
}

func TestConfigRequiresBoundedUniqueMaintenanceTargets(t *testing.T) {
	for _, targets := range [][]MaintenanceTarget{nil, {{WorkspaceID: "w"}}, {{WorkspaceID: "w", Subject: "s"}, {WorkspaceID: "w", Subject: "other"}}, make([]MaintenanceTarget, 101)} {
		c := validConfig()
		c.EventMaintenanceTargets = targets
		if c.Validate() == nil {
			t.Fatal("unsafe maintenance target set accepted")
		}
	}
}
