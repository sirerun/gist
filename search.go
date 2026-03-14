package gist

import (
	"context"
	"strings"
)

// SearchResult represents a single result from the three-tier search engine.
type SearchResult struct {
	// Title is the heading path from the matched chunk (e.g., "Config > Database > Connection Pool").
	Title string
	// Snippet is the smart-extracted context around the match.
	Snippet string
	// Source is the source label (e.g., file path or tool name).
	Source string
	// Score is the BM25 relevance score.
	Score float64
	// ContentType is "code" or "prose".
	ContentType string
	// MatchLayer indicates which search tier produced the match: "porter", "trigram", or "fuzzy".
	MatchLayer string
}

// SearchOption configures search behavior.
type SearchOption func(*searchConfig)

type searchConfig struct {
	limit        int
	sourceFilter string
	snippetLen   int
	budget       int
}

func defaultSearchConfig() searchConfig {
	return searchConfig{
		limit:      5,
		snippetLen: DefaultMaxSnippetLen,
	}
}

// WithLimit sets the maximum number of results to return. Default is 5.
func WithLimit(n int) SearchOption {
	return func(c *searchConfig) {
		if n > 0 {
			c.limit = n
		}
	}
}

// WithSourceFilter restricts results to chunks from the named source.
func WithSourceFilter(s string) SearchOption {
	return func(c *searchConfig) {
		c.sourceFilter = s
	}
}

// WithSnippetLen sets the maximum byte length of extracted snippets. Default is 1500.
func WithSnippetLen(n int) SearchOption {
	return func(c *searchConfig) {
		if n > 0 {
			c.snippetLen = n
		}
	}
}

// WithBudget sets a token budget. Results are accumulated until adding
// another would exceed this budget.
func WithBudget(tokens int) SearchOption {
	return func(c *searchConfig) {
		if tokens > 0 {
			c.budget = tokens
		}
	}
}

// Searcher performs three-tier search (porter → trigram → fuzzy) against a Store.
type Searcher struct {
	store Store
	vocab *Vocabulary
}

// NewSearcher creates a Searcher that queries store and uses vocab for fuzzy
// correction when both porter and trigram searches return no results.
func NewSearcher(store Store, vocab *Vocabulary) *Searcher {
	return &Searcher{store: store, vocab: vocab}
}

// Search executes a three-tier search: first porter stemming, then trigram
// substring matching, then fuzzy-corrected re-search. Results include
// smart-extracted snippets and respect any token budget.
func (s *Searcher) Search(ctx context.Context, query string, opts ...SearchOption) ([]SearchResult, error) {
	cfg := defaultSearchConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	params := SearchParams{
		Query:        query,
		Limit:        cfg.limit,
		SourceFilter: cfg.sourceFilter,
	}

	// Build a source-ID-to-label map for resolving source names.
	sources, err := s.store.Sources(ctx)
	if err != nil {
		return nil, err
	}
	sourceLabels := make(map[int]string, len(sources))
	for _, src := range sources {
		sourceLabels[src.ID] = src.Label
	}

	// Tier 1: Porter stemming.
	matches, err := s.store.SearchPorter(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return s.convertMatches(matches, sourceLabels, cfg), nil
	}

	// Tier 2: Trigram.
	matches, err = s.store.SearchTrigram(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return s.convertMatches(matches, sourceLabels, cfg), nil
	}

	// Tier 3: Fuzzy correction.
	if s.vocab == nil {
		return nil, nil
	}
	fuzzyResults := FuzzyMatch(query, s.vocab, 2)
	if len(fuzzyResults) == 0 {
		return nil, nil
	}

	// Use the best fuzzy match as the corrected query.
	corrected := fuzzyResults[0].Term
	params.Query = corrected

	// Try porter with corrected term first.
	matches, err = s.store.SearchPorter(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		// Try trigram with corrected term.
		matches, err = s.store.SearchTrigram(ctx, params)
		if err != nil {
			return nil, err
		}
	}

	// Override match layer to "fuzzy" since we used correction.
	for i := range matches {
		matches[i].MatchLayer = "fuzzy"
	}

	return s.convertMatches(matches, sourceLabels, cfg), nil
}

// convertMatches transforms store SearchMatch results into SearchResults with
// snippets and optional budget enforcement.
func (s *Searcher) convertMatches(matches []SearchMatch, sourceLabels map[int]string, cfg searchConfig) []SearchResult {
	var results []SearchResult
	var usedTokens int

	for _, m := range matches {
		// Find match positions for snippet extraction.
		positions := findMatchPositions(m.Content, strings.Fields(cfg.sourceFilter))
		snippet := ExtractSnippet(m.Content, positions, cfg.snippetLen)

		if cfg.budget > 0 {
			tokens := EstimateTokens(snippet)
			if usedTokens+tokens > cfg.budget && len(results) > 0 {
				break
			}
			usedTokens += tokens
		}

		results = append(results, SearchResult{
			Title:       m.HeadingPath,
			Snippet:     snippet,
			Source:      sourceLabels[m.SourceID],
			Score:       m.Score,
			ContentType: m.ContentType,
			MatchLayer:  m.MatchLayer,
		})
	}

	return results
}

// findMatchPositions locates byte positions of query terms within content.
func findMatchPositions(content string, terms []string) []MatchPosition {
	if len(terms) == 0 {
		return nil
	}
	lower := strings.ToLower(content)
	var positions []MatchPosition
	for _, term := range terms {
		t := strings.ToLower(term)
		offset := 0
		for {
			idx := strings.Index(lower[offset:], t)
			if idx < 0 {
				break
			}
			start := offset + idx
			positions = append(positions, MatchPosition{
				Start: start,
				End:   start + len(t),
			})
			offset = start + len(t)
		}
	}
	return positions
}
