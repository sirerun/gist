package ports

import (
	"encoding/json"
	"testing"
)

func TestEventJSONMatchesFrozenV1Wire(t *testing.T) {
	e := Event{ID: "evt-1", WorkspaceID: "internal-workspace", Type: EventVersionRevoked, Subject: ArtifactRef{WorkspaceID: "internal-workspace", Kind: KindSkill, ID: "skills/example", Version: "1.0.0"}, OccurredAt: 1700000000, PolicyGeneration: 7}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"event_id": "evt-1", "event_type": "version_revoked", "subject_ref": "skills/example@1.0.0", "occurred_at": "2023-11-14T22:13:20Z", "policy_generation": float64(7)}
	if len(got) != len(want) {
		t.Fatalf("event leaked non-schema fields: %s", raw)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s=%v want %v", k, got[k], v)
		}
	}
}
