package publicationv2

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"testing"

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
		entries, err := fs.Glob(schemaFiles, "schemas/"+ver+"/*.schema.json")
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
