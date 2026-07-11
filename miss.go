package gist

import (
	"sync"
	"time"
)

// MissClass classifies why a search was recorded as a miss.
type MissClass string

const (
	// MissNoContent indicates the query's terms are absent from the corpus entirely.
	MissNoContent MissClass = "no-content"
	// MissParaphraseDivergence indicates vocabulary terms exist near the query
	// (fuzzy correction found candidates) but scoring rejected them.
	MissParaphraseDivergence MissClass = "paraphrase-divergence"
	// MissChunkingTruncation indicates the matched source exists but the
	// matching chunk scored below the configured floor.
	MissChunkingTruncation MissClass = "chunking-truncation"
)

// defaultMissScoreFloor is the default BM25/relevance score below which a
// top result is considered a miss.
const defaultMissScoreFloor = 0.15

// NearMiss describes a low-scoring candidate considered but not returned
// with confidence as part of a miss record.
type NearMiss struct {
	// Source is the source label of the candidate.
	Source string
	// Title is the heading path of the candidate.
	Title string
	// Score is the relevance score of the candidate.
	Score float64
}

// MissRecord captures the details of a single retrieval miss.
type MissRecord struct {
	// Time is when the miss occurred.
	Time time.Time
	// Query is the search query string that missed.
	Query string
	// SessionID identifies the session the search belongs to, if any.
	SessionID string
	// Class is the machine-assigned classification of the miss.
	Class MissClass
	// NearMisses holds up to the top-3 low-scoring candidates.
	NearMisses []NearMiss
	// ScoreFloor is the score floor in effect when the miss was recorded.
	ScoreFloor float64
	// MatchLayer is the search tier reached ("porter", "trigram", "fuzzy", or "" if none).
	MatchLayer string
}

// MissLog is a bounded, thread-safe, in-process record of retrieval misses.
type MissLog struct {
	mu       sync.Mutex
	capacity int
	records  []MissRecord
	counts   map[MissClass]int
}

// NewMissLog creates a MissLog that retains at most capacity records,
// evicting the oldest entries once full.
func NewMissLog(capacity int) *MissLog {
	if capacity <= 0 {
		capacity = 1000
	}
	return &MissLog{
		capacity: capacity,
		counts:   make(map[MissClass]int),
	}
}

// Record appends a miss record to the log, evicting the oldest record if
// the log is at capacity.
func (l *MissLog) Record(rec MissRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.records = append(l.records, rec)
	l.counts[rec.Class]++

	if len(l.records) > l.capacity {
		evicted := l.records[0]
		l.records = l.records[1:]
		l.counts[evicted.Class]--
		if l.counts[evicted.Class] <= 0 {
			delete(l.counts, evicted.Class)
		}
	}
}

// Count returns the number of miss records currently retained.
func (l *MissLog) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.records)
}

// Recent returns up to n of the most recent miss records, newest last.
// If n <= 0, all retained records are returned.
func (l *MissLog) Recent(n int) []MissRecord {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n <= 0 || n > len(l.records) {
		n = len(l.records)
	}
	start := len(l.records) - n
	out := make([]MissRecord, n)
	copy(out, l.records[start:])
	return out
}

// ClassBreakdown returns a copy of the count of miss records by class.
func (l *MissLog) ClassBreakdown() map[MissClass]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[MissClass]int, len(l.counts))
	for k, v := range l.counts {
		out[k] = v
	}
	return out
}

// classifyMiss determines the MissClass for a search given its results and
// which match layer (if any) was reached.
func classifyMiss(results []SearchResult) MissClass {
	if len(results) == 0 {
		return MissNoContent
	}
	if results[0].MatchLayer == "fuzzy" {
		return MissParaphraseDivergence
	}
	return MissChunkingTruncation
}

// nearMisses extracts up to the top-3 results as NearMiss candidates.
func nearMisses(results []SearchResult) []NearMiss {
	n := len(results)
	if n > 3 {
		n = 3
	}
	out := make([]NearMiss, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, NearMiss{
			Source: results[i].Source,
			Title:  results[i].Title,
			Score:  results[i].Score,
		})
	}
	return out
}
