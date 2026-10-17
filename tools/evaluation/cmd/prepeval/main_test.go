package main

import (
	"encoding/json"
	"testing"
)

func TestUnsupportedMechanicsNeverBecomeStaticState(t *testing.T) {
	for _, field := range []string{"transition", "scenario"} {
		var item input
		if err := json.Unmarshal([]byte(`{"fixture":{"status":"revoked","`+field+`":{"when":"after_selection","state":"revoked"}}}`), &item); err != nil {
			t.Fatal(err)
		}
		if !unsupportedMechanics(item) {
			t.Fatal("silently discarded transition/scenario mechanics")
		}
	}
	var item input
	if err := json.Unmarshal([]byte(`{"fixture":{"status":"ready"}}`), &item); err != nil {
		t.Fatal(err)
	}
	if unsupportedMechanics(item) {
		t.Fatal("static fixture misclassified")
	}
}
