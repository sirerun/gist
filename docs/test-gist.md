# Gist MCP Integration Test Plan

Run these steps in a fresh Claude Code session to verify gist works end to end.

## Prerequisites

```bash
brew install sirerun/tap/gist   # or brew upgrade sirerun/tap/gist
gist setup claude
```

Restart Claude Code after running setup so it picks up the MCP server.

## Test 1: Verify tools are available

Ask Claude Code:

> What gist tools do you have available?

**Expected**: Claude lists `gist_index`, `gist_search`, and `gist_stats`.

## Test 2: Index a large file

Ask Claude Code:

> Read the file `gist.go` and index it with gist.

**Expected**: Claude reads the file, calls `gist_index` with the content and source label `gist.go`, and confirms indexing (reports chunk count).

## Test 3: Search indexed content

Ask Claude Code:

> Use gist to find how SearchResult is defined.

**Expected**: Claude calls `gist_search` with a query like "SearchResult" and returns a snippet showing the struct definition without re-reading the file.

## Test 4: Index multiple files then search across them

Ask Claude Code:

> Read and index these files: `search.go`, `chunk.go`, `store.go`. Then search gist for "how does fuzzy matching work".

**Expected**: Claude indexes all three files, then searches and returns relevant snippets from whichever file(s) mention fuzzy/Levenshtein matching.

## Test 5: Budget-constrained search

Ask Claude Code:

> Search gist for "store" with a budget of 500 tokens.

**Expected**: Claude calls `gist_search` with `budget: 500` and the results fit within a 500-token budget. Note that `budget` is denominated in tokens (approximately 4 characters per token), not bytes, so `bytes_used` may be up to ~2000 bytes.

## Test 6: Stats reporting

Ask Claude Code:

> Call gist_stats and tell me how much context was saved.

**Expected**: Claude calls `gist_stats` and reports bytes indexed, bytes returned, bytes saved, saved percentage, source count, chunk count, and search count. After the previous tests, bytes_saved should be positive and saved_percent should be well above 0%.

## Test 7: Source-filtered search

Ask Claude Code:

> Search gist for "Index" but only in store.go.

**Expected**: Claude calls `gist_search` with `source: "store.go"` filter and results come only from that file.

## Test 8: Automatic behavior

This tests that the CLAUDE.md instructions are working. Ask Claude Code:

> Explain the three-tier search strategy in this codebase.

**Expected**: Claude reads `search.go` (which is over 5KB), automatically indexes it via `gist_index`, then answers the question. If it reads additional files, it should index those too. At the end of the response or shortly after, Claude should report gist stats.

## Pass criteria

All 8 tests pass. The critical signal is that Claude Code can call `gist_index`, `gist_search`, and `gist_stats` without errors, and that search returns relevant snippets from previously indexed content.
