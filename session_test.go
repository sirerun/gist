package gist

import (
	"sync"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	s := NewSession("test-123")
	if s.ID() != "test-123" {
		t.Errorf("ID() = %q, want %q", s.ID(), "test-123")
	}
	if len(s.Events()) != 0 {
		t.Errorf("new session should have zero events, got %d", len(s.Events()))
	}
}

func TestSessionAddAndEvents(t *testing.T) {
	s := NewSession("s1")
	now := time.Now()

	s.AddEvent(IndexEvent{Source: "a.md", ChunkCount: 3, Time: now})
	s.AddEvent(SearchEvent{Query: "foo", ResultCount: 2, MatchLayer: "porter", Time: now.Add(time.Second)})
	s.AddEvent(ErrorEvent{Message: "fail", Context: "index", Time: now.Add(2 * time.Second)})

	events := s.Events()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Verify Events returns a copy.
	events[0] = nil
	if s.Events()[0] == nil {
		t.Error("Events() should return a copy, not the internal slice")
	}
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		wantType string
	}{
		{"index", IndexEvent{Source: "s", ChunkCount: 1, Time: time.Now()}, "index"},
		{"search", SearchEvent{Query: "q", ResultCount: 0, MatchLayer: "trigram", Time: time.Now()}, "search"},
		{"error", ErrorEvent{Message: "m", Context: "c", Time: time.Now()}, "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.EventType(); got != tt.wantType {
				t.Errorf("EventType() = %q, want %q", got, tt.wantType)
			}
			if tt.event.Timestamp().IsZero() {
				t.Error("Timestamp() should not be zero")
			}
		})
	}
}

func TestEventsByType(t *testing.T) {
	s := NewSession("s1")
	now := time.Now()

	s.AddEvent(IndexEvent{Source: "a", ChunkCount: 1, Time: now})
	s.AddEvent(SearchEvent{Query: "q1", ResultCount: 1, MatchLayer: "porter", Time: now})
	s.AddEvent(IndexEvent{Source: "b", ChunkCount: 2, Time: now})
	s.AddEvent(SearchEvent{Query: "q2", ResultCount: 3, MatchLayer: "fuzzy", Time: now})
	s.AddEvent(ErrorEvent{Message: "err", Context: "ctx", Time: now})

	tests := []struct {
		eventType string
		wantCount int
	}{
		{"index", 2},
		{"search", 2},
		{"error", 1},
		{"unknown", 0},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			got := s.EventsByType(tt.eventType)
			if len(got) != tt.wantCount {
				t.Errorf("EventsByType(%q) returned %d events, want %d", tt.eventType, len(got), tt.wantCount)
			}
		})
	}
}

func TestSince(t *testing.T) {
	s := NewSession("s1")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	t2 := t0.Add(2 * time.Minute)
	t3 := t0.Add(3 * time.Minute)

	s.AddEvent(IndexEvent{Source: "a", ChunkCount: 1, Time: t0})
	s.AddEvent(SearchEvent{Query: "q", ResultCount: 1, MatchLayer: "porter", Time: t1})
	s.AddEvent(IndexEvent{Source: "b", ChunkCount: 2, Time: t2})
	s.AddEvent(ErrorEvent{Message: "err", Context: "ctx", Time: t3})

	tests := []struct {
		name      string
		since     time.Time
		wantCount int
	}{
		{"before all", t0.Add(-time.Second), 4},
		{"after first", t0, 3},
		{"after second", t1, 2},
		{"after third", t2, 1},
		{"after all", t3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.Since(tt.since)
			if len(got) != tt.wantCount {
				t.Errorf("Since(%v) returned %d events, want %d", tt.since, len(got), tt.wantCount)
			}
		})
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		name     string
		events   []Event
		wantDur  time.Duration
	}{
		{
			name:    "no events",
			events:  nil,
			wantDur: 0,
		},
		{
			name:    "one event",
			events:  []Event{IndexEvent{Source: "a", ChunkCount: 1, Time: time.Now()}},
			wantDur: 0,
		},
		{
			name: "two events",
			events: []Event{
				IndexEvent{Source: "a", ChunkCount: 1, Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
				SearchEvent{Query: "q", ResultCount: 1, MatchLayer: "porter", Time: time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC)},
			},
			wantDur: 5 * time.Minute,
		},
		{
			name: "three events uses first and last",
			events: []Event{
				IndexEvent{Source: "a", ChunkCount: 1, Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
				SearchEvent{Query: "q", ResultCount: 1, MatchLayer: "porter", Time: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)},
				ErrorEvent{Message: "e", Context: "c", Time: time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)},
			},
			wantDur: 10 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession("s")
			for _, e := range tt.events {
				s.AddEvent(e)
			}
			if got := s.Duration(); got != tt.wantDur {
				t.Errorf("Duration() = %v, want %v", got, tt.wantDur)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	s := NewSession("s1")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s.AddEvent(IndexEvent{Source: "a", ChunkCount: 1, Time: t0})
	s.AddEvent(IndexEvent{Source: "b", ChunkCount: 2, Time: t0.Add(time.Second)})
	s.AddEvent(SearchEvent{Query: "q", ResultCount: 1, MatchLayer: "porter", Time: t0.Add(2 * time.Second)})
	s.AddEvent(ErrorEvent{Message: "e", Context: "c", Time: t0.Add(3 * time.Second)})

	sum := s.Summary()
	if sum.EventCounts["index"] != 2 {
		t.Errorf("index count = %d, want 2", sum.EventCounts["index"])
	}
	if sum.EventCounts["search"] != 1 {
		t.Errorf("search count = %d, want 1", sum.EventCounts["search"])
	}
	if sum.EventCounts["error"] != 1 {
		t.Errorf("error count = %d, want 1", sum.EventCounts["error"])
	}
	if sum.TotalDuration != 3*time.Second {
		t.Errorf("TotalDuration = %v, want %v", sum.TotalDuration, 3*time.Second)
	}
}

func TestSummaryEmpty(t *testing.T) {
	s := NewSession("empty")
	sum := s.Summary()
	if len(sum.EventCounts) != 0 {
		t.Errorf("expected empty counts, got %v", sum.EventCounts)
	}
	if sum.TotalDuration != 0 {
		t.Errorf("expected zero duration, got %v", sum.TotalDuration)
	}
}

func TestSessionConcurrency(t *testing.T) {
	s := NewSession("concurrent")
	now := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.AddEvent(IndexEvent{Source: "s", ChunkCount: n, Time: now.Add(time.Duration(n) * time.Millisecond)})
		}(i)
	}
	wg.Wait()

	events := s.Events()
	if len(events) != 100 {
		t.Errorf("expected 100 events, got %d", len(events))
	}

	// Concurrent reads should not race.
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); s.Events() }()
		go func() { defer wg.Done(); s.EventsByType("index") }()
		go func() { defer wg.Done(); s.Summary() }()
	}
	wg.Wait()
}
