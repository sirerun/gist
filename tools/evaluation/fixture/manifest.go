package fixture

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/sirerun/gist/catalog/integrity"
	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/ports"
)

const MaxManifestBytes = 8 << 20
const MaxManifestRecords = 100

type ManifestRecord struct {
	Record   ports.CatalogRecord `json:"record"`
	Artifact []byte              `json:"artifact"`
}
type Manifest struct {
	Schema      string           `json:"schema"`
	WorkspaceID string           `json:"workspace_id"`
	Records     []ManifestRecord `json:"records"`
}

// LoadManifest accepts an explicitly pinned regular JSON file. Artifacts are
// inline retained bytes, so no manifest-supplied filesystem path is followed.
func LoadManifest(path, expectedSHA string) (*Catalog, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxManifestBytes {
		return nil, errors.New("catalog manifest must be a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, MaxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxManifestBytes || Hash(raw).Value != expectedSHA {
		return nil, errors.New("catalog content SHA mismatch or size exceeded")
	}
	return DecodeManifest(raw)
}
func DecodeManifest(raw []byte) (*Catalog, error) {
	if len(raw) > MaxManifestBytes {
		return nil, errors.New("catalog manifest exceeds byte budget")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var m Manifest
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("trailing catalog JSON")
	}
	if m.Schema != "gist.evaluation.catalog/v1" || m.WorkspaceID != Principal.WorkspaceID || len(m.Records) == 0 || len(m.Records) > MaxManifestRecords {
		return nil, errors.New("catalog schema/workspace/record budget invalid")
	}
	c := &Catalog{Records: map[ports.ArtifactRef]ports.CatalogRecord{}, Bytes: map[ports.ArtifactRef][]byte{}}
	for _, entry := range m.Records {
		r := entry.Record
		ref := r.Ref
		if ref.WorkspaceID != m.WorkspaceID || ref.ID == "" || ref.Version == "" || len(ref.ID) > 256 || len(ref.Version) > 256 || strings.ContainsAny(ref.ID+ref.Version, "@\x00\n\r") || (r.State != "published" && r.State != "deprecated" && r.State != "revoked") {
			return nil, errors.New("catalog reference/state invalid")
		}
		switch ref.Kind {
		case ports.KindSkill, ports.KindCapability, ports.KindTool, ports.KindProvider, ports.KindBinding:
		default:
			return nil, errors.New("catalog artifact kind unsupported")
		}
		if _, exists := c.Records[ref]; exists {
			return nil, errors.New("duplicate catalog reference")
		}
		if r.Digest != Hash(entry.Artifact) || len(r.Metadata) == 0 || !json.Valid(r.Metadata) {
			return nil, errors.New("catalog retained bytes or metadata invalid")
		}
		var identity struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(r.Metadata, &identity); err != nil || identity.ID != ref.ID || identity.Version != ref.Version {
			return nil, errors.New("catalog metadata identity mismatch")
		}
		if ref.Kind == ports.KindSkill {
			archive, err := packages.ReadArchive(entry.Artifact, packages.Limits{MaxPackageBytes: 4 << 20, MaxFileBytes: 2 << 20, MaxFiles: 64})
			if err != nil {
				return nil, err
			}
			pkg, err := packages.Validate(archive, packages.DefaultLimits())
			if err != nil {
				return nil, err
			}
			if r.ManifestDigest != Hash(r.Metadata) || !bytes.Equal(r.Metadata, pkg.ManifestBytes) || pkg.Manifest.ID != ref.ID || pkg.Manifest.Version != ref.Version {
				return nil, errors.New("catalog skill metadata/manifest mismatch")
			}
			if r.PackageDigest.Algorithm != "" && (r.PackageDigest.Algorithm != "sha256" || r.PackageDigest.Value != pkg.PackageDigest) {
				return nil, errors.New("catalog inventory closure mismatch")
			}
		} else {
			if !bytes.Equal(entry.Artifact, r.Metadata) {
				return nil, errors.New("typed catalog metadata must retain exact document bytes")
			}
			if r.ManifestDigest != Hash(r.Metadata) && r.DocumentDigest != Hash(r.Metadata) {
				return nil, errors.New("catalog typed document digest/legacy alias invalid")
			}
		}
		c.Records[ref] = r
		c.Bytes[ref] = append([]byte(nil), entry.Artifact...)
	}
	return c, nil
}
func Export(c *Catalog) ([]byte, error) {
	m := Manifest{Schema: "gist.evaluation.catalog/v1", WorkspaceID: Principal.WorkspaceID}
	for ref, r := range c.Records {
		m.Records = append(m.Records, ManifestRecord{r, c.Bytes[ref]})
	}
	sort.Slice(m.Records, func(i, j int) bool {
		a, b := m.Records[i].Record.Ref, m.Records[j].Record.Ref
		return string(a.Kind)+":"+a.ID+"@"+a.Version < string(b.Kind)+":"+b.ID+"@"+b.Version
	})
	return json.Marshal(m)
}
func Clone(c *Catalog) *Catalog {
	out := &Catalog{Records: map[ports.ArtifactRef]ports.CatalogRecord{}, Bytes: map[ports.ArtifactRef][]byte{}, Deny: c.Deny}
	for ref, r := range c.Records {
		r.Metadata = append([]byte(nil), r.Metadata...)
		out.Records[ref] = r
		out.Bytes[ref] = append([]byte(nil), c.Bytes[ref]...)
	}
	return out
}

// A real inert package backs every static distractor. It has no execution recipe.
func syntheticSkill(id, name, summary string) ([]byte, []byte, error) {
	skill := []byte("---\nname: " + id + "\ndescription: " + summary + "\n---\nSynthetic source-only distractor. No maintained execution recipe is supplied.\n")
	inventory := []integrity.InventoryEntry{{Path: "SKILL.md", Size: int64(len(skill)), SHA256: Hash(skill).Value, MediaType: "text/markdown"}}
	digest, _, err := integrity.PackageDigest(inventory)
	if err != nil {
		return nil, nil, err
	}
	manifest, err := json.Marshal(map[string]any{"id": id, "version": "1.0.0", "logical_name": name, "summary": summary, "trust": "operator_asserted", "publication": map[string]string{"status": "approved", "provenance": "synthetic-source-only; no execution recipe"}, "required_capabilities": []any{}, "runtime_requirements": map[string]any{"local_execution": false, "filesystem": []string{}}, "package_digest": map[string]string{"algorithm": "sha256", "value": digest}, "inventory": inventory})
	if err != nil {
		return nil, nil, err
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"SKILL.md", skill}, {"manifest.json", manifest}} {
		part, err := writer.Create(entry.name)
		if err != nil {
			return nil, nil, err
		}
		if _, err := part.Write(entry.data); err != nil {
			return nil, nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}
	return buffer.Bytes(), manifest, nil
}
func Overlay(c *Catalog, scenario string) error {
	switch scenario {
	case "ready", "no_match":
		return nil
	case "missing_connection":
		for ref := range c.Records {
			if ref.Kind == ports.KindBinding {
				delete(c.Records, ref)
			}
		}
		return nil
	case "revoked":
		for ref, r := range c.Records {
			if ref.Kind == ports.KindProvider {
				r.State = "revoked"
				c.Records[ref] = r
			}
		}
		return nil
	case "unauthorized":
		c.Deny = true
		return nil
	case "tampered":
		for ref := range c.Bytes {
			if ref.Kind == ports.KindSkill {
				c.Bytes[ref] = []byte("controlled corrupt bytes")
			}
		}
		return nil
	case "unsupported_runtime":
		return errors.New("NOT_RUN: genuine unsupported runtime transition not represented by catalog fixture")
	default:
		return errors.New("NOT_RUN: scenario transition not implemented by controlled fixture")
	}
}
