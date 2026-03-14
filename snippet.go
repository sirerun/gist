package gist

import "unicode"

// DefaultMaxSnippetLen is the default maximum byte length for extracted snippets.
const DefaultMaxSnippetLen = 1500

// MatchPosition represents a byte-offset range within content where a search
// term was found.
type MatchPosition struct {
	// Start is the starting byte offset (inclusive).
	Start int
	// End is the ending byte offset (exclusive).
	End int
}

// ExtractSnippet returns the best snippet of content around the given match
// positions. For a single match it centers the window on the match. For
// multiple matches it finds the densest cluster and centers there. The
// returned snippet respects word boundaries and prefers sentence boundaries
// when they are close to the cut points.
//
// If positions is empty the function returns the first maxLen bytes of content
// (respecting word boundaries). If content is shorter than maxLen the full
// content is returned.
func ExtractSnippet(content string, positions []MatchPosition, maxLen int) string {
	if maxLen <= 0 {
		maxLen = DefaultMaxSnippetLen
	}
	if len(content) == 0 {
		return ""
	}
	if len(content) <= maxLen {
		return content
	}

	// Filter out-of-bounds positions.
	valid := make([]MatchPosition, 0, len(positions))
	for _, p := range positions {
		if p.Start < 0 {
			p.Start = 0
		}
		if p.End > len(content) {
			p.End = len(content)
		}
		if p.Start < len(content) && p.End > p.Start {
			valid = append(valid, p)
		}
	}

	var center int
	if len(valid) == 0 {
		// No valid positions — return prefix.
		return trimToWordBoundary(content, 0, maxLen)
	} else if len(valid) == 1 {
		center = (valid[0].Start + valid[0].End) / 2
	} else {
		center = densestClusterCenter(valid, maxLen)
	}

	// Build window centered on center.
	half := maxLen / 2
	start := center - half
	end := start + maxLen

	if start < 0 {
		start = 0
		end = maxLen
	}
	if end > len(content) {
		end = len(content)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}

	return trimToWordBoundary(content, start, end)
}

// densestClusterCenter finds the window of size windowSize that contains the
// most match positions and returns the center of that window.
func densestClusterCenter(positions []MatchPosition, windowSize int) int {
	bestCount := 0
	bestCenter := (positions[0].Start + positions[0].End) / 2

	for _, p := range positions {
		wStart := p.Start
		wEnd := wStart + windowSize
		count := 0
		for _, q := range positions {
			if q.Start >= wStart && q.End <= wEnd {
				count++
			} else if q.Start < wEnd && q.End > wStart {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			// Center of the window.
			bestCenter = wStart + windowSize/2
		}
	}
	return bestCenter
}

// trimToWordBoundary extracts content[start:end] and adjusts both boundaries
// outward to the nearest word boundary. It prefers sentence boundaries (. ! ?)
// when they are within 50 bytes of the cut point.
func trimToWordBoundary(content string, start, end int) string {
	if start <= 0 && end >= len(content) {
		return content
	}

	const sentenceSearchRange = 50

	// Adjust start forward to word boundary (unless at beginning).
	if start > 0 {
		// Look for sentence boundary first.
		found := false
		limit := start + sentenceSearchRange
		if limit > end {
			limit = end
		}
		for i := start; i < limit; i++ {
			if isSentenceEnd(content[i]) && i+1 < len(content) && content[i+1] == ' ' {
				start = i + 2
				found = true
				break
			}
		}
		if !found {
			// Move forward to next word boundary.
			for start < end && !isWordBoundary(content, start) {
				start++
			}
			// Skip whitespace after the break.
			for start < end && content[start] == ' ' {
				start++
			}
		}
	}

	// Adjust end backward to word boundary (unless at content end).
	if end < len(content) {
		// Look for sentence boundary first.
		found := false
		limit := end - sentenceSearchRange
		if limit < start {
			limit = start
		}
		for i := end - 1; i >= limit; i-- {
			if isSentenceEnd(content[i]) {
				end = i + 1
				found = true
				break
			}
		}
		if !found {
			// Move backward to word boundary.
			for end > start && !isWordBoundary(content, end) {
				end--
			}
		}
	}

	if start >= end {
		return ""
	}
	// Trim any trailing whitespace at the cut boundary.
	for end > start && content[end-1] == ' ' {
		end--
	}
	if start >= end {
		return ""
	}
	return content[start:end]
}

func isSentenceEnd(b byte) bool {
	return b == '.' || b == '!' || b == '?'
}

func isWordBoundary(s string, i int) bool {
	if i <= 0 || i >= len(s) {
		return true
	}
	return s[i-1] == ' ' || s[i] == ' '
}

// EstimateTokens returns a rough token count for the given text. It uses the
// common heuristic of 1 token per 4 bytes, which is a reasonable
// approximation for English text with typical LLM tokenizers.
func EstimateTokens(text string) int {
	n := len(text)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4 // round up
}

// TrimToTokenBudget trims text to fit within maxTokens, cutting at a word
// boundary. If the text already fits it is returned unchanged.
func TrimToTokenBudget(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	maxBytes := maxTokens * 4
	if len(text) <= maxBytes {
		return text
	}

	// Walk backward from maxBytes to find a word boundary.
	end := maxBytes
	for end > 0 && !unicode.IsSpace(rune(text[end])) {
		end--
	}
	if end == 0 {
		// Single long word — just truncate at byte limit.
		return text[:maxBytes]
	}
	return text[:end]
}
