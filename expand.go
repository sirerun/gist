package gist

import (
	"strings"
	"unicode"
)

// ExpandQuery performs tier-3.5 query expansion: identifier splitting
// (camelCase/snake_case) and multi-term OR expansion. It returns the
// original query terms plus any additional terms discovered by splitting
// compound identifiers, deduplicated and in first-seen order. This is a
// cheap lexical remedy for paraphrase divergence in code-adjacent corpora,
// tried before fuzzy correction or (if ever built) an embedding tier.
func ExpandQuery(query string) []string {
	seen := make(map[string]bool)
	var terms []string

	add := func(term string) {
		term = strings.ToLower(term)
		if term == "" || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}

	for _, word := range strings.Fields(query) {
		add(word)
		for _, part := range splitIdentifier(word) {
			add(part)
		}
	}

	return terms
}

// ExpandQueryString joins the expanded terms produced by ExpandQuery back
// into a space-separated query string suitable for the existing search
// tiers, which already treat space-separated terms as an OR-style match.
func ExpandQueryString(query string) string {
	return strings.Join(ExpandQuery(query), " ")
}

// splitIdentifier splits a single token on snake_case, kebab-case, and
// camelCase boundaries, returning its constituent words in lowercase.
// Tokens with no such boundaries return nil (nothing new to add).
func splitIdentifier(token string) []string {
	// snake_case / kebab-case: split first, then camelCase-split each piece.
	rawParts := strings.FieldsFunc(token, func(r rune) bool {
		return r == '_' || r == '-'
	})
	if len(rawParts) == 0 {
		return nil
	}

	hadDelimiter := len(rawParts) > 1 || rawParts[0] != token

	var words []string
	for _, part := range rawParts {
		words = append(words, splitCamelCase(part)...)
	}

	if !hadDelimiter && len(words) <= 1 {
		return nil
	}

	return words
}

// splitCamelCase splits a camelCase or PascalCase token into lowercase
// words on lower-to-upper and upper-to-upper-then-lower boundaries.
func splitCamelCase(s string) []string {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}

	var words []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		boundary := false
		if unicode.IsLower(prev) && unicode.IsUpper(cur) {
			boundary = true
		} else if unicode.IsUpper(prev) && unicode.IsUpper(cur) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
			boundary = true
		}
		if boundary {
			words = append(words, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	words = append(words, strings.ToLower(string(runes[start:])))

	return words
}
