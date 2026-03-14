package gist

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSnapshot(t *testing.T) {
	s := NewSession("snap-test")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s.AddEvent(IndexEvent{Source: "readme.md", ChunkCount: 5, Time: t0})
	s.AddEvent(SearchEvent{Query: "database", ResultCount: 3, MatchLayer: "porter", Time: t0.Add(time.Second)})
	s.AddEvent(ErrorEvent{Message: "timeout", Context: "search", Time: t0.Add(2 * time.Second)})
	s.AddEvent(IndexEvent{Source: "config.yaml", ChunkCount: 2, Time: t0.Add(3 * time.Second)})

	snap := BuildSnapshot(s, 1000)
	sections := snap.Sections()

	if len(sections) == 0 {
		t.Fatal("expected non-zero sections")
	}

	// Verify tier ordering: critical first, then high, medium, low.
	var prevTier SnapshotTier
	for i, sec := range sections {
		if i > 0 && sec.Tier < prevTier {
			t.Errorf("section %d tier %d is less than previous tier %d", i, sec.Tier, prevTier)
		}
		prevTier = sec.Tier
		if sec.TokenEstimate <= 0 {
			t.Errorf("section %q has non-positive token estimate %d", sec.Title, sec.TokenEstimate)
		}
	}
}

func TestBuildSnapshotSectionTitles(t *testing.T) {
	s := NewSession("titles")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s.AddEvent(IndexEvent{Source: "a.md", ChunkCount: 1, Time: t0})
	s.AddEvent(SearchEvent{Query: "q", ResultCount: 1, MatchLayer: "porter", Time: t0.Add(time.Second)})
	s.AddEvent(ErrorEvent{Message: "err", Context: "ctx", Time: t0.Add(2 * time.Second)})

	snap := BuildSnapshot(s, 0)
	sections := snap.Sections()

	titles := make(map[string]bool)
	for _, sec := range sections {
		titles[sec.Title] = true
	}

	expected := []string{"Errors", "Recent Activity", "Search Summary", "Indexed Sources", "Session Metadata"}
	for _, title := range expected {
		if !titles[title] {
			t.Errorf("missing expected section %q", title)
		}
	}
}

func TestBuildSnapshotNoErrors(t *testing.T) {
	s := NewSession("no-errors")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddEvent(IndexEvent{Source: "a.md", ChunkCount: 1, Time: t0})

	snap := BuildSnapshot(s, 0)
	for _, sec := range snap.Sections() {
		if sec.Title == "Errors" {
			t.Error("should not have Errors section when no error events exist")
		}
	}
}

func TestBuildSnapshotNoSearches(t *testing.T) {
	s := NewSession("no-search")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddEvent(IndexEvent{Source: "a.md", ChunkCount: 1, Time: t0})

	snap := BuildSnapshot(s, 0)
	for _, sec := range snap.Sections() {
		if sec.Title == "Search Summary" {
			t.Error("should not have Search Summary section when no search events exist")
		}
	}
}

func TestBuildSnapshotEmptySession(t *testing.T) {
	s := NewSession("empty")
	snap := BuildSnapshot(s, 0)
	sections := snap.Sections()

	// Should still have session metadata at minimum.
	if len(sections) < 1 {
		t.Fatal("expected at least metadata section for empty session")
	}

	found := false
	for _, sec := range sections {
		if sec.Title == "Session Metadata" {
			found = true
			if !strings.Contains(sec.Content, "empty") {
				t.Error("metadata should contain session ID")
			}
		}
	}
	if !found {
		t.Error("missing Session Metadata section")
	}
}

func TestBuildSnapshotRecentActivityLimit(t *testing.T) {
	s := NewSession("many")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.AddEvent(IndexEvent{Source: "s", ChunkCount: i, Time: t0.Add(time.Duration(i) * time.Second)})
	}

	snap := BuildSnapshot(s, 0)
	for _, sec := range snap.Sections() {
		if sec.Title == "Recent Activity" {
			lines := strings.Split(sec.Content, "\n")
			if len(lines) > 5 {
				t.Errorf("recent activity should show at most 5 events, got %d", len(lines))
			}
		}
	}
}

func TestSnapshotRender(t *testing.T) {
	s := NewSession("render")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s.AddEvent(IndexEvent{Source: "a.md", ChunkCount: 3, Time: t0})
	s.AddEvent(SearchEvent{Query: "test", ResultCount: 2, MatchLayer: "porter", Time: t0.Add(time.Second)})
	s.AddEvent(ErrorEvent{Message: "fail", Context: "search", Time: t0.Add(2 * time.Second)})

	snap := BuildSnapshot(s, 0)

	tests := []struct {
		name      string
		maxTokens int
		check     func(t *testing.T, output string)
	}{
		{
			name:      "unlimited renders all sections",
			maxTokens: 0,
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, "## Errors") {
					t.Error("missing Errors section")
				}
				if !strings.Contains(output, "## Session Metadata") {
					t.Error("missing Session Metadata section")
				}
			},
		},
		{
			name:      "limited renders priority sections first",
			maxTokens: 50,
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, "## Errors") {
					t.Error("should include critical Errors section")
				}
			},
		},
		{
			name:      "very small budget may exclude low priority",
			maxTokens: 10,
			check: func(t *testing.T, output string) {
				if len(output) == 0 {
					t.Error("expected some output even with small budget")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := snap.Render(tt.maxTokens)
			tt.check(t, output)
		})
	}
}

func TestSnapshotRenderSectionSeparators(t *testing.T) {
	s := NewSession("seps")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddEvent(IndexEvent{Source: "a", ChunkCount: 1, Time: t0})
	s.AddEvent(ErrorEvent{Message: "e", Context: "c", Time: t0.Add(time.Second)})

	snap := BuildSnapshot(s, 0)
	output := snap.Render(0)

	// Sections should be separated by double newlines.
	if !strings.Contains(output, "\n\n## ") {
		t.Error("sections should be separated by double newlines")
	}
}

func TestSnapshotSectionsCopy(t *testing.T) {
	s := NewSession("copy")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddEvent(IndexEvent{Source: "a", ChunkCount: 1, Time: t0})

	snap := BuildSnapshot(s, 0)
	sections := snap.Sections()
	sections[0].Title = "MODIFIED"

	// Original should be unchanged.
	if snap.Sections()[0].Title == "MODIFIED" {
		t.Error("Sections() should return a copy")
	}
}

func TestSnapshotTierConstants(t *testing.T) {
	// Verify tier ordering.
	if TierCritical >= TierHigh {
		t.Error("TierCritical should be less than TierHigh")
	}
	if TierHigh >= TierMedium {
		t.Error("TierHigh should be less than TierMedium")
	}
	if TierMedium >= TierLow {
		t.Error("TierMedium should be less than TierLow")
	}
}

func TestFormatEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantSub string
	}{
		{
			name:    "index event",
			event:   IndexEvent{Source: "readme.md", ChunkCount: 5, Time: time.Now()},
			wantSub: "readme.md",
		},
		{
			name:    "search event",
			event:   SearchEvent{Query: "test", ResultCount: 3, MatchLayer: "porter", Time: time.Now()},
			wantSub: "test",
		},
		{
			name:    "error event",
			event:   ErrorEvent{Message: "timeout", Context: "search", Time: time.Now()},
			wantSub: "timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEvent(tt.event)
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("formatEvent() = %q, should contain %q", got, tt.wantSub)
			}
		})
	}
}

func TestBuildSnapshotErrorContent(t *testing.T) {
	s := NewSession("errs")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.AddEvent(ErrorEvent{Message: "conn refused", Context: "postgres", Time: t0})
	s.AddEvent(ErrorEvent{Message: "timeout", Context: "search", Time: t0.Add(time.Second)})

	snap := BuildSnapshot(s, 0)
	for _, sec := range snap.Sections() {
		if sec.Title == "Errors" {
			if !strings.Contains(sec.Content, "conn refused") {
				t.Error("Errors section should contain first error message")
			}
			if !strings.Contains(sec.Content, "timeout") {
				t.Error("Errors section should contain second error message")
			}
			if !strings.Contains(sec.Content, "postgres") {
				t.Error("Errors section should contain error context")
			}
		}
	}
}
