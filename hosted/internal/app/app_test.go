package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{PublicOrigin: "https://registry.example.invalid", ResourceAudience: "https://registry.example.invalid", DatabaseURL: "postgres://registry@127.0.0.1:5432/registry", ObjectStoreRoot: "/tmp/registry-objects", RequestTimeout: 30 * time.Second, MaxPackageBytes: 10 << 20, MaxExpandedBytes: 50 << 20, MaxRequestBytes: 1 << 20, MaxResponseBytes: 2 << 20, MaxCatalogEntries: 100000, MaxConcurrentRequests: 80, MaxDiscoveryResults: 50}
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
