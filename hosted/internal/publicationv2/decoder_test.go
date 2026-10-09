package publicationv2

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestSixFrozenEnvelopesDecode(t *testing.T) {
	b, err := os.ReadFile("../../../contracts/registry/v2/fixtures/valid.json")
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]json.RawMessage
	if err = json.Unmarshal(b, &all); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]ports.ArtifactKind{"skill": ports.KindSkill, "capability": ports.KindCapability, "tool": ports.KindTool, "provider": ports.KindProvider, "binding": ports.KindBinding, "taxonomy": ports.KindTaxonomy}
	for name, kind := range kinds {
		t.Run(name, func(t *testing.T) {
			var envelope map[string]json.RawMessage
			_ = json.Unmarshal(all[name], &envelope)
			raw := append([]byte(nil), all[name]...)
			if _, err := Decode(kind, raw, DefaultLimits()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEmbeddedSchemaParity(t *testing.T) {
	for _, ver := range []string{"v1", "v2"} {
		entries, err := fs.Glob(schemaFiles, "schemas/"+ver+"/*.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range entries {
			base := name[len("schemas/"):]
			source, err := os.ReadFile("../../../contracts/registry/" + base)
			if err != nil {
				t.Fatal(err)
			}
			copy, err := schemaFiles.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if string(source) != string(copy) {
				t.Errorf("embedded schema differs from frozen source: %s", base)
			}
		}
	}
}

func TestEvidenceFixtureSchema(t *testing.T) {
	b, err := os.ReadFile("../../../contracts/registry/v2/fixtures/admission-evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateEvidenceJSON(b); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsDuplicateEscapedKeysAndOuterWhitespace(t *testing.T) {
	for _, raw := range []string{`{"x":1,"\u0078":2}`, `{"x":"\ud800"}`, "\ufeff{\"x\":1}", `{"x":1} {}`} {
		if err := checkJSON([]byte(raw)); !errors.Is(err, ErrValidation) {
			t.Fatalf("%q: expected validation error, got %v", raw, err)
		}
	}
}

func TestBudgetSentinel(t *testing.T) {
	raw := []byte(`{"artifact":{"id":"capability/demo","version":"1.0.0","document":{}},"max_bytes":1,"idempotency_key":"k"}`)
	_, err := Decode(ports.KindCapability, raw, Limits{MaxEnvelopeBytes: 8, MaxPackageBytes: 8, MaxExpandedBytes: 8, MaxFileBytes: 8, MaxFiles: 1})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestFrozenEnvelopeSemanticsAndDynamicSchemas(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/registry/v2/fixtures/valid.json")
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]json.RawMessage
	if err = json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	_ = json.Unmarshal(all["capability"], &env)
	env["idempotency_key"] = "bad key with spaces"
	b, _ := json.Marshal(env)
	if _, err = Decode(ports.KindCapability, b, DefaultLimits()); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid idempotency key accepted: %v", err)
	}
	var capEnv map[string]any
	_ = json.Unmarshal(all["capability"], &capEnv)
	art := capEnv["artifact"].(map[string]any)
	doc := art["document"].(map[string]any)
	doc["version"] = "v1"
	art["version"] = "v1"
	b, _ = json.Marshal(capEnv)
	if _, err = Decode(ports.KindCapability, b, DefaultLimits()); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid semantic version accepted: %v", err)
	}
	for _, schema := range []string{`{"type":17}`, `{"$ref":"https://untrusted.invalid/schema.json"}`, `{"$ref":"urn:gist:missing-schema"}`} {
		rawDoc := []byte(`{"input_schema":` + schema + `,"output_schema":{"type":"object"}}`)
		if err := validateDynamicSchemas(rawDoc); !errors.Is(err, ErrValidation) {
			t.Errorf("schema %s: expected closed compile failure, got %v", schema, err)
		}
	}
	recursive := []byte(`{"input_schema":{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"node":{"type":"object","properties":{"next":{"$ref":"#/$defs/node"}}}},"$ref":"#/$defs/node"},"output_schema":{"type":"object"}}`)
	if err := validateDynamicSchemas(recursive); err != nil {
		t.Fatalf("valid recursive local schema rejected: %v", err)
	}
}

func TestArchiveRejectsUnsafeMembersBeforeManifest(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		mode       uint32
	}{
		{"traversal", "../escape", 0100644}, {"control", "bad\nname", 0100644}, {"symlink", "link", 0120777}, {"directory", "folder", 0040755},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			zw := zip.NewWriter(&b)
			h := &zip.FileHeader{Name: tc.path}
			h.SetMode(os.FileMode(tc.mode))
			w, e := zw.CreateHeader(h)
			if e != nil {
				t.Fatal(e)
			}
			_, _ = w.Write([]byte("x"))
			_ = zw.Close()
			_, e = decodeSkill(b.Bytes(), DefaultLimits())
			if !errors.Is(e, ErrValidation) {
				t.Fatalf("unsafe archive accepted: %v", e)
			}
		})
	}
}

func TestEvidencePureAdmissionBindsSourceAndSyntheticFlag(t *testing.T) {
	fixture, err := os.ReadFile("../../../contracts/registry/v2/fixtures/valid.json")
	if err != nil {
		t.Fatal(err)
	}
	var envelopes map[string]json.RawMessage
	if err = json.Unmarshal(fixture, &envelopes); err != nil {
		t.Fatal(err)
	}
	prepared, err := Decode(ports.KindSkill, envelopes["skill"], DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	evidenceBytes, err := os.ReadFile("../../../contracts/registry/v2/fixtures/admission-evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := ValidateEvidenceJSON(evidenceBytes)
	if err != nil {
		t.Fatal(err)
	}
	principal := ports.Principal{WorkspaceID: evidence.WorkspaceID}
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	if err = ValidateEvidence(prepared, principal, evidence, now, true); err != nil {
		t.Fatalf("owned synthetic fixture should qualify source behavior: %v", err)
	}
	if err = ValidateEvidence(prepared, principal, evidence, now, false); !errors.Is(err, ErrValidation) {
		t.Fatalf("synthetic evidence accepted with default deny: %v", err)
	}
	evidence.Source.RetainedBytesBase64 = "eA=="
	if err = ValidateEvidence(prepared, principal, evidence, now, true); !errors.Is(err, ErrValidation) {
		t.Fatalf("forged retained source accepted: %v", err)
	}
}
