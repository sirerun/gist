package publicationv2

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestDynamicSchemaCannotReadUnretainedLocalFile(t *testing.T) {
	schemaPath := filepath.Join(t.TempDir(), "outside-schema.json")
	if err := os.WriteFile(schemaPath, []byte("{\"type\":\"string\"}"), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: schemaPath}).String()
	doc, err := json.Marshal(map[string]any{
		"input_schema":  map[string]any{"$ref": uri},
		"output_schema": map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDynamicSchemas(doc); !errors.Is(err, ErrValidation) {
		t.Fatalf("unretained file reference accepted: %v", err)
	}
}
