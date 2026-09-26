package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type catalogFixture struct {
	FixtureVersion  string            `json:"fixture_version"`
	CapabilityID    string            `json:"capability_id"`
	ContractVersion string            `json:"contract_version"`
	ProviderID      string            `json:"provider_id"`
	ToolID          string            `json:"tool_id"`
	ToolVersion     string            `json:"tool_version"`
	BindingID       string            `json:"binding_id"`
	BindingVersion  string            `json:"binding_version"`
	Cases           []json.RawMessage `json:"cases"`
}

type catalogBinding struct {
	ID            string `json:"id"`
	FixtureDigest struct {
		Algorithm string `json:"algorithm"`
		Value     string `json:"value"`
	} `json:"fixture_digest"`
	Conformance   string `json:"conformance"`
	ExactVersions bool   `json:"exact_versions"`
}

func registryRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "catalog", "registry"))
}

func readRegistryJSON(name string, dst any) error {
	b, err := os.ReadFile(filepath.Join(registryRoot(), name))
	if err != nil {
		return fmt.Errorf("read registry %s: %w", name, err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("decode registry %s: %w", name, err)
	}
	return nil
}

func canonicalDigest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
