package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirerun/gist"
)

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"inputSchema"`
}

// Schema represents a JSON Schema for tool input.
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property represents a JSON Schema property.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolDefinitions returns the list of MCP tool definitions.
func ToolDefinitions() []Tool {
	return []Tool{
		{
			Name:        "gist_index",
			Description: "Index content for later search. Chunks and stores content with optional source label and format.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"content": {Type: "string", Description: "The content to index"},
					"source":  {Type: "string", Description: "Source label (e.g., file path)"},
					"format":  {Type: "string", Description: "Content format: markdown, json, or plaintext"},
				},
				Required: []string{"content"},
			},
		},
		{
			Name:        "gist_search",
			Description: "Search indexed content using three-tier search (porter stemming, trigram, fuzzy correction). Returns results with bytes_used per result and total.",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]Property{
					"query":  {Type: "string", Description: "Search query"},
					"limit":  {Type: "integer", Description: "Maximum number of results (default 5)"},
					"source": {Type: "string", Description: "Filter results to a specific source"},
					"budget": {Type: "integer", Description: "Token budget for results"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "gist_stats",
			Description: "Return indexing and search statistics.",
			InputSchema: Schema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
	}
}

// dispatchTool routes a tool call to the appropriate handler.
func dispatchTool(ctx context.Context, g *gist.Gist, name string, args json.RawMessage) (any, error) {
	switch name {
	case "gist_index":
		return handleIndex(ctx, g, args)
	case "gist_search":
		return handleSearch(ctx, g, args)
	case "gist_stats":
		return handleStats(g)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// indexArgs are the arguments for gist_index.
type indexArgs struct {
	Content string `json:"content"`
	Source  string `json:"source"`
	Format  string `json:"format"`
}

func handleIndex(ctx context.Context, g *gist.Gist, args json.RawMessage) (any, error) {
	var a indexArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if a.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	var opts []gist.IndexOption
	if a.Source != "" {
		opts = append(opts, gist.WithSource(a.Source))
	}
	if a.Format != "" {
		f, err := parseFormat(a.Format)
		if err != nil {
			return nil, err
		}
		opts = append(opts, gist.WithFormat(f))
	}

	return g.Index(ctx, a.Content, opts...)
}

// searchArgs are the arguments for gist_search.
type searchArgs struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Source string `json:"source"`
	Budget int    `json:"budget"`
}

// searchResponse wraps search results with total bytes used.
type searchResponse struct {
	Results   []gist.SearchResult `json:"results"`
	BytesUsed int                 `json:"bytes_used"`
}

func handleSearch(ctx context.Context, g *gist.Gist, args json.RawMessage) (any, error) {
	var a searchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if a.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	var opts []gist.SearchOption
	if a.Limit > 0 {
		opts = append(opts, gist.WithLimit(a.Limit))
	}
	if a.Source != "" {
		opts = append(opts, gist.WithSourceFilter(a.Source))
	}
	if a.Budget > 0 {
		opts = append(opts, gist.WithBudget(a.Budget))
	}

	results, err := g.Search(ctx, a.Query, opts...)
	if err != nil {
		return nil, err
	}

	if results == nil {
		results = []gist.SearchResult{}
	}

	var totalBytesUsed int
	for _, r := range results {
		totalBytesUsed += r.BytesUsed
	}

	return searchResponse{
		Results:   results,
		BytesUsed: totalBytesUsed,
	}, nil
}

func handleStats(g *gist.Gist) (any, error) {
	return g.Stats(), nil
}

// parseFormat converts a string to a gist.Format.
func parseFormat(s string) (gist.Format, error) {
	switch strings.ToLower(s) {
	case "markdown", "md":
		return gist.FormatMarkdown, nil
	case "json":
		return gist.FormatJSON, nil
	case "plaintext", "plain", "text":
		return gist.FormatPlainText, nil
	default:
		return 0, fmt.Errorf("unknown format: %s (use markdown, json, or plaintext)", s)
	}
}
