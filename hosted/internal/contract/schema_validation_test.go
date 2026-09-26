package contract

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRegistrySchemasLoadWithJSONSchema2020(t *testing.T) {
	for _, name := range []string{"discover", "batch-get", "error"} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadJSONSchema(readRegistrySchema(t, name)); err != nil {
				t.Fatalf("schema must load under JSON Schema 2020-12: %v", err)
			}
		})
	}

	if _, err := LoadJSONSchema([]byte(`{"type":"not-a-json-schema-type"}`)); err == nil {
		t.Fatal("malformed schema accepted")
	}
}

func TestSchemaFixturesUseJSONSchema2020Validation(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		document   string
		wantReject bool
	}{
		{name: "discover positive", schema: "discover", document: `{"query":"x","max_bytes":1}`},
		{name: "batch positive", schema: "batch-get", document: `{"references":[{"reference":"x@1","kind":"manifest"}],"max_bytes":1}`},
		{name: "error positive", schema: "error", document: `{"code":"not_found","message":"x","request_id":"r","retryable":false}`},
		{name: "missing required field", schema: "discover", document: `{"query":"x"}`, wantReject: true},
		{name: "weakened constraint", schema: "discover", document: `{"query":"x","max_bytes":0}`, wantReject: true},
		{name: "unknown security-relevant field", schema: "discover", document: `{"query":"x","max_bytes":1,"security_mode":"admin"}`, wantReject: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONSchema(readRegistrySchema(t, tt.schema), []byte(tt.document))
			if tt.wantReject && err == nil {
				t.Fatal("invalid fixture accepted")
			}
			if !tt.wantReject && err != nil {
				t.Fatalf("valid fixture rejected: %v", err)
			}
		})
	}
}

func readRegistrySchema(t *testing.T, name string) []byte {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate schema test")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "../../../contracts/registry/v1", name+".schema.json"))
	if err != nil {
		t.Fatalf("read %s schema: %v", name, err)
	}
	return data
}
