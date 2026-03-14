package gist

import (
	"context"
	"strings"
	"testing"
)

// TestE2EIndexAndSearch exercises New -> Index markdown -> Search -> verify snippets.
func TestE2EIndexAndSearch(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	content := `# Database Configuration

The connection pool size should be set to 20 for production workloads.
Use pgbouncer for connection multiplexing when scaling beyond 50 connections.

## Timeouts

Set the idle timeout to 30 seconds and the max lifetime to 5 minutes.
`

	_, err = g.Index(ctx, content, WithSource("database.md"))
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := g.Search(ctx, "connection pool")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}

	found := false
	for _, r := range results {
		if strings.Contains(r.Snippet, "connection") || strings.Contains(r.Snippet, "pool") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected snippet containing 'connection' or 'pool', got %v", results)
	}
}

// TestE2ECrossDocumentSearch exercises New -> Index multiple documents -> Search -> verify cross-document results.
func TestE2ECrossDocumentSearch(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	docs := []struct {
		content string
		source  string
	}{
		{
			content: "# Authentication\n\nUse OAuth2 with JWT tokens for API authentication.\nRefresh tokens expire after 7 days.",
			source:  "auth.md",
		},
		{
			content: "# Deployment\n\nDeploy to Kubernetes using Helm charts.\nThe authentication sidecar handles token validation.",
			source:  "deploy.md",
		},
		{
			content: "# Monitoring\n\nUse Prometheus for metrics and Grafana for dashboards.\nAlert on p99 latency exceeding 500ms.",
			source:  "monitoring.md",
		},
	}

	for _, doc := range docs {
		_, err := g.Index(ctx, doc.content, WithSource(doc.source))
		if err != nil {
			t.Fatalf("Index %s: %v", doc.source, err)
		}
	}

	// Search for "authentication" which appears in both auth.md and deploy.md.
	results, err := g.Search(ctx, "authentication", WithLimit(10))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results across documents, got %d", len(results))
	}

	// Verify results come from multiple sources.
	sources := make(map[string]bool)
	for _, r := range results {
		sources[r.Source] = true
	}
	if len(sources) < 2 {
		t.Errorf("expected results from at least 2 sources, got sources: %v", sources)
	}
}

// TestE2EBatchIndex exercises New -> BatchIndex multiple items -> Search -> verify results.
func TestE2EBatchIndex(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	items := []BatchItem{
		{Content: "# API Design\n\nUse REST for CRUD operations and gRPC for internal services.", Source: "api.md", Format: FormatMarkdown},
		{Content: "# Error Handling\n\nReturn structured error responses with error codes and messages.", Source: "errors.md", Format: FormatMarkdown},
		{Content: "# Testing\n\nWrite unit tests for all business logic. Use integration tests for API endpoints.", Source: "testing.md", Format: FormatMarkdown},
	}

	results, err := g.BatchIndex(ctx, items)
	if err != nil {
		t.Fatalf("BatchIndex: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("BatchIndex returned %d results, want 3", len(results))
	}
	for i, r := range results {
		if r == nil {
			t.Fatalf("BatchIndex result[%d] is nil", i)
		}
	}

	// Search for content that was batch-indexed.
	searchResults, err := g.Search(ctx, "error handling", WithLimit(10))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(searchResults) == 0 {
		t.Fatal("expected at least one result from batch-indexed content")
	}

	found := false
	for _, r := range searchResults {
		if strings.Contains(r.Snippet, "error") || strings.Contains(r.Snippet, "Error") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected snippet about errors, got %v", searchResults)
	}
}

// TestE2EStatsAccuracy exercises New -> Index -> Stats -> verify counts and bytes.
func TestE2EStatsAccuracy(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Verify zero stats initially.
	stats := g.Stats()
	if stats.SourceCount != 0 {
		t.Errorf("initial SourceCount = %d, want 0", stats.SourceCount)
	}
	if stats.ChunkCount != 0 {
		t.Errorf("initial ChunkCount = %d, want 0", stats.ChunkCount)
	}
	if stats.BytesIndexed != 0 {
		t.Errorf("initial BytesIndexed = %d, want 0", stats.BytesIndexed)
	}

	content1 := "# First Document\n\nThis is the first document with some content."
	content2 := "# Second Document\n\nThis is the second document with different content."

	res1, err := g.Index(ctx, content1, WithSource("first.md"))
	if err != nil {
		t.Fatalf("Index first: %v", err)
	}
	res2, err := g.Index(ctx, content2, WithSource("second.md"))
	if err != nil {
		t.Fatalf("Index second: %v", err)
	}

	stats = g.Stats()

	if stats.SourceCount != 2 {
		t.Errorf("SourceCount = %d, want 2", stats.SourceCount)
	}

	wantBytes := int64(len(content1) + len(content2))
	if stats.BytesIndexed != wantBytes {
		t.Errorf("BytesIndexed = %d, want %d", stats.BytesIndexed, wantBytes)
	}

	wantChunks := res1.TotalChunks + res2.TotalChunks
	if stats.ChunkCount != wantChunks {
		t.Errorf("ChunkCount = %d, want %d", stats.ChunkCount, wantChunks)
	}

	if stats.SearchCount != 0 {
		t.Errorf("SearchCount = %d, want 0 (no searches yet)", stats.SearchCount)
	}

	// Perform a search and verify search count increments.
	_, err = g.Search(ctx, "document")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	stats = g.Stats()
	if stats.SearchCount != 1 {
		t.Errorf("SearchCount after search = %d, want 1", stats.SearchCount)
	}
}

// TestE2ESearchWithBudget exercises New -> Index -> Search with WithBudget -> verify truncation.
func TestE2ESearchWithBudget(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Index multiple documents so there are many potential results.
	for i := 0; i < 10; i++ {
		content := "# Configuration Section\n\nThe server configuration includes database settings, cache settings, and logging configuration for production environments."
		_, err := g.Index(ctx, content, WithSource("config.md"))
		if err != nil {
			t.Fatalf("Index %d: %v", i, err)
		}
	}

	// Search without budget.
	unbounded, err := g.Search(ctx, "configuration", WithLimit(10))
	if err != nil {
		t.Fatalf("Search unbounded: %v", err)
	}

	// Search with a very small budget (enough for ~1 result).
	bounded, err := g.Search(ctx, "configuration", WithLimit(10), WithBudget(10))
	if err != nil {
		t.Fatalf("Search bounded: %v", err)
	}

	// Budget-constrained search should return fewer or equal results.
	if len(bounded) > len(unbounded) {
		t.Errorf("bounded results (%d) should not exceed unbounded results (%d)", len(bounded), len(unbounded))
	}

	// With a budget of 10 tokens, we should get at most a small number of results.
	if len(bounded) == 0 {
		t.Fatal("expected at least one result even with small budget")
	}

	if len(unbounded) > 1 && len(bounded) >= len(unbounded) {
		t.Errorf("expected budget to truncate results: bounded=%d, unbounded=%d", len(bounded), len(unbounded))
	}
}

// TestE2EFuzzyFallback exercises New -> Index -> Search for a typo -> verify fuzzy fallback.
func TestE2EFuzzyFallback(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	content := `# Kubernetes Deployment

Deploy applications using kubectl apply with manifests stored in the repository.
Use namespaces to isolate environments.

## Scaling

Configure horizontal pod autoscaler for production workloads.
`

	_, err = g.Index(ctx, content, WithSource("k8s.md"))
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Search with a typo: "kuberntes" instead of "kubernetes".
	results, err := g.Search(ctx, "kuberntes")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected fuzzy fallback to find results for typo 'kuberntes'")
	}

	// Verify the result came from the fuzzy layer.
	fuzzyFound := false
	for _, r := range results {
		if r.MatchLayer == "fuzzy" {
			fuzzyFound = true
			break
		}
	}
	if !fuzzyFound {
		layers := make([]string, len(results))
		for i, r := range results {
			layers[i] = r.MatchLayer
		}
		t.Errorf("expected at least one result with MatchLayer='fuzzy', got layers: %v", layers)
	}
}
