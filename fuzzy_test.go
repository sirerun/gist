package gist

import (
	"sync"
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"kitten", "sitting", 3},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "abcd", 1},
		{"abc", "aXc", 1},
	}
	for _, tt := range tests {
		got := LevenshteinDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("LevenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFuzzyMatch_ExactMatch(t *testing.T) {
	v := NewVocabulary()
	v.Add("function", "variable", "constant")

	results := FuzzyMatch("function", v, 2)
	if len(results) == 0 {
		t.Fatal("expected at least one result for exact match")
	}
	if results[0].Term != "function" || results[0].Distance != 0 {
		t.Errorf("first result = %+v, want {Term:function Distance:0}", results[0])
	}
}

func TestFuzzyMatch_OneEditTypo(t *testing.T) {
	v := NewVocabulary()
	v.Add("function", "variable", "constant")

	tests := []struct {
		name  string
		query string
	}{
		{"transposition", "fucntion"},
		{"insertion", "functiion"},
		{"deletion", "functon"},
		{"substitution", "funxtion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := FuzzyMatch(tt.query, v, 2)
			found := false
			for _, r := range results {
				if r.Term == "function" && r.Distance <= 2 {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("FuzzyMatch(%q) did not return 'function', got %v", tt.query, results)
			}
		})
	}
}

func TestFuzzyMatch_TwoEditDistance(t *testing.T) {
	v := NewVocabulary()
	v.Add("database", "datastore", "datum")

	results := FuzzyMatch("datbase", v, 2)
	found := false
	for _, r := range results {
		if r.Term == "database" {
			found = true
			if r.Distance > 2 {
				t.Errorf("distance for 'database' = %d, want <= 2", r.Distance)
			}
		}
	}
	if !found {
		t.Error("expected 'database' in results for 'datbase'")
	}
}

func TestFuzzyMatch_BeyondThreshold(t *testing.T) {
	v := NewVocabulary()
	v.Add("function", "variable", "constant")

	results := FuzzyMatch("xyz", v, 2)
	if len(results) != 0 {
		t.Errorf("expected no results for query far from vocabulary, got %v", results)
	}
}

func TestFuzzyMatch_EmptyVocabulary(t *testing.T) {
	v := NewVocabulary()

	results := FuzzyMatch("anything", v, 2)
	if len(results) != 0 {
		t.Errorf("expected no results for empty vocabulary, got %v", results)
	}
}

func TestFuzzyMatch_CaseInsensitivity(t *testing.T) {
	v := NewVocabulary()
	v.Add("Function", "VARIABLE")

	results := FuzzyMatch("FUNCTION", v, 0)
	if len(results) != 1 || results[0].Term != "function" {
		t.Errorf("case insensitive match failed, got %v", results)
	}

	results = FuzzyMatch("variable", v, 0)
	if len(results) != 1 || results[0].Term != "variable" {
		t.Errorf("case insensitive match failed, got %v", results)
	}
}

func TestFuzzyMatch_SortOrder(t *testing.T) {
	v := NewVocabulary()
	v.Add("cat", "bat", "hat", "car", "can")

	results := FuzzyMatch("cat", v, 2)
	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// Verify sorted by distance ascending, then alphabetically
	for i := 1; i < len(results); i++ {
		prev, curr := results[i-1], results[i]
		if prev.Distance > curr.Distance {
			t.Errorf("results not sorted by distance: %+v before %+v", prev, curr)
		}
		if prev.Distance == curr.Distance && prev.Term > curr.Term {
			t.Errorf("results not sorted alphabetically within same distance: %+v before %+v", prev, curr)
		}
	}

	// First result should be exact match
	if results[0].Term != "cat" || results[0].Distance != 0 {
		t.Errorf("first result = %+v, want exact match for 'cat'", results[0])
	}
}

func TestFuzzyMatch_DefaultMaxDistance(t *testing.T) {
	v := NewVocabulary()
	v.Add("test")

	// maxDistance <= 0 should default to 2
	results := FuzzyMatch("tst", v, 0)
	if len(results) != 1 {
		t.Errorf("default maxDistance failed, got %v", results)
	}

	results = FuzzyMatch("tst", v, -1)
	if len(results) != 1 {
		t.Errorf("negative maxDistance should default to 2, got %v", results)
	}
}

func TestFuzzyMatch_ConcurrentAccess(t *testing.T) {
	v := NewVocabulary()
	v.Add("initial")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			v.Add("term" + string(rune('a'+i%26)))
		}(i)
		go func() {
			defer wg.Done()
			FuzzyMatch("initial", v, 2)
		}()
	}
	wg.Wait()

	if v.Len() == 0 {
		t.Error("vocabulary should not be empty after concurrent adds")
	}
}

func TestVocabulary_AddAndLen(t *testing.T) {
	v := NewVocabulary()
	if v.Len() != 0 {
		t.Errorf("new vocabulary Len() = %d, want 0", v.Len())
	}

	v.Add("a", "b", "c")
	if v.Len() != 3 {
		t.Errorf("Len() = %d, want 3", v.Len())
	}

	// Duplicates should not increase length
	v.Add("a", "b")
	if v.Len() != 3 {
		t.Errorf("Len() after duplicates = %d, want 3", v.Len())
	}
}

func TestVocabulary_Terms(t *testing.T) {
	v := NewVocabulary()
	v.Add("cherry", "apple", "banana")

	terms := v.Terms()
	want := []string{"apple", "banana", "cherry"}
	if len(terms) != len(want) {
		t.Fatalf("Terms() returned %d items, want %d", len(terms), len(want))
	}
	for i, got := range terms {
		if got != want[i] {
			t.Errorf("Terms()[%d] = %q, want %q", i, got, want[i])
		}
	}
}
