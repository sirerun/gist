package ports

import (
	"context"
	"encoding/json"
	"time"
)

type EventType string

const (
	EventVersionPublished EventType = "version_published"
	EventVersionRevoked   EventType = "version_revoked"
)

type Event struct {
	ID               string
	WorkspaceID      string
	Type             EventType
	Subject          ArtifactRef
	OccurredAt       int64
	PolicyGeneration uint64
}

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

type EventPage struct {
	Events       []Event
	Next         Cursor
	RetentionGap bool
}

type EventStore interface {
	Append(context.Context, Event) error
	Read(context.Context, Cursor) (EventPage, error)
}
