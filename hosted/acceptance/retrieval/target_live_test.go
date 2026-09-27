//go:build live

package retrieval

// Live target seam for E3. It only reads where the deployed service is and
// which credentials to use; seeding the synthetic sandbox workspaces through
// authorized publisher APIs is E3's job and is not implemented here. Every
// variable is required: a missing one fails the run.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// enforceThresholds is true for the deployed run: E3 requires every E1 target.
const enforceThresholds = true

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func openTarget(s frozenSuite) (*target, error) {
	base := strings.TrimRight(os.Getenv("REGISTRY_BASE_URL"), "/")
	rawTokens := os.Getenv("REGISTRY_SMOKE_TOKENS")
	catalog := os.Getenv("REGISTRY_SMOKE_CATALOG_SHA256")
	var missing []string
	for name, v := range map[string]string{"REGISTRY_BASE_URL": base, "REGISTRY_SMOKE_TOKENS": rawTokens, "REGISTRY_SMOKE_CATALOG_SHA256": catalog} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("live smoke environment missing %s", strings.Join(missing, ", "))
	}
	if !strings.HasPrefix(base, "https://") {
		return nil, errors.New("REGISTRY_BASE_URL must be an https origin")
	}
	if !hex64.MatchString(catalog) {
		return nil, errors.New("REGISTRY_SMOKE_CATALOG_SHA256 must be a SHA-256 hex digest of the seeded catalog")
	}
	// REGISTRY_SMOKE_TOKENS is a JSON object: principal fixture key -> token,
	// including principal-adversary-owner.
	tokens := map[string]string{}
	if err := json.Unmarshal([]byte(rawTokens), &tokens); err != nil {
		return nil, errors.New("REGISTRY_SMOKE_TOKENS must be a JSON object of principal key to token")
	}
	cfg, err := canonicalJSON(map[string]string{"target": "live", "base_url_sha256": sha256Hex([]byte(base))})
	if err != nil {
		return nil, err
	}
	return &target{name: "live", baseURL: base, client: &http.Client{Timeout: 15 * time.Second}, tokens: tokens, catalogHash: catalog, configHash: sha256Hex(cfg)}, nil
}
