package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type DetachedTransfer struct {
	ManifestDigest string
	ManifestSize   int64
}

type Package struct {
	Manifest      []byte
	Files         map[string][]byte
	Inventory     []InventoryEntry
	PackageDigest string
	Transfer      DetachedTransfer
}

func VerifyPackage(p Package) error {
	h := sha256.Sum256(p.Manifest)
	if hex.EncodeToString(h[:]) != p.Transfer.ManifestDigest || int64(len(p.Manifest)) != p.Transfer.ManifestSize {
		return fmt.Errorf("manifest detached digest or size mismatch")
	}
	want, _, err := PackageDigest(p.Inventory)
	if err != nil {
		return err
	}
	if want != p.PackageDigest {
		return fmt.Errorf("package digest mismatch")
	}
	if _, ok := p.Files["SKILL.md"]; !ok {
		return fmt.Errorf("missing SKILL.md")
	}
	declared := make(map[string]InventoryEntry, len(p.Inventory))
	for _, e := range p.Inventory {
		if !validSHA(e.SHA256) {
			return fmt.Errorf("invalid sha256 for %q", e.Path)
		}
		declared[e.Path] = e
	}
	for path, content := range p.Files {
		if path == "manifest.json" {
			return fmt.Errorf("manifest must be detached from inventory")
		}
		e, ok := declared[path]
		if !ok {
			return fmt.Errorf("undeclared file %q", path)
		}
		if int64(len(content)) != e.Size {
			return fmt.Errorf("size mismatch for %q", path)
		}
		d := sha256.Sum256(content)
		if hex.EncodeToString(d[:]) != e.SHA256 {
			return fmt.Errorf("digest mismatch for %q", path)
		}
	}
	for path := range declared {
		if _, ok := p.Files[path]; !ok {
			return fmt.Errorf("missing file %q", path)
		}
	}
	return nil
}

type AgentSkillsMetadata struct {
	Name, Description string
	Provenance        string
}

func ValidateAgentSkillsCore(skillMD []byte) (AgentSkillsMetadata, error) {
	s := string(skillMD)
	if !strings.HasPrefix(s, "---\n") {
		return AgentSkillsMetadata{}, fmt.Errorf("SKILL.md metadata frontmatter missing")
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 {
		return AgentSkillsMetadata{}, fmt.Errorf("SKILL.md frontmatter unterminated")
	}
	end += 4
	var name, description string
	for _, line := range strings.Split(s[4:end], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "name":
			name = v
		case "description":
			description = v
		}
	}
	if name == "" || description == "" {
		return AgentSkillsMetadata{}, fmt.Errorf("SKILL.md name and description are required")
	}
	return AgentSkillsMetadata{Name: name, Description: description, Provenance: "derived_unverified"}, nil
}
