package contract

import (
	"encoding/json"
	"testing"
)

func TestSchemaFixtureDocumentsAreValidJSON(t *testing.T) {
	for name, data := range map[string][]byte{"discover": []byte(`{"query":"x","max_bytes":1}`), "batch": []byte(`{"references":[{"reference":"x@1","kind":"manifest"}],"max_bytes":1}`), "error": []byte(`{"code":"not_found","message":"x","request_id":"r","retryable":false}`)} {
		t.Run(name, func(t *testing.T) {
			var v any
			if err := json.Unmarshal(data, &v); err != nil {
				t.Fatal(err)
			}
		})
	}
}
