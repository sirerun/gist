package gist

import (
	"context"
	"testing"
	"time"
)

// TestSupersedes verifies that WithSupersedes retires prior sources under
// the given label, excluding their chunks from search.
func TestSupersedes(t *testing.T) {
	ctx := context.Background()
	g, err := New(WithMemory())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Close()

	if _, err := g.Index(ctx, "old page content", WithSource("page-a")); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if _, err := g.Index(ctx, "new page content", WithSource("page-b"), WithSupersedes("page-a")); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	results, err := g.Search(ctx, "content")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, r := range results {
		if r.Source == "page-a" {
			t.Errorf("expected page-a to be superseded and excluded, got result: %+v", r)
		}
	}

	found := false
	for _, r := range results {
		if r.Source == "page-b" {
			found = true
		}
	}
	if !found {
		t.Error("expected page-b to be present in results")
	}
}

// TestSelfSupersession verifies that reusing a label without WithSupersedes
// or WithKeepPrevious implies self-supersession of prior sources.
func TestSelfSupersession(t *testing.T) {
	ctx := context.Background()
	g, err := New(WithMemory())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Close()

	if _, err := g.Index(ctx, "first version unique-marker-one", WithSource("snapshot")); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if _, err := g.Index(ctx, "second version unique-marker-two", WithSource("snapshot")); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	results, err := g.Search(ctx, "unique-marker-one")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected first version to be self-superseded, got %d results", len(results))
	}

	results, err = g.Search(ctx, "unique-marker-two")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Error("expected second version to still be searchable")
	}

	// WithKeepPrevious opts out of self-supersession.
	if _, err := g.Index(ctx, "third version unique-marker-three", WithSource("snapshot"), WithKeepPrevious()); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	results, err = g.Search(ctx, "unique-marker-two")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Error("expected second version to remain searchable when third indexed with WithKeepPrevious")
	}
}

// TestTTLExpiry verifies that sources indexed with WithTTL are excluded
// from search results once their TTL has elapsed.
func TestTTLExpiry(t *testing.T) {
	ctx := context.Background()
	g, err := New(WithMemory())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Close()

	if _, err := g.Index(ctx, "ephemeral tool output", WithSource("tool-output"), WithTTL(20*time.Millisecond)); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	results, err := g.Search(ctx, "ephemeral")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected result before TTL expiry")
	}

	time.Sleep(40 * time.Millisecond)

	results, err = g.Search(ctx, "ephemeral")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results after TTL expiry, got %d", len(results))
	}
}

// TestHalfLifeDecay verifies that WithHalfLife causes a source's relevance
// score to decay over time, while sources without a half-life do not decay.
func TestHalfLifeDecay(t *testing.T) {
	ctx := context.Background()
	g, err := New(WithMemory())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Close()

	if _, err := g.Index(ctx, "volatile content about widgets", WithSource("volatile"), WithHalfLife(10*time.Millisecond)); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	results, err := g.Search(ctx, "widgets")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected a result")
	}
	freshScore := results[0].Score

	time.Sleep(50 * time.Millisecond)

	results, err = g.Search(ctx, "widgets")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected a result after decay")
	}
	decayedScore := results[0].Score

	if decayedScore >= freshScore {
		t.Errorf("expected decayed score (%v) to be lower than fresh score (%v)", decayedScore, freshScore)
	}
}

// TestVacuum verifies that Vacuum removes superseded and expired sources
// and their chunks from the store.
func TestVacuum(t *testing.T) {
	ctx := context.Background()
	g, err := New(WithMemory())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Close()

	if _, err := g.Index(ctx, "will be superseded", WithSource("a")); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if _, err := g.Index(ctx, "supersedes a", WithSource("b"), WithSupersedes("a")); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if _, err := g.Index(ctx, "will expire soon", WithSource("c"), WithTTL(10*time.Millisecond)); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	statsBefore := g.Stats()
	if statsBefore.SourceCount != 3 {
		t.Fatalf("expected 3 sources before vacuum, got %d", statsBefore.SourceCount)
	}

	if err := g.Vacuum(ctx); err != nil {
		t.Fatalf("Vacuum() error = %v", err)
	}

	ms, ok := g.store.(*MemoryStore)
	if !ok {
		t.Fatal("expected underlying store to be *MemoryStore")
	}
	stats, err := ms.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.SourceCount != 1 {
		t.Errorf("expected 1 source remaining after vacuum, got %d", stats.SourceCount)
	}
}
