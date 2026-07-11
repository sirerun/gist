package gist

import (
	"context"
	"strings"
	"testing"
)

func TestMissClassification(t *testing.T) {
	tests := []struct {
		name    string
		results []SearchResult
		want    MissClass
	}{
		{
			name:    "no results at all",
			results: nil,
			want:    MissNoContent,
		},
		{
			name:    "empty slice",
			results: []SearchResult{},
			want:    MissNoContent,
		},
		{
			name: "fuzzy-corrected top result",
			results: []SearchResult{
				{Score: 0.05, MatchLayer: "fuzzy"},
			},
			want: MissParaphraseDivergence,
		},
		{
			name: "porter top result below floor",
			results: []SearchResult{
				{Score: 0.05, MatchLayer: "porter"},
			},
			want: MissChunkingTruncation,
		},
		{
			name: "trigram top result below floor",
			results: []SearchResult{
				{Score: 0.05, MatchLayer: "trigram"},
			},
			want: MissChunkingTruncation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyMiss(tt.results)
			if got != tt.want {
				t.Errorf("classifyMiss() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMissZeroResult(t *testing.T) {
	g, err := New(WithMemory())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	if _, err := g.Index(ctx, "alpha beta gamma delta", WithSource("doc1")); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := g.Search(ctx, "zzz_nonexistent_query_zzz")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected zero results, got %d", len(results))
	}

	stats := g.Stats()
	if stats.MissCount != 1 {
		t.Fatalf("expected MissCount 1, got %d", stats.MissCount)
	}
	if stats.MissRate != 1.0 {
		t.Fatalf("expected MissRate 1.0, got %f", stats.MissRate)
	}

	misses := g.Misses(0)
	if len(misses) != 1 {
		t.Fatalf("expected 1 miss record, got %d", len(misses))
	}
	if misses[0].Class != MissNoContent {
		t.Errorf("expected class %q, got %q", MissNoContent, misses[0].Class)
	}
	if misses[0].Query != "zzz_nonexistent_query_zzz" {
		t.Errorf("unexpected query recorded: %q", misses[0].Query)
	}

	breakdown := g.MissClassBreakdown()
	if breakdown[MissNoContent] != 1 {
		t.Errorf("expected breakdown[MissNoContent] = 1, got %d", breakdown[MissNoContent])
	}
}

func TestMissLowScore(t *testing.T) {
	g, err := New(WithMemory(), WithMissScoreFloor(0.5))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	// Long content where only a single query word matches, producing a low
	// MemoryStore porter score (matched words / total content words).
	content := "target " + strings.Repeat("filler ", 50)
	if _, err := g.Index(ctx, content, WithSource("doc1")); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := g.Search(ctx, "target")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one low-scoring result")
	}
	if results[0].Score >= 0.5 {
		t.Fatalf("expected top score below floor, got %f", results[0].Score)
	}

	stats := g.Stats()
	if stats.MissCount != 1 {
		t.Fatalf("expected MissCount 1, got %d", stats.MissCount)
	}

	misses := g.Misses(0)
	if len(misses) != 1 {
		t.Fatalf("expected 1 miss record, got %d", len(misses))
	}
	if misses[0].Class != MissChunkingTruncation {
		t.Errorf("expected class %q, got %q", MissChunkingTruncation, misses[0].Class)
	}
	if misses[0].ScoreFloor != 0.5 {
		t.Errorf("expected score floor 0.5, got %f", misses[0].ScoreFloor)
	}
	if len(misses[0].NearMisses) == 0 {
		t.Error("expected near-miss candidates to be recorded")
	}
}
