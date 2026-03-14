package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirerun/gist"
)

// mockStore implements gist.Store for testing without a database.
type mockStore struct {
	sources []gist.Source
	chunks  []gist.Chunk
	nextSrc int
	nextChk int
}

func (m *mockStore) SaveSource(_ context.Context, label string, format gist.Format) (gist.Source, error) {
	m.nextSrc++
	s := gist.Source{ID: m.nextSrc, Label: label, Format: format}
	m.sources = append(m.sources, s)
	return s, nil
}

func (m *mockStore) SaveChunk(_ context.Context, chunk gist.Chunk) (gist.Chunk, error) {
	m.nextChk++
	chunk.ID = m.nextChk
	m.chunks = append(m.chunks, chunk)
	return chunk, nil
}

func (m *mockStore) SearchPorter(_ context.Context, params gist.SearchParams) ([]gist.SearchMatch, error) {
	return m.search(params, "porter"), nil
}

func (m *mockStore) SearchTrigram(_ context.Context, params gist.SearchParams) ([]gist.SearchMatch, error) {
	return m.search(params, "trigram"), nil
}

func (m *mockStore) search(params gist.SearchParams, layer string) []gist.SearchMatch {
	var results []gist.SearchMatch
	for _, c := range m.chunks {
		if strings.Contains(strings.ToLower(c.Content), strings.ToLower(params.Query)) {
			results = append(results, gist.SearchMatch{
				ChunkID:     c.ID,
				SourceID:    c.SourceID,
				HeadingPath: c.HeadingPath,
				Content:     c.Content,
				ContentType: c.ContentType,
				Score:       1.0,
				MatchLayer:  layer,
			})
		}
		if len(results) >= params.Limit {
			break
		}
	}
	return results
}

func (m *mockStore) VocabularyTerms(_ context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockStore) Sources(_ context.Context) ([]gist.Source, error) {
	return m.sources, nil
}

func (m *mockStore) Stats(_ context.Context) (gist.StoreStats, error) {
	return gist.StoreStats{
		ChunkCount:   len(m.chunks),
		SourceCount:  len(m.sources),
		BytesIndexed: 0,
	}, nil
}

func (m *mockStore) Close() error { return nil }

func newTestGist(t *testing.T) *gist.Gist {
	t.Helper()
	g, err := gist.New(gist.WithStore(&mockStore{}))
	if err != nil {
		t.Fatalf("creating test gist: %v", err)
	}
	return g
}

func sendRequest(t *testing.T, s *Server, method string, id any, params any) jsonrpcResponse {
	t.Helper()

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if id != nil {
		req["id"] = id
	}
	if params != nil {
		req["params"] = params
	}

	line, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	line = append(line, '\n')

	var out bytes.Buffer
	s.reader = bytes.NewReader(line)
	s.writer = &out

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("serve error: %v", err)
	}

	var resp jsonrpcResponse
	if out.Len() == 0 {
		return resp
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshaling response %q: %v", out.String(), err)
	}
	return resp
}

func TestJSONRPCParsing(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError int
	}{
		{
			name:      "invalid json",
			input:     "not json\n",
			wantError: codeParseError,
		},
		{
			name:      "empty line is skipped",
			input:     "\n",
			wantError: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newTestGist(t)
			defer g.Close()
			s := NewServer(g)

			var out bytes.Buffer
			s.reader = strings.NewReader(tt.input)
			s.writer = &out

			s.Serve(context.Background())

			if tt.wantError == 0 {
				if out.Len() != 0 {
					t.Errorf("expected no output, got %q", out.String())
				}
				return
			}

			var resp jsonrpcResponse
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Error == nil {
				t.Fatal("expected error response")
			}
			if resp.Error.Code != tt.wantError {
				t.Errorf("error code = %d, want %d", resp.Error.Code, tt.wantError)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()
	s := NewServer(g)

	resp := sendRequest(t, s, "initialize", 1, nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var result initializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal init result: %v", err)
	}

	if result.ServerInfo.Name != "gist" {
		t.Errorf("server name = %q, want %q", result.ServerInfo.Name, "gist")
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability")
	}
	if result.ProtocolVersion == "" {
		t.Error("expected protocol version")
	}
}

func TestInitializedNotification(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()
	s := NewServer(g)

	// initialized is a notification (no id) — should produce no response.
	resp := sendRequest(t, s, "initialized", nil, nil)
	if resp.JSONRPC != "" {
		t.Error("expected no response for initialized notification")
	}
}

func TestUnknownMethod(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()
	s := NewServer(g)

	resp := sendRequest(t, s, "nonexistent/method", 1, nil)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != codeMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, codeMethodNotFound)
	}
}

func TestToolsList(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()
	s := NewServer(g)

	resp := sendRequest(t, s, "tools/list", 1, nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	wantNames := map[string]bool{
		"gist_index":  true,
		"gist_search": true,
		"gist_stats":  true,
	}

	if len(result.Tools) != len(wantNames) {
		t.Fatalf("got %d tools, want %d", len(result.Tools), len(wantNames))
	}

	for _, tool := range result.Tools {
		if !wantNames[tool.Name] {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
	}
}

func TestToolsCallUnknown(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()
	s := NewServer(g)

	resp := sendRequest(t, s, "tools/call", 1, map[string]any{
		"name":      "nonexistent",
		"arguments": map[string]any{},
	})

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result mcpToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("expected isError=true for unknown tool")
	}
}
