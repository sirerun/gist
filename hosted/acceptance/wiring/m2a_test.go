//go:build integration

package wiring

import (
	"os"
	"testing"
)

func requireFixture(t *testing.T) {
	t.Helper()
	for _, key := range []string{"REGISTRY_BASE_URL", "REGISTRY_AUDIENCE", "REGISTRY_TEST_ACCOUNT", "REGISTRY_TEST_ACCOUNT_SECRET", "REGISTRY_ARTIFACT_DIR"} {
		if os.Getenv(key) == "" {
			t.Fatalf("integration fixture %s is required; missing fixtures fail rather than skip", key)
		}
	}
}

func TestM2aWiringAgainstConfiguredService(t *testing.T) {
	requireFixture(t)
	t.Fatal("live M2a workload acceptance requires the reviewed service runner and token issuer; no fixture runner is configured in this sandbox")
}
