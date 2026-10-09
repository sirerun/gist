# Plan: Gist repository delivery

Plan ID: gist:delivery
Authority ID: github:sirerun/gist
Authoritative source: docs/plan.md. Normalized JSON is derived interchange only.

## Conformance repair -- 2026-10-08

This single-file plan consolidates all 266 existing tasks from main ad6b3fa, including 18 historical March tasks and 248 active registry tasks. IDs, checkboxes, owners, canonical dependencies, existing acceptance and authored evidence are retained. Missing 31 stages and 18 acceptance annotations are recovered from the original descriptions; Historical prerequisites omitted by the local parser are made explicit from the original task bullets; 74 cross-epic metadata references use unique local IDs without changing their canonical targets. Provenance, preserved source digests and reader bindings are in the [repair receipt](receipts/2026-10-08-plan-conformance-repair.md). Checkbox completion remains authored status, not authenticated evidence or runtime admission.

Gist remains an optional registry for Zatiti, with independently governed direct MCP and no demo dependency. ADR012 v2 publication implementation is owner-approved; frozen v1 stays unchanged with an explicit migration path. Registry/caller-owned production at https://gist.sire.run remains the terminal goal, gated by identity, provider, actual-consumer, release and AWS authority/evidence.

Ordinary code stages use supported bindings. Explicit release/deploy stages preserve actual operational meaning and are blocked in generic agent dispatch until a qualified scoped operational binding supplies the required authority; they must never be treated as code implementation. Active coding/checks/workers run on qualified DGX resources under the existing shared dispatch queue/cooldown. No provider/runtime/deployment authority comes from conformance.

## Historical and retained delivery record

## Historical plan: Gist -- Multi-Tool Setup, Byte Savings Visibility, and README Update

**Date:** 2026-03-14
**Prior work:** See `docs/design.md` for completed Phases 1-6.

## Context

### Problem Statement

Three usability gaps exist:

1. **Setup friction**: Integrating gist with an agentic coding tool (Claude Code, Gemini CLI, Cursor, VS Code Copilot, Codex CLI) requires manually editing two files per tool: an MCP server config and a system instructions file. Each tool has different file locations, JSON schemas, and instruction file formats. A `gist setup <tool>` subcommand should automate this into a single invocation per tool.

2. **Savings visibility**: Gist tracks context reduction via BytesIndexed, BytesReturned, BytesSaved, and SavedPercent in the Stats struct, but this data is underexposed. Individual search results do not report their byte footprint, the CLI stats output uses raw integers, and MCP search responses do not include savings information.

3. **README does not document setup or agentic integration**: The README documents the library API, CLI, and MCP server, but does not explain how to integrate gist with agentic coding tools. Users must discover the `gist setup` subcommand, the one-line install-and-configure workflow, and which tools are supported. The README should be the primary onboarding surface for agentic tool users.

### Tool Configuration Landscape

| Tool | MCP Config Path (Global) | MCP Key | Instructions File | Installed |
|------|--------------------------|---------|-------------------|-----------|
| Claude Code | `~/.claude/mcp.json` | `mcpServers` | `~/.claude/CLAUDE.md` | Yes |
| Gemini CLI | `~/.gemini/mcp.json` | `mcpServers` | `~/.gemini/GEMINI.md` | Yes |
| Cursor | `~/.cursor/mcp.json` | `mcpServers` | `~/.cursor/rules/gist.mdc` | No |
| VS Code Copilot | `~/.vscode/mcp.json` | `servers` | `~/.github/copilot-instructions.md` | No |
| Codex CLI | `~/.codex/config.toml` | TOML `[mcp_servers]` | N/A (no instructions file) | No |

### Objectives

1. Add `gist setup <tool>` subcommand supporting claude, gemini, cursor, copilot, codex.
2. Use an adapter pattern: each tool is a struct defining paths and formats.
3. Support `--global` (default), `--project`, `--uninstall`, and `--dry-run` flags.
4. Make setup idempotent.
5. Add BytesUsed to SearchResult.
6. Improve CLI stats formatting with human-readable byte units.
7. Add savings summary to MCP gist_search responses.
8. Update README to document agentic tool integration, `gist setup`, and the one-line install workflow.

### Non-Goals

- Interactive wizard or TUI.
- `gist setup --all`.
- Auto-detecting which tools are installed.
- Token-based metrics.
- Dollar-based cost estimation.
- Changing Stats struct fields or the Store interface.

### Constraints

- Zero new external dependencies.
- No breaking changes to existing public API.
- The setup subcommand must skip PersistentPreRunE (no database initialization).
- GOWORK=off for all go commands.
- Instructions content is the same across all tools.
- README changes must not remove or contradict existing content. Add new sections and update existing ones.

### Success Metrics

| Metric | Target |
|--------|--------|
| `gist setup claude` configures Claude Code | Yes |
| `gist setup gemini` configures Gemini CLI | Yes |
| Setup is idempotent for all supported tools | Yes |
| `--uninstall` cleanly removes config | Yes |
| `--dry-run` shows changes without writing | Yes |
| `gist setup` (no tool) prints supported tool list | Yes |
| SearchResult includes BytesUsed | Yes |
| CLI stats uses human-readable formatting | Yes |
| MCP search response includes savings | Yes |
| README documents `gist setup` and agentic integration | Yes |
| All tests pass with -race | Yes |

### Decision Rationale

- Setup subcommand design: See `docs/adr/003-setup-subcommand.md`.
- Byte vs token decision: See `docs/adr/002-token-first-stats.md`.

## Scope and Deliverables

### In Scope

- `cmd/gist/setup.go` -- Subcommand with tool adapter pattern and flag handling.
- `cmd/gist/setup_test.go` -- Tests for all adapters using temp directories.
- Add BytesUsed field to SearchResult in search.go.
- Update CLI stats command (cmd/gist/stats.go) to format bytes as human-readable units.
- Add savings summary to MCP gist_search response in mcp/tools.go.
- Update README.md with agentic tool integration section, `gist setup` documentation, and one-line install workflow.
- Tests for all new functionality.

### Out of Scope

- `gist setup --all`.
- Auto-detection of installed tools.
- Tool-specific instructions content.
- TOML parsing library for Codex CLI.
- Token-based metrics.
- Changing Stats struct fields.

### Deliverables

| ID | Description | Acceptance Criteria |
|----|-------------|-------------------|
| D1 | `gist setup <tool>` subcommand | Supports claude, gemini, cursor, copilot, codex. Flags: --global, --project, --uninstall, --dry-run. Idempotent. |
| D2 | Tool adapter pattern | Each tool defined by a single struct. Adding a tool requires no new logic. |
| D3 | SearchResult.BytesUsed | Each search result reports snippet byte count. JSON tag: bytes_used. |
| D4 | CLI stats formatting | `gist stats` prints human-readable byte values. |
| D5 | MCP search savings | gist_search response includes bytes_used per result and total. |
| D6 | README update | Documents agentic tool integration, `gist setup`, supported tools table, one-line install. |
| D7 | Tests | All new code tested. All pass with -race. |

## Checkable Work Breakdown

### E1: Setup Subcommand -- Adapter Pattern and Core Logic

- [x] T1.1 Define tool adapter types and registry  stage: implement  acc: [compiles, `gist setup` prints tool list, `gist setup invalidtool` prints error.]  deps: []  Owner: task-T1.1  Est: 30m  Done: 2026-03-14
  - Create cmd/gist/setup.go.
  - Define a `toolAdapter` struct:
    ```go
    type toolAdapter struct {
        Name             string   // "claude", "gemini", etc.
        DisplayName      string   // "Claude Code", "Gemini CLI", etc.
        GlobalMCPPath    string   // e.g., "~/.claude/mcp.json"
        ProjectMCPPath   string   // e.g., ".mcp.json"
        MCPKey           string   // "mcpServers" or "servers"
        GlobalInstPath   string   // e.g., "~/.claude/CLAUDE.md"
        ProjectInstPath  string   // e.g., "CLAUDE.md"
        InstSentinel     string   // "## Gist Context Management"
    }
    ```
  - Define a `toolRegistry` map[string]toolAdapter with entries for claude, gemini, cursor, copilot, codex.
  - Claude: GlobalMCPPath `~/.claude/mcp.json`, ProjectMCPPath `.mcp.json`, MCPKey `mcpServers`, GlobalInstPath `~/.claude/CLAUDE.md`, ProjectInstPath `CLAUDE.md`.
  - Gemini: GlobalMCPPath `~/.gemini/mcp.json`, ProjectMCPPath `.gemini/mcp.json`, MCPKey `mcpServers`, GlobalInstPath `~/.gemini/GEMINI.md`, ProjectInstPath `GEMINI.md`.
  - Cursor: GlobalMCPPath `~/.cursor/mcp.json`, ProjectMCPPath `.cursor/mcp.json`, MCPKey `mcpServers`, GlobalInstPath `~/.cursor/rules/gist.mdc`, ProjectInstPath `.cursor/rules/gist.mdc`.
  - Copilot: GlobalMCPPath `~/.vscode/mcp.json`, ProjectMCPPath `.vscode/mcp.json`, MCPKey `servers`, GlobalInstPath `~/.github/copilot-instructions.md`, ProjectInstPath `.github/copilot-instructions.md`.
  - Codex: GlobalMCPPath `~/.codex/config.toml`, ProjectMCPPath `.codex/config.toml`, MCPKey `mcp_servers` (TOML), GlobalInstPath `` (empty), ProjectInstPath `` (empty).
  - Define the gist instructions content as a Go constant.
  - Register the cobra Command under rootCmd. Override PersistentPreRunE with a no-op to skip database init.
  - Args validation: require exactly one argument that matches a key in toolRegistry, or zero args to print the list of supported tools.
  - Flags: `--project` (bool, default false), `--uninstall` (bool, default false), `--dry-run` (bool, default false).
  - Historical criterion: compiles, `gist setup` prints tool list, `gist setup invalidtool` prints error.
  - Historical prerequisites: none.

- [x] T1.2 Implement MCP config file manipulation  stage: implement  acc: [function creates/updates/removes MCP entry correctly for both JSON and TOML formats.]  deps: []  Owner: task-T1.2  Est: 30m  Done: 2026-03-14
  - In cmd/gist/setup.go, implement `func configureMCP(path string, mcpKey string, gistPath string, uninstall bool, dryRun bool) (changed bool, err error)`.
  - Detect gist binary path with `os.Executable()` + `filepath.EvalSymlinks()`.
  - Read existing file. If absent, start with `{"<mcpKey>": {}}`.
  - If file exists but is not valid JSON, return error (do not corrupt).
  - Parse as `map[string]any`. Navigate to the mcpKey (create if missing).
  - On install: set `<mcpKey>.gist` to `{"command": "<gist-path>", "args": ["serve"]}`. If already present with same values, return changed=false.
  - On uninstall: delete `<mcpKey>.gist` if present.
  - If dryRun, print what would be written to stderr but do not write.
  - Write with `json.MarshalIndent` (two-space indent) + trailing newline. Create parent directories with `os.MkdirAll`.
  - Special case for Codex (TOML format): write/read TOML using simple string operations. Generate: `[mcp_servers.gist]\ncommand = "<gist-path>"\n`. On read, check if `[mcp_servers.gist]` section exists. On uninstall, remove the section.
  - Historical criterion: function creates/updates/removes MCP entry correctly for both JSON and TOML formats.
  - Historical prerequisites: none.

- [x] T1.3 Implement instructions file manipulation  stage: implement  acc: [function appends/removes instructions section correctly.]  deps: []  Owner: task-T1.3  Est: 20m  Done: 2026-03-14
  - In cmd/gist/setup.go, implement `func configureInstructions(path string, sentinel string, uninstall bool, dryRun bool) (changed bool, err error)`.
  - If path is empty, return changed=false (tool has no instructions file).
  - Read existing file. If absent, start with empty string.
  - On install: if sentinel string not found in content, append a blank line + the gist instructions section. Return changed=true. If found, return changed=false.
  - On uninstall: if sentinel found, remove from sentinel line through the next `## ` heading (exclusive) or end of file. Trim trailing blank lines. If not found, return changed=false.
  - If dryRun, print what would be written to stderr but do not write.
  - Create parent directories with `os.MkdirAll`.
  - Historical criterion: function appends/removes instructions section correctly.
  - Historical prerequisites: none.

- [x] T1.4 Wire adapter, MCP, and instructions into cobra RunE  stage: implement  acc: [`gist setup claude` creates both files. `gist setup claude --uninstall` removes both entries.]  deps: [T1.1, T1.2, T1.3]  Owner: task-T1.4  Est: 15m  Done: 2026-03-14
  - In cmd/gist/setup.go, implement the cobra RunE function.
  - Look up the tool adapter from the registry using the first arg.
  - Determine target paths: if `--project`, use ProjectMCPPath and ProjectInstPath. Otherwise, use GlobalMCPPath and GlobalInstPath. Expand `~` to `os.UserHomeDir()`.
  - Call configureMCP and configureInstructions.
  - Print results to stderr: "Configured <DisplayName> MCP at <path>", "Added gist instructions to <path>", "Already configured (no changes)", "Removed gist from <path>".
  - Historical criterion: `gist setup claude` creates both files. `gist setup claude --uninstall` removes both entries.
  - Historical prerequisites: T1.1, T1.2, T1.3.

### E2: Setup Subcommand Tests

- [x] T2.1 Write MCP config manipulation tests  stage: verify  acc: [`GOWORK=off go test ./cmd/gist/ -run TestConfigureMCP -race -v` passes.]  deps: [T1.2]  Owner: task-T2.1  Est: 30m  Done: 2026-03-14
  - In cmd/gist/setup_test.go.
  - Test cases for JSON format (claude, gemini, cursor, copilot):
    - Fresh install: no file -> creates with gist entry.
    - Existing file with other servers -> adds gist, preserves others.
    - Already configured -> no change (idempotent).
    - Different gist path -> updates path.
    - Uninstall -> removes gist, preserves others.
    - Uninstall when not present -> no change.
    - Malformed JSON -> returns error.
  - Test cases for TOML format (codex):
    - Fresh install: no file -> creates with gist section.
    - Existing file with other sections -> adds gist section.
    - Already configured -> no change.
    - Uninstall -> removes gist section.
  - Test `mcpServers` vs `servers` key difference (copilot uses `servers`).
  - Historical criterion: `GOWORK=off go test ./cmd/gist/ -run TestConfigureMCP -race -v` passes.
  - Historical prerequisites: T1.2.

- [x] T2.2 Write instructions file manipulation tests  stage: verify  acc: [`GOWORK=off go test ./cmd/gist/ -run TestConfigureInstructions -race -v` passes.]  deps: [T1.3]  Owner: task-T2.2  Est: 20m  Done: 2026-03-14
  - In cmd/gist/setup_test.go.
  - Test cases:
    - Fresh install: no file -> creates with gist section.
    - Existing file without gist -> appends gist section.
    - Already has gist section -> no change (idempotent).
    - Uninstall -> removes gist section, preserves other content.
    - Uninstall when not present -> no change.
    - Content after gist section -> preserved on uninstall.
    - Empty path (codex) -> no change, no error.
  - Historical criterion: `GOWORK=off go test ./cmd/gist/ -run TestConfigureInstructions -race -v` passes.
  - Historical prerequisites: T1.3.

- [x] T2.3 Write end-to-end setup tests  stage: verify  acc: [`GOWORK=off go test ./cmd/gist/ -run TestSetupE2E -race -v` passes.]  deps: [T1.4]  Owner: task-T2.3  Est: 20m  Done: 2026-03-14
  - In cmd/gist/setup_test.go.
  - Test the full flow for each tool adapter: fresh setup, idempotent re-run, uninstall.
  - Use t.TempDir() as home directory substitute.
  - Verify both files are created/modified/cleaned correctly for each tool.
  - Historical criterion: `GOWORK=off go test ./cmd/gist/ -run TestSetupE2E -race -v` passes.
  - Historical prerequisites: T1.4.

### E3: Byte Savings Visibility

- [x] T3.1 Add BytesUsed to SearchResult  stage: implement  acc: [compiles, go vet clean.]  deps: []  Owner: task-T3.1  Est: 15m  Done: 2026-03-14
  - In search.go, add `BytesUsed int` field to SearchResult struct with json tag `bytes_used`.
  - In convertMatches(), set BytesUsed = len(snippet) for each result.
  - Historical criterion: compiles, go vet clean.
  - Historical prerequisites: none.

- [x] T3.2 Update CLI stats command with human-readable formatting  stage: implement  acc: [`gist stats` output shows human-readable byte values.]  deps: []  Owner: task-T3.2  Est: 15m  Done: 2026-03-14
  - In cmd/gist/stats.go, add a formatBytes helper function that returns "X B", "X.Y KB", or "X.Y MB" depending on magnitude.
  - Apply formatBytes to BytesIndexed, BytesReturned, BytesSaved output lines.
  - Keep SavedPercent, SourceCount, ChunkCount, SearchCount as plain integers.
  - Historical criterion: `gist stats` output shows human-readable byte values.
  - Historical prerequisites: none.

- [x] T3.3 Add savings summary to MCP gist_search response  stage: implement  acc: [gist_search MCP response includes bytes_used per result and total.]  deps: [T3.1]  Owner: task-T3.3  Est: 20m  Done: 2026-03-14
  - In mcp/tools.go handleSearch(), after calling g.Search(), compute total bytes used across results (sum of BytesUsed from each SearchResult).
  - Wrap the search response in a struct that includes the results array and a `bytes_used` total field.
  - Update the gist_search tool description in ToolDefinitions() to mention bytes_used.
  - Historical criterion: gist_search MCP response includes bytes_used per result and total.
  - Historical prerequisites: T3.1.

### E4: Test Updates for Savings Visibility

- [x] T4.1 Add SearchResult.BytesUsed tests  stage: verify  acc: [`GOWORK=off go test -run TestSearch -race -v` passes.]  deps: [T3.1]  Owner: task-T4.1  Est: 15m  Done: 2026-03-14
  - In search_test.go, add a test verifying BytesUsed = len(snippet) for each result.
  - In gist_test.go TestSearch, verify BytesUsed > 0 on results.
  - Historical criterion: `GOWORK=off go test -run TestSearch -race -v` passes.
  - Historical prerequisites: T3.1.

- [x] T4.2 Add CLI stats formatting test  stage: verify  acc: [`GOWORK=off go test ./cmd/gist/ -run TestFormatBytes -v` passes.]  deps: [T3.2]  Owner: task-T4.2  Est: 10m  Done: 2026-03-14
  - In cmd/gist/stats_test.go (new file), table-driven tests for formatBytes: 0 -> "0 B", 512 -> "512 B", 1024 -> "1.0 KB", 1536 -> "1.5 KB", 1048576 -> "1.0 MB", 1572864 -> "1.5 MB", negative -> "0 B".
  - Historical criterion: `GOWORK=off go test ./cmd/gist/ -run TestFormatBytes -v` passes.
  - Historical prerequisites: T3.2.

- [x] T4.3 Add MCP search savings test  stage: verify  acc: [`GOWORK=off go test ./mcp/ -race -v` passes.]  deps: [T3.1, T3.3]  Owner: task-T4.3  Est: 15m  Done: 2026-03-14
  - In mcp/tools_test.go, add a test that indexes content, searches, and verifies bytes_used total in response.
  - Update TestToolsCallSearch to verify the wrapped response structure.
  - Historical criterion: `GOWORK=off go test ./mcp/ -race -v` passes.
  - Historical prerequisites: T3.1, T3.3.

### E5: README Update

- [x] T5.1 Update README with agentic tool integration section  stage: author  acc: [README contains "Agentic Tool Integration" section with install commands, supported tools table, and flag documentation.]  deps: []  Owner: task-T5.1  Est: 30m  Done: 2026-03-14
  - In README.md, add a new section titled "## Agentic Tool Integration" after the "MCP Server" section and before the "Features" section.
  - Content of the new section:
    - Opening paragraph: Gist integrates with agentic coding tools as an MCP server. The `gist setup` command configures everything in one step.
    - One-line install-and-configure example per tool:
      ```
      brew install sirerun/tap/gist && gist setup claude
      brew install sirerun/tap/gist && gist setup gemini
      brew install sirerun/tap/gist && gist setup cursor
      brew install sirerun/tap/gist && gist setup copilot
      brew install sirerun/tap/gist && gist setup codex
      ```
    - Supported tools table (tool name, command, what it configures):
      | Tool | Command | Configures |
      |------|---------|------------|
      | Claude Code | `gist setup claude` | `~/.claude/mcp.json` + `~/.claude/CLAUDE.md` |
      | Gemini CLI | `gist setup gemini` | `~/.gemini/mcp.json` + `~/.gemini/GEMINI.md` |
      | Cursor | `gist setup cursor` | `~/.cursor/mcp.json` + `~/.cursor/rules/gist.mdc` |
      | VS Code Copilot | `gist setup copilot` | `~/.vscode/mcp.json` + `~/.github/copilot-instructions.md` |
      | Codex CLI | `gist setup codex` | `~/.codex/config.toml` |
    - Per-project setup: `gist setup claude --project` for project-scoped config.
    - Uninstall: `gist setup claude --uninstall`.
    - Dry-run: `gist setup claude --dry-run`.
    - What setup does: brief explanation that it adds gist as an MCP server and adds context management instructions so the tool uses gist automatically.
  - Historical prerequisites: none (README content can be written before the setup code exists; the README documents the planned interface).
  - Historical criterion: README contains "Agentic Tool Integration" section with install commands, supported tools table, and flag documentation.

- [x] T5.2 Update README MCP Server section  stage: author  acc: [MCP Server section references `gist setup` and provides both automatic and manual configuration paths.]  deps: []  Owner: task-T5.2  Est: 10m  Done: 2026-03-14
  - In README.md, update the existing "MCP Server" section.
  - Replace the manual JSON config example with a note that `gist setup <tool>` handles configuration automatically.
  - Keep the manual JSON example as a "Manual configuration" subsection for users who prefer manual setup or use unsupported tools.
  - Remove `--dsn` from the MCP example since in-memory is the default and sufficient for agentic tool sessions.
  - Historical criterion: MCP Server section references `gist setup` and provides both automatic and manual configuration paths.

- [x] T5.3 Update README CLI Usage section  stage: author  acc: [CLI Usage section includes `gist setup` example.]  deps: []  Owner: task-T5.3  Est: 10m  Done: 2026-03-14
  - In README.md, add `gist setup <tool>` to the CLI Usage code block.
  - Add a one-line description: "# Configure gist for your agentic coding tool".
  - Place it after the existing `gist serve` example.
  - Historical criterion: CLI Usage section includes `gist setup` example.

### E6: Quality Gates

- [x] T6.1 Run linter and fix findings  stage: verify  acc: [zero errors, zero warnings.]  deps: [T1.1, T1.2, T1.3, T1.4, T2.1, T2.2, T2.3, T3.1, T3.2, T3.3, T4.1, T4.2, T4.3, T5.1, T5.2, T5.3]  Owner: lead  Est: 10m  Done: 2026-03-14
  - `GOWORK=off go vet ./...`
  - `GOWORK=off go build ./...`
  - Historical criterion: zero errors, zero warnings.
  - Historical prerequisites: all of E1-E5.

- [x] T6.2 Run full test suite  stage: verify  acc: [all tests pass.]  deps: [T6.1]  Owner: lead  Est: 10m  Done: 2026-03-14
  - `GOWORK=off go test -race -count=1 ./...`
  - Historical criterion: all tests pass.
  - Historical prerequisites: T6.1.

## Parallel Work

### Current publication delivery batch — 2026-10-09

| Batch | Outcome/members | Dependencies | Owner and coverage | Review/merge/landed | Split condition |
|---|---|---|---|---|---|
| V2 contract preflight | PUBLISH.0 exact contracts, source write set and fixture qualification | Approved ADR012/PUBLISH.8; landed INTERFACE.6; conformant PR58 | Coordinator integrates isolated Luna contracts author, staging audit and typed/consumer audit; frozen-v1 parity, offline contract cases, actual source/tool qualification | PUBLISH.9 / PUBLISH.10 / PUBLISH.11 | Unresolved transport authority or incompatible ownership; production handler work waits for preflight landing |
| V2 publication implementation | PUBLISH.1 with PUBLISH.2/PUBLISH.3 behavior and compatibility verification | PUBLISH.11 | Luna publication plus independent verifier; authenticated HTTP, restricted-role PostgreSQL, owned object-store fixture, byte/digest/tenant/idempotency/outbox/rollback acceptance | PUBLISH.4 / PUBLISH.5 / PUBLISH.6 | Interface changes outside approved preflight, unqualified required fixture or unresolved review findings |

Guided Luna assignments retain coordinator ownership of PUBLISH.0 and the authoritative plan. Contract author owns only contracts/registry/v2; staging and typed/consumer auditors own separate preflight receipt files. Workers use isolated DGX worktrees and one fair dispatch turn each through the qualified shared queue, bounded to 20 minutes. Read-only audits may run beside the contract author at eligible capacity; production handlers remain held until PUBLISH.11. Coordinator alone integrates shared documents and closes tasks only against actual evidence.

### Track Layout

| Track | Tasks | Description |
|-------|-------|-------------|
| A: Setup Core | T1.1, T1.2, T1.3, T1.4 | Adapter types, MCP/instructions manipulation, wiring |
| B: Setup Tests | T2.1, T2.2, T2.3 | Tests for setup subcommand |
| C: Core API | T3.1, T4.1 | SearchResult.BytesUsed + tests |
| D: CLI Stats | T3.2, T4.2 | Human-readable formatting + tests |
| E: MCP Savings | T3.3, T4.3 | Search savings response + tests |
| F: README | T5.1, T5.2, T5.3 | Documentation updates |
| G: Quality | T6.1, T6.2 | Lint and full test suite |

### Sync Points

- T1.1, T1.2, T1.3 have no dependencies on each other.
- T1.4 depends on T1.1, T1.2, T1.3.
- T2.1 depends on T1.2. T2.2 depends on T1.3. T2.3 depends on T1.4.
- T3.1 has no dependencies. T3.3 depends on T3.1. T3.2 has no dependencies.
- T4.1 depends on T3.1. T4.2 depends on T3.2. T4.3 depends on T3.1 and T3.3.
- T5.1, T5.2, T5.3 have no code dependencies (they document the planned interface).
- All tracks must complete before G starts.

### Maximum Parallelism

**Wave 1** (5 tasks -- no dependencies, saturates all agent slots):
- T1.1: Define tool adapter types and registry
- T1.2: Implement MCP config file manipulation
- T1.3: Implement instructions file manipulation
- T3.1: Add BytesUsed to SearchResult
- T3.2: Update CLI stats command with human-readable formatting

**Wave 2** (5 tasks -- after Wave 1):
- T1.4: Wire adapter, MCP, and instructions into cobra RunE (needs T1.1, T1.2, T1.3)
- T2.1: Write MCP config manipulation tests (needs T1.2)
- T2.2: Write instructions file manipulation tests (needs T1.3)
- T4.1: Add SearchResult.BytesUsed tests (needs T3.1)
- T4.2: Add CLI stats formatting test (needs T3.2)

**Wave 3** (5 tasks -- after Wave 2):
- T2.3: Write end-to-end setup tests (needs T1.4)
- T3.3: Add savings summary to MCP gist_search response (needs T3.1)
- T4.3: Add MCP search savings test (needs T3.1, T3.3)
- T5.1: Update README with agentic tool integration section (no code deps)
- T5.2: Update README MCP Server section (no code deps)

**Wave 4** (3 tasks):
- T5.3: Update README CLI Usage section (no code deps, but logically after T5.1/T5.2 to avoid merge conflicts since all touch README.md)
- T6.1: Run linter and fix findings (needs all code tasks)
- T6.2: Run full test suite (needs T6.1)

Note: T5.1, T5.2, T5.3 all edit README.md. While worktrees allow parallel file edits, placing T5.3 in Wave 4 reduces merge conflict risk. Alternatively, all three README tasks could run in Wave 1 since they have no code dependencies, but serial execution within the README is safer.

## Timeline and Milestones

| Milestone | Tasks | Exit Criteria |
|-----------|-------|--------------|
| M1: Setup core works | T1.1-T1.4 | `gist setup claude` and `gist setup gemini` configure both files. |
| M2: Setup tested | T2.1-T2.3 | All setup tests pass with -race. |
| M3: Savings visible | T3.1-T3.3 | BytesUsed on results. CLI and MCP show savings. |
| M4: All tests green | T4.1-T4.3 | All savings tests pass with -race. |
| M5: README updated | T5.1-T5.3 | README documents agentic tool integration, setup, and CLI usage. |
| M6: Ship | T6.1, T6.2 | Lint clean. Full test suite green. |

## Risk Register

| ID | Risk | Impact | Likelihood | Mitigation |
|----|------|--------|------------|------------|
| R1 | Tool config file locations change in future versions | High | Low | Pin to documented locations. Print paths in output so users can verify. |
| R2 | os.Executable() returns temp binary path (go run) | Medium | Medium | filepath.EvalSymlinks resolves symlinks. Print resolved path. Document that setup should run from installed binary. |
| R3 | Concurrent writes to config files | Low | Low | Write atomically: write temp file then rename. |
| R4 | Codex TOML manipulation without a parser | Low | Medium | The config is small and predictable. String-based manipulation is sufficient. Test thoroughly. |
| R5 | Wrapping MCP search response changes JSON shape | Medium | Low | Gist is pre-1.0. No known external consumers. |
| R6 | Cursor .mdc format differs from plain markdown | Low | Medium | Use standard markdown content. .mdc files accept markdown. |
| R7 | README merge conflicts from parallel edits | Low | Medium | Run README tasks sequentially or in a single wave. |

## Operating Procedure

- **Definition of done**: Code compiles, tests pass with -race, go vet clean, committed with Conventional Commits.
- **Review**: Read all changed files before marking complete.
- **Testing**: Every new function must have at least one test.
- **Linting**: Run `GOWORK=off go vet ./...` and `GOWORK=off go build ./...` after every code change.
- **Commits**: Small, logical. Do not commit files from different directories together.

## Progress Log

### 2026-03-14 -- Plan Updated (README)

- Added E5 (README Update) with T5.1, T5.2, T5.3 to document agentic tool integration, `gist setup`, and CLI usage.
- Added D6 deliverable for README update.
- Added M5 milestone for README completion.
- Reorganized waves: README tasks placed in Wave 3 (T5.1, T5.2) and Wave 4 (T5.3) to avoid merge conflicts.
- Added R7 risk for README merge conflicts.

### 2026-03-14 -- Plan Updated (Multi-Tool Setup)

- Expanded setup subcommand from single-tool (`gist setup`) to multi-tool (`gist setup <tool>`).
- Added tool adapter pattern supporting claude, gemini, cursor, copilot, codex.
- Updated docs/adr/003-setup-subcommand.md with multi-tool design.
- Reorganized work breakdown: E1 split into 4 tasks (T1.1-T1.4), E2 has 3 test tasks (T2.1-T2.3).

### 2026-03-14 -- Plan Created (Setup + Savings)

- Created plan merging setup subcommand (E1) and byte savings visibility (E3-E4).
- Created docs/adr/003-setup-subcommand.md and docs/adr/002-token-first-stats.md.

## Hand-off Notes

- The CLI uses cobra (github.com/spf13/cobra). New subcommands go in `cmd/gist/<name>.go` and register via `init()`.
- To skip PersistentPreRunE (database init) for setup, override it on the setup command: `PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil }`.
- Claude Code global config: `~/.claude/mcp.json` + `~/.claude/CLAUDE.md`.
- Gemini CLI global config: `~/.gemini/mcp.json` + `~/.gemini/GEMINI.md`.
- Cursor global config: `~/.cursor/mcp.json` + `~/.cursor/rules/gist.mdc`.
- VS Code Copilot global config: `~/.vscode/mcp.json` + `~/.github/copilot-instructions.md`.
- Codex CLI global config: `~/.codex/config.toml` (no instructions file).
- The gist binary via Homebrew is at `/opt/homebrew/bin/gist`. `os.Executable()` + `filepath.EvalSymlinks()` resolves it.
- Use `GOWORK=off` for all go commands.
- README structure: Overview > Installation > Quick Start (Zero Deps) > Quick Start (PostgreSQL) > CLI Usage > MCP Server > Agentic Tool Integration > Features > API Reference > Contributing > License.

## Appendix

### gist setup Usage

```
gist setup                      # Print list of supported tools
gist setup claude               # Configure Claude Code globally
gist setup gemini               # Configure Gemini CLI globally
gist setup claude --project     # Configure for current project
gist setup claude --uninstall   # Remove gist config from Claude Code
gist setup claude --dry-run     # Preview changes without writing
```

### Tool Adapter Registry

| Tool | Name | Global MCP | Project MCP | MCP Key | Global Instructions | Project Instructions |
|------|------|------------|-------------|---------|--------------------|--------------------|
| Claude Code | claude | ~/.claude/mcp.json | .mcp.json | mcpServers | ~/.claude/CLAUDE.md | CLAUDE.md |
| Gemini CLI | gemini | ~/.gemini/mcp.json | .gemini/mcp.json | mcpServers | ~/.gemini/GEMINI.md | GEMINI.md |
| Cursor | cursor | ~/.cursor/mcp.json | .cursor/mcp.json | mcpServers | ~/.cursor/rules/gist.mdc | .cursor/rules/gist.mdc |
| VS Code Copilot | copilot | ~/.vscode/mcp.json | .vscode/mcp.json | servers | ~/.github/copilot-instructions.md | .github/copilot-instructions.md |
| Codex CLI | codex | ~/.codex/config.toml | .codex/config.toml | mcp_servers | (none) | (none) |

### README Agentic Tool Integration Section (Planned Content)

The new section will appear after "MCP Server" and before "Features" in the README. It includes:

1. Opening paragraph explaining gist integrates with agentic coding tools via MCP and `gist setup` automates configuration.
2. One-line install-and-configure examples for each tool.
3. Supported tools table with tool name, command, and what gets configured.
4. Per-project setup, uninstall, and dry-run flag documentation.
5. Brief explanation of what setup does (adds MCP server entry + context management instructions).

### Gist Instructions Content (Shared Across All Tools)

```markdown
## Gist Context Management

Use the gist MCP tools (gist_index, gist_search, gist_stats) to manage context efficiently:

- When reading files over 5KB or receiving tool output over 5KB, index the content with gist_index (set a descriptive source label like the file path).
- Instead of re-reading indexed files, use gist_search to retrieve only the relevant snippets.
- When exploring a codebase (reading multiple files, grepping across directories), index results into gist and search across them.
- After completing a task, call gist_stats and briefly report bytes saved (e.g., "Gist: indexed 48.2 KB, returned 3.1 KB, saved 93.6%").
```

### formatBytes Specification

| Input | Output |
|-------|--------|
| 0 | "0 B" |
| 512 | "512 B" |
| 1024 | "1.0 KB" |
| 1536 | "1.5 KB" |
| 1048576 | "1.0 MB" |
| 1572864 | "1.5 MB" |
| negative | "0 B" |

### MCP Search Response Shape (After)

```json
{
  "results": [
    {
      "title": "Config > Database",
      "snippet": "Connection pool size...",
      "source": "config.md",
      "score": 0.85,
      "content_type": "prose",
      "match_layer": "porter",
      "bytes_used": 142
    }
  ],
  "bytes_used": 142
}
```

## Current registry recovery planning supplement — 2026-10-04

The March plan above is preserved as historical completed work. The active bounded registry follow-on is [the registry recovery delivery plan](plans/registry-recovery.md), with split SDLC tasks and maximum qualified GPT-6-Luna parallelism. Planning only; use that explicit plan for future execution rather than reopening historical tasks.

## Full AWS production continuation — 2026-10-04

The founder selected registry plus caller-owned integrations, with completion only when https://gist.sire.run is running qualified AWS production. The included plan below is the canonical new work graph and supersedes earlier recovery-only closure/model assumptions; all March authored text and checked tasks above remain historical. Read [the production owner plan](plans/registry-recovery.md) for GPT-6-Luna pool limits, ownership, qualified operation routing and the final production predicate. No local merge/initial rollout/planning checkpoint completes this delivery. All stages and external authority are enforced from its task dependencies and evidence; /plan itself executes nothing.

### E-GR-PRODUCTION -- Full SDLC to canonical AWS production

Date: 2026-10-08. Status: **CORE source landed; owner-approved v2 publication preflight ready; production gates remain open**.
Planning contract: ordinary unenrolled repository delivery. Current source baseline inspected: Gist main `dacb0c92d2a535e45066927e59eb67d155a86065`; revalidate before dispatch. Earlier dated discovery below used84e1412 and remains historical. This is the active bounded recovery supplement to [registry-buildout.md](plans/registry-buildout.md), not a replacement of its completed tasks. [gateway-buildout.md](plans/gateway-buildout.md) remains a separate scope. This is the canonical production-owner plan, also included by docs/plan.md for default parser/dispatcher visibility; March task text and completed status remain historical. Destination/scope follow accepted ADR011. No executing lifecycle is admitted by this planning refinement.

## Current alignment and execution baseline -- 2026-10-08

The founder's final VISION-ALIGN-17 decision and four-owner VISION-ALIGN-20 closure make Gist an optional capability registry. Zatiti supports independently configured direct MCP connections; neither its general use, demo nor first app/host packet requires Gist. Context-index library adoption is separate, Serenity owns governed knowledge, and no managed gateway is selected. Discovery/resolution never grants execution authority. Direct connections have their own qualified binding and governance; they are not an automatic bypass for an unavailable, expired or revoked Gist grant. See [ADR013](adr/013-optional-caller-integrations.md).

Gist's own accepted full-SDLC goal remains the registry with caller-owned integrations running at https://gist.sire.run on AWS. The existing requirements/design, implementation, testing, independent review, merge, landed verification, release, deployment, live acceptance and operations rows remain first-class gates. An optional future Zatiti-specific seam packet needs a separately accepted owner/contract/plan; its proposal supplies no worker assignment and does not gate the Zatiti demo. Existing generic consumer acceptance is still required for Gist production and must identify an actual qualified consumer rather than substitute a synthetic fixture.

Current source baseline is dacb0c92d2a535e45066927e59eb67d155a86065: PR55 CORE43/43 and PR56 checkpoint are landed. Prior dated execution entries below are historical receipts, not current host admission. The owned DGX transfer preserves bundled history (including worktree heads),36unfinished paths and original local copies; archive and per-file hashes verified, no active Git operation, no credentials transferred. See [current preflight](receipts/2026-10-08-alignment-and-dgx-preflight.md).

Fresh configured Cloudflare MCP GETs on2026-10-08 returned200 for one active canonical zone and zero exact service-name DNS records. This supersedes earlier missing-binding observations only for connection/read availability. Write permissions, reviewed target records and rollout readiness remain unqualified; AWS identity/stack-account matching was not refreshed. Publication v2 implementation was approved on2026-10-08; its preflight/contract freeze is now ready. Identity/workspace custody, bounded provider approval, actual consumer acceptance and AWS target access remain open gates. No release, deployment, provider call or production acceptance occurred.

## Context and outcome

Deliver the complete agreed registry and caller-owned integrations in production on AWS at **https://gist.sire.run**. Complete the registry gaps preventing an authenticated caller-owned journey: invited enrollment/bounded issuance, persistent signing identity, canonical exact artifacts/resolution, durable revocation, and real Treg/Composio capability artifacts. Gist publishes/discovers/resolves; the consumer enforces and executes. Requirements, design, preflight, coding, behavior/security acceptance, format/lint/CI, independent code review, corrections/re-review, merge, landed checks, immutable release, authorized rollout, live/pilot acceptance and bounded operational handoff are first-class dependency-linked plan items.

No full generic gateway, arbitrary provider proxy, vault, billing ledger, global client setup, organization bootstrap, private application implementation or marketing publication is included. Portfolio hierarchy is consumer-owned; chiefs do not inherit cross-business credential/memory authority. Hosted memory and private relationship-data isolation remain separate qualification obligations. Production deployment at the stated AWS origin is explicitly in scope. Planning itself never performs deployment, account enrollment or provider spending; existing scoped grants persist and genuinely missing numerical/custody authority stays explicit.

## Discovery and use-case coverage

Fresh source confirms `app.go` generates a startup signing key and composes process-local events; composed resolve uses `skill`/`runtime_id` rather than frozen `skill_ref`/`runtime`; external enrollment remains incomplete. Composio capture is `source_unavailable` and its action is synthetic; no Treg capture/adapter was found. App MCP exposes only gist_discover/get/resolve/connect. Live acceptance evidence `M2a.json` is absent. No code graph binding was callable/fresh in this session: rg/file/API/route/test inspection is the qualified fallback. docs/design.md and ADR004-010 exist; docs/devlog.md now records prior planning events. No repository lifecycle enrollment was found or created.

Reuse the original eleven UC IDs and coverage matrix in registry-buildout.md; this production plan retains all eleven original use cases, with required later client/connection/taxonomy completeness reconciled from the original plan rather than silently dropped. Source presence and historical checked tasks do not prove a deployed journey. Relevant map:

| Use cases | Current gap | Recovery gate |
| --- | --- | --- |
| UC-001 local compatibility | Must preserve root library/CLI/stdio | T-GR-INTEGRATE.3/.6 |
| UC-002 publication; UC-003 discovery; UC-004 exact artifact | Real capture/closure and composed wire evidence incomplete | Provider lanes, WIRE, INTEGRATE |
| UC-005 resolution; UC-006 delegated connections | Canonical payload/actual binding/connection parity | WIRE, INTEGRATE |
| UC-007 isolation; UC-009 authentication | Enrollment and restart/replica identity | AUTH, KEYS, INTEGRATE |
| UC-008 revocation | Durable shared feed/cursor absent in app composition | EVENT, INTEGRATE, RELEASE |
| UC-010 runtime interop | Actual consumer receipt/live execution not qualified | SCOPE.7, RELEASE.6, M3 |
| UC-011 optional taxonomy | Existing optional behavior must remain accurate/attributed | M3/PROD regression and original requirements matrix |

## Scope, status preservation and deliverables

| Deliverable | New lane | Historical crosswalk, preserved |
| --- | --- | --- |
| Requirements/design/operator custody | SCOPE | A2/A4, ADR005/007 and Q5 credential-path note; amendment through reviewed implementation, not silent approval |
| Enrollment and bounded human/workload grants | AUTH | I1-I6 source presence preserved; actual enrollment/issuer integration is new missing work |
| Persistent shared signing identity | KEYS | I1/I4 and Q3 follow-on defect |
| Canonical API/MCP resolution and exact closure | WIRE | S1-S3/T1-T3/Q3 follow-on defect |
| Durable event/cursor composition | EVENT | B/S/Q3 follow-on repair; existing durable migrations alone did not prove wiring |
| Actual action-specific Treg/Composio artifacts | TREG/COMPOSIO | D1-D4 original fixtures preserved; no replacement of historical synthetic evidence |
| Actual app composition and qualification | INTEGRATE | Q3/Q4 checked history preserved; new integration receipts appended |
| Release/live/ops/milestone acceptance | RELEASE | Q5 remains open until every original obligation is genuinely met; X-R1/R3 real-build gaps retained |
| New-origin AWS/DNS/TLS/config cutover | AWS | ADR011 supersedes only ADR008 origin; existing infrastructure owner retains reviewed source |
| Full M2b/M3 production acceptance | NEXT/M2B/M3 | Original Q8/Q10/R4/R5/E3 coverage and IDs preserved |
| Terminal production acceptance | PROD | Final conjunction of actual running AWS service, requested hostname, registry/client/provider and operations evidence |
| Optional managed gateway | GATEWAY outline | Remains outside founder-confirmed production scope |

Each named task below is a planning task record, not a minted external-service task ID or admission. `acc` is opaque authored acceptance because Kazi is installed. Explicit `lane: agent` coding markers select the user-requested GPT-6-Luna worker instead of the default Kazi authoring lane; the user model instruction overrides any legacy frontier-model suggestion. Kazi may supply a qualified acceptance draft/verify helper just in time without replacing the specified worker model. Stage rows execute through stage guidance, not Kazi code convergence. Preserve canonical lifecycle tasks instead of this graph if the change is later enrolled.

## GPT-6-Luna concurrency contract

- All delegated implementation, verification, review and landed-check workers use **GPT-6-Luna**, explicit model binding `gpt-6-luna`. One coordinator retains requirements, contracts, ownership, plan writes, integration arbitration, decision routing and final verification. No automatic paid model/provider fallback.
- Use the maximum eligible GPT-6-Luna capacity from real dependency readiness, isolated ownership, independent reviewers and qualified resources. Do not reserve a synthetic worker count or infer concurrency from unused harness slots. The shared subscription bucket currently has an aggregate ceiling of four sessions and minimum launch spacing of sixty seconds; these are conservative dispatch settings, not published provider quotas. Read the live shared queue/cooldown before dispatch.
- Active non-Apple implementation, verification and workers run on qualified DGX storage. The Mac is limited to lightweight coordination and transfer. New task/worktree/cache/artifact directories are isolated under the owned task alias; preserve original checkouts and all unfinished source/history/receipts. Transfer hashes must match before resuming; credential stores, environment secrets and signing keys stay in their existing custody. Existing Apple-native exceptions belong to their owners.
- Use the existing cross-project atomic dispatch lease and shared queue/cooldown. Admit one session per fair turn, record acknowledgement/failure, release the exact owned token, and honor Foundry/Zatiti dependency-unblocking priority without starvation. A 429 pauses the shared bucket: honor Retry-After, otherwise start at sixty seconds and double up to fifteen minutes with jitter, halve concurrency and double spacing. After cooldown admit one probe; increase by at most one after five healthy minutes with twenty percent headroom. More hosts or paid fallback do not remove model quotas.
- Qualify DGX load, storage, tools, cache bounds and actual worker processes before starting. Keep at most two heavy build lanes per project and one full race suite; Mac build leases do not establish DGX capacity. Existing shared resources and foreign claims remain untouched. Documentation-only checks do not trigger application builds.
- Workers receive exact owned files, task IDs, base/head and accepted product/contract readbacks. Independent review uses a different author identity and the exact candidate. Refill only dependency-ready work; unavailable approval, model capacity, access or tooling blocks the affected stage. Coordinate through the existing project ajent.social, with no second bus, wake loop or global notifier.

## Schedule and dependency policy

The founder explicitly requested the entire SDLC as first-class items, so the known recovery deliverables include all stage obligations now, including gated release/live/operations rows. Those rows state verifiable outcomes rather than inventing a deployment design; their preflights revalidate actual landed inputs and authority. The user now explicitly requires full registry production completion. Known M2b/M3 obligations are therefore first-class stages with a receipt-driven planning refresh before dispatch; genuinely new unknown epics stay outlines with one triggered planning task. The unselected managed gateway stays outline and never blocks this completion. The split epic files contain the machine-readable wave assignments and task dependencies. Wave labels describe the horizon; they do not forbid pipelining independent verified candidates. Merge mutations to main are serialized by the coordinator, but unmerged branch work and disjoint reviews remain parallel. Real shared interface dependence requires landed receipts; speculate only with explicitly recorded immutable inputs and no delivery authority. Estimates remain TBD pending bounded preflight sizing, rather than inventing dates from unresolved scope.

Every candidate chain is preflight -> implement -> behavior/quality verify -> independent review -> guarded merge -> verify-landed. Accepted findings keep stable finding labels such as KEYS-R4 and allocate parser-supported numeric task IDs for explicit implement/verify/review successors and exact dependencies. Fixes depend on the finding/handoff, not successful completion of the failed review, preventing deadlock. The merge row waits on latest successful independent review; head/base/interface changes invalidate affected checks/review. Preserve original IDs and findings history; no automatic checkbox resets erase evidence.

## Checkable Work Breakdown

### E-GR-SCOPE -- Requirements and preflight

fidelity: executable
Acceptance: Requirements, designs, source baseline, ownership and tooling are reviewable; unresolved external authority remains blocked.

Requirements and design are separate outcomes; these rows do not approve their own proposals. Coordinator owns this plan, contracts, disposition of decisions and integration. Use the current enrolled-service snapshot instead if this change is actually enrolled later; no enrollment exists in this planning delivery. All worker/model choices below implement the founder's explicit GPT-6-Luna direction, superseding old Sonnet/Opus dispatch prose for this recovery scope only.

#### Wave 0: Requirements, design and environment

- [x] T-GR-SCOPE.1 Reconcile current source, historical receipts and concurrent ownership  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  acc: [Record full local/fetched/remote main SHAs, open PR heads and branch protection through gh, existing agent working directories, task/resource claims and dirty files; preserve all existing work and identify current source findings without treating historical checkboxes as fresh acceptance]
- [x] T-GR-SCOPE.2 Freeze recovery requirements and acceptance matrix  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009, UC-010]  blocked-by: [T-GR-SCOPE.1]  acc: [Version requirements for invited human access plus bounded workload issuance, canonical registry v1 interoperability, replica-safe keys/events and action-specific provider artifacts; map every required behavior and denial to a task and fixture; no registry metadata grants execution; record deferred gateway and customer-data boundaries]
- [x] T-GR-SCOPE.3 Review design and freeze concurrent lane contracts  Owner: independent-luna-design  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.2]  acc: [Independent architecture/security reviewer records agreed component interfaces, file/migration allocations, key custody and rotation proposal, auth/event transaction boundaries and caller revocation race limits; accepted contract changes update existing ADR005/007 through the owning candidate, not silently rewrite frozen v1 or accept new infrastructure]
- [ ] T-GR-SCOPE.4 Record operator enrollment and credential custody choice  stage: preflight  Owner: coordinator-founder  Est: TBD  kind: human  verifies: [UC-009]  blocked-by: [T-GR-SCOPE.2]  acc: [Actual approved operator/login proof, one-workspace invitation/enrollment bounds, human callback/token custodian and workload-parent issuance authority are recorded; manual SQL, public bootstrap and asserted operator flags are excluded]  blocked: Awaiting coordinator disposition of enrollment/operator trust and custody
- [ ] T-GR-SCOPE.5 Record scoped live environment and numerical provider authority  stage: preflight  Owner: coordinator-founder  Est: TBD  kind: human  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.2]  acc: [Record the explicit AWS production scope at https://gist.sire.run, reuse applicable accepted ADR008 stack grants without asking again, and record exact operator, permitted rollout plus any separately necessary preview/build envelope, provider/account/action/target, allowed request counts, per-call/per-user/aggregate ceilings and expiry; reconcile prior deployment grants without broadening them; current credits are not a numerical authority grant]  blocked: Provider numerical ceilings and any additional billable preview/build scope remain to be reconciled; AWS production destination is explicitly selected
- [x] T-GR-SCOPE.6 Qualify worker capacity, SSD worktrees and stage tooling  Owner: luna-preflight-capacity  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  acc: [Record available agent slots and GPT-6-Luna model binding; external build volume is mounted/writable with measured free space sufficient for planned worktrees and caches; uniquely allocate each worker external-SSD worktree and ownership/claim; qualify plan/stage tooling without global plugin activation; unavailable SSD or model binding blocks dispatch rather than falling back]
- [ ] T-GR-SCOPE.7 Collect exact consumer execution and import readiness receipt  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-010]  blocked-by: [T-GR-SCOPE.2]  acc: [Consumer owner supplies pinned source/build and admitted registry OAuth/import/native mapping/per-connection profile, protected funding and persistence contracts; callback/key-header handling and unknown outcomes are qualified; no business hierarchy grants cross-business credentials or memory]  blocked: Consumer owner reports runtime, per-profile header/custody and protected operator funding qualifications are not yet admitted; planning PR is not a runtime receipt
- [x] T-GR-SCOPE.8 Qualify required CI and merge policy  Owner: luna-preflight-ci  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  acc: [Record current GitHub required checks and whether jobs can start; preserve required checks; reconcile accepted ADR008 local-validation authority and current branch protections; reuse an applicable existing alternative without asking again, while any genuinely required unsatisfied check keeps its merge blocked; local passes never become CI evidence]
- [x] T-GR-SCOPE.9 Validate requirements/design handoff before lane admission  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.2, T-GR-SCOPE.3, T-GR-SCOPE.6]  acc: [Each lane being admitted has one accepted interface/write set, a RED baseline defect or new-capability criterion, mandatory negative fixtures and named independent review path; explicitly record held lanes and their unresolved choices so they do not prevent admission of independent qualified lanes; contracts remain compatible and no task starts from an unapproved trust or external-action choice]

- [x] T-GR-SCOPE.10 Inventory exact provider documentation and safe capture requirements  Owner: luna-preflight-providers  Est: TBD  kind: agent stage: preflight  verifies: [UC-002, UC-004, UC-005, UC-010]  acc: [Official Treg/Composio schema/version/auth/price/license sources and proposed read action capture paths are recorded without installs, credentials, session setup or underlying endpoint calls; identify missing real metadata and required capture permission; untrusted documentation cannot grant execution authority]

- [ ] T-GR-SCOPE.11 Qualify automation bindings for AWS production and DNS operations  Owner: luna-production-preflight  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.14, T-GR-SCOPE.17]  acc: [Record aws/Pulumi/gh and configured Cloudflare MCP bindings, actual permitted AWS operator role and stack workflow, CodeBuild/ECR artifact path, private IaC canonical task ownership and existing numerical grants; distinguish installed CLI from auth/permission; accepted ADR008 local validation is reconciled with current protections; unavailable binding is a real capability block, not an artificial human handoff]  blocked: Actual AWS session does not match stack account and configured Cloudflare DNS MCP is unavailable; refreshed read-only bindings do not qualify production writes


#### Wave 0: Accepted checkpoint privacy findings

- [x] T-GR-SCOPE.12 Fix PR56-F1 public receipt private infrastructure identity  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.9]  acc: [Remove private infrastructure project names and revisions from the public binding receipt while preserving actual dirty/stale status and unresolved account/DNS gates]
- [x] T-GR-SCOPE.13 Verify PR56-F1 sanitized receipt  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.12]  acc: [Exact changed receipt omits the identified private value; full diff formatting and installed plan parser preserve resolved acyclic dependencies and production gates; no application bytes change]
- [x] T-GR-SCOPE.14 Independently re-review PR56-F1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.13]  acc: [Nonauthor reviewer records exact corrected head/base and confirms privacy finding closure; checkpoint merge waits on successful final review]
- [x] T-GR-SCOPE.15 Fix PR56-F2 public receipt local home path  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.9]  acc: [Replace the local home path in the copied parser example with an installed-parser placeholder while preserving explicit read-only source and write_output=False semantics]
- [x] T-GR-SCOPE.16 Verify PR56-F2 sanitized receipt  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.15]  acc: [Exact changed receipt omits the identified private value; full diff formatting and installed plan parser preserve resolved acyclic dependencies and production gates; no application bytes change]
- [x] T-GR-SCOPE.17 Independently re-review PR56-F2  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.16]  acc: [Nonauthor reviewer records exact corrected head/base and confirms privacy finding closure; checkpoint merge waits on successful final review]


PR56-F1/F2 independently re-reviewed PASS at b89999bfb95eb5fcf8f9226746ea5c0794a90354/baseb91ec511153c5e2ed57569be532e0401ab8121a8 by the nonauthor checkpoint reviewer. Final documentation head still requires its own bounded comparison before guarded merge; external review provenance is attached to PR56.

### Current scope and execution refresh

- [x] T-GR-SCOPE.18 Reconcile final optional integration scope and qualified DGX custody  Owner: root  Est: 30m  kind: agent  stage: preflight  delivers: [ADR013 scope basis and integrity-verified relocation preflight]  acc: [Current main and final four-owner closure are reconciled;36unfinished paths/history preserved with matching hashes; DGX tools/resources qualified; no credentials transferred or runtime readiness inferred]  Done: 2026-10-08
- [x] T-GR-SCOPE.19 Update scope, execution plan and durable architecture/decision records  Owner: root  Est: 30m  kind: agent  stage: implement  blocked-by: [T-GR-SCOPE.18]  delivers: [ADR013, design, current recovery plan, preflight receipt and event log]  acc: [Gist remains optional/off Zatiti demo critical path; existing full registry SDLC and evidence retained; DGX-only execution and shared dispatch rules are explicit; v2 publication approval recorded; provider/identity/AWS/consumer gates remain blocked]  Done: 2026-10-08
- [x] T-GR-SCOPE.20 Verify documentation candidate and complete delivery graph  Owner: root  Est: 30m  kind: agent  stage: verify  blocked-by: [T-GR-SCOPE.19]  delivers: [Exact candidate validation receipt]  acc: [Parser has unique IDs, resolved dependencies, no cycles, supported stages, coherent counts and every active registry row joins PROD.9; old task statuses preserved; diff whitespace and relative links pass; no application code changed]  Done: 2026-10-08
- [x] T-GR-SCOPE.21 Independent review of alignment/publication-decision plan/design/ADRs/preflight/event candidate  Owner: independent-luna-reviewer  Est: 30m  kind: agent  stage: review  blocked-by: [T-GR-SCOPE.20]  delivers: [Independent exact-head review]  acc: [Reviewer is not a candidate author; review records base/head, all changed files, findings/dispositions and approval after accepted fixes; no documentation claim exceeds source/transfer/read-only evidence]  Done: 2026-10-08  Evidence: [PR57 actual review/merge/landed receipt](receipts/2026-10-08-plan-alignment-landed.md)
- [x] T-GR-SCOPE.22 Rebase merge independently reviewed alignment candidate  Owner: root  Est: 15m  kind: agent  stage: merge  blocked-by: [T-GR-SCOPE.21, T-GR-SCOPE.26]  delivers: [Guarded GitHub rebase merge receipt]  acc: [Exact approved PR head merged under unchanged repository policy; billing-only unavailable CI distinguished from local validation; actual landed revision recorded]  Done: 2026-10-08  Evidence: [PR57 actual review/merge/landed receipt](receipts/2026-10-08-plan-alignment-landed.md)
- [x] T-GR-SCOPE.23 Verify alignment artifact on actual landed revision  Owner: root  Est: 15m  kind: agent  stage: verify-landed  blocked-by: [T-GR-SCOPE.22]  delivers: [Landed documentation verification and ownership handoff]  acc: [Reviewed source-tree parity and main reachability confirmed; graph/link/whitespace validation passes on detached landed revision; receipts preserve exact base/head/landed SHAs; owned plan claim released without touching foreign work]  Done: 2026-10-08  Evidence: [PR57 actual review/merge/landed receipt](receipts/2026-10-08-plan-alignment-landed.md)

### Accepted plan review finding and recovery

Independent R1 at280081ff0080bf8d772cfd2fb47711a343d7170f againstdacb0c9 requested changes. PLAN57-R1 (P2) accepted: the publication footer incorrectly stated that owner approval/PUBLISH.8 were still pending. Original review evidence remains in the owned external receipt; negative review does not satisfy the merge gate.

- [x] T-GR-SCOPE.24 Fix PLAN57-R1 stale publication approval status  Owner: root  Est: 15m  kind: agent  stage: implement  blocked-by: [T-GR-SCOPE.19]  delivers: [Unambiguous historical drafting/current approved publication wording]  acc: [Historical PR54 prose no longer contradicts the2026-10-08 owner approval or completed PUBLISH.8; preserve original drafting SHA/evidence]  Done: 2026-10-08
- [x] T-GR-SCOPE.25 Verify PLAN57-R1 correction and affected graph/count/link claims  Owner: root  Est: 15m  kind: agent  stage: verify  blocked-by: [T-GR-SCOPE.24]  delivers: [Affected current-candidate documentation validation]  acc: [Strict parser/graph, regenerated progress counts, added relative links and whitespace pass on corrected candidate; original task statuses preserved except explicit approval; no application code changed]  Done: 2026-10-08
- [x] T-GR-SCOPE.26 Independent exact-head re-review after PLAN57-R1  Owner: independent-luna-reviewer  Est: 30m  kind: agent  stage: review  blocked-by: [T-GR-SCOPE.25]  delivers: [Approved exact corrected head plus original finding disposition]  acc: [Independent reviewer confirms PLAN57-R1 resolved and approves all affected docs/plan changes at recorded base/head; accepted new findings loop through tracked fixes/checks/re-review; original negative review alone never permits merge]  Done: 2026-10-08  Evidence: [PR57 actual review/merge/landed receipt](receipts/2026-10-08-plan-alignment-landed.md)

### E-GR-AUTH -- Enrollment and grants

fidelity: executable
Acceptance: Invited operator-proven user obtains and refreshes catalog:read without seeded tables; operator-authorized workload issuance is available through the approved boundary and preserves exact parent/workspace/audience/scopes; unauthenticated bootstrap never succeeds

Use cases: UC-007, UC-009. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own new enrollment domain/HTTP files, enrollment tests, allocated identity-store/invitation migration and narrowly assigned OAuth enrollment files. Existing shared ports, oauth storage and app.go change only through explicit coordinator allocations. Never edit key material, event tables or another lane tests. Preserve login delegation; no password database or manual SQL.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [ ] T-GR-AUTH.0 Preflight enrollment and grants requirements, design and write set  Owner: luna-auth  Est: TBD  kind: agent  stage: preflight  verifies: [UC-007, UC-009]  blocked-by: [T-GR-SCOPE.9, T-GR-SCOPE.4]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]
- [ ] T-GR-AUTH.1 Implement reviewed operator/invitation enrollment and human OAuth plus bounded workload issuance  Owner: luna-auth  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.0]  acc: [Invited operator-proven user obtains and refreshes catalog:read without seeded tables; operator-authorized workload issuance is available through the approved boundary and preserves exact parent/workspace/audience/scopes; unauthenticated bootstrap never succeeds; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [ ] T-GR-AUTH.2 Verify changed behavior and denial matrix  Owner: luna-verify-auth  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.1]  acc: [Golden enrollment/consent/refresh and actual HTTP status/body tests; wrong identity/workspace/audience, expired or replayed invite/code, denied consent, widened scopes, revoked member/refresh/parent and operator impersonation deny; browser consent golden and edge paths use approved isolated fixtures; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [ ] T-GR-AUTH.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-auth  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.1]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [ ] T-GR-AUTH.4 Independent code and security review of T-GR-AUTH.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.2, T-GR-AUTH.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [ ] T-GR-AUTH.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.4, T-GR-SCOPE.8]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [ ] T-GR-AUTH.6 Verify landed enrollment and grants behavior  Owner: luna-landed-auth  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]

### E-GR-KEYS -- Persistent signing keys

fidelity: executable
Acceptance: Two independently constructed issuers load the same approved key source and verify unexpired grants across restart/replica change; missing, malformed or unsafe key config fails closed; rotation preserves the agreed overlap and rejects retired keys without generating a fresh startup identity

Use cases: UC-007, UC-009. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own identity key loader/rotation files and key configuration component plus their tests. Treat hosted/internal/app/config.go and app.go as integration-owned; pass a reviewed additive key-source contract to integrator. Use existing admitted custody or mark the adapter blocked; do not silently create new cloud secrets/KMS resources.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [x] T-GR-KEYS.0 Preflight persistent signing keys requirements, design and write set  Owner: luna-keys  Est: TBD  kind: agent  stage: preflight  verifies: [UC-007, UC-009]  blocked-by: [T-GR-SCOPE.9]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]
- [x] T-GR-KEYS.1 Implement reviewed persistent key loading and rotation component  Owner: luna-keys  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-007, UC-009]  blocked-by: [T-GR-KEYS.0]  acc: [Two independently constructed issuers load the same approved key source and verify unexpired grants across restart/replica change; missing, malformed or unsafe key config fails closed; rotation preserves the agreed overlap and rejects retired keys without generating a fresh startup identity; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [x] T-GR-KEYS.2 Verify changed behavior and denial matrix  Owner: luna-verify-keys  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-009]  blocked-by: [T-GR-KEYS.1]  acc: [Restart/two-instance interoperability, wrong issuer/audience/algorithm, malformed key, stale or retired key and rotation-overlap cases; logs/errors/JWKS never reveal signing secret; Actual token mint/verify behavior is exercised through the existing issuer; composed API startup/status/body is verified in T-GR-INTEGRATE.2; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [x] T-GR-KEYS.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-keys  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-009]  blocked-by: [T-GR-KEYS.1]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [x] T-GR-KEYS.4 Independent code and security review of T-GR-KEYS.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-007, UC-009]  blocked-by: [T-GR-KEYS.2, T-GR-KEYS.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [x] T-GR-KEYS.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-007, UC-009]  blocked-by: [T-GR-KEYS.4, T-GR-KEYS.9, T-GR-SCOPE.8, T-GR-KEYS.12]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [x] T-GR-KEYS.6 Verify landed persistent signing keys behavior  Owner: luna-landed-keys  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-007, UC-009]  blocked-by: [T-GR-KEYS.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]

#### Accepted source-review remediation — 2026-10-05

- [x] T-GR-KEYS.7 Fix strict loader seed validation, bounded overlap and ambiguous JSON findings KEYS-R1/R2/R3  Owner: luna-keys  Est: TBD  kind: agent  stage: implement lane: agent  blocked-by: [T-GR-KEYS.1]  acc: [Reject inconsistent seed/public suffix and duplicate JSON fields with redacted errors; bound retired overlap to token lifetime plus skew; preserve exact-head review history and meaningful regression tests]
- [x] T-GR-KEYS.8 Verify accepted loader fixes on final candidate  Owner: luna-verify-keys  Est: TBD  kind: agent  stage: verify  blocked-by: [T-GR-KEYS.7]  acc: [New regression cases demonstrably fail before fixes and pass after; issuer restart/rotation actual tokens and package vet/race/lint pass on exact final head subject to load gate]
- [x] T-GR-KEYS.9 Independently re-review loader fixes  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-GR-KEYS.8, T-GR-KEYS.2, T-GR-KEYS.3]  acc: [Nonauthor exact-head/base reviewer records dispositions for KEYS-R1/R2/R3; merge remains dependent on this final review]

#### Accepted source review: KEYS-R4 retired KID reuse

- [x] T-GR-KEYS.10 Fix KEYS-R4 retired KID reuse  Owner: luna-keys  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-KEYS.1]  acc: [Reject retired or reused signing IDs under rotation lock; permit only byte-identical current-key no-op; an old ID cannot reset expiry or republish changed material]
- [x] T-GR-KEYS.11 Verify accepted source findings on final candidate  Owner: luna-verify-keys  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-KEYS.10]  acc: [Meaningful affected regressions fail before and pass after fix; exact final source has required scoped tests race vet and lint under actual load/lease gates; held checks never become passes]
- [x] T-GR-KEYS.12 Independently re-review accepted findings  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-KEYS.11, T-GR-KEYS.2, T-GR-KEYS.3]  acc: [Nonauthor/noncoauthor records exact head/base and finding dispositions; merge depends on this final review]

Final source review: receipt `d4f2bb71486c2d014bd4a15b50bde252f1d8f130`, exact head `da8bbf4959e2639512a108f022ddbee540c2a3b9`, base `235b1f3f86c66d6f943a910369688be46291e1d6`. PR49 rebase-merged to `190442c8098ccdd0544cc4e83247b466551c0392`; full tree parity and remote reachability verified. Landed identity unit/race/vet checks passed on actual190442c; see2026-10-05-keys-landed receipt.

### E-GR-INTERFACE -- Shared component interfaces

fidelity: executable
Acceptance: Independently reviewed shared ports and REST support land before dependent WIRE/EVENT component merges; this does not activate the composed app or resolve enrollment/provider authority.

Coordinator exclusively owns shared ports, REST errors/events/limits and their fixtures. Admitted interfaces are documented in the integration source receipt; public frozen v1 schemas remain unchanged. This source prerequisite is extracted from speculative INTEGRATE preparation to make dependency delivery explicit.

#### Wave 1: Shared component interface delivery

- [x] T-GR-INTERFACE.0 Qualify accepted component interfaces and isolated write set  acc: [Accepted KEYS/WIRE/EVENT contracts and actual baseline match; portable immutable pins, current-principal/context/budget event reads and public error mapping have bounded ownership and meaningful tests; no app activation or operator decision inferred]  Owner: coordinator  Est: TBD  kind: agent  verifies: [infrastructure]  stage: preflight  blocked-by: [T-GR-SCOPE.9] Completed 2026 10 05
- [x] T-GR-INTERFACE.1 Implement shared ports and REST support  Owner: coordinator  Est: TBD  kind: agent  verifies: [infrastructure]  stage: implement lane: agent  blocked-by: [T-GR-INTERFACE.0]  acc: [Portable persistence types represent complete immutable closure including the root skill pin (populated and tested by WIRE), supports durable request context/current principal and pre-commit budget checks, maps413/409/503 with redacted bodies and bounds client requests to server response limit; provide PR/head/base and changed file list]
- [x] T-GR-INTERFACE.2 Verify public routing and denial behavior  Owner: luna-interface-verifier  Est: TBD  kind: agent  verifies: [infrastructure]  stage: verify  blocked-by: [T-GR-INTERFACE.1]  acc: [Controlled real HTTP handler tests exercise context/current-principal handoff, resume without new cursor, client/server budget bounds and budget/foreign/expired/unavailable errors; absent real store is not promoted to durable-store proof]
- [x] T-GR-INTERFACE.3 Verify formatting vet lint and frozen contracts  Owner: luna-interface-verifier  Est: TBD  kind: agent  verifies: [infrastructure]  stage: verify  blocked-by: [T-GR-INTERFACE.1]  acc: [Owned formatting and scoped test/vet/lint pass on final source; record installed-tool/config migration accurately, Python/frozen checks pass; local evidence remains distinct from billing-blocked CI]
- [x] T-GR-INTERFACE.4 Independently review exact shared-interface candidate  Owner: independent-luna-reviewer  Est: TBD  kind: agent  verifies: [infrastructure]  stage: review  blocked-by: [T-GR-INTERFACE.2, T-GR-INTERFACE.3]  acc: [Nonauthor/noncoauthor reviews exact head/base, compatibility, cursor budget and public error boundary; stable accepted findings receive tracked fixes and fresh verification/re-review]
- [x] T-GR-INTERFACE.5 Merge reviewed shared-interface candidate  Owner: coordinator  Est: TBD  kind: agent  verifies: [infrastructure]  stage: merge  blocked-by: [T-GR-INTERFACE.4, T-GR-INTERFACE.9, T-GR-SCOPE.8, T-GR-INTERFACE.12, T-GR-INTERFACE.15, T-GR-INTERFACE.18, T-GR-INTERFACE.21]  acc: [Preserved policy and exact-head/base review plus required local checks permit guarded gh rebase merge; no protection or billing changes]
- [x] T-GR-INTERFACE.6 Verify remote landed shared interfaces  Owner: luna-interface-landed  Est: TBD  kind: agent  verifies: [infrastructure]  stage: verify-landed  blocked-by: [T-GR-INTERFACE.5]  acc: [Fresh remote main contains reviewed tree and affected source checks pass on actual landed SHA; dependent WIRE/EVENT merges then reconcile their bases and refresh invalidated evidence]

#### Accepted source-review finding INTERFACE-R1

- [x] T-GR-INTERFACE.7 Refuse unsafe legacy event reads before cursor mutation  Owner: coordinator  Est: TBD  kind: agent  verifies: [infrastructure]  stage: implement lane: agent  blocked-by: [T-GR-INTERFACE.1]  acc: [A store without budget-aware principal-bound reads receives503 before cursor creation/advance; real legacy-store regression proves retry data was not consumed; final app availability waits for safe EVENT wiring]
- [x] T-GR-INTERFACE.8 Verify fail-closed event fix and affected regressions  Owner: luna-interface-verifier  Est: TBD  kind: agent  verifies: [infrastructure]  stage: verify  blocked-by: [T-GR-INTERFACE.7]  acc: [Meaningful legacy regression fails before fix, passes after; REST test/vet/lint and schema checks pass on exact final source]
- [x] T-GR-INTERFACE.9 Independently re-review INTERFACE-R1 at final head  Owner: independent-luna-reviewer  Est: TBD  kind: agent  verifies: [infrastructure]  stage: review  blocked-by: [T-GR-INTERFACE.8, T-GR-INTERFACE.2, T-GR-INTERFACE.3]  acc: [Nonauthor/noncoauthor records final head/base and stable finding disposition; no unsafe fallback remains, and merge depends on this review]

#### Accepted review: INTERFACE-R2 initial page orphan cursor

- [x] T-GR-INTERFACE.10 Fix INTERFACE-R2 initial page orphan cursor  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-008]  blocked-by: [T-GR-INTERFACE.1]  acc: [HTTP starts require atomic budget-qualified open and page; rejected first pages cannot mutate or evict cursors; resumed requests retain current principal checks]
- [x] T-GR-INTERFACE.11 Verify atomic first-page budget behavior  Owner: luna-verify-interface  Est: TBD  kind: agent  stage: verify  verifies: [UC-008]  blocked-by: [T-GR-INTERFACE.10]  acc: [Meaningful regression fails before fix and passes after; exact final tests race vet lint and applicable real-store cap/rollback checks pass; no held or skipped-as-pass checks]
- [x] T-GR-INTERFACE.12 Independently re-review atomic first-page fix  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-008]  blocked-by: [T-GR-INTERFACE.11, T-GR-INTERFACE.2, T-GR-INTERFACE.3]  acc: [Nonauthor/noncoauthor records exact head/base and finding disposition; merge requires this final review]

#### Accepted integration finding INTERFACE-R3 frozen event serialization

- [x] T-GR-INTERFACE.13 Fix frozen v1 event serialization  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-008, UC-010]  blocked-by: [T-GR-INTERFACE.1]  acc: [Feed JSON contains exactly event_id event_type subject_ref occurred_at policy_generation; exact ID@version references and UTC RFC3339 time follow frozen schema, with workspace/domain-only fields excluded; shared serializer is used for both store budget qualification and HTTP output]
- [x] T-GR-INTERFACE.14 Verify serialized event schema and exact budget  Owner: luna-interface-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-008, UC-010]  blocked-by: [T-GR-INTERFACE.13]  acc: [Wire regression fails on old source and passes after; frozen event fields, reference and timestamp assertions plus scoped ports REST tests vet lint pass; actual PG caller uses identical serialized wrapper before cursor mutation]
- [x] T-GR-INTERFACE.15 Independently review final event serialization  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-008, UC-010]  blocked-by: [T-GR-INTERFACE.14, T-GR-INTERFACE.2, T-GR-INTERFACE.3]  acc: [Nonauthor/noncoauthor records exact head/base and frozen schema plus response-budget disposition; merge depends on this final review]

#### Accepted review finding INTERFACE-R4 frozen port bytes

- [x] T-GR-INTERFACE.16 Preserve frozen port source bytes with additive wire serializer  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-INTERFACE.13]  acc: [Place Event MarshalJSON in a new same-package additive file while retaining exact locked events.go bytes; no lock update or v1 schema drift]
- [x] T-GR-INTERFACE.17 Verify explicit freeze-lock gate and affected source  Owner: luna-interface-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-INTERFACE.16]  acc: [contracts --freeze-check fails before and passes after; plain contracts inventory is not a freeze assertion; ports REST test vet lint and canonical byte behavior are requalified on final source]
- [x] T-GR-INTERFACE.18 Independently re-review frozen port compatibility  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-INTERFACE.17, T-GR-INTERFACE.2, T-GR-INTERFACE.3]  acc: [Nonauthor/noncoauthor records exact final head/base and every lock entry, additive source compatibility and reviewed serializer; merge depends on this final review]

#### Accepted review finding INTERFACE-R5 frozen resolution ports

- [x] T-GR-INTERFACE.19 Preserve frozen resolution ports with additive pinned types  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-005, infrastructure]  blocked-by: [T-GR-INTERFACE.1]  acc: [Exact locked policy.go bytes remain; new PinnedResolution PinnedFinding ArtifactPin and PinnedResolutionStore retain immutable root and closure while existing Resolution Finding and ResolutionStore are unchanged; no lock edit]
- [x] T-GR-INTERFACE.20 Verify preserved locks and additive pinned record types  Owner: luna-interface-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-005, infrastructure]  blocked-by: [T-GR-INTERFACE.19]  acc: [Explicit contracts --freeze-check fails before and passes after on every entry; scoped ports REST test race vet lint plus existing legacy fixtures pass on final source; canonical WIRE store follows additive pinned interface]
- [x] T-GR-INTERFACE.21 Independently review frozen resolution compatibility  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-005, infrastructure]  blocked-by: [T-GR-INTERFACE.20, T-GR-INTERFACE.2, T-GR-INTERFACE.3]  acc: [Nonauthor/noncoauthor records exact final head/base and unchanged frozen ports plus rich additive persistence types; merge depends on this final review]

Completed source delivery: [PR52](https://github.com/sirerun/gist/pull/52), reviewed head `46b9a5cab5fb30709b3acf4777ad967f6a2ba411` against `190442c8098ccdd0544cc4e83247b466551c0392`, landed `8d11f53d3d94b5f4ba20545354c375708838edcf`. Independent receipt: `docs/receipts/2026-10-05-interface-final-review.md` on review branch commit `e8e0faec7e21286c632271036ba22ae7ac80c210`. Actual landed ports and REST tests plus explicit `contracts --freeze-check` passed with SSD caches and fresh load below 10. Full reviewed/landed tree equality was verified. Hosted CI did not start because of account billing; local evidence is not hosted CI or composed application acceptance.

### E-GR-WIRE -- Canonical registry wire

fidelity: executable
Acceptance: Frozen skill_ref/runtime/max_bytes inputs resolve through a principal-derived deterministic sole admitted binding; exact binding version and owned-connection semantics persist; discovery and resolution responses have actual immutable closure/digest serialization rather than accidental Go-field/base64 shapes

Use cases: UC-003, UC-004, UC-005, UC-006, UC-007, UC-010. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own resolution/wire helpers, assigned rest/catalog serialization and remotemcp input schema parity files with package tests. Frozen contracts and app resolver/lexicalAdapter edits are integrator-owned. If declared v1 cannot represent needed semantics, stop and propose a reviewed versioned amendment; do not teach clients undeclared fields.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [x] T-GR-WIRE.0 Preflight canonical registry wire requirements, design and write set  Owner: luna-wire  Est: TBD  kind: agent  stage: preflight  verifies: [UC-003, UC-004, UC-005, UC-006, UC-007, UC-010]  blocked-by: [T-GR-SCOPE.9]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]
- [x] T-GR-WIRE.1 Implement canonical decoding and explicit response serialization components  Owner: luna-wire  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-003, UC-004, UC-005, UC-006, UC-007, UC-010]  blocked-by: [T-GR-WIRE.0]  acc: [Frozen skill_ref/runtime/max_bytes inputs resolve through a principal-derived deterministic sole admitted binding; exact binding version and owned-connection semantics persist; discovery and resolution responses have actual immutable closure/digest serialization rather than accidental Go-field/base64 shapes; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [x] T-GR-WIRE.2 Verify changed behavior and denial matrix  Owner: luna-verify-wire  Est: TBD  kind: agent  stage: verify  verifies: [UC-003, UC-004, UC-005, UC-006, UC-007, UC-010]  blocked-by: [T-GR-WIRE.1, T-GR-WIRE.10]  acc: [Actual controlled HTTP/MCP request status/body and error parity fixtures for discover/exact-get/resolve; additional undeclared fields, ambiguous binding, malformed runtime ID, gateway-only, wrong-workspace, revoked or corrupted closure deny; multiple binding versions prove no hardcoded 1.0.0; no incompatible freeze-lock update; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [x] T-GR-WIRE.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-wire  Est: TBD  kind: agent  stage: verify  verifies: [UC-003, UC-004, UC-005, UC-006, UC-007, UC-010]  blocked-by: [T-GR-WIRE.1, T-GR-WIRE.10]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [x] T-GR-WIRE.4 Independent code and security review of T-GR-WIRE.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-003, UC-004, UC-005, UC-006, UC-007, UC-010]  blocked-by: [T-GR-WIRE.2, T-GR-WIRE.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [x] T-GR-WIRE.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-003, UC-004, UC-005, UC-006, UC-007, UC-010]  blocked-by: [T-GR-WIRE.4, T-GR-SCOPE.8, T-GR-INTERFACE.6, T-GR-WIRE.9, T-GR-WIRE.13, T-GR-WIRE.16, T-GR-WIRE.19]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [x] T-GR-WIRE.6 Verify landed canonical registry wire behavior  Owner: luna-landed-wire  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-003, UC-004, UC-005, UC-006, UC-007, UC-010]  blocked-by: [T-GR-WIRE.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]

#### Accepted source review: WIRE-R2/R3/R4 request ambiguity and durable root pin

- [x] T-GR-WIRE.7 Fix WIRE-R2/R3/R4 request ambiguity and durable root pin  Owner: luna-wire  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.1]  acc: [Reject case-variant duplicate request keys; persist exact root skill pin; coordinator normalizes actual catalog errors without inventing immutable metadata]
- [x] T-GR-WIRE.8 Verify accepted source findings on final candidate  Owner: luna-verify-wire  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.7]  acc: [Meaningful affected regressions fail before and pass after fix; exact final source has required scoped tests race vet and lint under actual load/lease gates; held checks never become passes]
- [x] T-GR-WIRE.9 Independently re-review accepted findings  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.8, T-GR-WIRE.2, T-GR-WIRE.3]  acc: [Nonauthor/noncoauthor records exact head/base and finding dispositions; merge depends on this final review]

- [x] T-GR-WIRE.10 Compose the canonical resolver into the real app  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-003, UC-004, UC-005, UC-007]  blocked-by: [T-GR-WIRE.1]  acc: [Coordinator-owned app.go and catalog adapter instantiate NewCanonicalResolver, remove replaced legacy adapter and normalize actual storage missing/revoked errors; controlled error fixtures and real REST/MCP use the same service; no ignored legacy request fields or uncompiled app package]

#### Accepted review finding WIRE-R5

- [x] T-GR-WIRE.11 Fix WIRE-R5 frozen resolution persistence  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.1]  acc: [Retain every frozen port byte; canonical persistence consumes additive PinnedResolutionStore and roundtrips complete principal root and closure pins using database-clock expiry and forced RLS; no lock updates]
- [x] T-GR-WIRE.12 Verify WIRE-R5 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.11]  acc: [Meaningful affected regression fails before and passes after; required scoped unit race vet lint and applicable real PostgreSQL checks pass with load lease SSD gates; held checks are not passes]
- [x] T-GR-WIRE.13 Independently re-review WIRE-R5  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.12, T-GR-WIRE.2, T-GR-WIRE.3]  acc: [Nonauthor and noncoauthor records exact final head base and accepted finding disposition; merge depends on this successful review]

#### Accepted review finding WIRE-R6

- [x] T-GR-WIRE.14 Fix WIRE-R6 duplicate keys across transports  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.1]  acc: [Preserve original resolution JSON through actual REST and MCP handlers so strict canonical duplicate-field checks run before persistence; duplicate skill_ref runtime.id and max_bytes reject through both public transports]
- [x] T-GR-WIRE.15 Verify WIRE-R6 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.14]  acc: [Meaningful affected regression fails before and passes after; required scoped unit race vet lint and applicable real PostgreSQL checks pass with load lease SSD gates; held checks are not passes]
- [x] T-GR-WIRE.16 Independently re-review WIRE-R6  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.15, T-GR-WIRE.2, T-GR-WIRE.3]  acc: [Nonauthor and noncoauthor records exact final head base and accepted finding disposition; merge depends on this successful review]

#### Accepted review finding WIRE-R7

- [x] T-GR-WIRE.17 Fix WIRE-R7 isolated PostgreSQL fixture identities  Owner: luna-wire  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.1]  acc: [Use unique bounded database and global role names preserving random suffix inside PostgreSQL identifier limits; cleanup only exact owned resources and handle partial creation failures; independent concurrent and stale-role qualification never drops another lane role]
- [x] T-GR-WIRE.18 Verify WIRE-R7 at exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.17]  acc: [Meaningful old-source regression and corrected actual PostgreSQL behavior qualify on final source with required scoped tests race vet lint and fresh load lease SSD controls; no fixture collision or held command is a pass]
- [x] T-GR-WIRE.19 Independently re-review WIRE-R7  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-WIRE.18, T-GR-WIRE.2, T-GR-WIRE.3]  acc: [Nonauthor and noncoauthor names exact final head/base and confirms finding disposition; merge depends on this successful final review]

Final source qualification: PR50 head `9ac3263b3c792413a13a02c7b6c66f1949148cac`, base `f162015238a94fbc930865f70bef014865f553d4`. Author affected unit/race/vet/lint and repeated tagged PostgreSQL app tests passed. Independent final review receipt commit `5025d602851d9a3b1835c98bb2908fbe82170b20` on `review/gist-wire-final-20261005` records actual tagged PostgreSQL replay, unchanged R6 production bytes, R7 isolation/cleanup, explicit freeze and range diff checks. Merge and landed behavior remain separate open rows.

### E-GR-EVENT -- Durable revocation feed

fidelity: executable
Acceptance: Publication/revocation events and cursors survive restart and replica change using durable tenant-scoped storage; publish/revoke commit and feed visibility remain consistent; no hidden process-local event authority

Use cases: UC-002, UC-007, UC-008. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own new storage event/cursor files and their integration tests plus allocated migration changes, reusing migration004 tables. Preserve existing event vocabulary and retention/cursor contracts. EVENT owns the storage-level atomic revoke/outbox implementation in storage/revocations.go; shared storage bootstrap, app-level publisher/revoker adapters and app.go wiring remain integration-owned. Preserve seven-day retention, five-minute sliding cursor TTL, 16 active cursors per principal with oldest eviction and 100 events per page.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [x] T-GR-EVENT.0 Preflight durable revocation feed requirements, design and write set  Owner: luna-event  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-007, UC-008]  blocked-by: [T-GR-SCOPE.9]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; freeze seven-day retention, five-minute sliding cursor TTL, 16/principal oldest-eviction cap and 100/page, tenant retention-floor semantics and budget-failure/no-lost-page behavior; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]
- [x] T-GR-EVENT.1 Implement shared transactional outbox and principal-bound cursor persistence  Owner: luna-event  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-007, UC-008]  blocked-by: [T-GR-EVENT.0]  acc: [Publication/revocation events and cursors survive restart and replica change using durable tenant-scoped storage; publish/revoke commit and feed visibility remain consistent; no hidden process-local event authority; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [x] T-GR-EVENT.2 Verify changed behavior and denial matrix  Owner: luna-verify-event  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-007, UC-008]  blocked-by: [T-GR-EVENT.1]  acc: [Real PostgreSQL publish/revoke-to-feed, transaction failure, replay/ordering, restart, two replica read, foreign principal/workspace, five-minute sliding expiry, seven-day retention gap versus inter-tenant sequence gaps, 16/principal oldest-eviction cap, 100/page limit, over-budget response with no silently lost events and unavailable-store cases; actual GET /v1/events status/body demonstrates cursor_expired versus service_unavailable; already-sent effects are not promised cancelled; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [x] T-GR-EVENT.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-event  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-007, UC-008]  blocked-by: [T-GR-EVENT.1]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [x] T-GR-EVENT.4 Independent code and security review of T-GR-EVENT.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-002, UC-007, UC-008]  blocked-by: [T-GR-EVENT.2, T-GR-EVENT.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [x] T-GR-EVENT.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-002, UC-007, UC-008]  blocked-by: [T-GR-EVENT.4, T-GR-SCOPE.8, T-GR-INTERFACE.6, T-GR-EVENT.9, T-GR-EVENT.12, T-GR-EVENT.15, T-GR-EVENT.18]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [x] T-GR-EVENT.6 Verify landed durable revocation feed behavior  Owner: luna-landed-event  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-002, UC-007, UC-008]  blocked-by: [T-GR-EVENT.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]

#### Accepted source review: EVENT-R1/R2 budget and database-clock findings

- [x] T-GR-EVENT.7 Fix EVENT-R1/R2 budget and database-clock findings  Owner: luna-event  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.1]  acc: [Nonpositive budgets deny before cursor mutation; expiry validation and writes use authoritative database clock after locks; record real retention-maintenance API and actual scheduler dependency]
- [x] T-GR-EVENT.8 Verify accepted source findings on final candidate  Owner: luna-verify-event  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.7]  acc: [Meaningful affected regressions fail before and pass after fix; exact final source has required scoped tests race vet and lint under actual load/lease gates; held checks never become passes]
- [x] T-GR-EVENT.9 Independently re-review accepted findings  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.8, T-GR-EVENT.2, T-GR-EVENT.3]  acc: [Nonauthor/noncoauthor records exact head/base and finding dispositions; merge depends on this final review]

#### Accepted review: EVENT-R3 atomic initial page

- [x] T-GR-EVENT.10 Fix EVENT-R3 atomic initial page  Owner: luna-events  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-008]  blocked-by: [T-GR-EVENT.1]  acc: [Postgres initial subscription qualifies exact empty page bytes before pruning or inserting; undersized or nonpositive budgets cannot evict any of sixteen existing cursors]
- [x] T-GR-EVENT.11 Verify atomic first-page budget behavior  Owner: luna-verify-event  Est: TBD  kind: agent  stage: verify  verifies: [UC-008]  blocked-by: [T-GR-EVENT.10]  acc: [Meaningful regression fails before fix and passes after; exact final tests race vet lint and applicable real-store cap/rollback checks pass; no held or skipped-as-pass checks]
- [x] T-GR-EVENT.12 Independently re-review atomic first-page fix  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-008]  blocked-by: [T-GR-EVENT.11, T-GR-EVENT.2, T-GR-EVENT.3]  acc: [Nonauthor/noncoauthor records exact head/base and finding disposition; merge requires this final review]

#### Accepted lint finding EVENT-R4 object cleanup

- [x] T-GR-EVENT.13 Fix required storage object cleanup errors  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.1]  acc: [Owned temporary-file cleanup propagates actual removal failures while ignoring an already-renamed temporary file; failed writable-file close joins the write error; read-only S3 body cleanup is explicit under the Go skill exception; source commits e671e25 and ae92522 are handed off without unrelated file writes]
- [x] T-GR-EVENT.14 Verify full storage source including object cleanup  Owner: luna-verify-events  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.13]  acc: [Storage unit real-PG integration race vet and faithful migrated-config lint pass on final source; write/persist/object tests cover changed storage behavior; no old source or held lint is upgraded to final pass]
- [x] T-GR-EVENT.15 Independently review object cleanup and final storage source  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.14, T-GR-EVENT.2, T-GR-EVENT.3]  acc: [Nonauthor/noncoauthor covers root-owned object cleanup plus EVENT candidate final head/base; no discarded write/cleanup failure and no disabled lint; merge depends on this review]

#### Accepted review finding EVENT-R5

- [x] T-GR-EVENT.16 Fix EVENT-R5 deferred temporary cleanup return  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.1]  acc: [Deferred temporary-file removal errors join the actual named return slot; both successful commit and primary write or rename failure retain cleanup errors; per-instance fault injection cannot affect other backends]
- [x] T-GR-EVENT.17 Verify EVENT-R5 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.16]  acc: [Meaningful affected old-source failures and corrected behavior pass with actual applicable unit PostgreSQL race vet lint and fresh load lease SSD checks; no skipped or held check counts as a pass]
- [x] T-GR-EVENT.18 Independently re-review EVENT-R5  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-EVENT.17, T-GR-EVENT.2, T-GR-EVENT.3]  acc: [Nonauthor and noncoauthor records exact final head/base and finding disposition; guarded merge waits for successful final review]

### E-GR-TREG -- Treg action artifacts

fidelity: executable
Acceptance: One selected exact endpoint has real pinned schemas/provenance, method/recipient/effect/credential-route and cost-unit/upper-bound metadata; complete provider/tool/binding/skill bytes have verified digests and a caller-execution requirement; no generic call grants blanket read authority

Use cases: UC-002, UC-004, UC-005, UC-010. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own newly allocated catalog/registry/captures/treg-* and catalog/registry/packages/treg-* plus capture/conformance tests in separately assigned files. Integrator alone changes shared tools/providers/bindings/capabilities indexes. First endpoint/task and metadata redistribution must be evidenced, not guessed from a docs example.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [ ] T-GR-TREG.0 Preflight treg action artifacts requirements, design and write set  Owner: luna-treg  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-SCOPE.9, T-GR-SCOPE.10]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]  blocked: Exact schema and request mapping, license and retention, account and credential route, owner-selected target and limits remain unqualified; see October6 preflight receipt
- [ ] T-GR-TREG.1 Capture and publish one real immutable Treg read capability closure  Owner: luna-treg  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-TREG.0]  acc: [One selected exact endpoint has real pinned schemas/provenance, method/recipient/effect/credential-route and cost-unit/upper-bound metadata; complete provider/tool/binding/skill bytes have verified digests and a caller-execution requirement; no generic call grants blanket read authority; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [ ] T-GR-TREG.2 Verify changed behavior and denial matrix  Owner: luna-verify-treg  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-TREG.1]  acc: [Offline schema/closure/import fixtures reject missing or drifted schema, unknown pricing, disallowed recipient/overflow route, changed binding/digest and synthetic metadata; external capture methods are documented and require separate scoped account authority when authenticated; no billed endpoint call in this lane; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [ ] T-GR-TREG.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-treg  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-TREG.1]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [ ] T-GR-TREG.4 Independent code and security review of T-GR-TREG.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-TREG.2, T-GR-TREG.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [ ] T-GR-TREG.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-TREG.4, T-GR-SCOPE.8]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [ ] T-GR-TREG.6 Verify landed treg action artifacts behavior  Owner: luna-landed-treg  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-TREG.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]


2026-10-06 metadata-only preflight: [current source and remaining blockers](receipts/2026-10-06-treg-artifact-preflight.md). The exact source/version/license/account/target qualification is incomplete; .0 remains open and no executable artifact or live call is admitted.

### E-GR-COMPOSIO -- Composio action artifacts

fidelity: executable
Acceptance: Actual selected GitHub PR-read or subsequently approved action schema/version/account/session constraints form a complete immutable closure; preserve historical source_unavailable/synthetic fixtures; generic execute/meta/proxy/workbench access and instant charging are excluded

Use cases: UC-002, UC-004, UC-005, UC-010. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own new catalog/registry/captures/composio-* and catalog/registry/packages/composio-* plus assigned conformance tests. Shared catalog indexes and canonical binding selection are integration-owned. Evidence license/redistribution and exact session transport/version before public capture; a docs URL is not runtime conformance.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [ ] T-GR-COMPOSIO.0 Preflight composio action artifacts requirements, design and write set  Owner: luna-composio  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-SCOPE.9, T-GR-SCOPE.10]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]  blocked: Qualified metadata-only key binding and capture authority, action schemas and mapping, license and retention, account and target limits remain unqualified; see October6 preflight receipt
- [ ] T-GR-COMPOSIO.1 Capture and publish one real Composio action-specific capability closure  Owner: luna-composio  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-COMPOSIO.0]  acc: [Actual selected GitHub PR-read or subsequently approved action schema/version/account/session constraints form a complete immutable closure; preserve historical source_unavailable/synthetic fixtures; generic execute/meta/proxy/workbench access and instant charging are excluded; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [ ] T-GR-COMPOSIO.2 Verify changed behavior and denial matrix  Owner: luna-verify-composio  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-COMPOSIO.1]  acc: [Offline schema/closure/import tests reject synthetic identity action, foreign account/user/toolkit, extra operation, schema/config drift, undeclared fees and unsupported version pinning; capture x-api-key custody/header requirements without secrets; no provider session creation or live call absent scoped authority; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [ ] T-GR-COMPOSIO.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-composio  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-COMPOSIO.1]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [ ] T-GR-COMPOSIO.4 Independent code and security review of T-GR-COMPOSIO.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-COMPOSIO.2, T-GR-COMPOSIO.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [ ] T-GR-COMPOSIO.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-COMPOSIO.4, T-GR-SCOPE.8]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [ ] T-GR-COMPOSIO.6 Verify landed composio action artifacts behavior  Owner: luna-landed-composio  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-GR-COMPOSIO.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]


2026-10-06 metadata-only preflight: [current source and remaining blockers](receipts/2026-10-06-composio-artifact-preflight.md). The exact source/version/license/account/target qualification is incomplete; .0 remains open and no executable artifact or live call is admitted.

### E-GR-PUBLISH -- Canonical publication

fidelity: executable
Acceptance: An explicitly settled publication transport publishes every agreed artifact kind through actual authenticated HTTP with immutable bytes, typed conformance, bounded response and replay semantics. Existing route doubles and catalog seeding are not publication acceptance.

Accepted finding PUBLISH-R1: frozen publish.schema requires artifact/max_bytes/idempotency_key but leaves the artifact-object representation unspecified; OpenAPI refers to a different generic body, REST forwards opaque bytes and the app treats every kind as a skill ZIP. No base64, multipart or file-map representation is selected implicitly. Existing frozen entries remain byte-identical until an explicitly approved versioned amendment. Coordinator owns contract integration; a disjoint Luna design author may draft the decision and fixtures without activating it.

#### Wave 1: Contract settlement and source delivery

- [x] T-GR-PUBLISH.7 Draft grounded publication transport decision and compatibility path  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-SCOPE.9]  acc: [Compare current frozen schema OpenAPI SDK and actual upload/import paths; provide concrete envelope and skill byte transport alternatives plus recommendation, per-kind typed validation, idempotency byte-budget and catalog/outbox atomicity design; preserve v1 locks and existing caller-owned boundary]
- [x] T-GR-PUBLISH.8 Settle material publication contract decision  stage: preflight  acc: [Review concrete compatibility proposal and explicitly settle canonical artifact representation, skill package transport and any versioned amendment before changing public semantics; offline silence is not approval]  Owner: founder-and-contract-owner  Est: TBD  kind: human  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-PUBLISH.7]  Done: 2026-10-08  Evidence: [owner approval](receipts/2026-10-08-alignment-and-dgx-preflight.md); accepted v2 envelope/base64 skill ZIP/typed raw JSON, frozen v1 unchanged with explicit migration path; no provider/release/deployment grant
- [x] T-GR-PUBLISH.0 Qualify publication contract write set and actual source fixtures  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-PUBLISH.8, T-GR-INTERFACE.6]  acc: [Exact approved schemas and method/media/version binding, bounded publisher write set, tenant-safe staged object ownership/retention/reconciliation and real local acceptance fixtures are recorded; preserve concurrent ownership and use isolated qualified DGX worktree]  Done: 2026-10-09  Evidence: [exact source contracts and fixture preflight](registry/publication-v2-preflight.md); independent review/merge/landed gates9/10/11 remain open
- [ ] T-GR-PUBLISH.9 Independently review frozen v2 publication preflight  Owner: independent-luna-reviewer  Est: 30m  kind: agent  stage: review  blocked-by: [T-GR-PUBLISH.0, T-GR-PUBLISH.14]  acc: [Nonauthor independently approves exact base/head schemas method/media limits digest mappings migration behavior tenant staging retention/reconciliation and implementation fixture/write set; accepted findings retain tracked fix verification and re-review obligations before handlers are written]
- [ ] T-GR-PUBLISH.10 Guarded merge of frozen publication preflight  Owner: coordinator  Est: 15m  kind: agent  stage: merge  blocked-by: [T-GR-PUBLISH.9, T-GR-SCOPE.8]  acc: [Reviewed exact head merges by guarded gh rebase under current unchanged policy; local evidence remains distinct from unavailable billing-only hosted CI; actual source landing is recorded without runtime authority]
- [ ] T-GR-PUBLISH.11 Verify actual landed v2 publication contract and preflight  Owner: coordinator  Est: 20m  kind: agent  stage: verify-landed  blocked-by: [T-GR-PUBLISH.10]  acc: [Fresh remote main contains independently reviewed schema and design bytes; actual landed contract fixtures frozen-v1 parity and full plan conformance pass before dependent production handlers start; no provider release or deployment acceptance inferred]
- [x] T-GR-PUBLISH.12 Fix accepted PUBLISH59-R1 receipt whitespace  Owner: coordinator  Est: 10m  kind: agent  stage: author  blocked-by: [T-GR-PUBLISH.0]  acc: [Remove only trailing spaces from the copied PR57 landed receipt while preserving all original wording and exact original report outside this candidate; retain the negative independent review and no approval inference]  Done: 2026-10-09  Evidence: [preserved PUBLISH59-R1](receipts/2026-10-09-publication-preflight-review-r1.md)
- [x] T-GR-PUBLISH.13 Verify PUBLISH59-R1 fix against the complete committed candidate  Owner: coordinator  Est: 15m  kind: agent  stage: verify  blocked-by: [T-GR-PUBLISH.12]  acc: [Full base-to-head diff whitespace, graph links archive/frozen-v1/hosted parity contract gate and complete Wazi conformance pass; untracked-file omissions cannot count as full candidate verification]  Done: 2026-10-09
- [ ] T-GR-PUBLISH.14 Independently re-review exact corrected publication preflight  Owner: independent-luna-reviewer  Est: 30m  kind: agent  stage: review  blocked-by: [T-GR-PUBLISH.13]  acc: [Nonauthor resolves PUBLISH59-R1 and approves the complete exact corrected base/head candidate including all new recovery tasks and receipts; additional accepted findings require tracked repair verification and re-review before merge]
- [ ] T-GR-PUBLISH.1 Implement canonical per-kind publication  Owner: luna-publication  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-PUBLISH.11]  acc: [Actual handlers consume approved canonical envelope, strict per-kind schema and immutable package closure, authenticated current maintainer and namespace admission, requested/server byte budget before mutation, replay identity, catalog/outbox atomicity and qualified staged object reconciliation without deleting shared committed digests; no skill validator substituted for other artifact kinds]
- [ ] T-GR-PUBLISH.2 Verify publication journeys and denials with real stores  Owner: luna-publication-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-007, UC-008, UC-010]  blocked-by: [T-GR-PUBLISH.1]  acc: [Actual authenticated HTTP plus restricted-role PostgreSQL and object store publish and retrieve each agreed kind; meaningful old-source RED and new-source PASS, repeat same bytes/key, changed bytes conflict, malformed/foreign/unauthorized/revoked namespace, oversized request/response and transaction/outbox rollback and physical staging ownership/retention/reconciliation qualify without seeded-catalog success claims]
- [ ] T-GR-PUBLISH.3 Verify publication compatibility race vet lint and contract checks  Owner: luna-publication-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-001, infrastructure]  blocked-by: [T-GR-PUBLISH.1]  acc: [Required affected hosted/root CLI unit integration race vet lint schema checks pass on exact final source using fresh load shared lease and SSD rules; approved versioned changes have explicit compatibility fixtures and legacy locks remain intact]
- [ ] T-GR-PUBLISH.4 Independently review final publication candidate  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-002, UC-004, UC-007, UC-008, UC-010]  blocked-by: [T-GR-PUBLISH.2, T-GR-PUBLISH.3]  acc: [Nonauthor and noncoauthor records exact head/base contract approval and integrity/tenant/cost/idempotency/budget/outbox dispositions; accepted findings receive tracked fixes verification and re-review]
- [ ] T-GR-PUBLISH.5 Guarded merge of reviewed publication  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-PUBLISH.4, T-GR-SCOPE.8]  acc: [Immediate final head/base policy and relevant dependency comparison permits guarded gh rebase merge without policy changes; merge is source delivery only]
- [ ] T-GR-PUBLISH.6 Verify actual remote-main publication  Owner: luna-publication-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-002, UC-004, UC-007, UC-008, UC-010]  blocked-by: [T-GR-PUBLISH.5]  acc: [Fresh remote main contains reviewed source and actual relevant HTTP/store acceptance passes at recorded landed SHA; no release production enrollment or provider execution inferred]

Historical proposal-draft delivery on2026-10-05: ADR012 and its required compatibility, budget and raw-byte decisions were independently reviewed and landed in PR54 at d75ac183fe8b949a4f1381e8ebca3226f0c0e282. Owner approval was pending at that drafting checkpoint. The2026-10-08 approval below supersedes that status: PUBLISH.8 is complete and source contract preflight is ready. [Landed plan receipt](receipts/2026-10-05-plan-landed-verification.md).

2026-10-08 owner decision: ADR012 v2 source implementation approved. T-GR-PUBLISH.0 can freeze the exact versioned route/schema/readback/strict byte and archive limits, raw-byte fidelity, canonical persistence/outbox and tenant-safe orphan lifecycle. Frozen v1 remains unchanged; production migration/deprecation claims require explicit release evidence. No provider or deployment gate is waived.

### E-GR-CORE -- Startup and event composition

fidelity: executable
Acceptance: Independently reviewed core application startup and shared event composition land and pass real local replica/tenant tests. This is code delivery, with deployment credentials, enrollment/provider and final production acceptance still required by their original gates.

Integrator owns app startup/config, publisher/revoker adapters, retention scheduler and command config. A designated fixture author may own only the newly assigned composition integration test file in an isolated worktree. This extracts source preparation already permitted in INTEGRATE.0/.1 into an independently runnable delivery chain, removing unrelated enrollment/provider capture from the core source merge barrier. Final INTEGRATE remains dependent on AUTH/TREG/COMPOSIO and core landed acceptance. No operator, maintenance subject, secret source or AWS binding is selected implicitly.

#### Wave 2: Core source delivery

- [x] T-GR-CORE.0 Reconcile actual landed core interfaces and maintenance design  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-007, UC-008, UC-009]  blocked-by: [T-GR-SCOPE.9, T-GR-INTERFACE.6, T-GR-KEYS.6, T-GR-WIRE.6, T-GR-EVENT.6]  acc: [Revalidate exact landed key/resolver/event API and forced-RLS transaction contracts; explicitly owned startup/config/outbox/cancellation fixtures and maintenance bounds match; deployment input remains unapproved until existing operator/custody gates pass]
- [x] T-GR-CORE.1 Compose persistent startup identity and tenant event feed  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-007, UC-008, UC-009]  blocked-by: [T-GR-CORE.0]  acc: [Startup requires explicit strict key set and trusted configured existing maintainers; real HTTP/MCP services use canonical resolver and Postgres atomic first-page feed; publication/revocation and outbox are joined in tenant transactions; one-minute bounded maintenance refreshes current membership/generation/role before purge and exits on cancellation; no random startup identity or process-local feed remains]
- [x] T-GR-CORE.2 Verify actual replica restart and tenant transaction behavior  Owner: luna-core-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-008, UC-009]  blocked-by: [T-GR-CORE.1]  acc: [Actual New/Handler plus disposable PostgreSQL NOSUPERUSER NOBYPASSRLS role qualify shared JWKS/token/cursor across replicas and restart, missing/malformed-key denial, policy/issuer/generation/tenant denial, publication/revocation rollback and repeated revocation, seven-day bounded per-tenant purge, failed maintenance readiness and cancellation; fixture membership seeding is explicitly not external enrollment proof]
- [x] T-GR-CORE.3 Verify affected modules race lint and frozen schemas  Owner: luna-core-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-001, infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Affected hosted source/test/tagged integration/race/vet/lint plus CLI config and root compatibility checks pass on final exact source; frozen wire fixtures validate; fresh load lease SSD rules apply and CI billing is distinguished]
- [x] T-GR-CORE.4 Independently review final startup and event composition  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor/noncoauthor records exact head/base and key custody config/redaction, current-policy maintenance, tenant transaction boundaries, canonical budget and cancellation dispositions; accepted findings have explicit fixes and fresh verification/review]
- [x] T-GR-CORE.5 Guarded merge of reviewed core source  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-CORE.4, T-GR-SCOPE.8, T-GR-CORE.9, T-GR-CORE.12, T-GR-CORE.15, T-GR-CORE.18, T-GR-CORE.21, T-GR-CORE.24, T-GR-CORE.27, T-GR-CORE.30, T-GR-CORE.33, T-GR-CORE.36, T-GR-CORE.39, T-GR-CORE.42]  acc: [Immediate exact head/base and policy comparison permits guarded gh rebase merge; record actual landed SHA; no deployment, runtime credential selection or production status inferred]
- [x] T-GR-CORE.6 Verify core behavior on actual remote main  Owner: luna-core-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-007, UC-008, UC-009]  blocked-by: [T-GR-CORE.5]  acc: [Fresh remote main contains reviewed source and actual affected real-store/API/MCP tests pass at recorded landed SHA; final INTEGRATE then joins external enrollment/provider/consumer gates and production remains open]

#### Accepted review finding CORE-R1

- [x] T-GR-CORE.7 Fix CORE-R1 bounded retention catchup  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Purge bounded batches within current authorization and request deadline; retention backlog fails readiness rather than silently reporting healthy after only one batch]
- [x] T-GR-CORE.8 Verify CORE-R1 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.7]  acc: [Meaningful affected regression fails before and passes after; required scoped unit race vet lint and applicable real PostgreSQL checks pass with load lease SSD gates; held checks are not passes]
- [x] T-GR-CORE.9 Independently re-review CORE-R1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.8, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor records exact final head base and accepted finding disposition; merge depends on this successful review]

#### Accepted review finding CORE-R2

- [x] T-GR-CORE.10 Fix CORE-R2 strict maintenance target decoding  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Reject duplicate unknown and case-variant fields trailing JSON duplicate workspaces missing and empty fields before any database operation; only exact configured current trusted maintainers are eligible]
- [x] T-GR-CORE.11 Verify CORE-R2 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.10]  acc: [Meaningful affected regression fails before and passes after; required scoped unit race vet lint and applicable real PostgreSQL checks pass with load lease SSD gates; held checks are not passes]
- [x] T-GR-CORE.12 Independently re-review CORE-R2  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.11, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor records exact final head base and accepted finding disposition; merge depends on this successful review]

#### Accepted review finding CORE-R3

- [x] T-GR-CORE.13 Fix CORE-R3 target failure isolation  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Attempt every configured authorized workspace despite one target failure within the bounded request deadline and aggregate failures without bypassing current membership or policy]
- [x] T-GR-CORE.14 Verify CORE-R3 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.13]  acc: [Meaningful affected regression fails before and passes after; required scoped unit race vet lint and applicable real PostgreSQL checks pass with load lease SSD gates; held checks are not passes]
- [x] T-GR-CORE.15 Independently re-review CORE-R3  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.14, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor records exact final head base and accepted finding disposition; merge depends on this successful review]

#### Accepted review finding CORE-R4

- [x] T-GR-CORE.16 Fix CORE-R4 startup database deadline  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Bound pre-traffic startup maintenance by configured RequestTimeout while retaining caller cancellation; database lock wait cannot leave construction indefinitely pending with a background caller context; failed startup never advertises readiness]
- [x] T-GR-CORE.17 Verify CORE-R4 at exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.16]  acc: [Meaningful old-source regression and corrected actual PostgreSQL behavior qualify on final source with required scoped tests race vet lint and fresh load lease SSD controls; no fixture collision or held command is a pass]
- [x] T-GR-CORE.18 Independently re-review CORE-R4  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.17, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor names exact final head/base and confirms finding disposition; merge depends on this successful final review]

#### Accepted review finding CORE-R5

- [x] T-GR-CORE.19 Fix CORE-R5 exact bounded retention completion  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [After the maximum allowed delete batches refresh current authority and use a principal-bound forced-RLS database-clock existence probe without an extra deletion; exactly20000 rows complete healthy while20001 remains bounded and unavailable]
- [x] T-GR-CORE.20 Verify CORE-R5 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.19]  acc: [Meaningful affected old-source failures and corrected behavior pass with actual applicable unit PostgreSQL race vet lint and fresh load lease SSD checks; no skipped or held check counts as a pass]
- [x] T-GR-CORE.21 Independently re-review CORE-R5  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.20, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor records exact final head/base and finding disposition; guarded merge waits for successful final review]

#### Accepted review finding CORE-R6

- [x] T-GR-CORE.22 Fix CORE-R6 publication server response budget  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Serialize the complete existing component publication response and qualify server budget before object staging catalog or outbox mutation; preserve response fields with safe JSON serialization and actual component success rollback and over-budget fixtures; no canonical HTTP envelope proof inferred]
- [x] T-GR-CORE.23 Verify CORE-R6 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.22]  acc: [Meaningful affected old-source failures and corrected behavior pass with actual applicable unit PostgreSQL race vet lint and fresh load lease SSD checks; no skipped or held check counts as a pass]
- [x] T-GR-CORE.24 Independently re-review CORE-R6  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.23, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor records exact final head/base and finding disposition; guarded merge waits for successful final review]

#### Accepted review finding CORE-R7

- [x] T-GR-CORE.25 Fix CORE-R7 maintenance Unicode authority input  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Reject invalid UTF8 and unpaired escaped UTF16 surrogates before maintenance identity decoding; accept valid paired Unicode and literal escaped backslashes without guessing or replacement of authority identity]
- [x] T-GR-CORE.26 Verify CORE-R7 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.25]  acc: [Meaningful affected old-source failures and corrected behavior pass with actual applicable unit PostgreSQL race vet lint and fresh load lease SSD checks; no skipped or held check counts as a pass]
- [x] T-GR-CORE.27 Independently re-review CORE-R7  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.26, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor records exact final head/base and finding disposition; guarded merge waits for successful final review]

#### Accepted review finding CORE-R8

- [x] T-GR-CORE.28 Fix CORE-R8 shutdown caller deadline  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Shutdown caller timeout bounds waiting for janitor completion while cancellation and exactly-once owned cleanup continue; repeated callers receive actual cleanup result rather than fabricated success; actual active-query cancellation and bounded-wait fixtures qualify]
- [x] T-GR-CORE.29 Verify CORE-R8 on exact source  Owner: luna-verifier  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.28]  acc: [Meaningful affected old-source failures and corrected behavior pass with actual applicable unit PostgreSQL race vet lint and fresh load lease SSD checks; no skipped or held check counts as a pass]
- [x] T-GR-CORE.30 Independently re-review CORE-R8  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.29, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor records exact final head/base and finding disposition; guarded merge waits for successful final review]

#### Accepted fixture compatibility finding CORE-R9

- [x] T-GR-CORE.31 Adapt CORE-R9 existing acceptance startup fixtures  Owner: luna-fixture-author  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-007, UC-008, infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Existing retrieval wiring and named-client local fixtures supply explicit test-only persistent signing configuration and preexisting dedicated maintenance grants before actual app startup; preserve original reader client and adversary grants; no production authority selection or skipped acceptance]
- [x] T-GR-CORE.32 Verify CORE-R9 fixture startup compatibility  Owner: luna-fixture-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-008, infrastructure]  blocked-by: [T-GR-CORE.31]  acc: [Meaningful previous startup failures and corrected affected real PostgreSQL fixture startup pass on exact source; required full hosted unit vet and affected integration race lint remain qualified with load lease SSD rules; existing canonical publication acceptance failures stay explicit]
- [x] T-GR-CORE.33 Independently re-review CORE-R9  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.32, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor binds final exact head/base and verifies test authority seed order dedicated maintained scope preservation and actual checks; core merge waits for successful review]

#### Accepted fixture conformance finding CORE-R10

- [x] T-GR-CORE.34 Migrate CORE-R10 legacy acceptance resolve fixtures  Owner: coordinator-and-luna-fixture-author  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-004, UC-008, infrastructure]  blocked-by: [T-GR-CORE.1, T-GR-WIRE.6]  acc: [Existing local HTTP and MCP fixtures send frozen skill_ref/runtime/max_bytes instead of obsolete skill/runtime_id/local_execution, read aggregate readiness and seed the exact manifest with matching immutable id/version; preserve successful readiness and revoked-artifact denial assertions without public schema changes or weaker outcomes]
- [x] T-GR-CORE.35 Verify CORE-R10 frozen request compatibility  Owner: luna-fixture-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-008, infrastructure]  blocked-by: [T-GR-CORE.34, T-GR-CORE.31]  acc: [Meaningful old-shape validation failures and corrected real HTTP/MCP resolve success and revoked-artifact denial pass on exact source; affected local fixture tests race vet lint qualify; named external client/runtime receipts and canonical publication remain separate gates]
- [x] T-GR-CORE.36 Independently re-review CORE-R10  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.35, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor binds final exact head/base and confirms immutable canonical schemas and preserved positive/negative test intent; core merge waits for this successful review]

#### Accepted plan dispatch finding CORE-R11

- [x] T-GR-CORE.37 Qualify CORE-R11 cross-epic dependency references  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Active recovery dependencies crossing epic namespaces use literal fully qualified task IDs accepted by the installed dispatcher; preserve all original task IDs authored states owners acceptance and dependency semantics without patching global tools or treating human gates as approved]
- [x] T-GR-CORE.38 Verify CORE-R11 actual dispatcher dependency resolution  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.37]  acc: [Actual default-plan parser resolves every active dependency literally or within the source namespace; no missing edges duplicates cycles or wave diagnostics; original graph edges stay identical and every active task joins PROD.9; CORE.34 becomes open when its implemented prerequisites are done while pending human inputs remain blocked]
- [x] T-GR-CORE.39 Independently re-review CORE-R11  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.38, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor and noncoauthor binds final exact head/base and confirms literal dispatcher edges complete and production gates remain enforced; guarded core merge joins successful final review]

#### Accepted receipt formatting finding CORE-R12

- [x] T-GR-CORE.40 Remove CORE-R12 receipt trailing whitespace  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-CORE.1]  acc: [Remove only Markdown trailing spaces in the owner decision receipt while preserving its recommendations and pending decisions]
- [x] T-GR-CORE.41 Verify CORE-R12 complete candidate diff formatting  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-CORE.40]  acc: [git diff --check against exact reviewed base passes for the complete committed candidate and working delta; actual default-plan parser resolves all active prerequisites and preserves production gates]
- [x] T-GR-CORE.42 Independently re-review CORE-R12  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-CORE.41, T-GR-CORE.2, T-GR-CORE.3]  acc: [Nonauthor binds final exact head/base and confirms receipt whitespace fix preserves all pending owner decisions and complete candidate formatting passes; guarded merge joins this review]

Independent source review: [immutable d746/base d75 review receipt](https://github.com/sirerun/gist/blob/bec089404cdd0376f7e9589c80fac328e1dc7802/docs/receipts/2026-10-05-core-independent-review-final.md) passes CORE and all twelve finding dispositions. Review checkboxes reflect that executed review; guarded merge still requires independent comparison of this final receipt/checkbox-only head with the same base. Production predicates remain open.


CORE.5/.6 completed: PR55 landed b91ec511153c5e2ed57569be532e0401ab8121a8 with exact reviewed tree parity and fresh actual-source checks. [CORE landed receipt](receipts/2026-10-06-core-landed-verification.md) distinguishes fresh runs, carried checks and programmatic client fixture limits. Final INTEGRATE and PROD.9 remain gated.

### E-GR-INTEGRATE -- Composition and interoperability

fidelity: executable
Acceptance: A reviewed, landed composed registry passes meaningful local integration gates; actual external runtime and production remain separately gated.

Sole writer for hosted/internal/app/app.go, app/config.go, shared ports/store bootstrap, publisher/revoker composition, shared catalog indexes, final assigned migrations and joined acceptance fixtures. Integrator reconciles component interfaces rather than guessing unavailable adapters. Consumer code remains externally owned.

Core startup/key/event preparation is now separately tracked through E-GR-CORE; its independent source merge does not close the external enrollment/provider/consumer gates below.

Speculative dependency: source preparation of shared interfaces and integration fixtures may start after T-GR-SCOPE.9 against admitted component candidate interfaces. This is preparatory work within T-GR-INTEGRATE.0/.1; the existing landed-component dependencies must still pass before completing these rows or activating a final composed candidate. No trust selection, external provider capture or production change is implied.

#### Wave 2: Composition and interoperability SDLC

- [ ] T-GR-INTEGRATE.0 Reconcile landed interfaces and integration design  Owner: luna-integrator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009, UC-010]  blocked-by: [T-GR-PUBLISH.6, T-GR-CORE.6, T-GR-INTERFACE.6, T-GR-AUTH.6, T-GR-KEYS.6, T-GR-WIRE.6, T-GR-EVENT.6, T-GR-TREG.6, T-GR-COMPOSIO.6]  acc: [Record actual landed components and merged dependency graph, key/identity/event transaction contracts, real captures and immutable closure; re-read current main and reserve all shared files; no placeholder component is wired as success]
- [ ] T-GR-INTEGRATE.1 Compose final enrollment provider and consumer journey with landed core  Owner: luna-integrator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009, UC-010]  blocked-by: [T-GR-INTEGRATE.0]  acc: [Real app uses sanctioned enrollment/grants, persistent signing identity, canonical REST/MCP contract serialization, exact provider closure and durable outbox/cursors; remove replaced unsafe startup paths; PR exact-head handoff includes updated existing ADR/design decisions and no incompatible v1 or gateway route]
- [ ] T-GR-INTEGRATE.2 Verify full composed app and consumer contract fixtures  Owner: luna-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009, UC-010]  blocked-by: [T-GR-INTEGRATE.1]  acc: [Real HTTP/MCP app plus PostgreSQL tests complete enroll/issue/publish/discover/exact-get/resolve/events and restart/replica isolation; joint fixture cases include connection/funding/expiry/revocation/unknown outcomes using controlled provider fixtures; no real provider spend or unqualified customer data; acceptance subset versus external runtime receipt is explicit]
- [ ] T-GR-INTEGRATE.3 Run full module, race, lint, schema and CI gates  Owner: luna-quality  Est: TBD  kind: agent  stage: verify  verifies: [UC-001, infrastructure]  blocked-by: [T-GR-INTEGRATE.1]  acc: [Root and hosted modules pass required GOWORK=off tests/vet/lint, owned formatting, frozen contracts and registry Python checks; concurrency-sensitive suite runs race in exactly one lane with load/lease held; record independent module/CI evidence, full test counts and no silently skipped required environment]
- [ ] T-GR-INTEGRATE.4 Independent integration/security code review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-INTEGRATE.2, T-GR-INTEGRATE.3]  acc: [Independent nonauthor reviewer records base/head SHAs and covers startup/enrollment/key custody/canonical serialization/durable revocation/provider bounds/tenant isolation and existing ADR changes; every accepted finding receives tracked fix, re-verification and final-head re-review]
- [ ] T-GR-INTEGRATE.5 Guarded merge of final composed candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-INTEGRATE.4, T-GR-SCOPE.8]  acc: [Required CI/policy and independent final-head approval verified through gh; approved merge method records immutable landed SHA; latest main conflict resolution is reviewed and no unrelated work is replaced]
- [ ] T-GR-INTEGRATE.6 Re-verify composed behavior on remote landed SHA  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-GR-INTEGRATE.5]  acc: [Remote main contains exact reviewed composition; affected real-store/API/MCP and compatibility acceptance run on landed SHA with version/digest receipts; this closes code delivery only, not release/deploy/live provider or historical Q5 acceptance]

### E-GR-AWS -- AWS origin and infrastructure

fidelity: executable
Acceptance: Reviewed landed AWS inputs and independently verified canonical DNS/TLS/origin/audience migration serve the production target without exposing data or widening operator authority.

Scope: existing AWS registry stack, no second permanent service/account, and the founder-selected canonical origin https://gist.sire.run. Infrastructure source is owned by its existing private owner; these are this delivery's bounded owner tasks/receipts, not permission for a Gist worker to overwrite that repository or run a second lifecycle scheduler. Reconcile any already-enrolled canonical owner jobs before assigning work; consume their combined review/delivery receipts instead of duplicating them. IaC writer uses its own SSD worktree and ownership claim. DNS runs through configured Cloudflare MCP; AWS inspection/build runs through aws CLI; stack changes follow the reviewed Pulumi workflow. Never use the stale original IaC checkout as deployment input.

AWS read-only source inspection at aa62ca046511015d427e067129cec05a374608c2 confirms a fixed registry.sire.run domain/origin in edge/task config and drift checker. Treat account, ARN, service and certificate outputs as private evidence. HTTPS ALB requires a verified certificate; the reviewed ECS service already declares rollback circuit-breaker behavior, which needs qualified rollout proof rather than reinvention.

#### Wave 1: AWS origin and rollout SDLC

- [ ] T-GR-AWS.0 Preflight AWS origin migration and infrastructure ownership  Owner: luna-aws  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.2, T-GR-SCOPE.11]  acc: [Record current remote IaC SHA, exact existing AWS account/region/stack and operator role privately; accepted ADR011 target, DNS/certificate/origin/audience/issuer/redirect migration, no-new-permanent-infrastructure boundaries and full rollback/old-origin cutoff contract; approved credential custody and current operator tools are qualified before mutations]
- [ ] T-GR-AWS.1 Implement reviewed IaC and DNS drift changes for canonical origin  Owner: luna-aws  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-AWS.0]  acc: [Own infrastructure candidate sets new certificate/ALB routing, GIST_PUBLIC_ORIGIN and identical GIST_RESOURCE_AUDIENCE to https://gist.sire.run, metadata/callback migration and read-only DNS drift checks; private store/task/database roles retained; two-phase certificate validation and rollout are automation-safe; PR exact base/head and no unexpected resource replacement recorded]
- [ ] T-GR-AWS.2 Verify IaC origin configuration and migration regressions  Owner: luna-aws-verify  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-AWS.1]  acc: [Owned mock/property tests and reviewed nonmutating preview prove new TLS/origin/audience config, private storage/networking, old-audience denial, no wildcard/mixed issuer, certificate readiness gates and rollback config; record any real preview auth failure rather than guessing state or bypassing validation]
- [ ] T-GR-AWS.3 Run infrastructure formatting lint and release input checks  Owner: luna-aws-quality  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-AWS.1]  acc: [Owned gofmt/vet/lint/tests and Containerfile/CodeBuild/migration validation pass with lease/load rules; new artifact pin references exact source/digest and no secrets; CI or accepted ADR008 local-validation policy is recorded without silently bypassing required protections]
- [ ] T-GR-AWS.4 Independent infrastructure and migration code review  Owner: independent-luna-aws  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-AWS.2, T-GR-AWS.3]  acc: [Non-author reviewer records exact repository/base/head and scope, certificate/DNS/operator custody/audience/rollback/data-loss findings; blocking findings get tracked fix, affected tests and independent final-head re-review]
- [ ] T-GR-AWS.5 Merge exact reviewed infrastructure candidate through owner delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-AWS.4, T-GR-SCOPE.8]  acc: [Infrastructure owner satisfies own required policy/checks through gh and records reviewed PR/source/landed SHA; if a native owner lifecycle exists consume its canonical landed gate rather than issuing a competing merge]
- [ ] T-GR-AWS.6 Verify landed AWS deployment inputs and owner receipt  Owner: luna-aws-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-GR-AWS.5]  acc: [Owner remote main contains exact reviewed configuration and affected IaC/drift/rollout tests pass on landed source; private operator receipts pin stack/source/config and current custody; no apply has been implied by a landed PR]

#### Wave 3: Qualified DNS and TLS cutover workflow

- [ ] T-GR-AWS.7 Prepare new-domain certificate through reviewed AWS stack workflow  stage: deploy  Owner: luna-aws-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-AWS.6, T-GR-SCOPE.11, T-GR-RELEASE.2]  acc: [Qualified agent/operator workflow rechecks role and preview-matches-merged-source, requests or reuses the exact new-domain ACM certificate through reviewed certificate-only phase, records validation outputs privately and preserves existing working endpoint; no unreviewed partial apply or ad hoc console mutation]
- [ ] T-GR-AWS.8 Apply exact ACM validation records through Cloudflare MCP  stage: deploy  Owner: luna-dns-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-AWS.7]  acc: [Qualified scoped Cloudflare MCP writes only reviewed ACM validation records for the target zone, preserves unrelated records and existing traffic, and records before/after reconciliation; missing or denied tool binding blocks without guessed browser/API fallback or OAuth changes]
- [ ] T-GR-AWS.9 Verify ACM issued and prepared HTTPS listener inputs  Owner: luna-aws-verify  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-AWS.8]  acc: [aws CLI confirms matching certificate issued in admitted region/account and reviewed HTTPS listener/task configuration is ready for final rollout; complete new origin/audience/issuer/redirect config and canonical metadata are pinned; no live credential requests routed to mixed old config]
- [ ] T-GR-AWS.10 Activate canonical DNS after new-origin AWS rollout  stage: deploy  Owner: luna-dns-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-AWS.9, T-GR-RELEASE.4]  acc: [Cloudflare MCP activates exactly reviewed gist.sire.run routing to stack outputs after matching service/config rollout, with recorded proxy/TLS behavior and drift reconciliation; all unrelated records are preserved and rollback remains bounded]
- [ ] T-GR-AWS.11 Verify external canonical DNS TLS and OAuth resource boundary  Owner: luna-domain-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-AWS.10]  acc: [From an external client gist.sire.run resolves, certificate matches, HTTPS health/readiness and canonical URLs work, HTTP redirects safely, OAuth metadata and MCP advertise the exact new resource; old-audience tokens and unregistered callbacks deny; actual positive fresh consent/token journey reaches the new origin]
- [ ] T-GR-AWS.12 Complete bounded legacy-origin handoff and rollback qualification  Owner: luna-aws-operator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-AWS.11]  acc: [Recorded old-origin cutoff/credential-free redirect or denial policy is enforced without accepting old tokens at the new resource; complete-config rollback procedure is qualified in an isolated admitted environment, existing production data preserved, and no permanent unintended dual-origin credential path remains]

### E-GR-RELEASE -- Release acceptance and operations

fidelity: executable
Acceptance: Immutable release, authorized rollout, actual authenticated consumer/provider journey, independent acceptance, bounded operations handoff and truthful landed milestone evidence are separate outcomes.

Production is explicitly in scope at https://gist.sire.run on AWS. Release, DNS and deployment are first-class agent deliverables executed through qualified operator workflows and scoped grants; they are not intentionally human-only stopping points. Ordinary parser stage vocabulary has no deploy/release token, so operational rows use kind: agent with lane: agent and named workflow acceptance, while coding candidates retain all six stage markers. The executing coordinator must route these operations to their qualified AWS/Cloudflare/IaC bindings, never Kazi code convergence or a guessed executor. A genuinely unavailable binding/authority blocks that action with concrete evidence. /plan itself performs none of them.

Before these rows run, revalidate the exact environment, operator workflow, credentials, granted numerical envelope and current policy. If deployment requires code/IaC/migration changes beyond the reviewed candidate, create its own six-stage candidate chain and make the rollout depend on its landed receipt. No code change can hide inside a release row.

#### Wave 3: Release, rollout and live qualification

- [ ] T-GR-RELEASE.0 Preflight AWS production release and operational handoff design  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-GR-INTEGRATE.6]  acc: [Prepared runbook pins https://gist.sire.run and AWS account/region/stack privately plus source/image/config/contract/migration hashes, operator/environment authority, rollout health/security checks, backup/restore and forward recovery, monitoring/redaction, credential/key rotation, invocation limits and bounded observation; no new infrastructure or provider action outside current scope]
- [ ] T-GR-RELEASE.1 Independent release and recovery runbook review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.0]  acc: [Independent operations/security reviewer confirms migratory data compatibility and failure/restore/rotation/unknown-outcome procedures, exact artifact/authority and live checks; approved runbook findings are resolved; destructive production restore is not silently included]
- [ ] T-GR-RELEASE.2 Confirm qualified release/deployment operator authorization  Owner: luna-production-operator  Est: TBD  kind: agent stage: preflight  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.1, T-GR-SCOPE.5, T-GR-SCOPE.8, T-GR-SCOPE.11]  acc: [Founder-selected AWS target and exact requested deployment scope and release workflow match actual standing or new authorization, required checks and credentials; no unapproved paid runner/account/service/permission change; qualified agent/operator receipt recorded; re-present no routine approval already granted by the session or accepted standing stack policy]
- [ ] T-GR-RELEASE.3 Produce immutable release artifact through qualified workflow  stage: release  Owner: luna-release-operator  Est: TBD  kind: agent lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.2]  acc: [Existing owner-reviewed AWS CodeBuild/ECR release workflow through aws CLI builds/publishes the approved landed SHA with immutable image digest, provenance/check receipts and config contract; no retagging or substituting another commit; artifact-release success is distinct from deployment]
- [ ] T-GR-RELEASE.4 Deploy and migrate the approved bounded environment  stage: deploy  Owner: luna-production-operator  Est: TBD  kind: agent lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.3, T-GR-AWS.9]  acc: [Qualified reviewed existing AWS stack workflow applies pinned artifact/config/migrations within exact authority; https://gist.sire.run origin/resource config, exact running ECS/ECR digest, task count and ALB target health/readiness and no-public-object/secret checks pass; source/image/environment receipts and failure/forward-recovery outcome recorded; no manual DB seed or infrastructure bypass]
- [ ] T-GR-RELEASE.5 Verify authenticated deployed registry journey and security denials  Owner: luna-live-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009]  blocked-by: [T-GR-RELEASE.4, T-GR-AWS.11]  acc: [Actual deployed https://gist.sire.run enrollment/human OAuth and sanctioned workload token journey publishes/discovers/fetches/resolves exact artifacts and consumes events; wrong tenant/issuer/audience, expiry/revocation, retention gap and restart/replica behavior deny correctly; run live-tag acceptance with named builds and record observed statuses without fixtures-as-production]
- [ ] T-GR-RELEASE.6 Verify governed caller execution with a real pinned consumer  Owner: joint-registry-consumer  Est: TBD  kind: agent  stage: verify  verifies: [UC-008, UC-010]  blocked-by: [T-GR-RELEASE.5, T-GR-SCOPE.7, T-GR-SCOPE.5]  acc: [Named admitted runtime imports exact closure and performs only approved bounded Treg/Composio actions; action/account/recipient/revision/budget checks precede send; provider call IDs, actual cost and attempt outcomes persist; unknown stays unknown without duplicate replay; absent numerical or custody authority leaves this row blocked]
- [ ] T-GR-RELEASE.7 Independent live acceptance and operational readiness review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.5, T-GR-RELEASE.6]  acc: [Independent reviewer checks authentic environment/build/receipt provenance, separate provider pass/fail status, tenant and secret redaction, negative coverage, restore/rotation and bounded observation readiness; local/CI/live/provider/runtime evidence are separately labeled and no unexplained skipped gate passes]
- [ ] T-GR-RELEASE.8 Record technical production acceptance and retain explicit residual risks  Owner: coordinator  Est: TBD  kind: agent stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.7]  acc: [Coordinator records pass/fail against actual agreed technical acceptance using the delivered journey evidence; remaining registry-client/production/provider limitations and caller revocation race are explicit; no invented founder-pilot sign-off or business publication; separate genuine human decisions remain separately scoped]
- [ ] T-GR-RELEASE.9 Hand off ownership and complete bounded post-rollout observation  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.8, T-GR-AWS.12]  acc: [Named operator receives pinned runbook, service/key/feed/credential ownership and incident path; approved bounded health/error/revocation observations at recorded sample points show no new regression; no indefinite watcher/scheduler; maintenance issues become tracked scoped tasks]

#### Wave 4: Evidence delivery SDLC

- [ ] T-GR-EVIDENCE.0 Preflight truthful milestone closure and evidence ownership  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-GR-RELEASE.9]  acc: [Map all original Q5/R3/X-R1 and required named-client obligations to actual receipts; resolve case-sensitive M2a evidence filename from scripts/registry/check.py; qualify public redaction and list any original requirements not met; unmet Q5 scope remains open]
- [ ] T-GR-EVIDENCE.1 Author actual milestone evidence and retrospective handoff candidate  Owner: luna-evidence  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-EVIDENCE.0]  acc: [Create docs/registry/gates/M2a.json only when every original gate requirement is actually passed; preserve historical task IDs/status and annotate newer receipts; sanitize private run details; update design/ADRs for accepted choices and devlog for events; PR/head handoff includes release/ops evidence and explicit unqualified scope]
- [ ] T-GR-EVIDENCE.2 Validate evidence hashes, commands, counts and historical coverage  Owner: luna-evidence-check  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-EVIDENCE.1]  acc: [Registry M2a evidence checker and applicable contract/receipt checks pass on candidate; every recorded hash/build/command/test count and named-runtime receipt matches observed run; redaction/link/status checks pass; no gate fabricated to clear a checkbox]
- [ ] T-GR-EVIDENCE.3 Independent evidence and code review of milestone candidate  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-EVIDENCE.2]  acc: [Non-author reviewer records exact base/head, evidence provenance and Q5 coverage with findings/dispositions; any code/script/schema edit gets full applicable behavior/lint checks and review; missing original requirement blocks milestone closure]
- [ ] T-GR-EVIDENCE.4 Merge exact reviewed evidence candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-EVIDENCE.3, T-GR-SCOPE.8]  acc: [Required final-head review/policy/checks satisfied and gh records evidence PR/landed SHA using approved method; no stale artifact hashes after main moves]
- [ ] T-GR-EVIDENCE.5 Verify landed evidence and final recovery acceptance  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-GR-EVIDENCE.4]  acc: [Remote landed SHA passes truthful M2a evidence/freeze/receipt checks with all original Q5 requirements accounted for; residual M2b/M3/gateway work remains open; report final code/release/deploy/live/pilot states and close this M2a recovery submilestone only if its obligations passed; continue automatically to NEXT/M2b/M3/final AWS production rather than treating this row as the delivery finish]

### E-GR-NEXT -- Remaining registry milestones

fidelity: executable
Acceptance: Next reachable registry milestone has an evidence-reconciled full SDLC plan without invented provider or client readiness.

Preserve the remaining original M2b/M3 tasks and IDs in registry-buildout.md. Expand preview teardown, public OAuth/connector clients, both runtime archetypes and retrieval evaluation from actual recovery evidence before dispatching the known remaining production qualification lanes. Do not duplicate already qualified behavior or imply discovery savings without measurements.

#### Wave 5: Dependency-triggered next planning pass

- [ ] T-GR-NEXT.0 PLAN: refresh and reconcile remaining M2b and M3 production delivery from receipts  stage: author  Owner: coordinator  Est: TBD  kind: plan  verifies: [infrastructure]  blocked-by: [T-GR-EVIDENCE.5]  acc: [Reconcile original R4/R5/Q6-Q10/O2-O4/external receipts and accepted waivers; Reconcile the explicit M2b/M3 stage rows against current code and activate only dependency-ready accepted scope; newly discovered epics use exactly one triggered planning task until expandable; include distinct requirements/design/preflight/implementation/tests/lint/independent review/merge/landed/release/live/ops tasks and GPT-6-Luna ownership; preserve historical IDs and evidence]

This stable planning row is a continuation checkpoint. Completing it never satisfies production. The coordinator revalidates the known M2b/M3 scope, adds tracked fix or outline-expansion tasks as evidence requires, and continues execution without another generic “proceed?” question. Material new scope, missing numerical authority or rejected binding is a concrete blocker; drafting alone never admits a new lifecycle.

### E-GR-M2B -- OAuth preview and production

fidelity: executable
Acceptance: Actual named connector/OAuth behavior runs at https://gist.sire.run on AWS with qualified tenant isolation and complete preview teardown; original Q8 obligations are met.

Known remaining RFC-002 acceptance is in scope, with original R4/O2/O3/O4/Q6/Q7/Q8 IDs preserved in the historical plan. The user explicitly asks a complete production SDLC; these stage obligations are decomposed now, and preflight rechecks assumptions from real M2a receipts. No extra product design is invented. Any newly discovered implementation gap gets its own scoped six-stage correction chain. Shared acceptance/IaC ownership is allocated by coordinator; infrastructure owner retains its canonical jobs.

#### Wave 5: M2b qualification code SDLC

- [ ] T-GR-M2B.0 Preflight OAuth connector preview and production acceptance  Owner: luna-oauth-acceptance  Est: TBD  kind: agent  stage: preflight  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-GR-NEXT.0]  acc: [Reconcile actual original I4-I6/R4/O2-O4/Q6-Q8/W1-W5 evidence, current named connector builds and AWS preview workflow/cost authority; freeze complete OAuth and browser/client denial matrix, preview isolation/max lifetime, finally-teardown and canonical new-origin promotion contract; missing provider/identity permission blocks only its affected action]
- [ ] T-GR-M2B.1 Implement only remaining OAuth connector and preview qualification gaps  Owner: luna-oauth-acceptance  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-GR-M2B.0]  acc: [Owned actual browser/HTTP/MCP connector tests and isolated AWS preview workflow close proven missing coverage without synthetic build labels; teardown always executes on success/failure/cancellation; required service/IaC changes have explicit author scopes and repository/head handoffs rather than hidden deployment edits]
- [ ] T-GR-M2B.2 Verify OAuth consent token tenant and failure behavior  Owner: luna-oauth-verify  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-GR-M2B.1]  acc: [Meaningful real HTTP status/body and browser golden/denial tests cover registration, PKCE S256, consent, resource/audience, refresh rotation/reuse/revoke, session/workspace switching, tenant caches, reconnect/limits and uncertainty; required absent config is not counted as a passing skip]
- [ ] T-GR-M2B.3 Run changed-package quality and preview workflow checks  Owner: luna-oauth-quality  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-M2B.1]  acc: [Owned format/lint/vet/tests and applicable freeze/schema/workflow checks pass at exact candidate; both affected repository candidates receive their required checks under build limits; preview cleanup is tested with failed/cancelled attempts]
- [ ] T-GR-M2B.4 Independent OAuth and preview security code review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-M2B.2, T-GR-M2B.3]  acc: [Non-author reviewer records exact base/head for every included coding/IaC candidate, credential/tenant/callback/teardown findings and dispositions; blockers require tracked fixes, affected verification and re-review before delivery]
- [ ] T-GR-M2B.5 Merge exact reviewed qualification candidates through owners  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-M2B.4, T-GR-SCOPE.8]  acc: [All included candidate owners satisfy actual check/policy and independent approval at final heads; gh or canonical enrolled owner lifecycle records landed receipts; no competing scheduler or main overwrite]
- [ ] T-GR-M2B.6 Verify landed OAuth qualification and workflow code  Owner: luna-oauth-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-GR-M2B.5]  acc: [Affected behavior and cleanup tests pass on actual remote landed source(s); pin service and acceptance build identities separately; no deployment completion inferred]

#### Wave 6: AWS preview and M2b production workflow

- [ ] T-GR-M2B.7 Prepare immutable preview and production release inputs  stage: release  Owner: luna-release-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-M2B.6, T-GR-SCOPE.5, T-GR-SCOPE.11]  acc: [Qualified AWS workflow builds or reuses exact immutable service digest and matching acceptance revision; reviewed preview account/region/stack/expiry/backup/cost scope is recorded, isolated synthetic workspace and private test secrets prepared, always-run cleanup ownership registered before provisioning]
- [ ] T-GR-M2B.8 Provision and execute the bounded isolated preview workflow  stage: deploy  Owner: luna-preview-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-M2B.7]  acc: [Reviewed AWS preview workflow provisions its separate origin/audience/database/object namespace/credentials within approved scope, runs configured acceptance attempt and captures terminal attempt receipt, invokes finally teardown on success/failure/cancel before returning; neither failed test nor cleanup is mislabeled passing]
- [ ] T-GR-M2B.9 Verify actual connector product and OAuth preview acceptance  Owner: luna-connector-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-GR-M2B.8]  acc: [Actual qualified connector/native browser products with named builds complete registration/consent/exact retrieval and denial matrix against that preview; operator/cloud provider fixtures do not replace required client evidence; failures create tracked repair/retest work, not a production promotion]
- [ ] T-GR-M2B.10 Verify preview teardown and expiry cleanup independently  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-M2B.8]  acc: [Finally hook receipts plus aws/approved DNS checks prove temporary stack/resources/client grants/secrets/DNS removed or revoked after every terminal attempt; no production data altered and no cleanup error hidden; workflow failure leaves M2b/promotion open until cleanup proof and repairs pass]
- [ ] T-GR-M2B.11 Promote reviewed immutable OAuth service revision to AWS production  stage: deploy  Owner: luna-production-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-M2B.9, T-GR-M2B.10, T-GR-SCOPE.11]  acc: [Existing authorized registry-aws workflow applies only qualified landed source/digest/config at https://gist.sire.run; reuse already deployed identical service revision when changes are acceptance-only and prove identity; newer service changes require actual rollout and fresh affected acceptance; no manual data or cloud policy bypass]
- [ ] T-GR-M2B.12 Verify production OAuth connectors and private-catalog isolation  Owner: luna-live-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-GR-M2B.11]  acc: [Live HTTPS/MCP canonical new-origin OAuth and named connector journey pass all original Q8 obligations; scope/membership/resource/refresh/revocation/caches/outage denials and old-origin policy verified; no preview-only evidence substituted]
- [ ] T-GR-M2B.13 Independent M2b production acceptance review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-M2B.12, T-GR-M2B.10]  acc: [Independent reviewer checks source/image/config/client/environment receipts, original Q8/W gate coverage, actual production result and zero leaked preview resources; findings receive tracked correction, re-verification/re-review]
- [ ] T-GR-M2B.14 Record truthful M2b production gate and continue  Owner: luna-evidence  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-M2B.13]  acc: [Prepare checker-compatible actual M2b evidence with exact receipt/hashes/test counts and preserved original IDs; Q8 closes only on production plus cleanup proof; handoff to M3 automatically, not final completion; final dossier chain reviews and lands these artifacts]

### E-GR-M3 -- Clients runtime and production metrics

fidelity: executable
Acceptance: Required named clients, both runtime archetypes and frozen measured retrieval/interaction gates are qualified at the latest running AWS service on https://gist.sire.run.

Known original R5/E3/Q9/Q10 obligations remain required: named coding/connector clients, both runtime archetypes, and measured retrieval/interaction behavior. The fixture-only R3 history remains visible. Parallelize client/runtime/retrieval verification workers within pool capacity and one protected approved funding envelope. Use exact frozen labels/thresholds rather than changing them to pass.

#### Wave 7: M3 qualification SDLC

- [ ] T-GR-M3.0 Preflight required named clients both runtime archetypes and retrieval gate  Owner: luna-client-matrix  Est: TBD  kind: agent  stage: preflight  verifies: [UC-001, UC-003, UC-004, UC-005, UC-007, UC-009, UC-010]  blocked-by: [T-GR-M2B.14]  acc: [Reconcile original R5/E3/Q9/Q10 and X-R2/Z receipts; pin actual required client/runtime builds, service/config and retrieval corpus/labels/thresholds; reuse qualified consumer imports and explicitly externally driven mode if accepted; no silent client substitution, invented model budget or fake local-build identity]
- [ ] T-GR-M3.1 Implement remaining client runtime and measured-retrieval acceptance gaps  Owner: luna-client-matrix  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [UC-003, UC-004, UC-005, UC-007, UC-009, UC-010]  blocked-by: [T-GR-M3.0]  acc: [Owned harnesses exercise exact named client products and importing/policy-gated runtime paths with URI/digest/grant/revocation contracts; measurement runner emits reproducible search quality and complete task interaction counts; service fixes identified here receive scoped six-stage delivery instead of being hidden in test setup]
- [ ] T-GR-M3.2 Verify changed client and retrieval behavior with meaningful fixtures  Owner: luna-client-verify  Est: TBD  kind: agent  stage: verify  verifies: [UC-003, UC-004, UC-005, UC-007, UC-009, UC-010]  blocked-by: [T-GR-M3.1]  acc: [Real API/MCP harness regression tests verify full package closure, honest resolution, connection/error/expiry/revocation, authorized optional taxonomy filters with attribution, and preserved local library/CLI behavior; captured scenario data is synthetic and fixture versus live evidence remains explicit]
- [ ] T-GR-M3.3 Run candidate quality checks and frozen metric-contract validation  Owner: luna-quality  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-M3.1]  acc: [Changed package format/lint/vet/tests plus root compatibility/freeze and retrieval checker pass at exact candidate; frozen corpus/threshold changes require recorded rationale and independently reviewed revision, never silent pass-by-retuning]
- [ ] T-GR-M3.4 Independent client runtime and evaluation code review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-M3.2, T-GR-M3.3]  acc: [Non-author reviewer records exact base/head and named scope with interoperability/security/measurement findings, accepted dispositions and tracked fix/retest/re-review for blockers]
- [ ] T-GR-M3.5 Merge exact reviewed M3 qualification candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-M3.4, T-GR-SCOPE.8]  acc: [Actual required policy/checks and independent final-head approval verified through gh; approved method records PR/landed SHA; scope-dependent evidence refreshed after relevant base changes]
- [ ] T-GR-M3.6 Verify landed M3 harness and affected service behavior  Owner: luna-client-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-GR-M3.5]  acc: [Remote landed source contains the reviewed harness/service changes; affected checks pass on that source and source/config/corpus receipts are immutable; absent live product/client remains unresolved]

#### Wave 8: Latest-source production and parallel qualification

- [ ] T-GR-M3.7 Refresh exact production artifact when service changes require rollout  stage: deploy  Owner: luna-release-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-M3.6, T-GR-SCOPE.11]  acc: [Compare deployed service inputs with landed candidate: build/pin and roll out new immutable AWS service digest for service changes through authorized workflow, or prove existing running digest matches unchanged service inputs for harness-only changes; preserve canonical origin and capture running task/ALB status]
- [ ] T-GR-M3.8 Verify required named coding and connector products in production  Owner: luna-client-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-005, UC-009, UC-010]  blocked-by: [T-GR-M3.7]  acc: [Every required original client product with actual pinned version passes https://gist.sire.run setup/metadata/auth/exact retrieval/resolve/expiry/error/revocation matrix; explicit bridge differs from native support; required unsupported client keeps M3 open]
- [ ] T-GR-M3.9 Verify both real runtime archetypes and caller-owned provider receipts  Owner: joint-registry-consumer  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-005, UC-008, UC-010]  blocked-by: [T-GR-M3.7, T-GR-SCOPE.7, T-GR-SCOPE.5]  acc: [Actual importing and policy-gated runtime builds obtain complete pinned closure and preserve native URI/grant/parent/connection/revocation lifecycle; authorized Treg/Composio actions have bounded actual provider/cost/unknown-outcome persistence receipts; exact fields/accounts/revision checked before sending; no generic gateway required]
- [ ] T-GR-M3.10 Run measured production retrieval and complete-task interaction smoke  Owner: luna-retrieval-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-003, UC-005, UC-007]  blocked-by: [T-GR-M3.7, T-GR-SCOPE.5]  acc: [Freeze-qualified production corpus/query/threshold run measures actual recall/quality/tenant exclusion, latency and complete task discovery/get/resolve/turn counts with real source/config/date; no measured-saving or BM25 claim without evidence; privacy/cost envelope and failure-to-no-match semantics preserved]
- [ ] T-GR-M3.11 Independent full-registry production acceptance review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-M3.8, T-GR-M3.9, T-GR-M3.10]  acc: [Independent reviewer checks actual client/runtime/provider/evaluation receipts, original R5/E3/Q9/Q10 coverage and newest deployed source/config; all accepted defects become tracked SDLC corrections with affected fresh production checks before closure]
- [ ] T-GR-M3.12 Record final M3 gate and latest production handoff  Owner: luna-evidence  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-M3.11]  acc: [Actual M3 evidence reflects all original Q10 obligations at the running canonical AWS origin; stage provenance, quality thresholds and test counts match receipts; prepare artifacts for independent final dossier delivery and continue to terminal production checks]

Current consumer boundary: [ADR013](adr/013-optional-caller-integrations.md) makes Zatiti optional, with no demo/first-packet prerequisite. These generic native-client/runtime qualification obligations remain Gist production gates and must name an accepted consumer. Previously completed fixture tests retain their source-only evidence; they do not demonstrate live Zatiti integration. No Zatiti-specific packet is assigned here.

### E-GR-PROD -- Terminal production delivery

fidelity: executable
Acceptance: The full confirmed registry/caller scope is actually running and independently qualified at https://gist.sire.run on AWS; no earlier submilestone can terminate delivery.

This is the only delivery terminal. Earlier landed, release, recovery, NEXT planning or health-only results cannot complete the whole plan. The coordinator keeps scheduling eligible Luna work, including exact-head fixes and new immutable releases for affected service changes, until this terminal is proved or a concrete authority/capability/decision blocker is reported. Human feedback is retained when genuinely required; arbitrary periodic confirmation does not replace autonomous authorized work.

#### Wave 9: Final operations and evidence delivery

- [ ] T-GR-PROD.0 Preflight complete-production evidence accounting  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-GR-EVIDENCE.5, T-GR-M2B.14, T-GR-M3.12, T-GR-AWS.12, T-GR-RELEASE.9]  acc: [Account for all accepted registry M1-M3/original Q5/Q8/Q10 obligations and new domain/AWS/provider/ops gates; identify latest running app and IaC source versus acceptance/docs source; no incomplete required row or newer unqualified service candidate is hidden; optional gateway and unrelated products remain outside the confirmed scope]
- [ ] T-GR-PROD.1 Verify monitoring recovery security and bounded production soak  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-PROD.0]  acc: [At recorded bounded sample points canonical HTTPS app remains healthy and actual AWS tasks/ALB/DB/private objects/secret/key/event configuration match approved running inputs; named alarms/log redaction/operator ownership and backup/restore/key rotation/restart procedures are qualified in approved isolated scope; thresholds/observation duration recorded before run, no indefinite watcher or destructive customer restore]
- [ ] T-GR-PROD.2 Track and close all blocking production findings through SDLC  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-PROD.1]  acc: [Every accepted failed production/review/soak check has stable owned fix, affected behavior/quality checks, independent exact-head re-review, guarded merge/landed receipt and new release/rollout/live checks if service inputs changed; root completion remains false until no unresolved required finding or unqualified deployment exists]
- [ ] T-GR-PROD.3 Author complete canonical production dossier candidate  Owner: luna-evidence  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [infrastructure]  blocked-by: [T-GR-PROD.2]  acc: [Owned sanitized docs/gates include truthful M2a/M2b/M3/domain/AWS/provider/ops receipts, full source/digest/config/build/check counts and residual accepted constraints; existing historical IDs/evidence preserved; public hostname is https://gist.sire.run; no secret/account/customer raw data; PR exact base/head handed off]
- [ ] T-GR-PROD.4 Verify final dossier links hashes milestone schemas and closure criteria  Owner: luna-evidence-check  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-PROD.3]  acc: [Installed registry contract/evidence/eval/receipt checks and case-correct original M2a/M2b/M3 gate requirements pass on actual candidate; source hashes and running AWS/URL observations agree, original and new tasks are covered, no missing environment/client outcome recast as pass]
- [ ] T-GR-PROD.5 Independent final production evidence code and domain review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-PROD.4]  acc: [Independent non-author reviewer records exact base/head and authentic current production evidence with findings/dispositions; every code/config/script edit has required changed-behavior quality evidence; no terminal acceptance based only on design/fixture/merge/health]
- [ ] T-GR-PROD.6 Merge exact reviewed production dossier  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-PROD.5, T-GR-SCOPE.8]  acc: [Required exact-head approval/policy/checks verified through gh, guarded owner merge records landed SHA; evidence-only commit does not force an unnecessary app rebuild, and source/provenance distinction remains correct]
- [ ] T-GR-PROD.7 Verify landed dossier and latest running service alignment  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-GR-PROD.6]  acc: [Remote main contains accepted dossier and actual gate checks pass on landed evidence source; latest accepted service/IaC digest is running on AWS, and canonical HTTPS/authenticated smoke agrees after delivery; relevant intervening code/config changes invalidate stale receipts and create follow-on work]
- [ ] T-GR-PROD.8 Independently verify complete production predicate  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-GR-PROD.7]  acc: [Independent final checker proves all required task/gate receipts, canonical DNS/TLS/origin/OAuth metadata, running AWS service/digest/health, authenticated catalog and real caller/provider behavior, full M2a/M2b/M3 acceptance, no preview leaks, qualified ops/recovery and no unresolved blocking findings; no public registry.sire.run alias is accepted as the requested target]
- [ ] T-GR-PROD.9 Complete delivery only while gist.sire.run is running qualified AWS production  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.23, T-GR-PROD.8]  acc: [All accepted required registry/caller tasks and final independent predicate pass with current timestamps and immutable source/deploy/evidence receipts; https://gist.sire.run serves the qualified authenticated service on AWS now, Treg/Composio caller journeys and operations are evidenced; coordinator reports the actual URL and states with no required work remaining, otherwise keeps plan open and progresses/records concrete blockers]

The preserved [optional gateway planning record](plans/E-GR-GATEWAY-optional-managed-gateway.md) and [original gateway plan](plans/gateway-buildout.md) are deferred records outside this active graph, following the founder's explicit registry/caller scope choice. Their IDs and open status remain unchanged; they are not required for, or dispatched by, this production delivery.

## Milestones and acceptance boundaries

| Milestone | Dependency join | Exit evidence |
| --- | --- | --- |
| Design-ready lanes | SCOPE.9 and lane-specific decisions | Accepted scoped design/write sets; no implementation completion |
| Component delivery | Each lane .6, independently | Exact reviewed/landed component receipts |
| Code integration | INTEGRATE.6 | Composed local real-store/API/MCP and compatibility evidence |
| Release/rollout | RELEASE.3/.4 | Immutable release versus deployed environment evidence recorded separately |
| Live consumer/pilot | RELEASE.5-.9 | Actual authenticated runtime/provider/ops acceptance with numerical scope |
| Historical milestone closure | EVIDENCE.5 | Truthful landed M2a file and all original Q5 obligations met |
| Remaining registry production | NEXT.0 -> M2B.14 -> M3.12 | Original Q8/Q10 plus actual new-origin acceptance; continuation, not finish |
| Entire delivery terminal | PROD.9 | Requested AWS origin currently running the fully qualified registry/caller scope with independent final evidence |
| Separate optional gateway | conditional GATEWAY.0 | Outside the confirmed production scope |

## Required validation and evidence

Implementation verification names actual package tests, including real HTTP status/body assertions for changed API routes, real PostgreSQL transactions/restart/replica behavior, OAuth browser golden/denial cases, exact artifact/digest/version fixtures and one meaningful regression per defect. Use GOWORK=off for Go. Integration runs both module tests/vet, changed-package golangci-lint, owned gofmt/goimports, Python registry unit tests and `python3 scripts/registry/check.py contracts --freeze-check`; one lease-qualified race lane covers concurrency changes. Live rows use the existing live-tag workload/wiring/runtime suites and actually pinned client/runtime builds. Required environment absence is not success or a skip waiver.

Each receipt records task, owner, full base/head/landed/source/image/config/contract digests, environment/client/runtime identifiers, commands/test counts, independent reviewer, findings/dispositions and evidence location. Private credentials, account/host/user/customer identifiers and raw production traces stay in access-controlled records. Public channel summaries are sanitized and ignored in this public repository. Source maps: app/identity/oauth/storage/events/resolution/rest/remotemcp, catalog/registry, contracts/registry/v1, scripts/registry and registry CI workflows.

Capability selection was run in planning: baseline/delivery/go profiles; CLI go/gh/aws/golangci-lint/kazi/composio available. This reports installed binaries, not auth, live permissions or runtime interoperability. Use gh for GitHub, aws CLI for AWS and configured Cloudflare MCP only if an in-scope CF need arises. No fresh code-graph tool binding was available; direct source reads are the qualified fallback. Profile suggestions do not activate plugins. The generic parser supports the declared preflight/implement/verify/review/merge/verify-landed markers; qualified agent-owned operational release/deployment rows deliberately use no unsupported stage: deploy/release token; the coordinator routes each named workflow to actual authorized AWS/Cloudflare bindings, not a generic code executor. No native controller JSON, approval, enrollment or runtime grant is produced.

Stable architecture remains in docs/design.md and RFC/ADRs. Accepted design changes amend their existing owner records during reviewed delivery; events and debugging go to docs/devlog.md. This plan links the earlier proposed integration packet and provider assessment for technical detail rather than duplicating their complete fixtures. Future stage execution must revalidate any stale permission, auth, CI, remote-main or deployment claim.

## Open decisions and risks

1. Operator/login proof, human/workload credential custody and qualified shared signing-key source: enrollment is blocked until the actual scope/custody decision; other independent offline lanes need not wait for all live decisions.
2. Actual first provider task, exact Treg endpoint/target and Composio action/account/version/license/price plus numerical envelopes: artifacts cannot invent missing schema or costs; live calls stay blocked separately.
3. Consumer readiness, existing required CI availability/policy and current deployment/tool binding: production scope is now explicit and applicable standing grants must be reused; local composition does not qualify those boundaries. Do not change OAuth restrictions or required checks to gain progress.
4. Caller revocation has a check-to-send race; unknown outcomes and already accepted effects remain honest. Memory, business hierarchy and friend-approved marketing publication are not registry authority.

## Planning handoff

Original planning handoff (superseded by authorized October4/5 ship execution): this was a draft refinement only. All new checkboxes remain open, historical checkboxes and authored evidence are retained, and no fixes/builds/tests/reviews/merges/releases/deployments/worker starts have been performed by /plan. The executing coordinator reads this file, revalidates SCOPE.1/.6/.8/.11 and keeps scheduling/refining dependency-ready in-scope stages until T-GR-PROD.9 is true; no routine proceed-confirmation at local merge, rollout or NEXT planning checkpoint. External missing choices remain coordinator-routed. Planning validation is parser/graph/coverage and artifact consistency, not application acceptance.

Planning validation (2026-10-04): installed parser accepted 77 open tasks in 11 epics with all six supported code-stage markers; every task has owner, acceptance and wave assignment. Additional graph checks found no missing dependency, cycle, duplicate local ID, malformed acceptance, unguarded coding merge or TOC count mismatch. Exactly two outline epics have one trigger planning task each. Hash checks confirm preexisting plan/design contents remain byte-identical prefixes and the gateway plan is untouched. Whitespace checks passed; no application checks were run. Parser IDs namespace the preserved local IDs for display; they are not minted lifecycle or claim IDs.

## Production completion and continuation contract — 2026-10-04 refinement

The founder explicitly confirmed registry plus caller-owned integrations and selected https://gist.sire.run on AWS. `production_done` is true only when T-GR-PROD.9 has fresh independent evidence of all required registry/caller milestones, canonical external DNS/TLS/health/origin/OAuth boundary, expected live AWS service/task/image/config/private storage, authenticated publish/discover/exact-get/resolve/events, real bounded Treg/Composio caller/runtime receipts, all required original M2a/M2b/M3 clients/metrics, complete preview cleanup and qualified monitoring/recovery/ownership. A local pass, PR, merge, initial rollout, website200, drafted plan or unsupported client is insufficient.

The executing coordinator persists task outcomes and receipt dependencies after every lane boundary. Within already accepted scope it refills Luna slots, invokes dependency-triggered planning refinements, assigns accepted findings to full fix/verification/review/merge/landed chains, rebuilds and redeploys changed service inputs, reruns affected live acceptance and continues. It does not stop just because one epic is done or because ordinary Markdown lacks a deploy stage token. Real denied tools/credentials, absent operator/provider budget, rejected auto-approval or material scope changes are recorded concrete blockers; they are not silently bypassed. No worker can accept its own code or production findings.

Known AWS, M2b/M3 and final production obligations are now decomposed to fulfill the user's explicit complete-SDLC request; every preflight must revalidate actual immutable inputs before dispatch. User/domain decisions, cloud inventory and numerical limits are not invented. The earlier77-task validation is historical evidence of that earlier revision only; validate this new graph and counts before handing it off.

Official deployment references used for this planning refinement: ALB HTTPS requires a matching certificate ([AWS listener documentation](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/create-https-listener.html)); ECS rolling deployment supports failure detection/rollback and image-digest consistency ([AWS deployment documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-ecs.html)). These support the task design, not an assertion that the deployed service already meets it.

Tooling observation for this refinement: aws, pulumi and gh binaries are available; no Cloudflare MCP function was exposed in the current tool inventory. T-GR-SCOPE.11 must qualify the configured binding before DNS writes. This is a real execution-capability prerequisite, not permission to use a generic connector/browser or change organization OAuth restrictions. No AWS authentication, stack apply or DNS mutation was attempted here.

Historical planning-only validation (2026-10-04, before shipping started): 128 open active tasks in 14 epics; default docs/plan.md parses those tasks once alongside 18 unchanged historical completed tasks (146 total). All 127 other active tasks are ancestors of terminal T-GR-PROD.9. Unique IDs, resolved acyclic dependencies, owner/acceptance/wave coverage, all six supported code stages, independent-review dependencies, guarded merges, split TOC counts and whitespace checks passed. All 77 prior recovery IDs/statuses are retained: 76 remain active and the unchanged optional gateway trigger is preserved in its deferred file. Prior plan/design/devlog contents remain byte-identical prefixes; ADR008 and the original gateway plan are unchanged. These are planning-artifact checks only; no application test or execution was run.

## Shipping checkpoint — 2026-10-04

User invoked /ship. T-GR-SCOPE.1/.2/.6/.8 have source-only receipts in docs/receipts; component contracts await independent T-GR-SCOPE.3 disposition before engineering admission. Four slots are occupied by one coordinator and three explicitly requested GPT-6-Luna workers, with isolated external-SSD ownership. Engineering builds are held while one-minute host load exceeds 10; no build lease is bypassed. Production bindings are partially qualified but blocked by AWS session/stack-account mismatch and absent configured Cloudflare MCP. Enrollment and provider targets/caps are pending a decision brief requested by the founder. No production_done, code/lifecycle admission, CI pass, release or live acceptance is asserted. Read-only preflights are not application acceptance.

Current shipping validation: 128 total active production tasks, five source-only preflights checked and 123 open; deferred gateway is outside that count. Founder identity/pilot choices remain pending. Consumer owner reports its actual runtime/header/funding qualifications are not yet admitted; T-GR-SCOPE.7 stays blocked rather than accepting its planning PR as runtime evidence. Exact source review findings R1 (open-PR accuracy) and R2 (historical/current count distinction) were corrected in this candidate; fresh independent review is still required.


## October5 source delivery continuation

PR48 landed the original reviewed production graph. KEYS/WIRE/EVENT are admitted source components; trust, live pilot and AWS/DNS operator bindings remain held. KEYS has three accepted loader findings tracked as7/8/9, with independent re-review required. Shared-interface source delivery is now a separate prerequisite so dependent components can land without completing the held final app composition. Graph rows retain one terminal production gate and the same registry/caller scope.


2026-10-05 CORE-R9 source-preparation assignment: existing retrieval, wiring and named-client acceptance constructors predate strict key configuration and startup maintenance qualification. The designated fixture author owns only their local startup helper files in an isolated worktree; dedicated test-only maintenance grants must precede actual New without broadening original reader or client authority. Numeric fix/verify/independent-review rows join the core merge gate. This records prepared compatibility work, not passing application or production acceptance.


2026-10-05 component landed closure: WIRE PR50 reviewed9ac3263 and landed93c5c45 passed exact landed app REST MCP unit/race/vet/lint and actual PostgreSQL checks ([receipt](https://github.com/sirerun/gist/blob/8bfd8695f5301612fc005dd8eff8f6540d4a0cfa/docs/receipts/2026-10-05-wire-landed-verification.md)). EVENT PR51 reviewed80cc432 and landed57927cd passed exact landed tagged storage PostgreSQL/race/vet/lint and frozen contracts ([receipt](https://github.com/sirerun/gist/blob/e58cce4c9d09c6933a53ee2ebd686a2fa1473444/docs/receipts/2026-10-05-event-landed-verification.md)). All component finding chains and merge/landed rows now close on that evidence. CORE clean composition is based on actual landed components; implemented source/fixes are checked while final composed verification and independent review remain open. Fixture-only membership seeding is not external enrollment.

CORE transaction disposition: catalog and outbox roll back together, but failed catalog/outbox publication can leave a private unbound content-addressed blob staged before the database transaction. Component tests explicitly retain that physical boundary; no compensation or storage garbage-collection claim is made. Canonical PUBLISH preflight/implementation/verification must qualify tenant-safe staging ownership, retention and reconciliation before the downstream production gate; unsafe deletion of a shared digest is excluded.


CORE-R10 fixture conformance: after explicit startup adaptation the wiring resolve case fails with validation_failed because it sends the obsolete skill/runtime_id/local_execution shape. Frozen v1 instead requires skill_ref/runtime/max_bytes. The coordinator owns three wiring request fixtures; the fixture author owns the client MCP request in its already assigned file. Successful readiness and revoked-artifact denial expectations remain intact, with numeric fix/verify/independent-review gates before core merge; no public semantics are amended.

CORE-R11 dispatcher disposition: cross-epic bare task IDs were structurally present but unresolved by the installed namespace-first dispatcher. Active split-plan metadata now qualifies those references literally, preserving original semantic dependencies and human production gates; numeric fix, actual-parser verification and independent review join core merge.


2026-10-06 CORE landed closure: [actual b91 verification](receipts/2026-10-06-core-landed-verification.md) completes43/43 core source rows. Publication proposal drafting is complete1/9, but owner choice remains open. [Fresh production bindings](receipts/2026-10-06-production-bindings-refresh.md) still show AWS target-account mismatch and unavailable Cloudflare DNS capability. Operator/custody, live provider targets/caps, consumer runtime evidence and final production acceptance remain blocked. No deployment occurred.

2026-10-08 alignment delivery: E-GR-SCOPE.T-GR-SCOPE.18–23 cover the current scope/execution refresh through exact-head independent review, guarded rebase merge and landed verification. Earlier completed task IDs/evidence remain unchanged.

2026-10-08 current scope/execution refresh: [ADR013](adr/013-optional-caller-integrations.md) records optional Gist/direct MCP/no Zatiti demo dependency. [Preflight](receipts/2026-10-08-alignment-and-dgx-preflight.md) verifies preservation/relocation and fresh Cloudflare read availability. Active coding/checks/workers use qualified DGX storage and existing shared dispatch. SCOPE.18–23 deliver the plan/design/decision update without advancing production gates.

2026-10-08 owner explicitly approves ADR012 v2 publication implementation; T-GR-PUBLISH.8 is complete and T-GR-PUBLISH.0 becomes ready after planning delivery. Source contract/readback/tenant-safe orphan qualification and independent review remain required. Provider/release/deployment gates retain their own authority.

## Non-authoritative source archive

Only docs/plan.md is authoritative. The following split files remain byte-for-byte historical evidence, not alternative active plans. Do not dispatch from them or modify them as the delivery graph. The original root is preserved as an opaque .source snapshot; original split references remain available for existing receipts.

| Original source | SHA256 |
| --- | --- |
| [docs/plan.md](plans/archive/20261008/plan.md.source) | 79bbcb800c438431f59b85575ff83514f8641ac41bfa0090a9cafd606c1c98af |
| [docs/plans/registry-recovery.md](plans/registry-recovery.md) | e4bd42c96843fa3975fb1a5ad4ab91801b44ba0240b01b510b4a2571e5be274e |
| [docs/plans/E-GR-SCOPE-requirements-and-preflight.md](plans/E-GR-SCOPE-requirements-and-preflight.md) | 0f0749ec3975af8ee98a4635494f12e79ebedc5e061b0a7349db73adbc1780f6 |
| [docs/plans/E-GR-AUTH-enrollment-and-grants.md](plans/E-GR-AUTH-enrollment-and-grants.md) | 03a25a2ca9f07cec205c77b5419077f9c1508f3ad7f330ca6edb89f1696acfec |
| [docs/plans/E-GR-KEYS-persistent-signing-keys.md](plans/E-GR-KEYS-persistent-signing-keys.md) | c50248ce4fced0b6ef006451bd3c539b58957b18cacc5c20edb7562273afc0b9 |
| [docs/plans/E-GR-INTERFACE-shared-component-interfaces.md](plans/E-GR-INTERFACE-shared-component-interfaces.md) | 5e1e41e3bba077c20c9a444f9919085e7d371f456cb6726710d98d6ef2299c29 |
| [docs/plans/E-GR-WIRE-canonical-registry-wire.md](plans/E-GR-WIRE-canonical-registry-wire.md) | 2fc8d621a0bd2b56d42eb892ffe689d536339e6512300c91f9d43a1a3b250358 |
| [docs/plans/E-GR-EVENT-durable-revocation-feed.md](plans/E-GR-EVENT-durable-revocation-feed.md) | 5223897bddfb4a1dad8813f448936f03c7208ff1d32317f2ba5afea5a93e637a |
| [docs/plans/E-GR-TREG-treg-action-artifacts.md](plans/E-GR-TREG-treg-action-artifacts.md) | b3761ceb8750f7083f3b1d8552a289474278474f456d2ba6eacc8c0238edc1f5 |
| [docs/plans/E-GR-COMPOSIO-composio-action-artifacts.md](plans/E-GR-COMPOSIO-composio-action-artifacts.md) | 8c084bf43c9e3592c301b6d53a002f21fe77fa76e7b1df29291b52f5eeab14b2 |
| [docs/plans/E-GR-PUBLISH-canonical-publication.md](plans/E-GR-PUBLISH-canonical-publication.md) | b151eaf88c59c2ffd5d70961880cf425b760c1db8fa167fd34595f63d9ba3dbe |
| [docs/plans/E-GR-CORE-startup-and-event-composition.md](plans/E-GR-CORE-startup-and-event-composition.md) | 0f85fdc7cf24b4ce832b8fbc3b3708f8d1b6450ec5437d915b2e651837c88da2 |
| [docs/plans/E-GR-INTEGRATE-composition-and-interoperability.md](plans/E-GR-INTEGRATE-composition-and-interoperability.md) | 67aa355220bd02c4910c62ed44b6bec97a139f984263ee9b5eb2d492ab2da77c |
| [docs/plans/E-GR-AWS-aws-origin-and-infrastructure.md](plans/E-GR-AWS-aws-origin-and-infrastructure.md) | 62109b991866804e749820fb3a2f2ef751df5fcfdda7cb8ef3aaed992489f826 |
| [docs/plans/E-GR-RELEASE-release-acceptance-and-operations.md](plans/E-GR-RELEASE-release-acceptance-and-operations.md) | bd6d7099349a98d21bf53194e7da79c1e9ad4661d0f6552254b1fd8544d37dd2 |
| [docs/plans/E-GR-NEXT-remaining-registry-milestones.md](plans/E-GR-NEXT-remaining-registry-milestones.md) | 2b849d6760194dd9e1c3c5309e373d9df9aa30f794ff947c3fd846ce7fd6cfc1 |
| [docs/plans/E-GR-M2B-oauth-preview-and-production.md](plans/E-GR-M2B-oauth-preview-and-production.md) | 92a84ea8ab7269ad41c6310e5e72afddc4da51dc1dbb4cd0b73e0fe21c4b26d1 |
| [docs/plans/E-GR-M3-clients-runtime-and-production-metrics.md](plans/E-GR-M3-clients-runtime-and-production-metrics.md) | 29e8db76a90d68184619fe416f7bf1445eec51159be5a6e954dad8fe8721deb6 |
| [docs/plans/E-GR-PROD-terminal-production-delivery.md](plans/E-GR-PROD-terminal-production-delivery.md) | fb903764ba01f0de7100c9e7b6f0f6c51787c24e5052f76c67a1a61db821cc2d |
