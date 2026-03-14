# Gist

**Context intelligence for AI agents** — index everything, retrieve only what matters.

[![Go Reference](https://pkg.go.dev/badge/github.com/sirerun/gist.svg)](https://pkg.go.dev/github.com/sirerun/gist)
[![CI](https://github.com/sirerun/gist/actions/workflows/ci.yml/badge.svg)](https://github.com/sirerun/gist/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

## The Problem

Every tool call in an agent pipeline dumps raw output into the context window. A browser snapshot is 56 KB. Twenty GitHub issues are 59 KB. A log file is 200 KB. After thirty minutes of real work, the context window is dominated by stale output the model will never reference again.

This creates three compounding failures:

- **Cost climbs.** Wasted input tokens are wasted dollars. Agents that run continuously become expensive fast.
- **Quality degrades.** Relevant context gets pushed out by irrelevant output. The model loses track of what matters.
- **Sessions die early.** Workflows that should run for hours hit token limits in minutes.

Context window bloat is the single largest bottleneck to running AI agents at scale. Not model capability. Not tool availability. Context.

## The Solution

Gist sits between your data and your LLM. Content is chunked and indexed, then retrieved on demand using a three-tier search engine that handles exact matches, partial terms, and typos. Set a token budget, get back only what matters.

```go
g, _ := gist.New()
g.Index(ctx, doc, gist.WithSource("api-spec"))
results, _ := g.Search(ctx, "authentication flow", gist.WithBudget(4000))
```

Index once. Search as many times as you need. Gist handles chunking, ranking, and budget fitting — so your agent gets the context it needs without the noise it doesn't.

## Quick Start

No setup required. `gist.New()` works out of the box with an in-memory store.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sirerun/gist"
)

func main() {
	ctx := context.Background()

	g, err := gist.New()
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()

	// Index content from any source.
	g.Index(ctx, "## Connection Pool\n\nSet max_connections to 100 for production.",
		gist.WithSource("config-guide"))

	g.Index(ctx, "## Authentication\n\nUse OAuth 2.0 with PKCE for public clients.",
		gist.WithSource("security-guide"))

	// Search across all indexed content.
	results, _ := g.Search(ctx, "connection settings", gist.WithBudget(2000))
	for _, r := range results {
		fmt.Printf("[%s] %s\n", r.Source, r.Snippet)
	}
}
```

For persistent, production-grade search, connect PostgreSQL:

```go
g, err := gist.New(gist.WithPostgres("postgres://localhost:5432/gist"))
```

PostgreSQL enables `tsvector` full-text search and `pg_trgm` trigram matching. The API is identical — switch storage backends without changing application code.

## Installation

**Homebrew:**

```sh
brew install sirerun/tap/gist
```

**Go install:**

```sh
go install github.com/sirerun/gist/cmd/gist@latest
```

**Library:**

```sh
go get github.com/sirerun/gist
```

## CLI

The CLI works immediately with no configuration. When `--dsn` is omitted, it uses an in-memory store.

```sh
# Index files — Markdown, JSON, YAML, or plain text
gist index README.md docs/*.md --format markdown

# Search across indexed content
gist search "connection pool" --limit 10 --budget 4000

# View indexing and search statistics
gist stats

# Run performance benchmarks
gist bench --docs 500 --searches 200

# Check runtime environment and dependencies
gist doctor
```

For persistent storage, set `GIST_DSN` or pass `--dsn`:

```sh
export GIST_DSN="postgres://localhost:5432/gist"
gist index README.md
gist search "authentication"
```

## MCP Server

`gist serve` exposes Gist as an MCP tool provider over stdio, compatible with any MCP client:

```json
{
  "mcpServers": {
    "gist": {
      "command": "gist",
      "args": ["serve", "--dsn", "postgres://localhost:5432/gist"]
    }
  }
}
```

Tools provided:

| Tool | Description |
|------|-------------|
| `gist_index` | Index content with source label and format |
| `gist_search` | Search with query, limit, source filter, and token budget |
| `gist_stats` | Return indexing and search statistics |

## Use Cases

**Long-running agent workflows.** Agents that operate for hours accumulate massive context. Gist indexes tool outputs as they arrive and retrieves only what's relevant for the current step, keeping the agent within its token budget.

**Multi-document search.** Index codebases, documentation, API specs, and log files. Search across all of them with a single query. Gist's three-tier search handles exact terms, partial matches, and misspellings.

**Token budget management.** Set a budget with `WithBudget(4000)` and Gist returns the most relevant results that fit. No manual truncation. No hoping the important parts survive.

**Batch processing.** Index hundreds of documents concurrently with `BatchIndex` and configurable goroutine pools. Process large codebases or document collections without blocking.

## How Search Works

Gist uses a three-tier fallback strategy to find the best results for any query:

1. **Porter stemming** — full-text search that matches word roots. "configuring" matches "configuration."
2. **Trigram matching** — substring search for partial terms. "conn_pool" finds "connection_pooling."
3. **Fuzzy correction** — Levenshtein distance for typos. "authetication" finds "authentication."

Each tier fires only if the previous one returns no results. This means exact queries are fast, and fuzzy queries still work.

## Features

| Feature | Description |
|---------|-------------|
| Three-tier search | Porter stemming, trigram matching, and fuzzy correction with automatic fallback |
| Budget-aware retrieval | Set token budgets — results are ranked and truncated to fit |
| Structured chunking | Heading-aware Markdown, JSON, YAML, and plain text chunking that preserves code blocks |
| Batch indexing | Concurrent indexing with configurable goroutine pools |
| Session tracking | Track indexed content and searches across long-running workflows |
| MCP server | Expose Gist as an MCP tool provider for any AI agent |
| In-memory or PostgreSQL | Works instantly with no dependencies; connect PostgreSQL for production persistence |
| Zero CGO | Pure Go, single static binary, cross-platform |

## The Mint Pattern

Gist is part of the open infrastructure layer for AI agent development from [Sire](https://sire.run), alongside [Mint](https://github.com/sirerun/mint).

| Project | What it does |
|---------|-------------|
| [Mint](https://github.com/sirerun/mint) | Generate MCP servers from OpenAPI specs — connect agents to any API |
| [Gist](https://github.com/sirerun/gist) | Index and retrieve context intelligently — keep results within token budgets |

Mint connects your agents to APIs. Gist makes sure the results fit in the context window. Both are Apache 2.0 licensed, zero-CGO Go projects that compile to a single static binary.

## API Reference

Full documentation is on [pkg.go.dev](https://pkg.go.dev/github.com/sirerun/gist).

**Core API:**

```go
// Create a Gist instance (in-memory by default, or with PostgreSQL).
g, err := gist.New()
g, err := gist.New(gist.WithPostgres(dsn))
g, err := gist.New(gist.WithMemory())

// Index content with options.
result, err := g.Index(ctx, content, gist.WithSource("label"), gist.WithFormat(gist.FormatMarkdown))

// Batch index multiple items concurrently.
results, err := g.BatchIndex(ctx, items, gist.WithConcurrency(8))

// Search with options.
results, err := g.Search(ctx, "query", gist.WithLimit(10), gist.WithBudget(4000))

// Get statistics.
stats := g.Stats()
```

## Contributing

Contributions are welcome. Please open an issue to discuss your idea before submitting a pull request.

1. Fork the repository
2. Create a feature branch
3. Run tests: `GOWORK=off go test ./... -race`
4. Submit a pull request

If you're building AI agents and fighting context limits, give Gist a try. Star the repo to follow along, file issues, or open PRs.

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.
