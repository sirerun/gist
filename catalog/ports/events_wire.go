package ports

import (
	"encoding/json"
	"time"
)

// MarshalJSON follows the frozen v1 event schema. Domain workspace bindings
// stay internal; exact artifact references use the same ID@version form as resolve.
func (e Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID               string    `json:"event_id"`
		Type             EventType `json:"event_type"`
		Subject          string    `json:"subject_ref"`
		OccurredAt       time.Time `json:"occurred_at"`
		PolicyGeneration uint64    `json:"policy_generation"`
	}{e.ID, e.Type, e.Subject.ID + "@" + e.Subject.Version, time.Unix(e.OccurredAt, 0).UTC(), e.PolicyGeneration})
}
