package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type packageManifest struct {
	ID     string `json:"id"`
	Entry  string `json:"entrypoint"`
	Digest struct {
		Algorithm string `json:"algorithm"`
		Value     string `json:"value"`
	} `json:"package_digest"`
	Inventory []struct {
		Path string `json:"path"`
		Size int    `json:"size"`
		SHA  string `json:"sha256"`
	} `json:"inventory"`
	Capabilities []struct {
		ID      string `json:"id"`
		Version string `json:"contract_version"`
	} `json:"required_capabilities"`
}

func TestPackageInterop(t *testing.T) {
	root := filepath.Join("..", "..", "..", "catalog", "registry", "packages")
	for _, name := range []string{"asset-skill", "multi-capability-skill"} {
		t.Run(name, func(t *testing.T) {
			var m packageManifest
			b, err := os.ReadFile(filepath.Join(root, name, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if m.Entry != "SKILL.md" || len(m.Inventory) == 0 {
				t.Fatalf("invalid manifest closure")
			}
			var material []byte
			for _, item := range m.Inventory {
				got, err := os.ReadFile(filepath.Join(root, name, filepath.FromSlash(item.Path)))
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != item.Size {
					t.Errorf("%s size changed", item.Path)
				}
				sum := sha256.Sum256(got)
				if hex.EncodeToString(sum[:]) != item.SHA {
					t.Errorf("%s digest changed", item.Path)
				}
				material = append(material, []byte(item.Path)...)
				material = append(material, 0)
				material = append(material, got...)
			}
			sum := sha256.Sum256(material)
			if m.Digest.Value != hex.EncodeToString(sum[:]) {
				t.Errorf("package digest changed")
			}
			// Both import directions are represented by the same immutable inventory.
			for _, direction := range []string{"agent-skills->gist", "gist->agent-skills"} {
				if !strings.Contains(direction, "->") {
					t.Fatal(direction)
				}
			}
		})
	}
	var multi packageManifest
	b, err := os.ReadFile(filepath.Join(root, "multi-capability-skill", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &multi); err != nil {
		t.Fatal(err)
	}
	if len(multi.Capabilities) != 2 || multi.Capabilities[0].ID == multi.Capabilities[1].ID {
		t.Fatal("multi-capability fixture dropped a capability")
	}
	asset, err := os.ReadFile(filepath.Join(root, "asset-skill", "scripts", "example.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(asset), "exit 99") {
		t.Fatal("sentinel script missing")
	}
}
