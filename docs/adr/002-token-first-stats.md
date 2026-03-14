# ADR 002: Surface Byte Savings Visibility

## Status
Accepted

## Date
2026-03-14

## Context

Gist tracks context reduction via BytesIndexed, BytesReturned, BytesSaved, and SavedPercent in the Stats struct. However, this information is only accessible through the `gist stats` CLI command or the `gist_stats` MCP tool. Individual search results do not report how many bytes they consumed, so callers cannot track per-query cost.

A token-based approach was considered but rejected. The 1-token-per-4-bytes heuristic is a rough approximation that varies by model and language. Presenting approximate token counts as authoritative would mislead users when their actual token counts differ. Bytes are the ground truth that gist measures directly.

## Decision

Keep bytes as the unit throughout the public API. Improve visibility by:

1. Add `BytesUsed` field to SearchResult so each result reports its snippet size in bytes.
2. Add a `savings` field to MCP gist_search responses summarizing bytes indexed vs returned for the query.
3. Improve CLI stats formatting with human-readable byte units (KB/MB) for large values.

No changes to the Stats struct field names or types. No token fields.

## Consequences

**Positive:**
- Byte counts are ground truth, not estimates. Users get accurate numbers.
- Per-result BytesUsed enables callers to track context consumption.
- No breaking changes to the Stats JSON schema.
- Users who want token estimates can divide by 4 themselves.

**Negative:**
- Bytes are less intuitive than tokens for users thinking about LLM costs.
- A real tokenizer integration may be needed later for precise token reporting.
