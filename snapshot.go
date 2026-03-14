package gist

import (
	"fmt"
	"strings"
)

// SnapshotTier represents the priority level of a snapshot section.
type SnapshotTier int

const (
	// TierCritical contains errors and the most recent events.
	TierCritical SnapshotTier = iota
	// TierHigh contains search results summary.
	TierHigh
	// TierMedium contains indexed sources summary.
	TierMedium
	// TierLow contains older events and session metadata.
	TierLow
)

// SnapshotSection is a single section within a snapshot, tagged with a
// priority tier and an estimated token cost.
type SnapshotSection struct {
	// Tier is the priority level of this section.
	Tier SnapshotTier
	// Title is a human-readable label for the section.
	Title string
	// Content is the rendered text of the section.
	Content string
	// TokenEstimate is the approximate token cost of Content.
	TokenEstimate int
}

// Snapshot holds prioritised sections built from session events.
type Snapshot struct {
	sections []SnapshotSection
}

// Sections returns a copy of all sections in the snapshot.
func (s *Snapshot) Sections() []SnapshotSection {
	out := make([]SnapshotSection, len(s.sections))
	copy(out, s.sections)
	return out
}

// Render outputs sections in priority order (critical first) until
// maxTokens is exhausted. It uses EstimateTokens from snippet.go to
// verify fit. If maxTokens is <= 0 all sections are included.
func (s *Snapshot) Render(maxTokens int) string {
	// Sections are already stored in priority order by BuildSnapshot.
	var b strings.Builder
	used := 0
	for _, sec := range s.sections {
		tokens := EstimateTokens(sec.Content)
		if maxTokens > 0 && used+tokens > maxTokens {
			// Try to fit a trimmed version.
			remaining := maxTokens - used
			if remaining <= 0 {
				break
			}
			trimmed := TrimToTokenBudget(sec.Content, remaining)
			if len(trimmed) == 0 {
				break
			}
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("## ")
			b.WriteString(sec.Title)
			b.WriteString("\n")
			b.WriteString(trimmed)
			break
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(sec.Title)
		b.WriteString("\n")
		b.WriteString(sec.Content)
		used += tokens
	}
	return b.String()
}

// BuildSnapshot creates a Snapshot from session events, organising them
// into priority tiers. The budget parameter hints at total token capacity
// but does not truncate — use Render(maxTokens) for that.
func BuildSnapshot(session *Session, budget int) *Snapshot {
	events := session.Events()

	var sections []SnapshotSection

	// Critical: errors.
	errorEvents := session.EventsByType("error")
	if len(errorEvents) > 0 {
		var lines []string
		for _, e := range errorEvents {
			ee, _ := e.(ErrorEvent)
			line := fmt.Sprintf("- [%s] %s", ee.Context, ee.Message)
			lines = append(lines, line)
		}
		content := strings.Join(lines, "\n")
		sections = append(sections, SnapshotSection{
			Tier:          TierCritical,
			Title:         "Errors",
			Content:       content,
			TokenEstimate: EstimateTokens(content),
		})
	}

	// Critical: most recent events (up to 5).
	if len(events) > 0 {
		n := len(events)
		start := n - 5
		if start < 0 {
			start = 0
		}
		var lines []string
		for _, e := range events[start:] {
			line := fmt.Sprintf("- [%s] %s", e.EventType(), formatEvent(e))
			lines = append(lines, line)
		}
		content := strings.Join(lines, "\n")
		sections = append(sections, SnapshotSection{
			Tier:          TierCritical,
			Title:         "Recent Activity",
			Content:       content,
			TokenEstimate: EstimateTokens(content),
		})
	}

	// High: search results summary.
	searchEvents := session.EventsByType("search")
	if len(searchEvents) > 0 {
		var lines []string
		for _, e := range searchEvents {
			se, _ := e.(SearchEvent)
			line := fmt.Sprintf("- query=%q results=%d layer=%s", se.Query, se.ResultCount, se.MatchLayer)
			lines = append(lines, line)
		}
		content := strings.Join(lines, "\n")
		sections = append(sections, SnapshotSection{
			Tier:          TierHigh,
			Title:         "Search Summary",
			Content:       content,
			TokenEstimate: EstimateTokens(content),
		})
	}

	// Medium: indexed sources summary.
	indexEvents := session.EventsByType("index")
	if len(indexEvents) > 0 {
		var lines []string
		for _, e := range indexEvents {
			ie, _ := e.(IndexEvent)
			line := fmt.Sprintf("- source=%q chunks=%d", ie.Source, ie.ChunkCount)
			lines = append(lines, line)
		}
		content := strings.Join(lines, "\n")
		sections = append(sections, SnapshotSection{
			Tier:          TierMedium,
			Title:         "Indexed Sources",
			Content:       content,
			TokenEstimate: EstimateTokens(content),
		})
	}

	// Low: session metadata.
	summary := session.Summary()
	var metaLines []string
	metaLines = append(metaLines, fmt.Sprintf("- session_id=%q", session.ID()))
	metaLines = append(metaLines, fmt.Sprintf("- duration=%s", summary.TotalDuration))
	metaLines = append(metaLines, fmt.Sprintf("- total_events=%d", len(events)))
	for typ, count := range summary.EventCounts {
		metaLines = append(metaLines, fmt.Sprintf("- %s_count=%d", typ, count))
	}
	metaContent := strings.Join(metaLines, "\n")
	sections = append(sections, SnapshotSection{
		Tier:          TierLow,
		Title:         "Session Metadata",
		Content:       metaContent,
		TokenEstimate: EstimateTokens(metaContent),
	})

	return &Snapshot{sections: sections}
}

// formatEvent returns a one-line summary of an event.
func formatEvent(e Event) string {
	switch v := e.(type) {
	case IndexEvent:
		return fmt.Sprintf("indexed %q (%d chunks)", v.Source, v.ChunkCount)
	case SearchEvent:
		return fmt.Sprintf("searched %q (%d results, %s)", v.Query, v.ResultCount, v.MatchLayer)
	case ErrorEvent:
		return fmt.Sprintf("error: %s (%s)", v.Message, v.Context)
	default:
		return e.EventType()
	}
}
