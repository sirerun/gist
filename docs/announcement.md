# Gist: Context Intelligence for AI Agents

AI agents are getting more capable every month. The bottleneck is no longer what they can do — it is what they can remember.

## The Problem

Every tool call in an agent pipeline dumps raw output into the context window. A browser snapshot is 56 KB. Twenty GitHub issues are 59 KB. A log file is 200 KB. After thirty minutes of real work, over 40% of the context window is consumed by stale output the model will never reference again.

This creates three compounding failures:

- **Cost climbs.** Wasted input tokens are wasted dollars. Agents that run continuously become expensive fast.
- **Quality degrades.** Relevant context gets pushed out by irrelevant output. The model loses track of what matters.
- **Sessions die early.** Workflows that should run for hours hit token limits in minutes.

Context window bloat is the single largest bottleneck to running AI agents at scale. Not model capability. Not tool availability. Context.

## Introducing Gist

[Gist](https://github.com/sirerun/gist) is a Go library and CLI that indexes everything and retrieves only what matters. Import it as a package, run it from the command line, or use it as an MCP server.

```go
g, _ := gist.New(gist.WithPostgres(dsn))
g.Index(ctx, doc, gist.WithSource("api-spec"))
results, _ := g.Search(ctx, "authentication flow")
fmt.Println(results[0].Snippet)
```

Index your content once. Search it as many times as you need. Gist handles chunking, ranking, and fitting results within your token budget — so your agent gets the context it needs without the noise it does not.

### What Gist Does

- **Index any content.** Markdown, JSON, YAML, plain text. Gist chunks content intelligently — preserving code blocks, respecting heading hierarchy, maintaining structure.

- **Search that handles real queries.** Three-tier search covers exact matches, substring and partial terms, and typo correction. Misspell a function name? Gist still finds it.

- **Budget-aware retrieval.** Set a token budget. Gist returns the most relevant results that fit, ranked by relevance. No more manually truncating outputs or hoping the important parts survive.

- **Session tracking.** Long-running agent workflows accumulate context across dozens of tool calls. Gist tracks what has been indexed and searched across a session, so agents can pick up where they left off.

- **Use it however you want.** Import `github.com/sirerun/gist` as a Go library. Run `gist search` from the CLI. Or start `gist serve` to expose it as an MCP server over stdio. Same engine, three interfaces.

## The Mint Pattern

Gist is the second open-source project from [Sire](https://sire.run), following the same playbook as [Mint](https://github.com/sirerun/mint).

Mint generates MCP servers from OpenAPI specs — point it at any API and get a working MCP server. Gist manages the context those servers produce. Together, they form the open infrastructure layer for AI agent development:

- **Mint** connects your agents to any API.
- **Gist** makes sure the results fit in the context window.

Both are Apache 2.0 licensed, zero-CGO Go projects that compile to a single static binary.

## Getting Started

Install the CLI:

```bash
go install github.com/sirerun/gist/cmd/gist@latest
```

Or add the library to your project:

```bash
go get github.com/sirerun/gist
```

Index a file and search it:

```bash
gist index README.md
gist search "getting started"
```

The full API is documented on [pkg.go.dev](https://pkg.go.dev/github.com/sirerun/gist).

## What's Next

Gist is open source and we are building it in the open. We welcome contributions — whether that is new content formats, storage backends, performance improvements, or bug reports.

If you are building AI agents and fighting context limits, give Gist a try. File issues, open PRs, or just star the repo to follow along.

## Links

- **GitHub:** [github.com/sirerun/gist](https://github.com/sirerun/gist)
- **Go Package:** [pkg.go.dev/github.com/sirerun/gist](https://pkg.go.dev/github.com/sirerun/gist)
- **Mint:** [github.com/sirerun/mint](https://github.com/sirerun/mint)
- **License:** Apache 2.0
