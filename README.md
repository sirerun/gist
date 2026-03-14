# Gist

**Context intelligence for LLM applications** — index everything, retrieve only what matters.

[![Go Reference](https://pkg.go.dev/badge/github.com/sirerun/gist.svg)](https://pkg.go.dev/github.com/sirerun/gist)
[![CI](https://github.com/sirerun/gist/actions/workflows/ci.yml/badge.svg)](https://github.com/sirerun/gist/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

## Overview

AI agents accumulate massive amounts of raw tool output — browser snapshots, API responses, log files — that rapidly fills context windows. Most of that content is never referenced again, but it still costs tokens, degrades reasoning quality, and shortens session lifetimes.

Gist solves this by sitting between your data and your LLM. Content is chunked and indexed, then retrieved on demand using a three-tier search engine that handles exact matches, partial terms, and typos. Callers set token budgets and get back only the most relevant snippets, ranked by relevance.

Gist is a Go library first: import it as a package, use it as a CLI, or run it as an MCP server. Zero CGO, single static binary, works on any platform. No external dependencies required — `gist.New()` uses an in-memory store by default, or connect PostgreSQL for production persistence.

## Installation

**Homebrew (macOS/Linux):**

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

## Quick Start (Zero Dependencies)

No PostgreSQL needed — get started with three lines of setup:

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

	// No arguments: uses an in-memory store (no PostgreSQL required).
	g, err := gist.New()
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()

	// Index content with a source label.
	result, err := g.Index(ctx, "Gist chunks and indexes content for LLM retrieval.", gist.WithSource("intro"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Indexed %d chunks\n", result.TotalChunks)

	// Search with a token budget.
	results, err := g.Search(ctx, "LLM retrieval", gist.WithBudget(2000))
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Printf("[%s] score=%.2f\n%s\n", r.Source, r.Score, r.Snippet)
	}
}
```

## Quick Start (PostgreSQL)

For persistent, production-grade search, connect to PostgreSQL:

```go
g, err := gist.New(gist.WithPostgres("postgres://localhost:5432/gist"))
if err != nil {
	log.Fatal(err)
}
defer g.Close()

// Index and search exactly as above — the API is identical.
```

PostgreSQL enables `tsvector` full-text search and `pg_trgm` trigram matching. The in-memory store provides the same three-tier search using pure Go, suitable for testing, prototyping, and small workloads.

## CLI Usage

The `--dsn` flag (or `GIST_DSN` environment variable) is optional. When omitted, the CLI uses an in-memory store — data does not persist across runs.

```sh
# In-memory (no setup required)
gist index README.md docs/*.md --format markdown
gist search "connection pool" --limit 10 --budget 4000

# With PostgreSQL (persistent storage)
export GIST_DSN="postgres://localhost:5432/gist"
gist index README.md docs/*.md --format markdown
gist search "connection pool" --limit 10 --budget 4000

# View indexing and search statistics
gist stats

# Run performance benchmarks
gist bench --docs 500 --searches 200

# Check runtime environment and dependencies
gist doctor

# Start MCP server over stdio
gist serve
```

## MCP Server

`gist serve` exposes Gist as an MCP tool provider over stdio, compatible with any MCP client. It provides three tools:

- **gist_index** — Index content with optional source label and format
- **gist_search** — Search indexed content with query, limit, source filter, and token budget
- **gist_stats** — Return indexing and search statistics

Add it to your MCP client configuration:

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

## Features

- **Three-tier search** — Porter stemming, trigram substring matching, and fuzzy correction with automatic fallback
- **Budget-aware retrieval** — Set token budgets and get results that fit, ranked by BM25 relevance
- **Structured chunking** — Heading-aware Markdown chunking that preserves code blocks, plus JSON, YAML, and plain text support
- **Batch indexing** — Concurrent indexing with configurable goroutine pools
- **MCP server** — Expose as an MCP tool provider for any AI agent
- **Zero CGO** — Pure Go, single static binary, cross-platform
- **In-memory or PostgreSQL** — Works out of the box with no dependencies; connect PostgreSQL for persistent, production-grade full-text and trigram search

## API Reference

See the full API documentation on [pkg.go.dev](https://pkg.go.dev/github.com/sirerun/gist).

## Contributing

Contributions are welcome. Please open an issue to discuss your idea before submitting a pull request.

1. Fork the repository
2. Create a feature branch
3. Run tests: `go test ./...`
4. Submit a pull request

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.
