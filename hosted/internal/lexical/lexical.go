// Package lexical is the registry's deterministic lexical retrieval model
// (RFC-002 §8): tokenized query terms are matched against an artifact's name,
// description, capability IDs and taxonomy fields, and matches are ranked with
// fixed field weights and total-order tie-breaks. There is no vector search,
// no reranker and no forced similarity: a record that matches no query term is
// not a candidate, and a query made only of stopwords matches nothing.
//
// Both the storage candidate generator and the discovery service rank with
// this package, so the order a workspace-scoped store returns is the order
// discovery reproduces after authorization filtering.
package lexical

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Field weights. A term counts once, at the highest-weighted field it occurs
// in: naming fields identify an artifact, capability and taxonomy fields
// classify it, and descriptive prose only describes it.
const (
	WeightName        = 3
	WeightTaxonomy    = 2
	WeightDescription = 1
)

// stopwords are function words that carry no retrieval signal. Without them a
// query such as "deploy service to production" would match any description
// containing "to".
var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "in": true, "into": true,
	"is": true, "it": true, "its": true, "of": true, "on": true, "or": true,
	"that": true, "the": true, "these": true, "this": true, "those": true,
	"through": true, "to": true, "via": true, "with": true, "without": true,
}

// suffixes is an ordered, conservative English suffix-stripping table; the
// first suffix that applies wins and the remaining stem keeps at least three
// letters. It conflates the inflections a registry query commonly uses
// (parse/parsing/parser, create/creation, capability/capabilities) without a
// dictionary.
var suffixes = []struct{ suffix, replacement string }{
	{"sses", "ss"},
	{"ies", "y"},
	{"ational", "ate"},
	{"ation", "ate"},
	{"ing", ""},
	{"ed", ""},
	{"er", ""},
	{"s", ""},
}

// Terms lowercases text, splits it on every non-letter, non-digit rune, drops
// stopwords and stems each remaining word. Order is preserved and duplicates
// are kept; use Query for a de-duplicated query term list.
func Terms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(words))
	for _, w := range words {
		if stopwords[w] {
			continue
		}
		out = append(out, Stem(w))
	}
	return out
}

// Query returns the distinct terms of a query in first-occurrence order.
func Query(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range Terms(text) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// Stem applies the suffix table once and then drops a trailing "e", so that
// "parse", "parser" and "parsing" all become "pars".
func Stem(w string) string {
	if len(w) <= 3 {
		return w
	}
	for _, s := range suffixes {
		if !strings.HasSuffix(w, s.suffix) {
			continue
		}
		if s.suffix == "s" && (strings.HasSuffix(w, "ss") || strings.HasSuffix(w, "us") || strings.HasSuffix(w, "is")) {
			break
		}
		if base := w[:len(w)-len(s.suffix)] + s.replacement; len(base) >= 3 {
			w = base
		}
		break
	}
	if len(w) > 3 && strings.HasSuffix(w, "e") {
		w = w[:len(w)-1]
	}
	return w
}

// Document is the searchable projection of one catalog record.
type Document struct {
	// Name fields: the artifact ID and its logical name.
	ID, LogicalName string
	// Kind, capability IDs (required capabilities and a binding's capability
	// reference, without the @version pin) and taxonomy labels.
	Kind         string
	Capabilities []string
	Taxonomy     []string
	// Description is the summary or description prose.
	Description string
}

// metadata is the subset of catalog metadata that lexical retrieval reads.
// Unknown fields are ignored; malformed metadata is reported to the caller.
type metadata struct {
	LogicalName          string   `json:"logical_name"`
	Summary              string   `json:"summary"`
	Description          string   `json:"description"`
	Tags                 []string `json:"tags"`
	Taxonomy             []string `json:"taxonomy"`
	RequiredCapabilities []string `json:"required_capabilities"`
	CapabilityRef        string   `json:"capability_ref"`
}

// DocumentFromMetadata builds the searchable projection of a record from its
// kind, artifact ID and stored metadata JSON.
func DocumentFromMetadata(kind, id string, raw []byte) (Document, error) {
	var m metadata
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return Document{}, err
		}
	}
	caps := make([]string, 0, len(m.RequiredCapabilities)+1)
	for _, c := range m.RequiredCapabilities {
		caps = append(caps, unpin(c))
	}
	if m.CapabilityRef != "" {
		caps = append(caps, unpin(m.CapabilityRef))
	}
	taxonomy := append(append([]string{}, m.Tags...), m.Taxonomy...)
	description := strings.TrimSpace(m.Summary + " " + m.Description)
	return Document{ID: id, LogicalName: m.LogicalName, Kind: kind, Capabilities: caps, Taxonomy: taxonomy, Description: description}, nil
}

func unpin(ref string) string {
	if i := strings.LastIndex(ref, "@"); i > 0 {
		return ref[:i]
	}
	return ref
}

// Match is how well one document matches one query.
type Match struct {
	// Terms is the number of distinct query terms the document contains.
	Terms int
	// Weight sums, per matched term, the weight of the best field it hit.
	Weight int
}

// Matched reports whether the document is a candidate at all.
func (m Match) Matched() bool { return m.Terms > 0 }

// Better orders matches: more distinct query terms first, then more weight.
// It returns 0 when the matches tie, so callers apply their own total-order
// tie-break on the artifact reference.
func (m Match) Better(o Match) int {
	switch {
	case m.Terms != o.Terms:
		return sign(m.Terms - o.Terms)
	case m.Weight != o.Weight:
		return sign(m.Weight - o.Weight)
	}
	return 0
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// Index holds a document's terms per field weight.
type Index struct{ fields [3]map[string]bool }

// NewIndex analyzes a document once so it can be scored against a query.
func NewIndex(d Document) Index {
	add := func(set map[string]bool, texts ...string) {
		for _, text := range texts {
			for _, t := range Terms(text) {
				set[t] = true
			}
		}
	}
	var ix Index
	for i := range ix.fields {
		ix.fields[i] = map[string]bool{}
	}
	add(ix.fields[0], d.ID, d.LogicalName)
	add(ix.fields[1], d.Kind)
	add(ix.fields[1], d.Capabilities...)
	add(ix.fields[1], d.Taxonomy...)
	add(ix.fields[2], d.Description)
	return ix
}

// Score matches query terms (from Query) against the index.
func (ix Index) Score(terms []string) Match {
	weights := [3]int{WeightName, WeightTaxonomy, WeightDescription}
	var m Match
	for _, t := range terms {
		for i, field := range ix.fields {
			if field[t] {
				m.Terms++
				m.Weight += weights[i]
				break
			}
		}
	}
	return m
}

// Score is a convenience for scoring a single document against query text.
func Score(query string, d Document) Match { return NewIndex(d).Score(Query(query)) }
