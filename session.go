package gist

import (
	"sync"
	"time"
)

// Event represents a tracked occurrence during a session.
type Event interface {
	// EventType returns the type identifier for this event (e.g. "index", "search", "error").
	EventType() string
	// Timestamp returns when the event occurred.
	Timestamp() time.Time
}

// IndexEvent records that content was indexed from a source.
type IndexEvent struct {
	// Source is the label of the indexed source.
	Source string
	// ChunkCount is the number of chunks produced.
	ChunkCount int
	// Time is when the indexing occurred.
	Time time.Time
}

// EventType returns "index".
func (e IndexEvent) EventType() string { return "index" }

// Timestamp returns the event time.
func (e IndexEvent) Timestamp() time.Time { return e.Time }

// SearchEvent records that a search was performed.
type SearchEvent struct {
	// Query is the search query string.
	Query string
	// ResultCount is the number of results returned.
	ResultCount int
	// MatchLayer is which search tier matched (e.g. "porter", "trigram", "fuzzy").
	MatchLayer string
	// Time is when the search occurred.
	Time time.Time
}

// EventType returns "search".
func (e SearchEvent) EventType() string { return "search" }

// Timestamp returns the event time.
func (e SearchEvent) Timestamp() time.Time { return e.Time }

// ErrorEvent records an error that occurred during a session.
type ErrorEvent struct {
	// Message is the error description.
	Message string
	// Context provides additional context about where the error occurred.
	Context string
	// Time is when the error occurred.
	Time time.Time
}

// EventType returns "error".
func (e ErrorEvent) EventType() string { return "error" }

// Timestamp returns the event time.
func (e ErrorEvent) Timestamp() time.Time { return e.Time }

// SessionSummary holds aggregate statistics for a session.
type SessionSummary struct {
	// EventCounts maps event type to count.
	EventCounts map[string]int
	// TotalDuration is the time between first and last event.
	TotalDuration time.Duration
}

// Session tracks events that occur during a user interaction session.
// All methods are safe for concurrent use.
type Session struct {
	mu     sync.Mutex
	id     string
	events []Event
}

// NewSession creates a new Session with the given identifier.
func NewSession(id string) *Session {
	return &Session{id: id}
}

// ID returns the session identifier.
func (s *Session) ID() string {
	return s.id
}

// AddEvent appends an event to the session.
func (s *Session) AddEvent(e Event) {
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
}

// Events returns a copy of all events in the session.
func (s *Session) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// EventsByType returns events matching the given type string.
func (s *Session) EventsByType(eventType string) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, e := range s.events {
		if e.EventType() == eventType {
			out = append(out, e)
		}
	}
	return out
}

// Since returns all events with timestamps after t.
func (s *Session) Since(t time.Time) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, e := range s.events {
		if e.Timestamp().After(t) {
			out = append(out, e)
		}
	}
	return out
}

// Duration returns the time between the first and last event.
// If fewer than two events exist it returns 0.
func (s *Session) Duration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) < 2 {
		return 0
	}
	first := s.events[0].Timestamp()
	last := s.events[len(s.events)-1].Timestamp()
	return last.Sub(first)
}

// Summary returns aggregate statistics for the session.
func (s *Session) Summary() SessionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := make(map[string]int)
	for _, e := range s.events {
		counts[e.EventType()]++
	}

	var dur time.Duration
	if len(s.events) >= 2 {
		dur = s.events[len(s.events)-1].Timestamp().Sub(s.events[0].Timestamp())
	}

	return SessionSummary{
		EventCounts:   counts,
		TotalDuration: dur,
	}
}
