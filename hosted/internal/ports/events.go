package ports

import "context"

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

type EventPage struct {
	Events       []Event
	Next         Cursor
	RetentionGap bool
}

type EventStore interface {
	Append(context.Context, Event) error
	Read(context.Context, Cursor) (EventPage, error)
}
