package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/hosted/internal/directorymcp"
	"github.com/sirerun/gist/hosted/internal/semantic"
)

type hostedTestEmbedder struct{}

func (hostedTestEmbedder) ModelVersion() string { return "fixture-768" }
func (hostedTestEmbedder) Embed(_ context.Context, q preparation.EmbeddingRequest) (preparation.EmbeddingResult, error) {
	r := preparation.EmbeddingResult{ModelVersion: "fixture-768"}
	for range q.Texts {
		v := make([]float64, 768)
		v[0] = 1
		r.Vectors = append(r.Vectors, v)
	}
	return r, nil
}
func TestHostedDirectoryAndMCPShareSemanticSearch(t *testing.T) {
	index, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := directory.DefaultApps(index)
	if err != nil {
		t.Fatal(err)
	}
	search := semantic.New(index, apps, hostedTestEmbedder{}, nil)
	if err = search.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := &directoryAPI{index: index, apps: apps, search: search, origin: "https://gist.sire.run"}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/directory?q=find+leads+for+my+startup&limit=5", nil))
	var result directory.PublicResult
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil || response.Header().Get("X-Gist-Search-Mode") != "hybrid" || len(result.Items) == 0 {
		t.Fatalf("REST %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `"search_mode"`) {
		t.Fatal("broke strict existing JSON readers")
	}
	// Exact same service and query, through the actual MCP boundary.
	mcp := directorymcp.New(index, "https://gist.sire.run", 2).WithSearch(search)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"gist_directory_search","arguments":{"query":"find leads for my startup","limit":5}}}`
	request := httptest.NewRequest("POST", "/directory/mcp", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response = httptest.NewRecorder()
	mcp.ServeHTTP(response, request)
	if response.Code != 200 || response.Header().Get("X-Gist-Search-Mode") != "hybrid" || !strings.Contains(response.Body.String(), result.Items[0].ID) {
		t.Fatalf("MCP %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/directory/apps?q=find+leads+for+my+startup", nil))
	var appResult directory.AppResult
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &appResult) != nil || response.Header().Get("X-Gist-Search-Mode") != "hybrid" || len(appResult.Items) == 0 {
		t.Fatalf("Library apps %d %s", response.Code, response.Body.String())
	}
}
