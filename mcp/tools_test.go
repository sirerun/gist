package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirerun/gist"
)

func TestHandleIndex(t *testing.T) {
	tests := []struct {
		name    string
		args    indexArgs
		wantErr string
	}{
		{
			name:    "missing content",
			args:    indexArgs{},
			wantErr: "content is required",
		},
		{
			name:    "invalid format",
			args:    indexArgs{Content: "hello", Format: "xml"},
			wantErr: "unknown format",
		},
		{
			name: "valid with defaults",
			args: indexArgs{Content: "# Hello\nWorld"},
		},
		{
			name: "valid with source and format",
			args: indexArgs{Content: "some text", Source: "test.md", Format: "markdown"},
		},
		{
			name: "plaintext format",
			args: indexArgs{Content: "plain content", Format: "plaintext"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newTestGist(t)
			defer g.Close()

			argsJSON, _ := json.Marshal(tt.args)
			result, err := handleIndex(context.Background(), g, argsJSON)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			ir, ok := result.(*gist.IndexResult)
			if !ok {
				t.Fatalf("result type = %T, want *gist.IndexResult", result)
			}
			if ir.TotalChunks == 0 {
				t.Error("expected at least one chunk")
			}
		})
	}
}

func TestHandleSearch(t *testing.T) {
	tests := []struct {
		name    string
		args    searchArgs
		wantErr string
	}{
		{
			name:    "missing query",
			args:    searchArgs{},
			wantErr: "query is required",
		},
		{
			name: "valid query",
			args: searchArgs{Query: "hello"},
		},
		{
			name: "with options",
			args: searchArgs{Query: "hello", Limit: 3, Source: "test", Budget: 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newTestGist(t)
			defer g.Close()

			// Index some content first so search has something to find.
			g.Index(context.Background(), "hello world", gist.WithSource("test"))

			argsJSON, _ := json.Marshal(tt.args)
			_, err := handleSearch(context.Background(), g, argsJSON)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleStats(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()

	result, err := handleStats(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats, ok := result.(*gist.Stats)
	if !ok {
		t.Fatalf("result type = %T, want *gist.Stats", result)
	}

	if stats.SourceCount != 0 {
		t.Errorf("expected 0 sources, got %d", stats.SourceCount)
	}
}

func TestToolsCallIndex(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()
	s := NewServer(g)

	resp := sendToolCall(t, s, "gist_index", map[string]any{
		"content": "# Test\nHello world",
		"source":  "test.md",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result := extractToolResult(t, resp)
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
}

func TestToolsCallSearch(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()

	// Index first.
	g.Index(context.Background(), "hello world content", gist.WithSource("test"))

	s := NewServer(g)
	resp := sendToolCall(t, s, "gist_search", map[string]any{
		"query": "hello",
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result := extractToolResult(t, resp)
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	// Parse the JSON and verify searchResponse wrapper fields.
	var sr searchResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &sr); err != nil {
		t.Fatalf("unmarshal searchResponse: %v", err)
	}
	if len(sr.Results) == 0 {
		t.Fatal("expected at least one search result")
	}
	var sum int
	for _, r := range sr.Results {
		if r.BytesUsed <= 0 {
			t.Errorf("expected BytesUsed > 0, got %d", r.BytesUsed)
		}
		sum += r.BytesUsed
	}
	if sr.BytesUsed != sum {
		t.Errorf("bytes_used = %d, want sum of results %d", sr.BytesUsed, sum)
	}
}

func TestHandleSearchBytesUsed(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()

	// Index content so search has something to find.
	_, err := g.Index(context.Background(), "hello world testing bytes used", gist.WithSource("bytes-test"))
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	argsJSON, _ := json.Marshal(searchArgs{Query: "hello"})
	raw, err := handleSearch(context.Background(), g, argsJSON)
	if err != nil {
		t.Fatalf("handleSearch: %v", err)
	}

	sr, ok := raw.(searchResponse)
	if !ok {
		t.Fatalf("result type = %T, want searchResponse", raw)
	}

	if len(sr.Results) == 0 {
		t.Fatal("expected at least one search result")
	}

	var sum int
	for i, r := range sr.Results {
		if r.BytesUsed <= 0 {
			t.Errorf("result[%d].BytesUsed = %d, want > 0", i, r.BytesUsed)
		}
		sum += r.BytesUsed
	}

	if sr.BytesUsed != sum {
		t.Errorf("BytesUsed = %d, want sum of results %d", sr.BytesUsed, sum)
	}
	if sr.BytesUsed <= 0 {
		t.Error("expected total BytesUsed > 0")
	}
}

func TestToolsCallStats(t *testing.T) {
	g := newTestGist(t)
	defer g.Close()
	s := NewServer(g)

	resp := sendToolCall(t, s, "gist_stats", map[string]any{})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result := extractToolResult(t, resp)
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	// Parse the stats JSON from the content.
	var stats gist.Stats
	if err := json.Unmarshal([]byte(result.Content[0].Text), &stats); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    gist.Format
		wantErr bool
	}{
		{"markdown", gist.FormatMarkdown, false},
		{"md", gist.FormatMarkdown, false},
		{"json", gist.FormatJSON, false},
		{"plaintext", gist.FormatPlainText, false},
		{"plain", gist.FormatPlainText, false},
		{"text", gist.FormatPlainText, false},
		{"MARKDOWN", gist.FormatMarkdown, false},
		{"xml", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseFormat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// sendToolCall is a helper that sends a tools/call request.
func sendToolCall(t *testing.T, s *Server, name string, args map[string]any) jsonrpcResponse {
	t.Helper()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
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
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

// extractToolResult extracts the mcpToolResult from a response.
func extractToolResult(t *testing.T, resp jsonrpcResponse) mcpToolResult {
	t.Helper()
	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result mcpToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	return result
}
