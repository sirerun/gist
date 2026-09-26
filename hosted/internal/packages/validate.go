package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sirerun/gist/hosted/internal/contract"
)

type Manifest struct {
	ID            string `json:"id"`
	Version       string `json:"version"`
	PackageDigest struct {
		Algorithm string `json:"algorithm"`
		Value     string `json:"value"`
	} `json:"package_digest"`
	Inventory   []contract.InventoryEntry `json:"inventory"`
	Publication struct {
		Status     string `json:"status"`
		Provenance string `json:"provenance"`
	} `json:"publication"`
	Trust string `json:"trust"`
}
type Package struct {
	Manifest       Manifest
	ManifestBytes  []byte
	Files          map[string][]byte
	PackageDigest  string
	ManifestDigest string
}

func Validate(a Archive, limits Limits) (Package, error) {
	manifestBytes, ok := a.Files["manifest.json"]
	if !ok {
		return Package{}, errors.New("packages: manifest.json is required")
	}
	skill, ok := a.Files["SKILL.md"]
	if !ok {
		return Package{}, errors.New("packages: SKILL.md is required")
	}
	var m Manifest
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		return Package{}, fmt.Errorf("packages: invalid manifest: %w", err)
	}
	if m.ID == "" || m.Version == "" || m.PackageDigest.Value == "" {
		return Package{}, errors.New("packages: manifest identity and digest are required")
	}
	if m.Trust != "operator_asserted" {
		return Package{}, errors.New("packages: only operator_asserted trust is accepted")
	}
	if m.Publication.Status != "screening" && m.Publication.Status != "approved" {
		return Package{}, errors.New("packages: package is not in a reviewable state")
	}
	if _, err := contract.ValidateAgentSkillsCore(skill); err != nil {
		return Package{}, fmt.Errorf("packages: validate SKILL.md: %w", err)
	}
	manifestDigest, _, err := contract.PackageDigest(m.Inventory)
	if err != nil {
		return Package{}, fmt.Errorf("packages: validate inventory: %w", err)
	}
	if manifestDigest != m.PackageDigest.Value {
		return Package{}, fmt.Errorf("packages: inventory digest mismatch: got %s want %s", manifestDigest, m.PackageDigest.Value)
	}
	// VerifyPackage checks every declared byte, rejects undeclared members, and
	// verifies the detached manifest transfer digest before publication.
	if err := contract.VerifyPackage(contract.Package{
		Manifest:      manifestBytes,
		Files:         a.FilesWithoutManifest(),
		Inventory:     m.Inventory,
		PackageDigest: m.PackageDigest.Value,
		Transfer:      contract.DetachedTransfer{ManifestDigest: digestBytes(manifestBytes), ManifestSize: int64(len(manifestBytes))},
	}); err != nil {
		return Package{}, fmt.Errorf("packages: verify package closure: %w", err)
	}
	return Package{Manifest: m, ManifestBytes: append([]byte(nil), manifestBytes...), Files: a.Files, PackageDigest: manifestDigest, ManifestDigest: digestBytes(manifestBytes)}, nil
}

func (a Archive) FilesWithoutManifest() map[string][]byte {
	files := make(map[string][]byte, len(a.Files)-1)
	for name, content := range a.Files {
		if name != "manifest.json" {
			files[name] = content
		}
	}
	return files
}

func digestBytes(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func ScreenText(p Package) error {
	for name, b := range p.Files {
		if name == "manifest.json" {
			continue
		}
		text := strings.ToLower(string(b))
		for _, marker := range []string{"curl ", "wget ", "exfiltrate", "send your password", "ignore previous instructions"} {
			if strings.Contains(text, marker) {
				return fmt.Errorf("packages: screening finding in %s: %q", name, marker)
			}
		}
	}
	return nil
}
