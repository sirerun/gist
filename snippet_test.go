package gist

import (
	"strings"
	"testing"
)

func TestExtractSnippet(t *testing.T) {
	// Build a long content string for tests that need it.
	// ~200 words of prose with clear sentence boundaries.
	longContent := "The quick brown fox jumps over the lazy dog. " +
		"Pack my box with five dozen liquor jugs. " +
		"How vexingly quick daft zebras jump. " +
		"The five boxing wizards jump quickly. " +
		"Sphinx of black quartz judge my vow. " +
		strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 40)

	tests := []struct {
		name      string
		content   string
		positions []MatchPosition
		maxLen    int
		check     func(t *testing.T, result string)
	}{
		{
			name:      "empty content",
			content:   "",
			positions: []MatchPosition{{Start: 0, End: 5}},
			maxLen:    100,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			},
		},
		{
			name:      "no positions returns prefix",
			content:   longContent,
			positions: nil,
			maxLen:    100,
			check: func(t *testing.T, result string) {
				if len(result) > 100 {
					t.Errorf("expected len <= 100, got %d", len(result))
				}
				if !strings.HasPrefix(longContent, result[:20]) {
					t.Errorf("expected prefix of content, got %q", result[:20])
				}
			},
		},
		{
			name:      "content shorter than maxLen returns all",
			content:   "short text",
			positions: []MatchPosition{{Start: 0, End: 5}},
			maxLen:    100,
			check: func(t *testing.T, result string) {
				if result != "short text" {
					t.Errorf("expected full content, got %q", result)
				}
			},
		},
		{
			name:    "single match in middle",
			content: longContent,
			positions: []MatchPosition{
				{Start: len(longContent) / 2, End: len(longContent)/2 + 10},
			},
			maxLen: 200,
			check: func(t *testing.T, result string) {
				if len(result) > 220 {
					t.Errorf("expected len around 200, got %d", len(result))
				}
				if len(result) == 0 {
					t.Error("expected non-empty snippet")
				}
			},
		},
		{
			name:    "single match at start",
			content: longContent,
			positions: []MatchPosition{
				{Start: 0, End: 10},
			},
			maxLen: 200,
			check: func(t *testing.T, result string) {
				if !strings.HasPrefix(result, "The quick") {
					t.Errorf("expected snippet starting at beginning, got %q...", result[:30])
				}
			},
		},
		{
			name:    "single match at end",
			content: longContent,
			positions: []MatchPosition{
				{Start: len(longContent) - 10, End: len(longContent)},
			},
			maxLen: 200,
			check: func(t *testing.T, result string) {
				// The snippet should be from the tail region of the content.
				trimmed := strings.TrimRight(longContent, " ")
				if !strings.HasSuffix(trimmed, strings.TrimRight(result, " ")) {
					t.Errorf("expected snippet from tail region of content")
				}
			},
		},
		{
			name:    "multiple matches densest cluster",
			content: longContent,
			positions: []MatchPosition{
				{Start: 0, End: 5},
				// Cluster of three matches close together near position 500.
				{Start: 500, End: 510},
				{Start: 520, End: 530},
				{Start: 540, End: 550},
			},
			maxLen: 200,
			check: func(t *testing.T, result string) {
				// The densest cluster is around 500-550, so the snippet should
				// contain text from that region, not from position 0.
				midContent := longContent[480:560]
				// At least some of the cluster region should appear.
				found := false
				for i := 0; i < len(midContent)-10; i++ {
					if strings.Contains(result, midContent[i:i+10]) {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected snippet to contain text from densest cluster region")
				}
			},
		},
		{
			name:      "word boundary preservation",
			content:   "hello world this is a test of word boundaries in snippet extraction",
			positions: []MatchPosition{{Start: 20, End: 25}},
			maxLen:    30,
			check: func(t *testing.T, result string) {
				// Should not cut in the middle of a word.
				if len(result) > 0 {
					first := result[0]
					last := result[len(result)-1]
					if first == ' ' {
						t.Error("snippet starts with space")
					}
					if last == ' ' {
						t.Error("snippet ends with space")
					}
				}
			},
		},
		{
			name:    "positions out of bounds are clamped",
			content: "hello world",
			positions: []MatchPosition{
				{Start: -5, End: 50},
			},
			maxLen: 100,
			check: func(t *testing.T, result string) {
				if result != "hello world" {
					t.Errorf("expected full content, got %q", result)
				}
			},
		},
		{
			name:    "positions completely out of bounds",
			content: "hello world",
			positions: []MatchPosition{
				{Start: 100, End: 200},
			},
			maxLen: 100,
			check: func(t *testing.T, result string) {
				// Out of bounds positions are filtered, so we get prefix behavior.
				if result != "hello world" {
					t.Errorf("expected full content, got %q", result)
				}
			},
		},
		{
			name:      "default maxLen when zero",
			content:   strings.Repeat("a ", 1000),
			positions: []MatchPosition{{Start: 500, End: 510}},
			maxLen:    0,
			check: func(t *testing.T, result string) {
				if len(result) > DefaultMaxSnippetLen+50 {
					t.Errorf("expected len around %d, got %d", DefaultMaxSnippetLen, len(result))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSnippet(tt.content, tt.positions, tt.maxLen)
			tt.check(t, result)
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "empty", text: "", want: 0},
		{name: "one byte", text: "a", want: 1},
		{name: "four bytes", text: "abcd", want: 1},
		{name: "five bytes", text: "abcde", want: 2},
		{name: "eight bytes", text: "abcdefgh", want: 2},
		{name: "100 bytes", text: strings.Repeat("a", 100), want: 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestTrimToTokenBudget(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		maxTokens int
		check     func(t *testing.T, result string)
	}{
		{
			name:      "empty text",
			text:      "",
			maxTokens: 10,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty, got %q", result)
				}
			},
		},
		{
			name:      "zero budget",
			text:      "hello world",
			maxTokens: 0,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty, got %q", result)
				}
			},
		},
		{
			name:      "negative budget",
			text:      "hello world",
			maxTokens: -1,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty, got %q", result)
				}
			},
		},
		{
			name:      "text fits within budget",
			text:      "hello world",
			maxTokens: 100,
			check: func(t *testing.T, result string) {
				if result != "hello world" {
					t.Errorf("expected full text, got %q", result)
				}
			},
		},
		{
			name:      "text trimmed at word boundary",
			text:      "hello world this is a longer sentence that needs trimming",
			maxTokens: 5,
			check: func(t *testing.T, result string) {
				// 5 tokens * 4 bytes = 20 bytes max
				if len(result) > 20 {
					t.Errorf("expected len <= 20, got %d (%q)", len(result), result)
				}
				// Should end at a word boundary (no partial words).
				if len(result) > 0 && result[len(result)-1] == ' ' {
					t.Error("result ends with space")
				}
			},
		},
		{
			name:      "single long word truncated at byte limit",
			text:      strings.Repeat("x", 100),
			maxTokens: 5,
			check: func(t *testing.T, result string) {
				if len(result) != 20 {
					t.Errorf("expected len 20, got %d", len(result))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimToTokenBudget(tt.text, tt.maxTokens)
			tt.check(t, result)
		})
	}
}
