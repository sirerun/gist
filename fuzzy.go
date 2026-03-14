package gist

import (
	"sort"
	"strings"
	"sync"

	"github.com/agnivade/levenshtein"
)

// FuzzyResult holds a vocabulary term and its edit distance from the query.
type FuzzyResult struct {
	Term     string
	Distance int
}

// Vocabulary is a thread-safe collection of known terms used for fuzzy matching.
type Vocabulary struct {
	mu    sync.RWMutex
	terms map[string]struct{}
}

// NewVocabulary creates an empty Vocabulary.
func NewVocabulary() *Vocabulary {
	return &Vocabulary{terms: make(map[string]struct{})}
}

// Add inserts one or more terms into the vocabulary. Duplicate terms are ignored.
func (v *Vocabulary) Add(terms ...string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, t := range terms {
		v.terms[strings.ToLower(t)] = struct{}{}
	}
}

// Terms returns all vocabulary terms in sorted order.
func (v *Vocabulary) Terms() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]string, 0, len(v.terms))
	for t := range v.terms {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Len returns the number of terms in the vocabulary.
func (v *Vocabulary) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.terms)
}

// LevenshteinDistance returns the edit distance between two strings.
// It is a convenience wrapper around the agnivade/levenshtein library.
func LevenshteinDistance(a, b string) int {
	return levenshtein.ComputeDistance(a, b)
}

// FuzzyMatch finds vocabulary terms within maxDistance edits of query.
// Both the query and vocabulary terms are compared in lowercase.
// Results are sorted by distance ascending, then alphabetically for ties.
// If maxDistance is 0 or negative, it defaults to 2.
func FuzzyMatch(query string, vocab *Vocabulary, maxDistance int) []FuzzyResult {
	if maxDistance <= 0 {
		maxDistance = 2
	}
	q := strings.ToLower(query)

	vocab.mu.RLock()
	var results []FuzzyResult
	for term := range vocab.terms {
		d := levenshtein.ComputeDistance(q, term)
		if d <= maxDistance {
			results = append(results, FuzzyResult{Term: term, Distance: d})
		}
	}
	vocab.mu.RUnlock()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Distance != results[j].Distance {
			return results[i].Distance < results[j].Distance
		}
		return results[i].Term < results[j].Term
	})
	return results
}
