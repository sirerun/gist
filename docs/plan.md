# Plan: Gist repository delivery

Plan ID: gist:delivery
Authority ID: github:sirerun/gist
Authoritative source: docs/plan.md. Normalized JSON is derived interchange only.

## Conformance repair -- 2026-10-08

This single-file plan consolidates all 266 existing tasks from main ad6b3fa, including 18 historical March tasks and 248 active registry tasks. IDs, checkboxes, owners, canonical dependencies, existing acceptance and authored evidence are retained. Missing 31 stages and 18 acceptance annotations are recovered from the original descriptions; Historical prerequisites omitted by the local parser are made explicit from the original task bullets; 74 cross-epic metadata references use unique local IDs without changing their canonical targets. Provenance, preserved source digests and reader bindings are in the [repair receipt](receipts/2026-10-08-plan-conformance-repair.md). Checkbox completion remains authored status, not authenticated evidence or runtime admission.

Current owner direction (2026-10-16): reusable Gist, library and Zati first, catalog only with execution elsewhere. [ADR016](adr/016-zati-library-first-rfc003-gateway-boundary.md) retires RFC-003 execution, provider-token custody, approvals/journals and utility billing. Registry authentication and publisher provenance remain. Hosted expansion pauses except repairs and existing consumer obligations, with no service shutdown. The PR62 decision candidate changed documents only. Later explicitly recorded ship scope and evaluation planning below supersede that implementation hold only for their named library work; deployment and spending remain separately gated.


Current composition direction (2026-10-17): [ADR021](adr/021-capability-composition-and-bounded-discovery.md)
and E-COMPOSE below own new reusable capability composition, Zati migration and
10k/100k qualification. Earlier inbox/composite source and evaluation sections
are retained historical evidence, not generic composition readiness. All old
Qwen launch envelopes below are CLOSED/EXPIRED; the retained outcome has zero
model requests and 12 NOT_RUN attempts. Fresh runtime authority is required.
E-COMPOSE does not reopen hosted expansion, execution or semantic adoption.

Current scoped hosting resumption (2026-10-17): the owner explicitly instructed
writing proper IaC and deploying through AWS CLI and Cloudflare MCP. E-HOSTED
below resumes only the existing registry-aws catalog service at gist.sire.run,
including required startup/auth configuration and live acceptance. Historical
hosted holds remain for unselected expansion, provider comparisons and gateway
execution. The accepted ADR008 stack architecture, standing apply grant and
data protections apply. No Zati marketplace rollout is implied by this hosting
request.

### Dispatch disposition and precedence

This section supersedes older continuation instructions, milestone tables and unqualified “current” infrastructure notes below, which are dated historical records. All original task rows, IDs, completion states and acceptance evidence remain. Unfinished gateway rows depend on the permanently non-dispatchable retirement sentinel; catalog provenance rows .31/.35 are retained under catalog ownership with replacement prerequisites. Unfinished prior hosted/guidance rows depend on the hold sentinel. A bounded repair/current-consumer obligation requires a separately recorded scoped resumption, not blanket reactivation. No held or retired row is counted complete. The library terminal depends on neither the retired graph nor hosted production.

Non-executable sentinel stages deliberately preserve true disposition rather than masquerading as implementation. They are not work to run or owner questions waiting for an answer. The metadata gate must normalize them and all their dependents as blocked.


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

Guided Luna assignments retain coordinator ownership of PUBLISH.0 and the authoritative plan. PUBLISH.1 uses a shared-interface author first, followed by isolated pure validator, durable storage and REST/app authors; each has explicit file ownership under one coherent implementation outcome. Coordinator integrates, owns shared plan/composition decisions, and keeps actual-store verification, independent review and landing separate. Contract author owns only contracts/registry/v2; staging and typed/consumer auditors own separate preflight receipt files. Workers use isolated DGX worktrees and one fair dispatch turn each through the qualified shared queue, bounded to 30 minutes for implementation authors (20 minutes for the completed preflight/review lanes). Read-only audits may run beside the contract author at eligible capacity; production handlers remain held until PUBLISH.11. Coordinator alone integrates shared documents and closes tasks only against actual evidence. Current PUBLISH.1 lane ownership: luna-decoder owns only new hosted/internal/publicationv2 (strict decoding, frozen-schema parity, pure evidence checks, receipt and targeted units); luna-storage owns hosted/internal/storage and migration 009 (durable attempts/ledger, exact owned object keys, raw catalog projection, cleanup and storage units); luna-app owns hosted/internal/app, hosted/internal/rest, hosted/acceptance/wiring and narrowly scoped hosted/internal/resolution document-pin compatibility (current authority/evidence/offline verifier, authenticated routes, composition, maintenance and actual HTTP/store acceptance). Coordinator owns plan, docs, dependency changes, integration and aggregate checks. These are file assignments within PUBLISH.1, not independent merge or production authority. The shared interface commit cec59810ec88fd8a9721ea5f34a6937d0452d257 is integrated as 483f1ca9746bd3268c07eb07238b213378ae4ff1; interface-only gofmt/diff checks are not behavior acceptance. Integration continuation assignments refine these same PUBLISH.1 file boundaries: luna-storage-pg owns publication.go, postgres.go, migration009 and new restricted-role PostgreSQL transaction/cleanup tests; luna-object-safety owns objects.go, objects_fs.go, objects_s3.go and object backend tests; luna-app-r2 owns the app/REST/resolution/wiring paths and actual per-kind authenticated HTTP acceptance. Coordinator added explicit server-validated AdmissionEvidence and exact ArtifactSize port fields to close taxonomy rights and durable-object length seams. First aggregate units passed decoder/REST/app but exposed legacy FS cleanup and S3 conditional-write regressions; no PUBLISH.1/2/3 completion is asserted until those and all actual-store acceptance pass. The app continuation proved only the actual restricted-role TLS skill journey; luna-app-matrix owns the same app/REST/resolution/wiring paths to complete the other five kinds, genuine synthetic offline conformance execution and denial matrix. Its qualified fixture executor is source/local evidence only; production verifier/provider qualification remains in separate gates. Coordinator owns the common synthetic HTTP fixture and six-kind source matrix; luna-cleanup owns publication.go, migration009 and storage publication_v2 tests for retired retry, late PUT, deleting claim, fairness/fault and foreign-write RLS acceptance. Remaining dependency-ready authors receive smaller explicit case groups rather than incomplete whole-matrix handoffs. Current bounded continuation owners: luna-authority-denials owns only a new authority/evidence HTTP integration test file; luna-dependency-proof owns only a new dependency/verifier/resolution integration test file; luna-budget-restart owns only a new budget/restart HTTP integration test file. They use the common committed restricted-role TLS harness; coordinator alone fixes shared app/ports/config and integrates results. Six-kind happy-path and cleanup local checks do not close these denial/compatibility requirements.

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

## Canonical delivery plan and preserved source history

This file (`docs/plan.md`) is the sole active delivery graph for RFC-002 and RFC-003. Earlier split plans remain available as byte-preserved historical source snapshots under [the plan archive](plans/archive/20261011/); their former paths now contain pointers only. Do not dispatch or infer current status from split-plan copies. The authoritative task IDs, owners, dependencies, and evidence are below.

## Full AWS production continuation — 2026-10-04

At the 2026-10-08 checkpoint, the founder selected registry plus caller-owned integrations, with registry completion only when https://gist.sire.run is running qualified AWS production. ADR014 (2026-10-10) reopened RFC-003 gateway delivery; the owner direction in ADR016 (2026-10-16) makes that gateway conditional on a separately identified client and removes it from Zati's library path. The current gateway and duplicate dynamic-guide task graphs need reconciliation before dispatch. This file remains the canonical work graph and supersedes earlier recovery-only closure/model assumptions; all March authored text and checked tasks above remain historical. The former [registry recovery plan](plans/registry-recovery.md) is a pointer to its preserved snapshot, not an execution plan or terminal predicate. GPT-6-Luna pool limits, ownership, qualified operation routing, and the production predicate are recorded in this canonical graph and its current receipts. No local merge/initial rollout/planning checkpoint completes this delivery. All stages and external authority are enforced from its task dependencies and evidence; /plan itself executes nothing.

### E-GR-PRODUCTION -- Full SDLC to canonical AWS production

Date: 2026-10-08. Status: **CORE source landed; owner-approved v2 publication preflight ready; remaining hosted work paused except repairs by owner direction dated 2026-10-16**.
Current execution scope: preserve the running service and accept necessary correctness, security, or regression repairs. Do not start new hosted RFC-002 feature, release, or deployment work until the owner resumes that scope. This hold does not retire or authorize shutdown of the service.
Planning contract refreshed 2026-10-11; owner direction updated 2026-10-16. This canonical plan consolidates and supersedes the preserved [registry buildout source](plans/registry-buildout.md); completed historical tasks and evidence remain in the archive. RFC-003 gateway delivery is conditional under [ADR016](adr/016-zati-library-first-rfc003-gateway-boundary.md); its current task graph is preserved pending reconciliation and is not admitted for dispatch. Destination follows accepted ADR011. March task text and completed status remain historical. No executing lifecycle is admitted by this planning refinement.

## Current alignment and execution baseline -- 2026-10-08

The 2026-10-16 owner direction supersedes the prior optional-Gist-to-Zatiti boundary: Zati is the flagship and must use Gist as a library. Zati already embeds the root package for local shortlisting, while hosted catalog retrieval/resolution is not yet exposed as a public consumer API. ADR016 keeps authority, skill lifecycle, bindings, approvals and effects in Zati. Discovery/resolution never grants execution authority. Independently configured direct connections retain their own qualified binding and governance; they are not an automatic bypass for an unavailable, expired or revoked Gist grant. Hosted RFC-002 work is paused except for repairs; no hosted deployment or shutdown is authorized by this direction.

The RFC-002 hosted-service goal remains the registry with caller-owned integrations running at https://gist.sire.run on AWS. The in-process Gist library plus real Zati consumer path is a separate first-class acceptance goal. The registry requirements/design, implementation, testing, independent review, merge, landed verification, release, deployment, live acceptance and operations remain first-class hosted-service gates. RFC-003 gateway delivery is conditional on a separately named client need and no longer gates Gist or Zati completion. Historical gateway and dynamic-guide task rows remain in this candidate pending reconciliation against ADR016; they are not current admission to gateway implementation or hosted guide deployment. Any Zati consumer acceptance must use a real skill with both an asset and an admitted required capability, not a synthetic catalog fixture.

Current source baseline is dacb0c92d2a535e45066927e59eb67d155a86065: PR55 CORE43/43 and PR56 checkpoint are landed. Prior dated execution entries below are historical receipts, not current host admission. The owned DGX transfer preserves bundled history (including worktree heads),36unfinished paths and original local copies; archive and per-file hashes verified, no active Git operation, no credentials transferred. See [current preflight](receipts/2026-10-08-alignment-and-dgx-preflight.md).

Fresh configured Cloudflare MCP GETs on2026-10-08 returned200 for one active canonical zone and zero exact service-name DNS records. This supersedes earlier missing-binding observations only for connection/read availability. Write permissions, reviewed target records and rollout readiness remain unqualified; AWS identity/stack-account matching was not refreshed. Publication v2 implementation was approved on2026-10-08; its preflight/contract freeze is now ready. Identity/workspace custody, bounded provider approval, actual consumer acceptance and AWS target access remain open gates. No release, deployment, provider call or production acceptance occurred.

## Context and outcome

Deliver the complete agreed RFC-002 hosted registry and caller-owned integrations in production on AWS at **https://gist.sire.run**, while first prioritizing the library capabilities and Zati consumer slice selected in ADR016. Gist publishes/discovers/resolves; Zati owns eligibility, authorization, context, credential custody, approvals and effects. Requirements, design, preflight, coding, behavior/security acceptance, format/lint/CI, independent code review, corrections/re-review, merge, landed checks, immutable release, authorized rollout, live/pilot acceptance and bounded operational handoff remain first-class for their applicable hosted-service scope.

RFC-003 gateway delivery is conditional on a separately named independent client that lacks its own admitted execution path and on separately accepted scope. Arbitrary provider proxying, global client setup, organization bootstrap, private application implementation and marketing publication are not implied. Portfolio hierarchy is consumer-owned; chiefs do not inherit cross-business credential/memory authority. Hosted memory and private relationship-data isolation remain separate qualification obligations. Production deployment at the stated AWS origin is explicitly in scope for RFC-002. Planning itself never performs deployment, account enrollment or provider spending; existing scoped grants persist and genuinely missing numerical/custody authority stays explicit.

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
| RFC-003 managed execution gateway | GATEWAY active, G0.4 gated | Included for Gist by ADR014; implementation waits for approved design and plan conformance |

Each named task below is a planning task record, not a minted external-service task ID or admission. `acc` is opaque authored acceptance because Kazi is installed. Explicit `lane: agent` coding markers select the user-requested GPT-6-Luna worker instead of the default Kazi authoring lane; the user model instruction overrides any legacy frontier-model suggestion. Kazi may supply a qualified acceptance draft/verify helper just in time without replacing the specified worker model. Stage rows execute through stage guidance, not Kazi code convergence. Preserve canonical lifecycle tasks instead of this graph if the change is later enrolled.

## GPT-6-Luna concurrency contract

- All delegated implementation, verification, review and landed-check workers use **GPT-6-Luna**, explicit model binding `gpt-6-luna`. One coordinator retains requirements, contracts, ownership, plan writes, integration arbitration, decision routing and final verification. No automatic paid model/provider fallback.
- Use the maximum eligible GPT-6-Luna capacity from real dependency readiness, isolated ownership, independent reviewers and qualified resources. Do not reserve a synthetic worker count or infer concurrency from unused harness slots. The shared subscription bucket currently has an aggregate ceiling of four sessions and minimum launch spacing of sixty seconds; these are conservative dispatch settings, not published provider quotas. Read the live shared queue/cooldown before dispatch.
- Active non-Apple implementation, verification and workers run on qualified DGX storage. The Mac is limited to lightweight coordination and transfer. New task/worktree/cache/artifact directories are isolated under the owned task alias; preserve original checkouts and all unfinished source/history/receipts. Transfer hashes must match before resuming; credential stores, environment secrets and signing keys stay in their existing custody. Existing Apple-native exceptions belong to their owners.
- Use the existing cross-project atomic dispatch lease and shared queue/cooldown. Admit one session per fair turn, record acknowledgement/failure, release the exact owned token, and honor Foundry/Zatiti dependency-unblocking priority without starvation. A 429 pauses the shared bucket: honor Retry-After, otherwise start at sixty seconds and double up to fifteen minutes with jitter, halve concurrency and double spacing. After cooldown admit one probe; increase by at most one after five healthy minutes with twenty percent headroom. More hosts or paid fallback do not remove model quotas.
- Qualify DGX load, storage, tools, cache bounds and actual worker processes before starting. Keep at most two heavy build lanes per project and one full race suite; Mac build leases do not establish DGX capacity. Existing shared resources and foreign claims remain untouched. Documentation-only checks do not trigger application builds.
- Workers receive exact owned files, task IDs, base/head and accepted product/contract readbacks. Independent review uses a different author identity and the exact candidate. Refill only dependency-ready work; unavailable approval, model capacity, access or tooling blocks the affected stage. Coordinate through the existing project ajent.social, with no second bus, wake loop or global notifier.

## Schedule and dependency policy

The founder explicitly requested the entire SDLC as first-class items, so known in-scope recovery deliverables include gated release/live/operations rows. Those rows state verifiable outcomes rather than inventing a deployment design; their preflights revalidate landed inputs and authority. RFC-002 registry production remains a terminal hosted-service delivery, and the library-first Zati consumer path needs its own complete SDLC rows. Under ADR016, RFC-003 gateway production is conditional and is not a whole-project terminal. The separate dynamic-guide graph is also pending reconciliation into the library path rather than an assumed hosted endpoint. This canonical plan contains the machine-readable task assignments and dependencies; archived split plans are source history only. Wave labels describe the horizon; they do not forbid pipelining independent verified candidates. Merge mutations to main are serialized by the coordinator, but unmerged branch work and disjoint reviews remain parallel. Real shared interface dependence requires landed receipts; speculate only with explicitly recorded immutable inputs and no delivery authority. Estimates remain TBD pending bounded preflight sizing, rather than inventing dates from unresolved scope.

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
- [ ] T-GR-SCOPE.4 Complete RFC-002 operator enrollment inputs and credential custody record  stage: preflight  Owner: coordinator-founder  Est: TBD  kind: human  verifies: [UC-009]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.2]  acc: [Approved enrollment mechanism is permissions-free GitHub App sign-in with stable numeric account ID, invitation to one private workspace, no stored GitHub token, and Gist-issued bounded grants; record exact invited workspace and operator account when supplied; separate Treg/Composio provider connections from identity; record workload-parent grant authority and human custodian; reject manual SQL, public bootstrap, and asserted operator flags]  blocked: Enrollment mechanism approved; concrete workspace and operator account are pending owner input
- [ ] T-GR-SCOPE.5 Record scoped live environment and numerical provider authority  stage: preflight  Owner: coordinator-founder  Est: TBD  kind: human  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.2]  acc: [Record the explicit AWS production scope at https://gist.sire.run, reuse applicable accepted ADR008 stack grants without asking again, and record exact operator, permitted rollout plus any separately necessary preview/build envelope, provider/account/action/target, allowed request counts, per-call/per-user/aggregate ceilings and expiry; reconcile prior deployment grants without broadening them; current credits are not a numerical authority grant]  blocked: Provider numerical ceilings and any additional billable preview/build scope remain to be reconciled; AWS production destination is explicitly selected
- [x] T-GR-SCOPE.6 Qualify worker capacity, SSD worktrees and stage tooling  Owner: luna-preflight-capacity  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  acc: [Record available agent slots and GPT-6-Luna model binding; external build volume is mounted/writable with measured free space sufficient for planned worktrees and caches; uniquely allocate each worker external-SSD worktree and ownership/claim; qualify plan/stage tooling without global plugin activation; unavailable SSD or model binding blocks dispatch rather than falling back]
- [ ] T-GR-SCOPE.7 Collect exact consumer execution and import readiness receipt  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.2]  acc: [Consumer owner supplies pinned source/build and admitted registry OAuth/import/native mapping/per-connection profile, protected funding and persistence contracts; callback/key-header handling and unknown outcomes are qualified; no business hierarchy grants cross-business credentials or memory]  blocked: Consumer owner reports runtime, per-profile header/custody and protected operator funding qualifications are not yet admitted; planning PR is not a runtime receipt
- [x] T-GR-SCOPE.8 Qualify required CI and merge policy  Owner: luna-preflight-ci  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  acc: [Record current GitHub required checks and whether jobs can start; preserve required checks; reconcile accepted ADR008 local-validation authority and current branch protections; reuse an applicable existing alternative without asking again, while any genuinely required unsatisfied check keeps its merge blocked; local passes never become CI evidence]
- [x] T-GR-SCOPE.9 Validate requirements/design handoff before lane admission  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.2, T-GR-SCOPE.3, T-GR-SCOPE.6]  acc: [Each lane being admitted has one accepted interface/write set, a RED baseline defect or new-capability criterion, mandatory negative fixtures and named independent review path; explicitly record held lanes and their unresolved choices so they do not prevent admission of independent qualified lanes; contracts remain compatible and no task starts from an unapproved trust or external-action choice]

- [x] T-GR-SCOPE.10 Inventory exact provider documentation and safe capture requirements  Owner: luna-preflight-providers  Est: TBD  kind: agent stage: preflight  verifies: [UC-002, UC-004, UC-005, UC-010]  acc: [Official Treg/Composio schema/version/auth/price/license sources and proposed read action capture paths are recorded without installs, credentials, session setup or underlying endpoint calls; identify missing real metadata and required capture permission; untrusted documentation cannot grant execution authority]

- [ ] T-GR-SCOPE.11 Qualify automation bindings for AWS production and DNS operations  Owner: luna-production-preflight  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.14, T-GR-SCOPE.17]  acc: [Record aws/Pulumi/gh and configured Cloudflare MCP bindings, actual permitted AWS operator role and stack workflow, CodeBuild/ECR artifact path, private IaC canonical task ownership and existing numerical grants; distinguish installed CLI from auth/permission; accepted ADR008 local validation is reconciled with current protections; unavailable binding is a real capability block, not an artificial human handoff]  blocked: The 2026-10-11 refresh confirms the AWS user is in the Organizations management account and the registry-aws target `gist-registry` member account is ACTIVE, consistent with the documented cross-account topology; no role assumption or deploy permission was tested. Cloudflare shows an active sire.run zone and a registry.sire.run CNAME, but gist.sire.run has no record and public HTTPS cannot resolve it. OAuth authorization-server metadata from the existing registry origin advertises that old origin as issuer and authorization/token endpoints; the health 200 is not canonical Gist acceptance. Current role trust/operator permission, deployment and CodeBuild/ECR path, issuer/redirect migration, numerical change/cost grants and DNS-write authority remain unqualified; see [binding refresh receipt](receipts/2026-10-11-production-binding-refresh.md). Read-only evidence does not qualify production writes

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

### Dynamic caller-guidance direction delivery
- [x] T-GR-SCOPE.27 Carry forward the endorsed dynamic tool-use guidance direction  Owner: root  Est: 30m  kind: agent  stage: author  verifies: [UC-004, UC-008]  blocked-by: [T-GR-SCOPE.19]  delivers: [RFC-002 and RFC-003 direction plus gateway-plan note]  acc: [Record task-specific caller guidance assembled from validated capabilities, maintained recipes and current authorized resolution; keep authored craft skills separate, disclose provenance and freshness, include no credentials, and grant no execution authority; preserve frozen registry v1 and the optional managed-gateway boundary]
- [x] T-GR-SCOPE.28 Verify the scoped documentation candidate and production preflight refresh  Owner: root  Est: 30m  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-GR-SCOPE.27]  delivers: [Current binding preflight receipt and conformance evidence]  acc: [Only intended documentation and evidence files change; no registry v1 wire/source inventory or provider/runtime behavior changes; plan graph and relative links validate; current AWS and Cloudflare observations remain distinct from deployment/write permission]
- [ ] T-GR-SCOPE.29 Independently review the complete dynamic-guidance and plan candidate  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.28]  acc: [A non-author reviewer records exact base/head, all changed paths, concrete findings and dispositions; accepted findings receive tracked fixes, affected verification and exact-head re-review before merge]
- [ ] T-GR-SCOPE.30 Guarded merge of the independently reviewed documentation candidate  Owner: root  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.29, T-GR-SCOPE.8]  acc: [The exact approved head merges by GitHub rebase under unchanged protections; unavailable hosted checks remain distinct from local evidence; actual merge and landed source revisions are recorded]
- [ ] T-GR-SCOPE.31 Verify the landed guidance, plan and preflight evidence  Owner: root  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.30]  acc: [Fresh remote main contains the reviewed files with tree parity; actual Wazi and documentation checks pass on the landed SHA; frozen v1 bytes and provider/runtime scope remain unchanged; ownership is handed off]
### E-GR-AUTH -- Enrollment and grants

fidelity: executable
Acceptance: Invited operator-proven user obtains and refreshes catalog:read without seeded tables; operator-authorized workload issuance is available through the approved boundary and preserves exact parent/workspace/audience/scopes; unauthenticated bootstrap never succeeds

Use cases: UC-007, UC-009. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own new enrollment domain/HTTP files, enrollment tests, allocated identity-store/invitation migration and narrowly assigned OAuth enrollment files. Existing shared ports, oauth storage and app.go change only through explicit coordinator allocations. Never edit key material, event tables or another lane tests. Preserve login delegation; no password database or manual SQL.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [x] T-GR-AUTH.0 Preflight enrollment and grants requirements, design and write set  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-007, UC-009]  blocked-by: [T-GR-SCOPE.9]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; implement operator sign-in only through ADR015's permissionless GitHub App numeric-ID proof with no persisted GitHub token; keep MCP OAuth and Gist workload grants separate; use synthetic subject/workspace and controlled GitHub API fixtures for source and preproduction tests, so no real account or workspace is required to design or implement; exact owner-selected operator/workspace values remain required for live enrollment and the production terminal; qualify auth/key/fixture/version/capture permissions and license; claim task/resource and isolate DGX worktree before writing; unresolved required decision blocks only the dependent path]  Done: 2026-10-11  Evidence: [RFC-002 operator enrollment preflight](receipts/2026-10-11-rfc002-auth-preflight.md)
- [x] T-GR-AUTH.1 Implement invite-only GitHub operator enrollment component  Owner: luna-auth  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.0]  acc: [GitHub App user authorization requests no repository or organization permissions/scopes, uses the exact configured callback, state and PKCE S256, and calls only GET /user; no authorization code or GitHub token is persisted or logged; one-time owner bootstrap is limited to the exact configured founder numeric ID/workspace and disables after first success; an authenticated maintainer can issue expiring single-use invitations bound to exact numeric ID/workspace; invitation redemption and workspace membership commit atomically; no manual SQL or public bootstrap; runtime MCP OAuth and Gist workload grants remain separate; synthetic GitHub identity and PostgreSQL fixtures exercise the enrollment handler without seeded tables; the component returns only its workspace-bound host session through the injected SessionIssuer and does not itself issue or refresh a Gist OAuth access token; complete app startup, consent, bounded catalog:read access-token refresh and revocation linkage are verified in T-GR-INTEGRATE.2; source tests imply no live account, invite, credential or GitHub App registration; handoff PR/base/head/tests without merging]  Done: 2026-10-11  Evidence: [PR #63 component and exact test receipt](https://github.com/sirerun/gist/blob/e943603bf26e1e14f02938fde5ce31f04da50fa9/docs/receipts/2026-10-11-rfc002-auth-component.md)
- [x] T-GR-AUTH.2 Verify enrollment component behavior and denial matrix  Owner: luna-verify-auth  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.1]  acc: [Golden GitHub enrollment/consent and actual HTTP status/body tests; confirm no repository or organization permissions, only GET /user, exact numeric-ID invitation matching, no GitHub token/code persistence, and wrong identity/workspace, missing config, expired/replayed invite/code/state, PKCE mismatch, external return path, denied consent, widened scopes, repeated bootstrap, unauthorized/cross-workspace/CSRF invitation issuance, revoked membership and operator impersonation deny; use controlled browser/API fixtures and restricted-role PostgreSQL migration tests; Gist OAuth access-token issue/refresh, wrong audience and member/parent revocation through the composed app remain in T-GR-INTEGRATE.2; record exact tested source SHA and no skipped-as-pass outcomes]  Done: 2026-10-11  Evidence: [PR #63 component and exact test receipt](https://github.com/sirerun/gist/blob/e943603bf26e1e14f02938fde5ce31f04da50fa9/docs/receipts/2026-10-11-rfc002-auth-component.md)
- [x] T-GR-AUTH.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-auth  Est: TBD  kind: agent  stage: verify  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.1]  acc: [Changed Go files are gofmt/goimports clean, scoped GOWORK=off go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; Go toolchain is module-qualified; record real CI separately and no broad formatting of other ownership]  Done: 2026-10-11  Evidence: [PR #63 component and exact test receipt](https://github.com/sirerun/gist/blob/e943603bf26e1e14f02938fde5ce31f04da50fa9/docs/receipts/2026-10-11-rfc002-auth-component.md)
- [x] T-GR-AUTH.4 Independent code and security review of T-GR-AUTH.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.2, T-GR-AUTH.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]  Done: 2026-10-11  Evidence: [auth review and landed verification](receipts/2026-10-11-rfc002-auth-merge-verification.md)
- [x] T-GR-AUTH.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.4, T-GR-SCOPE.8]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]  Done: 2026-10-11  Evidence: [auth review and landed verification](receipts/2026-10-11-rfc002-auth-merge-verification.md)
- [x] T-GR-AUTH.6 Verify landed enrollment and grants behavior  Owner: luna-landed-auth  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-007, UC-009]  blocked-by: [T-GR-AUTH.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]  Done: 2026-10-11  Evidence: [auth review and landed verification](receipts/2026-10-11-rfc002-auth-merge-verification.md)

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

- [ ] T-GR-TREG.0 Preflight treg action artifacts requirements, design and write set  Owner: luna-treg  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.9, T-GR-SCOPE.10]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]  blocked: Exact schema and request mapping, license and retention, account and credential route, owner-selected target and limits remain unqualified; see October6 preflight receipt
- [ ] T-GR-TREG.1 Capture and publish one real immutable Treg read capability closure  Owner: luna-treg  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-TREG.0]  acc: [One selected exact endpoint has real pinned schemas/provenance, method/recipient/effect/credential-route and cost-unit/upper-bound metadata; complete provider/tool/binding/skill bytes have verified digests and a caller-execution requirement; no generic call grants blanket read authority; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [ ] T-GR-TREG.2 Verify changed behavior and denial matrix  Owner: luna-verify-treg  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-TREG.1]  acc: [Offline schema/closure/import fixtures reject missing or drifted schema, unknown pricing, disallowed recipient/overflow route, changed binding/digest and synthetic metadata; external capture methods are documented and require separate scoped account authority when authenticated; no billed endpoint call in this lane; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [ ] T-GR-TREG.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-treg  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-TREG.1]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [ ] T-GR-TREG.4 Independent code and security review of T-GR-TREG.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-TREG.2, T-GR-TREG.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [ ] T-GR-TREG.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-TREG.4, T-GR-SCOPE.8]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [ ] T-GR-TREG.6 Verify landed treg action artifacts behavior  Owner: luna-landed-treg  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-TREG.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]


2026-10-06 metadata-only preflight: [current source and remaining blockers](receipts/2026-10-06-treg-artifact-preflight.md). The exact source/version/license/account/target qualification is incomplete; .0 remains open and no executable artifact or live call is admitted.

### E-GR-COMPOSIO -- Composio action artifacts

fidelity: executable
Acceptance: Actual selected GitHub PR-read or subsequently approved action schema/version/account/session constraints form a complete immutable closure; preserve historical source_unavailable/synthetic fixtures; generic execute/meta/proxy/workbench access and instant charging are excluded

Use cases: UC-002, UC-004, UC-005, UC-010. Historical crosswalk: this is follow-on repair/qualification, not a retick of the original registry buildout.

**Ownership:** Own new catalog/registry/captures/composio-* and catalog/registry/packages/composio-* plus assigned conformance tests. Shared catalog indexes and canonical binding selection are integration-owned. Evidence license/redistribution and exact session transport/version before public capture; a docs URL is not runtime conformance.

**Evidence boundary:** these component candidates are not the composed application. E-GR-INTEGRATE owns the real startup/routes and joined behavior. Preflight enumerates exact changed files and RED-before/GREEN-after evidence. Estimates remain TBD until that bounded scope is confirmed; no invented delivery dates.

#### Wave 1: Independent component SDLC

- [ ] T-GR-COMPOSIO.0 Preflight composio action artifacts requirements, design and write set  Owner: luna-composio  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.9, T-GR-SCOPE.10]  acc: [Record exact base SHA, named scope/files, component interface and meaningful tests; qualify applicable auth/key/fixture/version/capture permissions and license; claim task/resource and isolate external-SSD worktree before writing; unresolved required decision blocks this lane only]  blocked: Qualified metadata-only key binding and capture authority, action schemas and mapping, license and retention, account and target limits remain unqualified; see October6 preflight receipt
- [ ] T-GR-COMPOSIO.1 Capture and publish one real Composio action-specific capability closure  Owner: luna-composio  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-COMPOSIO.0]  acc: [Actual selected GitHub PR-read or subsequently approved action schema/version/account/session constraints form a complete immutable closure; preserve historical source_unavailable/synthetic fixtures; generic execute/meta/proxy/workbench access and instant charging are excluded; hand off PR URL, candidate/base SHA, changed file list and test evidence without merging]
- [ ] T-GR-COMPOSIO.2 Verify changed behavior and denial matrix  Owner: luna-verify-composio  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-COMPOSIO.1]  acc: [Offline schema/closure/import tests reject synthetic identity action, foreign account/user/toolkit, extra operation, schema/config drift, undeclared fees and unsupported version pinning; capture x-api-key custody/header requirements without secrets; no provider session creation or live call absent scoped authority; record exact tested candidate SHA and failures without skipped-as-pass outcomes]
- [ ] T-GR-COMPOSIO.3 Run owned-file formatting, package lint and required candidate checks  Owner: luna-verify-composio  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-COMPOSIO.1]  acc: [Changed Go files are gofmt/goimports clean, scoped go test/go vet/golangci-lint and applicable Python/schema checks pass on the candidate; GOWORK=off and qualified shared lease/load/cache controls apply; record real CI separately and no broad formatting of other ownership]
- [ ] T-GR-COMPOSIO.4 Independent code and security review of T-GR-COMPOSIO.1  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-COMPOSIO.2, T-GR-COMPOSIO.3]  acc: [Reviewer did not author or coauthor this candidate; record reviewer identity, exact base/head SHAs, covered scope, security/behavior findings and dispositions; accepted blockers have tracked fixes, affected re-verification and independent re-review on final head]
- [ ] T-GR-COMPOSIO.5 Merge exact reviewed candidate through guarded GitHub delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-COMPOSIO.4, T-GR-SCOPE.8]  acc: [Required policy/CI and independent approval are satisfied at exact final head/base; gh merges to main using existing approved method and records PR/head/landed SHA; intervening relevant main/head changes trigger re-verification/re-review rather than accepting stale evidence]
- [ ] T-GR-COMPOSIO.6 Verify landed composio action artifacts behavior  Owner: luna-landed-composio  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-002, UC-004, UC-005, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-COMPOSIO.5]  acc: [Fetch remote main, prove candidate changes are present, run affected acceptance on recorded landed SHA and publish scoped receipt; local branch or merge API success alone cannot complete this row; changed integration dependencies invalidate applicable evidence]


2026-10-06 metadata-only preflight: [current source and remaining blockers](receipts/2026-10-06-composio-artifact-preflight.md). The exact source/version/license/account/target qualification is incomplete; .0 remains open and no executable artifact or live call is admitted.

### E-GR-PUBLISH -- Canonical publication

fidelity: executable
Acceptance: An explicitly settled publication transport publishes every agreed artifact kind through actual authenticated HTTP with immutable bytes, typed conformance, bounded response and replay semantics. Existing route doubles and catalog seeding are not publication acceptance.

Accepted finding PUBLISH-R1: frozen publish.schema requires artifact/max_bytes/idempotency_key but leaves the artifact-object representation unspecified; OpenAPI refers to a different generic body, REST forwards opaque bytes and the app treats every kind as a skill ZIP. No base64, multipart or file-map representation is selected implicitly. Existing frozen entries remain byte-identical until an explicitly approved versioned amendment. Coordinator owns contract integration; a disjoint Luna design author may draft the decision and fixtures without activating it.

#### Wave 1: Contract settlement and source delivery

- [x] T-GR-PUBLISH.7 Draft grounded publication transport decision and compatibility path  Owner: coordinator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-SCOPE.9]  acc: [Compare current frozen schema OpenAPI SDK and actual upload/import paths; provide concrete envelope and skill byte transport alternatives plus recommendation, per-kind typed validation, idempotency byte-budget and catalog/outbox atomicity design; preserve v1 locks and existing caller-owned boundary]
- [x] T-GR-PUBLISH.8 Settle material publication contract decision  stage: preflight  acc: [Review concrete compatibility proposal and explicitly settle canonical artifact representation, skill package transport and any versioned amendment before changing public semantics; offline silence is not approval]  Owner: founder-and-contract-owner  Est: TBD  kind: human  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-PUBLISH.7]  Done: 2026-10-08  Evidence: [owner approval](receipts/2026-10-08-alignment-and-dgx-preflight.md); accepted v2 envelope/base64 skill ZIP/typed raw JSON, frozen v1 unchanged with explicit migration path; no provider/release/deployment grant
- [x] T-GR-PUBLISH.0 Qualify publication contract write set and actual source fixtures  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-PUBLISH.8, T-GR-INTERFACE.6]  acc: [Exact approved schemas and method/media/version binding, bounded publisher write set, tenant-safe staged object ownership/retention/reconciliation and real local acceptance fixtures are recorded; preserve concurrent ownership and use isolated qualified DGX worktree]  Done: 2026-10-09  Evidence: [exact source contracts and fixture preflight](registry/publication-v2-preflight.md); preflight completion preceded gates9/10/11; their actual PR59 closure is recorded below
- [x] T-GR-PUBLISH.9 Independently review frozen v2 publication preflight  Owner: independent-luna-reviewer  Est: 30m  kind: agent  stage: review  blocked-by: [T-GR-PUBLISH.0, T-GR-PUBLISH.14]  acc: [Nonauthor independently approves exact base/head schemas method/media limits digest mappings migration behavior tenant staging retention/reconciliation and implementation fixture/write set; accepted findings retain tracked fix verification and re-review obligations before handlers are written]  Done: 2026-10-09  Evidence: [PR59 actual preflight landing](receipts/2026-10-09-publication-preflight-landed.md)
- [x] T-GR-PUBLISH.10 Guarded merge of frozen publication preflight  Owner: coordinator  Est: 15m  kind: agent  stage: merge  blocked-by: [T-GR-PUBLISH.9, T-GR-SCOPE.8]  acc: [Reviewed exact head merges by guarded gh rebase under current unchanged policy; local evidence remains distinct from unavailable billing-only hosted CI; actual source landing is recorded without runtime authority]  Done: 2026-10-09  Evidence: [PR59 actual preflight landing](receipts/2026-10-09-publication-preflight-landed.md)
- [x] T-GR-PUBLISH.11 Verify actual landed v2 publication contract and preflight  Owner: coordinator  Est: 20m  kind: agent  stage: verify-landed  blocked-by: [T-GR-PUBLISH.10]  acc: [Fresh remote main contains independently reviewed schema and design bytes; actual landed contract fixtures frozen-v1 parity and full plan conformance pass before dependent production handlers start; no provider release or deployment acceptance inferred]  Done: 2026-10-09  Evidence: [PR59 actual preflight landing](receipts/2026-10-09-publication-preflight-landed.md)
- [x] T-GR-PUBLISH.12 Fix accepted PUBLISH59-R1 receipt whitespace  Owner: coordinator  Est: 10m  kind: agent  stage: author  blocked-by: [T-GR-PUBLISH.0]  acc: [Remove only trailing spaces from the copied PR57 landed receipt while preserving all original wording and exact original report outside this candidate; retain the negative independent review and no approval inference]  Done: 2026-10-09  Evidence: [preserved PUBLISH59-R1](receipts/2026-10-09-publication-preflight-review-r1.md)
- [x] T-GR-PUBLISH.13 Verify PUBLISH59-R1 fix against the complete committed candidate  Owner: coordinator  Est: 15m  kind: agent  stage: verify  blocked-by: [T-GR-PUBLISH.12]  acc: [Full base-to-head diff whitespace, graph links archive/frozen-v1/hosted parity contract gate and complete Wazi conformance pass; untracked-file omissions cannot count as full candidate verification]  Done: 2026-10-09
- [x] T-GR-PUBLISH.14 Independently re-review exact corrected publication preflight  Owner: independent-luna-reviewer  Est: 30m  kind: agent  stage: review  blocked-by: [T-GR-PUBLISH.13]  acc: [Nonauthor resolves PUBLISH59-R1 and approves the complete exact corrected base/head candidate including all new recovery tasks and receipts; additional accepted findings require tracked repair verification and re-review before merge]  Done: 2026-10-09  Evidence: [PR59 actual preflight landing](receipts/2026-10-09-publication-preflight-landed.md)
- [x] T-GR-PUBLISH.1 Implement canonical per-kind publication  Owner: luna-publication  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-004, UC-010]  blocked-by: [T-GR-PUBLISH.11]  acc: [Actual handlers consume approved canonical envelope, strict per-kind schema and immutable package closure, authenticated current maintainer and namespace admission, requested/server byte budget before mutation, replay identity, catalog/outbox atomicity and qualified staged object reconciliation without deleting shared committed digests; no skill validator substituted for other artifact kinds]  Done: 2026-10-09  Evidence: [actual source verification and retained failures](receipts/2026-10-09-publication-source-verification.md); source gate completion preceded review/merge/landing; actual PR60 closure is recorded below
- [x] T-GR-PUBLISH.2 Verify publication journeys and denials with real stores  Owner: luna-publication-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-004, UC-007, UC-008, UC-010]  blocked-by: [T-GR-PUBLISH.1]  acc: [Actual authenticated HTTP plus restricted-role PostgreSQL and object store publish and retrieve each agreed kind; meaningful old-source RED and new-source PASS, repeat same bytes/key, changed bytes conflict, malformed/foreign/unauthorized/revoked namespace, oversized request/response and transaction/outbox rollback and physical staging ownership/retention/reconciliation qualify without seeded-catalog success claims]  Done: 2026-10-09  Evidence: [actual source verification and retained failures](receipts/2026-10-09-publication-source-verification.md); source gate completion preceded review/merge/landing; actual PR60 closure is recorded below
- [x] T-GR-PUBLISH.3 Verify publication compatibility race vet lint and contract checks  Owner: luna-publication-verifier  Est: TBD  kind: agent  stage: verify  verifies: [UC-001, infrastructure]  blocked-by: [T-GR-PUBLISH.1]  acc: [Required affected hosted/root CLI unit integration race vet lint schema checks pass on exact final source using fresh load shared lease and SSD rules; approved versioned changes have explicit compatibility fixtures and legacy locks remain intact]  Done: 2026-10-09  Evidence: [actual source verification and retained failures](receipts/2026-10-09-publication-source-verification.md); source gate completion preceded review/merge/landing; actual PR60 closure is recorded below
- [x] T-GR-PUBLISH.4 Independently review final publication candidate  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [UC-002, UC-004, UC-007, UC-008, UC-010]  blocked-by: [T-GR-PUBLISH.2, T-GR-PUBLISH.3]  acc: [Nonauthor and noncoauthor records exact head/base contract approval and integrity/tenant/cost/idempotency/budget/outbox dispositions; accepted findings receive tracked fixes verification and re-review]  Done: 2026-10-09  Evidence: [PR60 exact independent final-head approval guarded rebase and actual landed acceptance](receipts/2026-10-09-publication-landed.md); production remains separately open
- [x] T-GR-PUBLISH.5 Guarded merge of reviewed publication  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-GR-PUBLISH.4, T-GR-PUBLISH.17, T-GR-PUBLISH.20, T-GR-SCOPE.8]  acc: [Immediate final head/base policy and relevant dependency comparison permits guarded gh rebase merge without policy changes; merge is source delivery only]  Done: 2026-10-09  Evidence: [PR60 exact independent final-head approval guarded rebase and actual landed acceptance](receipts/2026-10-09-publication-landed.md); production remains separately open
- [x] T-GR-PUBLISH.6 Verify actual remote-main publication  Owner: luna-publication-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [UC-002, UC-004, UC-007, UC-008, UC-010]  blocked-by: [T-GR-PUBLISH.5]  acc: [Fresh remote main contains reviewed source and actual relevant HTTP/store acceptance passes at recorded landed SHA; no release production enrollment or provider execution inferred]  Done: 2026-10-09  Evidence: [PR60 exact independent final-head approval guarded rebase and actual landed acceptance](receipts/2026-10-09-publication-landed.md); production remains separately open

Historical proposal-draft delivery on2026-10-05: ADR012 and its required compatibility, budget and raw-byte decisions were independently reviewed and landed in PR54 at d75ac183fe8b949a4f1381e8ebca3226f0c0e282. Owner approval was pending at that drafting checkpoint. The2026-10-08 approval below supersedes that status: PUBLISH.8 is complete and source contract preflight is ready. [Landed plan receipt](receipts/2026-10-05-plan-landed-verification.md).

2026-10-08 owner decision: ADR012 v2 source implementation approved. T-GR-PUBLISH.0 can freeze the exact versioned route/schema/readback/strict byte and archive limits, raw-byte fidelity, canonical persistence/outbox and tenant-safe orphan lifecycle. Frozen v1 remains unchanged; production migration/deprecation claims require explicit release evidence. No provider or deployment gate is waived.


#### Accepted coordinator verification finding PUBLISH60-R1

Three independent leaves approved original head864b492 at base8a24c72. A subsequent actual-store shutdown probe still read an owned object after completed App.Shutdown (retained runtime RED). Original approvals remain historical and do not approve the lifetime correction.

- [x] T-GR-PUBLISH.15 Fix PUBLISH60-R1 pinned object-store lifetime  Owner: coordinator  Est: 15m  kind: agent  stage: implement  blocked-by: [T-GR-PUBLISH.1]  acc: [Owned filesystem root closes exactly once after requests and both janitors finish; construction failure closes newly owned backend resources and preserves actual errors; S3 has no invented cleanup effect]  Done: 2026-10-09  Evidence: [lifetime correction real RED/PASS and refreshed source](receipts/2026-10-09-publication-source-verification.md); source correction preceded final-head gate17; actual PR60 closure is recorded below
- [x] T-GR-PUBLISH.16 Verify PUBLISH60-R1 corrected lifetime and affected aggregate  Owner: coordinator-verifier  Est: 20m  kind: agent  stage: verify  blocked-by: [T-GR-PUBLISH.15]  acc: [Retained actual HTTP/store shutdown RED passes at corrected source; repeated shutdown and close are stable; affected hosted unit full app/store integration publication race vet build and delta lint pass with exact source evidence and no source-only or fixture claim upgraded]  Done: 2026-10-09  Evidence: [lifetime correction real RED/PASS and refreshed source](receipts/2026-10-09-publication-source-verification.md); source correction preceded final-head gate17; actual PR60 closure is recorded below
- [x] T-GR-PUBLISH.17 Independently re-review final PUBLISH60-R1 candidate  Owner: independent-luna-reviewer  Est: 20m  kind: agent  stage: review  blocked-by: [T-GR-PUBLISH.16, T-GR-PUBLISH.2, T-GR-PUBLISH.3]  acc: [Nonauthor independently approves exact corrected head/base lifecycle error/cancellation/resource boundaries plus full candidate coverage including current plan and prior approved unchanged source; merge waits for final approval]  Done: 2026-10-09  Evidence: [PR60 exact independent final-head approval guarded rebase and actual landed acceptance](receipts/2026-10-09-publication-landed.md); production remains separately open



#### Accepted independent finding GIST-PUBLISH60-LEAD-R1

Final lead REQUEST_CHANGES at0006a12/base8a24c72: Server.Close after a shutdown deadline does not wait for active handlers; root closure could break an already admitted object reader. Original report is retained. A controlled actual HTTP/store handler reproduced completed cleanup while still admitted; the first fixture teardown hang was corrected before the completed runtime RED. Retained review whitespace is normalized without changing its words.

- [x] T-GR-PUBLISH.18 Fix LEAD-R1 active-handler resource drain  Owner: coordinator  Est: 20m  kind: agent  stage: implement  blocked-by: [T-GR-PUBLISH.15]  acc: [Stop new request admission atomically before drain; keep pinned root and pool alive until admitted handlers and both janitors actually finish after forced connection close; caller deadline still bounds waiting and actual shutdown error is retained; canonical shutdown response remains independently bounded]  Done: 2026-10-09  Evidence: [accepted LEAD-R1 actual drain RED/PASS and refreshed aggregate](receipts/2026-10-09-publication-source-verification.md); source correction preceded exact corrected-head gate20; actual PR60 closure is recorded below
- [x] T-GR-PUBLISH.19 Verify LEAD-R1 forced shutdown with actual active reader  Owner: coordinator-verifier  Est: 20m  kind: agent  stage: verify  blocked-by: [T-GR-PUBLISH.18]  acc: [Actual controlled TLS handler/store old-source RED and corrected PASS prove root stays usable for admitted work until handler return, late admission denies and final cleanup closes root; repeated/deadline behavior and affected hosted integration unit race vet build delta lint and complete candidate diff check pass]  Done: 2026-10-09  Evidence: [accepted LEAD-R1 actual drain RED/PASS and refreshed aggregate](receipts/2026-10-09-publication-source-verification.md); source correction preceded exact corrected-head gate20; actual PR60 closure is recorded below
- [x] T-GR-PUBLISH.20 Independently re-review final active-drain candidate  Owner: independent-luna-reviewer  Est: 20m  kind: agent  stage: review  blocked-by: [T-GR-PUBLISH.19, T-GR-PUBLISH.16]  acc: [Nonauthor records exact final head/base and accepted LEAD-R1 disposition; full candidate and tracked gates remain truthful; approved unchanged source and corrected code are independently covered before guarded merge]  Done: 2026-10-09  Evidence: [PR60 exact independent final-head approval guarded rebase and actual landed acceptance](receipts/2026-10-09-publication-landed.md); production remains separately open



#### Accepted checkpoint finding PUB-CLOSEOUT-PRIVACY-1

Independent checkpoint review REQUEST_CHANGES at7058490/baseeb1bd865 found private home/worktree tool/config paths in the landed command table. The same pattern is present in the earlier source-verification table, so repair covers both current public receipts. Exact private original records and public Git history remain preserved; no historical erasure is claimed.

- [x] T-GR-PUBLISH.21 Fix checkpoint private-path disclosure  Owner: coordinator  Est: 10m  kind: agent  stage: implement  blocked-by: [T-GR-PUBLISH.6]  acc: [Normalize public command tables to portable qualified tool/config forms and remove home paths private hosts and private addresses from current new publication receipts; retain original private evidence and results without rewriting Git history]  Done: 2026-10-09  Evidence: corrected current source and landed receipt command forms; exact raw logs remain private
- [x] T-GR-PUBLISH.22 Verify corrected checkpoint privacy and conformance  Owner: coordinator-verifier  Est: 10m  kind: agent  stage: verify  blocked-by: [T-GR-PUBLISH.21]  acc: [Current new publication receipt scan full diff/link/task-status preservation and full pinned Wazi actual-reader/schema/owning-semantic checks pass on exact corrected source; no code wire archive or production status mutation]  Done: 2026-10-09  Evidence: current receipt privacy scan portable links full diff and original task-metadata preservation pass; full pinned Wazi281 qualified, exact original logs remain private
- [x] T-GR-PUBLISH.23 Independently re-review checkpoint privacy repair  Owner: independent-luna-reviewer  Est: 20m  kind: agent  stage: review  blocked-by: [T-GR-PUBLISH.22]  acc: [Nonauthor records exact corrected checkpoint head/base and privacy finding disposition; final artifact delivery still uses exact-head review guard and actual landed conformance; source and production evidence remain separate]  Done: 2026-10-09  Evidence: [actual R2 privacy-source approval at0c1c9ae/baseeb1bd865](receipts/2026-10-09-publication-checkpoint-review.md); recording this evidence still requires final artifact exact-head review and guarded landing


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

- [ ] T-GR-INTEGRATE.0 Reconcile landed interfaces and integration design  Owner: luna-integrator  Est: TBD  kind: agent  stage: preflight  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PUBLISH.6, T-GR-CORE.6, T-GR-INTERFACE.6, T-GR-AUTH.6, T-GR-KEYS.6, T-GR-WIRE.6, T-GR-EVENT.6, T-GR-TREG.6, T-GR-COMPOSIO.6]  acc: [Record actual landed components and merged dependency graph, key/identity/event transaction contracts, real captures and immutable closure; re-read current main and reserve all shared files; no placeholder component is wired as success]
- [ ] T-GR-INTEGRATE.1 Compose final enrollment provider and consumer journey with landed core  Owner: luna-integrator  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-INTEGRATE.0]  acc: [Real app uses sanctioned enrollment/grants, persistent signing identity, canonical REST/MCP contract serialization, exact provider closure and durable outbox/cursors; remove replaced unsafe startup paths; PR exact-head handoff includes updated existing ADR/design decisions and no incompatible v1 or gateway route; partial preparatory evidence from [PR #64 enrollment-to-consent return flow](receipts/2026-10-11-rfc002-auth-merge-verification.md) does not satisfy this full row]
- [ ] T-GR-INTEGRATE.2 Verify full composed app and consumer contract fixtures  Owner: luna-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-INTEGRATE.1]  acc: [Real HTTP/MCP app plus PostgreSQL tests complete enrollment, GitHub operator sign-in, Gist OAuth consent, bounded catalog:read access-token issue and refresh, publish/discover/exact-get/resolve/events and restart/replica isolation; expired invitation, wrong audience, member/parent revocation, and scope narrowing deny through current policy; joint fixtures cover connection/funding/expiry/revocation/unknown outcomes using controlled provider fixtures; no real provider spend or unqualified customer data; acceptance subset versus external runtime receipt is explicit]
- [ ] T-GR-INTEGRATE.3 Run full module, race, lint, schema and CI gates  Owner: luna-quality  Est: TBD  kind: agent  stage: verify  verifies: [UC-001, infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-INTEGRATE.1]  acc: [Root and hosted modules pass required GOWORK=off tests/vet/lint, owned formatting, frozen contracts and registry Python checks; concurrency-sensitive suite runs race in exactly one lane with load/lease held; record independent module/CI evidence, full test counts and no silently skipped required environment]
- [ ] T-GR-INTEGRATE.4 Independent integration/security code review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-INTEGRATE.2, T-GR-INTEGRATE.3]  acc: [Independent nonauthor reviewer records base/head SHAs and covers startup/enrollment/key custody/canonical serialization/durable revocation/provider bounds/tenant isolation and existing ADR changes; every accepted finding receives tracked fix, re-verification and final-head re-review]
- [ ] T-GR-INTEGRATE.5 Guarded merge of final composed candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-INTEGRATE.4, T-GR-SCOPE.8]  acc: [Required CI/policy and independent final-head approval verified through gh; approved merge method records immutable landed SHA; latest main conflict resolution is reviewed and no unrelated work is replaced]
- [ ] T-GR-INTEGRATE.6 Re-verify composed behavior on remote landed SHA  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-INTEGRATE.5]  acc: [Remote main contains exact reviewed composition; affected real-store/API/MCP and compatibility acceptance run on landed SHA with version/digest receipts; this closes code delivery only, not release/deploy/live provider or historical Q5 acceptance]

### E-GR-AWS -- AWS origin and infrastructure

fidelity: executable
Acceptance: Reviewed landed AWS inputs and independently verified canonical DNS/TLS/origin/audience migration serve the production target without exposing data or widening operator authority.

Scope: existing AWS registry stack, no second permanent service/account, and the founder-selected canonical origin https://gist.sire.run. Infrastructure source is owned by its existing private owner; these are this delivery's bounded owner tasks/receipts, not permission for a Gist worker to overwrite that repository or run a second lifecycle scheduler. Reconcile any already-enrolled canonical owner jobs before assigning work; consume their combined review/delivery receipts instead of duplicating them. IaC writer uses its own SSD worktree and ownership claim. DNS runs through configured Cloudflare MCP; AWS inspection/build runs through aws CLI; stack changes follow the reviewed Pulumi workflow. Never use the stale original IaC checkout as deployment input.

AWS read-only source inspection at aa62ca046511015d427e067129cec05a374608c2 confirms a fixed registry.sire.run domain/origin in edge/task config and drift checker. Treat account, ARN, service and certificate outputs as private evidence. HTTPS ALB requires a verified certificate; the reviewed ECS service already declares rollback circuit-breaker behavior, which needs qualified rollout proof rather than reinvention.

#### Wave 1: AWS origin and rollout SDLC

- [ ] T-GR-AWS.0 Preflight AWS origin migration and infrastructure ownership  Owner: luna-aws  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.2, T-GR-SCOPE.11]  acc: [Record current remote IaC SHA, exact existing AWS account/region/stack and operator role privately; accepted ADR011 target, DNS/certificate/origin/audience/issuer/redirect migration, no-new-permanent-infrastructure boundaries and full rollback/old-origin cutoff contract; approved credential custody and current operator tools are qualified before mutations]
- [ ] T-GR-AWS.1 Implement reviewed IaC and DNS drift changes for canonical origin  Owner: luna-aws  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.0]  acc: [Own infrastructure candidate sets new certificate/ALB routing, GIST_PUBLIC_ORIGIN and identical GIST_RESOURCE_AUDIENCE to https://gist.sire.run, metadata/callback migration and read-only DNS drift checks; private store/task/database roles retained; two-phase certificate validation and rollout are automation-safe; PR exact base/head and no unexpected resource replacement recorded]
- [ ] T-GR-AWS.2 Verify IaC origin configuration and migration regressions  Owner: luna-aws-verify  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.1]  acc: [Owned mock/property tests and reviewed nonmutating preview prove new TLS/origin/audience config, private storage/networking, old-audience denial, no wildcard/mixed issuer, certificate readiness gates and rollback config; record any real preview auth failure rather than guessing state or bypassing validation]
- [ ] T-GR-AWS.3 Run infrastructure formatting lint and release input checks  Owner: luna-aws-quality  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.1]  acc: [Owned gofmt/vet/lint/tests and Containerfile/CodeBuild/migration validation pass with lease/load rules; new artifact pin references exact source/digest and no secrets; CI or accepted ADR008 local-validation policy is recorded without silently bypassing required protections]
- [ ] T-GR-AWS.4 Independent infrastructure and migration code review  Owner: independent-luna-aws  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.2, T-GR-AWS.3]  acc: [Non-author reviewer records exact repository/base/head and scope, certificate/DNS/operator custody/audience/rollback/data-loss findings; blocking findings get tracked fix, affected tests and independent final-head re-review]
- [ ] T-GR-AWS.5 Merge exact reviewed infrastructure candidate through owner delivery  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.4, T-GR-SCOPE.8]  acc: [Infrastructure owner satisfies own required policy/checks through gh and records reviewed PR/source/landed SHA; if a native owner lifecycle exists consume its canonical landed gate rather than issuing a competing merge]
- [ ] T-GR-AWS.6 Verify landed AWS deployment inputs and owner receipt  Owner: luna-aws-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.5]  acc: [Owner remote main contains exact reviewed configuration and affected IaC/drift/rollout tests pass on landed source; private operator receipts pin stack/source/config and current custody; no apply has been implied by a landed PR]

#### Wave 3: Qualified DNS and TLS cutover workflow

- [ ] T-GR-AWS.7 Prepare new-domain certificate through reviewed AWS stack workflow  stage: deploy  Owner: luna-aws-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.6, T-GR-SCOPE.11, T-GR-RELEASE.2]  acc: [Qualified agent/operator workflow rechecks role and preview-matches-merged-source, requests or reuses the exact new-domain ACM certificate through reviewed certificate-only phase, records validation outputs privately and preserves existing working endpoint; no unreviewed partial apply or ad hoc console mutation]
- [ ] T-GR-AWS.8 Apply exact ACM validation records through Cloudflare MCP  stage: deploy  Owner: luna-dns-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.7]  acc: [Qualified scoped Cloudflare MCP writes only reviewed ACM validation records for the target zone, preserves unrelated records and existing traffic, and records before/after reconciliation; missing or denied tool binding blocks without guessed browser/API fallback or OAuth changes]
- [ ] T-GR-AWS.9 Verify ACM issued and prepared HTTPS listener inputs  Owner: luna-aws-verify  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.8]  acc: [aws CLI confirms matching certificate issued in admitted region/account and reviewed HTTPS listener/task configuration is ready for final rollout; complete new origin/audience/issuer/redirect config and canonical metadata are pinned; no live credential requests routed to mixed old config]
- [ ] T-GR-AWS.10 Activate canonical DNS after new-origin AWS rollout  stage: deploy  Owner: luna-dns-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.9, T-GR-RELEASE.4]  acc: [Cloudflare MCP activates exactly reviewed gist.sire.run routing to stack outputs after matching service/config rollout, with recorded proxy/TLS behavior and drift reconciliation; all unrelated records are preserved and rollback remains bounded]
- [ ] T-GR-AWS.11 Verify external canonical DNS TLS and OAuth resource boundary  Owner: luna-domain-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.10]  acc: [From an external client gist.sire.run resolves, certificate matches, HTTPS health/readiness and canonical URLs work, HTTP redirects safely, OAuth metadata and MCP advertise the exact new resource; old-audience tokens and unregistered callbacks deny; actual positive fresh consent/token journey reaches the new origin]
- [ ] T-GR-AWS.12 Complete bounded legacy-origin handoff and rollback qualification  Owner: luna-aws-operator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-AWS.11]  acc: [Recorded old-origin cutoff/credential-free redirect or denial policy is enforced without accepting old tokens at the new resource; complete-config rollback procedure is qualified in an isolated admitted environment, existing production data preserved, and no permanent unintended dual-origin credential path remains]

### E-GR-RELEASE -- Release acceptance and operations

fidelity: executable
Acceptance: Immutable release, authorized rollout, actual authenticated consumer/provider journey, independent acceptance, bounded operations handoff and truthful landed milestone evidence are separate outcomes.

Production is explicitly in scope at https://gist.sire.run on AWS. Release, DNS and deployment are first-class agent deliverables executed through qualified operator workflows and scoped grants; they are not intentionally human-only stopping points. Ordinary parser stage vocabulary has no deploy/release token, so operational rows use kind: agent with lane: agent and named workflow acceptance, while coding candidates retain all six stage markers. The executing coordinator must route these operations to their qualified AWS/Cloudflare/IaC bindings, never Kazi code convergence or a guessed executor. A genuinely unavailable binding/authority blocks that action with concrete evidence. /plan itself performs none of them.

Before these rows run, revalidate the exact environment, operator workflow, credentials, granted numerical envelope and current policy. If deployment requires code/IaC/migration changes beyond the reviewed candidate, create its own six-stage candidate chain and make the rollout depend on its landed receipt. No code change can hide inside a release row.

#### Wave 3: Release, rollout and live qualification

- [ ] T-GR-RELEASE.0 Preflight AWS production release and operational handoff design  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-INTEGRATE.6]  acc: [Prepared runbook pins https://gist.sire.run and AWS account/region/stack privately plus source/image/config/contract/migration hashes, operator/environment authority, rollout health/security checks, backup/restore and forward recovery, monitoring/redaction, credential/key rotation, invocation limits and bounded observation; no new infrastructure or provider action outside current scope]
- [ ] T-GR-RELEASE.1 Independent release and recovery runbook review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.0]  acc: [Independent operations/security reviewer confirms migratory data compatibility and failure/restore/rotation/unknown-outcome procedures, exact artifact/authority and live checks; approved runbook findings are resolved; destructive production restore is not silently included]
- [ ] T-GR-RELEASE.2 Confirm qualified release/deployment operator authorization  Owner: luna-production-operator  Est: TBD  kind: agent stage: preflight  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.1, T-GR-SCOPE.5, T-GR-SCOPE.8, T-GR-SCOPE.11]  acc: [Founder-selected AWS target and exact requested deployment scope and release workflow match actual standing or new authorization, required checks and credentials; no unapproved paid runner/account/service/permission change; qualified agent/operator receipt recorded; re-present no routine approval already granted by the session or accepted standing stack policy]
- [ ] T-GR-RELEASE.3 Produce immutable release artifact through qualified workflow  stage: release  Owner: luna-release-operator  Est: TBD  kind: agent lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.2]  acc: [Existing owner-reviewed AWS CodeBuild/ECR release workflow through aws CLI builds/publishes the approved landed SHA with immutable image digest, provenance/check receipts and config contract; no retagging or substituting another commit; artifact-release success is distinct from deployment]
- [ ] T-GR-RELEASE.4 Deploy and migrate the approved bounded environment  stage: deploy  Owner: luna-production-operator  Est: TBD  kind: agent lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.3, T-GR-AWS.9]  acc: [Qualified reviewed existing AWS stack workflow applies pinned artifact/config/migrations within exact authority; https://gist.sire.run origin/resource config, exact running ECS/ECR digest, task count and ALB target health/readiness and no-public-object/secret checks pass; source/image/environment receipts and failure/forward-recovery outcome recorded; no manual DB seed or infrastructure bypass]
- [ ] T-GR-RELEASE.5 Verify authenticated deployed registry journey and security denials  Owner: luna-live-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-002, UC-003, UC-004, UC-005, UC-007, UC-008, UC-009]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.4, T-GR-AWS.11]  acc: [Actual deployed https://gist.sire.run enrollment/human OAuth and sanctioned workload token journey publishes/discovers/fetches/resolves exact artifacts and consumes events; wrong tenant/issuer/audience, expiry/revocation, retention gap and restart/replica behavior deny correctly; run live-tag acceptance with named builds and record observed statuses without fixtures-as-production]
- [ ] T-GR-RELEASE.6 Verify governed caller execution with a real pinned consumer  Owner: joint-registry-consumer  Est: TBD  kind: agent  stage: verify  verifies: [UC-008, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.5, T-GR-SCOPE.7, T-GR-SCOPE.5]  acc: [Named admitted runtime imports exact closure and performs only approved bounded Treg/Composio actions; action/account/recipient/revision/budget checks precede send; provider call IDs, actual cost and attempt outcomes persist; unknown stays unknown without duplicate replay; absent numerical or custody authority leaves this row blocked]
- [ ] T-GR-RELEASE.7 Independent live acceptance and operational readiness review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.5, T-GR-RELEASE.6]  acc: [Independent reviewer checks authentic environment/build/receipt provenance, separate provider pass/fail status, tenant and secret redaction, negative coverage, restore/rotation and bounded observation readiness; local/CI/live/provider/runtime evidence are separately labeled and no unexplained skipped gate passes]
- [ ] T-GR-RELEASE.8 Record technical production acceptance and retain explicit residual risks  Owner: coordinator  Est: TBD  kind: agent stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.7]  acc: [Coordinator records pass/fail against actual agreed technical acceptance using the delivered journey evidence; remaining registry-client/production/provider limitations and caller revocation race are explicit; no invented founder-pilot sign-off or business publication; separate genuine human decisions remain separately scoped]
- [ ] T-GR-RELEASE.9 Hand off ownership and complete bounded post-rollout observation  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.8, T-GR-AWS.12]  acc: [Named operator receives pinned runbook, service/key/feed/credential ownership and incident path; approved bounded health/error/revocation observations at recorded sample points show no new regression; no indefinite watcher/scheduler; maintenance issues become tracked scoped tasks]

#### Wave 4: Evidence delivery SDLC

- [ ] T-GR-EVIDENCE.0 Preflight truthful milestone closure and evidence ownership  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-RELEASE.9]  acc: [Map all original Q5/R3/X-R1 and required named-client obligations to actual receipts; resolve case-sensitive M2a evidence filename from scripts/registry/check.py; qualify public redaction and list any original requirements not met; unmet Q5 scope remains open]
- [ ] T-GR-EVIDENCE.1 Author actual milestone evidence and retrospective handoff candidate  Owner: luna-evidence  Est: TBD  kind: agent  stage: implement lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-EVIDENCE.0]  acc: [Create docs/registry/gates/M2a.json only when every original gate requirement is actually passed; preserve historical task IDs/status and annotate newer receipts; sanitize private run details; update design/ADRs for accepted choices and devlog for events; PR/head handoff includes release/ops evidence and explicit unqualified scope]
- [ ] T-GR-EVIDENCE.2 Validate evidence hashes, commands, counts and historical coverage  Owner: luna-evidence-check  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-EVIDENCE.1]  acc: [Registry M2a evidence checker and applicable contract/receipt checks pass on candidate; every recorded hash/build/command/test count and named-runtime receipt matches observed run; redaction/link/status checks pass; no gate fabricated to clear a checkbox]
- [ ] T-GR-EVIDENCE.3 Independent evidence and code review of milestone candidate  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-EVIDENCE.2]  acc: [Non-author reviewer records exact base/head, evidence provenance and Q5 coverage with findings/dispositions; any code/script/schema edit gets full applicable behavior/lint checks and review; missing original requirement blocks milestone closure]
- [ ] T-GR-EVIDENCE.4 Merge exact reviewed evidence candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-EVIDENCE.3, T-GR-SCOPE.8]  acc: [Required final-head review/policy/checks satisfied and gh records evidence PR/landed SHA using approved method; no stale artifact hashes after main moves]
- [ ] T-GR-EVIDENCE.5 Verify landed evidence and final recovery acceptance  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-EVIDENCE.4]  acc: [Remote landed SHA passes truthful M2a evidence/freeze/receipt checks with all original Q5 requirements accounted for; residual M2b/M3/gateway work remains open; report final code/release/deploy/live/pilot states and close this M2a recovery submilestone only if its obligations passed; continue automatically to NEXT/M2b/M3/final AWS production rather than treating this row as the delivery finish]

### E-GR-NEXT -- Remaining registry milestones

fidelity: executable
Acceptance: Next reachable registry milestone has an evidence-reconciled full SDLC plan without invented provider or client readiness.

Preserve the remaining original M2b/M3 tasks and IDs in registry-buildout.md. Expand preview teardown, public OAuth/connector clients, both runtime archetypes and retrieval evaluation from actual recovery evidence before dispatching the known remaining production qualification lanes. Do not duplicate already qualified behavior or imply discovery savings without measurements.

#### Wave 5: Dependency-triggered next planning pass

- [ ] T-GR-NEXT.0 PLAN: refresh and reconcile remaining M2b and M3 production delivery from receipts  stage: author  Owner: coordinator  Est: TBD  kind: plan  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-EVIDENCE.5]  acc: [Reconcile original R4/R5/Q6-Q10/O2-O4/external receipts and accepted waivers; Reconcile the explicit M2b/M3 stage rows against current code and activate only dependency-ready accepted scope; newly discovered epics use exactly one triggered planning task until expandable; include distinct requirements/design/preflight/implementation/tests/lint/independent review/merge/landed/release/live/ops tasks and GPT-6-Luna ownership; preserve historical IDs and evidence]

This stable planning row is a continuation checkpoint. Completing it never satisfies production. The coordinator revalidates the known M2b/M3 scope, adds tracked fix or outline-expansion tasks as evidence requires, and continues execution without another generic “proceed?” question. Material new scope, missing numerical authority or rejected binding is a concrete blocker; drafting alone never admits a new lifecycle.

### E-GR-M2B -- OAuth preview and production

fidelity: executable
Acceptance: Actual named connector/OAuth behavior runs at https://gist.sire.run on AWS with qualified tenant isolation and complete preview teardown; original Q8 obligations are met.

Known remaining RFC-002 acceptance is in scope, with original R4/O2/O3/O4/Q6/Q7/Q8 IDs preserved in the historical plan. The user explicitly asks a complete production SDLC; these stage obligations are decomposed now, and preflight rechecks assumptions from real M2a receipts. No extra product design is invented. Any newly discovered implementation gap gets its own scoped six-stage correction chain. Shared acceptance/IaC ownership is allocated by coordinator; infrastructure owner retains its canonical jobs.

#### Wave 5: M2b qualification code SDLC

- [ ] T-GR-M2B.0 Preflight OAuth connector preview and production acceptance  Owner: luna-oauth-acceptance  Est: TBD  kind: agent  stage: preflight  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-CATALOG-HOLD.0, T-GR-NEXT.0]  acc: [Reconcile actual original I4-I6/R4/O2-O4/Q6-Q8/W1-W5 evidence, current named connector builds and AWS preview workflow/cost authority; freeze complete OAuth and browser/client denial matrix, preview isolation/max lifetime, finally-teardown and canonical new-origin promotion contract; missing provider/identity permission blocks only its affected action]
- [ ] T-GR-M2B.1 Implement only remaining OAuth connector and preview qualification gaps  Owner: luna-oauth-acceptance  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.0]  acc: [Owned actual browser/HTTP/MCP connector tests and isolated AWS preview workflow close proven missing coverage without synthetic build labels; teardown always executes on success/failure/cancellation; required service/IaC changes have explicit author scopes and repository/head handoffs rather than hidden deployment edits]
- [ ] T-GR-M2B.2 Verify OAuth consent token tenant and failure behavior  Owner: luna-oauth-verify  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.1]  acc: [Meaningful real HTTP status/body and browser golden/denial tests cover registration, PKCE S256, consent, resource/audience, refresh rotation/reuse/revoke, session/workspace switching, tenant caches, reconnect/limits and uncertainty; required absent config is not counted as a passing skip]
- [ ] T-GR-M2B.3 Run changed-package quality and preview workflow checks  Owner: luna-oauth-quality  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.1]  acc: [Owned format/lint/vet/tests and applicable freeze/schema/workflow checks pass at exact candidate; both affected repository candidates receive their required checks under build limits; preview cleanup is tested with failed/cancelled attempts]
- [ ] T-GR-M2B.4 Independent OAuth and preview security code review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.2, T-GR-M2B.3]  acc: [Non-author reviewer records exact base/head for every included coding/IaC candidate, credential/tenant/callback/teardown findings and dispositions; blockers require tracked fixes, affected verification and re-review before delivery]
- [ ] T-GR-M2B.5 Merge exact reviewed qualification candidates through owners  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.4, T-GR-SCOPE.8]  acc: [All included candidate owners satisfy actual check/policy and independent approval at final heads; gh or canonical enrolled owner lifecycle records landed receipts; no competing scheduler or main overwrite]
- [ ] T-GR-M2B.6 Verify landed OAuth qualification and workflow code  Owner: luna-oauth-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.5]  acc: [Affected behavior and cleanup tests pass on actual remote landed source(s); pin service and acceptance build identities separately; no deployment completion inferred]

#### Wave 6: AWS preview and M2b production workflow

- [ ] T-GR-M2B.7 Prepare immutable preview and production release inputs  stage: release  Owner: luna-release-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.6, T-GR-SCOPE.5, T-GR-SCOPE.11]  acc: [Qualified AWS workflow builds or reuses exact immutable service digest and matching acceptance revision; reviewed preview account/region/stack/expiry/backup/cost scope is recorded, isolated synthetic workspace and private test secrets prepared, always-run cleanup ownership registered before provisioning]
- [ ] T-GR-M2B.8 Provision and execute the bounded isolated preview workflow  stage: deploy  Owner: luna-preview-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.7]  acc: [Reviewed AWS preview workflow provisions its separate origin/audience/database/object namespace/credentials within approved scope, runs configured acceptance attempt and captures terminal attempt receipt, invokes finally teardown on success/failure/cancel before returning; neither failed test nor cleanup is mislabeled passing]
- [ ] T-GR-M2B.9 Verify actual connector product and OAuth preview acceptance  Owner: luna-connector-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.8]  acc: [Actual qualified connector/native browser products with named builds complete registration/consent/exact retrieval and denial matrix against that preview; operator/cloud provider fixtures do not replace required client evidence; failures create tracked repair/retest work, not a production promotion]
- [ ] T-GR-M2B.10 Verify preview teardown and expiry cleanup independently  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.8]  acc: [Finally hook receipts plus aws/approved DNS checks prove temporary stack/resources/client grants/secrets/DNS removed or revoked after every terminal attempt; no production data altered and no cleanup error hidden; workflow failure leaves M2b/promotion open until cleanup proof and repairs pass]
- [ ] T-GR-M2B.11 Promote reviewed immutable OAuth service revision to AWS production  stage: deploy  Owner: luna-production-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.9, T-GR-M2B.10, T-GR-SCOPE.11]  acc: [Existing authorized registry-aws workflow applies only qualified landed source/digest/config at https://gist.sire.run; reuse already deployed identical service revision when changes are acceptance-only and prove identity; newer service changes require actual rollout and fresh affected acceptance; no manual data or cloud policy bypass]
- [ ] T-GR-M2B.12 Verify production OAuth connectors and private-catalog isolation  Owner: luna-live-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-007, UC-009]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.11]  acc: [Live HTTPS/MCP canonical new-origin OAuth and named connector journey pass all original Q8 obligations; scope/membership/resource/refresh/revocation/caches/outage denials and old-origin policy verified; no preview-only evidence substituted]
- [ ] T-GR-M2B.13 Independent M2b production acceptance review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.12, T-GR-M2B.10]  acc: [Independent reviewer checks source/image/config/client/environment receipts, original Q8/W gate coverage, actual production result and zero leaked preview resources; findings receive tracked correction, re-verification/re-review]
- [ ] T-GR-M2B.14 Record truthful M2b production gate and continue  Owner: luna-evidence  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.13]  acc: [Prepare checker-compatible actual M2b evidence with exact receipt/hashes/test counts and preserved original IDs; Q8 closes only on production plus cleanup proof; handoff to M3 automatically, not final completion; final dossier chain reviews and lands these artifacts]

### E-GR-M3 -- Clients runtime and production metrics

fidelity: executable
Acceptance: Required named clients, both runtime archetypes and frozen measured retrieval/interaction gates are qualified at the latest running AWS service on https://gist.sire.run.

Known original R5/E3/Q9/Q10 obligations remain required: named coding/connector clients, both runtime archetypes, and measured retrieval/interaction behavior. The fixture-only R3 history remains visible. Parallelize client/runtime/retrieval verification workers within pool capacity and one protected approved funding envelope. Use exact frozen labels/thresholds rather than changing them to pass.

#### Wave 7: M3 qualification SDLC

- [ ] T-GR-M3.0 Preflight required named clients both runtime archetypes and retrieval gate  Owner: luna-client-matrix  Est: TBD  kind: agent  stage: preflight  verifies: [UC-001, UC-003, UC-004, UC-005, UC-007, UC-009, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M2B.14]  acc: [Reconcile original R5/E3/Q9/Q10 and X-R2/Z receipts; pin actual required client/runtime builds, service/config and retrieval corpus/labels/thresholds; reuse qualified consumer imports and explicitly externally driven mode if accepted; no silent client substitution, invented model budget or fake local-build identity]
- [ ] T-GR-M3.1 Implement remaining client runtime and measured-retrieval acceptance gaps  Owner: luna-client-matrix  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [UC-003, UC-004, UC-005, UC-007, UC-009, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.0]  acc: [Owned harnesses exercise exact named client products and importing/policy-gated runtime paths with URI/digest/grant/revocation contracts; measurement runner emits reproducible search quality and complete task interaction counts; service fixes identified here receive scoped six-stage delivery instead of being hidden in test setup]
- [ ] T-GR-M3.2 Verify changed client and retrieval behavior with meaningful fixtures  Owner: luna-client-verify  Est: TBD  kind: agent  stage: verify  verifies: [UC-003, UC-004, UC-005, UC-007, UC-009, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.1]  acc: [Real API/MCP harness regression tests verify full package closure, honest resolution, connection/error/expiry/revocation, authorized optional taxonomy filters with attribution, and preserved local library/CLI behavior; captured scenario data is synthetic and fixture versus live evidence remains explicit]
- [ ] T-GR-M3.3 Run candidate quality checks and frozen metric-contract validation  Owner: luna-quality  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.1]  acc: [Changed package format/lint/vet/tests plus root compatibility/freeze and retrieval checker pass at exact candidate; frozen corpus/threshold changes require recorded rationale and independently reviewed revision, never silent pass-by-retuning]
- [ ] T-GR-M3.4 Independent client runtime and evaluation code review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.2, T-GR-M3.3]  acc: [Non-author reviewer records exact base/head and named scope with interoperability/security/measurement findings, accepted dispositions and tracked fix/retest/re-review for blockers]
- [ ] T-GR-M3.5 Merge exact reviewed M3 qualification candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.4, T-GR-SCOPE.8]  acc: [Actual required policy/checks and independent final-head approval verified through gh; approved method records PR/landed SHA; scope-dependent evidence refreshed after relevant base changes]
- [ ] T-GR-M3.6 Verify landed M3 harness and affected service behavior  Owner: luna-client-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.5]  acc: [Remote landed source contains the reviewed harness/service changes; affected checks pass on that source and source/config/corpus receipts are immutable; absent live product/client remains unresolved]

#### Wave 8: Latest-source production and parallel qualification

- [ ] T-GR-M3.7 Refresh exact production artifact when service changes require rollout  stage: deploy  Owner: luna-release-operator  Est: TBD  kind: agent  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.6, T-GR-SCOPE.11]  acc: [Compare deployed service inputs with landed candidate: build/pin and roll out new immutable AWS service digest for service changes through authorized workflow, or prove existing running digest matches unchanged service inputs for harness-only changes; preserve canonical origin and capture running task/ALB status]
- [ ] T-GR-M3.8 Verify required named coding and connector products in production  Owner: luna-client-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-005, UC-009, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.7]  acc: [Every required original client product with actual pinned version passes https://gist.sire.run setup/metadata/auth/exact retrieval/resolve/expiry/error/revocation matrix; explicit bridge differs from native support; required unsupported client keeps M3 open]
- [ ] T-GR-M3.9 Verify both real runtime archetypes and caller-owned provider receipts  Owner: joint-registry-consumer  Est: TBD  kind: agent  stage: verify  verifies: [UC-004, UC-005, UC-008, UC-010]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.7, T-GR-SCOPE.7, T-GR-SCOPE.5]  acc: [Actual importing and policy-gated runtime builds obtain complete pinned closure and preserve native URI/grant/parent/connection/revocation lifecycle; authorized Treg/Composio actions have bounded actual provider/cost/unknown-outcome persistence receipts; exact fields/accounts/revision checked before sending; no generic gateway required]
- [ ] T-GR-M3.10 Run measured production retrieval and complete-task interaction smoke  Owner: luna-retrieval-acceptance  Est: TBD  kind: agent  stage: verify  verifies: [UC-003, UC-005, UC-007]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.7, T-GR-SCOPE.5]  acc: [Freeze-qualified production corpus/query/threshold run measures actual recall/quality/tenant exclusion, latency and complete task discovery/get/resolve/turn counts with real source/config/date; no measured-saving or BM25 claim without evidence; privacy/cost envelope and failure-to-no-match semantics preserved]
- [ ] T-GR-M3.11 Independent full-registry production acceptance review  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.8, T-GR-M3.9, T-GR-M3.10]  acc: [Independent reviewer checks actual client/runtime/provider/evaluation receipts, original R5/E3/Q9/Q10 coverage and newest deployed source/config; all accepted defects become tracked SDLC corrections with affected fresh production checks before closure]
- [ ] T-GR-M3.12 Record final M3 gate and latest production handoff  Owner: luna-evidence  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-M3.11]  acc: [Actual M3 evidence reflects all original Q10 obligations at the running canonical AWS origin; stage provenance, quality thresholds and test counts match receipts; prepare artifacts for independent final dossier delivery and continue to terminal production checks]

Current consumer boundary: [ADR016](adr/016-zati-library-first-rfc003-gateway-boundary.md) makes Zati the flagship Gist library consumer. These generic hosted native-client/runtime qualification obligations remain separate Gist production gates and must name an accepted consumer. Previously completed fixture tests retain their source-only evidence; they do not demonstrate live Zati library integration. The Zati-specific consumer graph has not yet been added pending the exact real skill/capability pair and owner-owned adapter files.

### E-GR-PROD -- Terminal production delivery

fidelity: executable
Acceptance: The confirmed RFC-002 registry/caller scope is actually running and independently qualified at https://gist.sire.run on AWS; this is the hosted-service terminal. Whole-project Gist completion also requires the accepted library-first Zati consumer path. RFC-003 gateway production is conditional and is not part of this terminal unless a separate client activates it.

This is the RFC-002 hosted-service delivery terminal. Earlier landed, release, recovery, NEXT planning or health-only results cannot complete the gate. The coordinator keeps scheduling eligible work, including exact-head fixes and new immutable releases for affected service changes, until this terminal is proved or a concrete authority/capability/decision blocker is reported. Human feedback is retained when genuinely required; arbitrary periodic confirmation does not replace autonomous authorized work.

#### Wave 9: Final operations and evidence delivery

- [ ] T-GR-PROD.0 Preflight complete-production evidence accounting  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-EVIDENCE.5, T-GR-M2B.14, T-GR-M3.12, T-GR-AWS.12, T-GR-RELEASE.9, T-GR-SCOPE.4]  acc: [Account for all accepted registry M1-M3/original Q5/Q8/Q10 obligations and new domain/AWS/provider/ops gates; identify latest running app and IaC source versus acceptance/docs source; no incomplete required row or newer unqualified service candidate is hidden; RFC-003 gateway requirements remain a separate production gate and cannot be counted as registry acceptance; unrelated products remain outside this scope]
- [ ] T-GR-PROD.1 Verify monitoring recovery security and bounded production soak  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.0]  acc: [At recorded bounded sample points canonical HTTPS app remains healthy and actual AWS tasks/ALB/DB/private objects/secret/key/event configuration match approved running inputs; named alarms/log redaction/operator ownership and backup/restore/key rotation/restart procedures are qualified in approved isolated scope; thresholds/observation duration recorded before run, no indefinite watcher or destructive customer restore]
- [ ] T-GR-PROD.2 Track and close all blocking production findings through SDLC  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.1]  acc: [Every accepted failed production/review/soak check has stable owned fix, affected behavior/quality checks, independent exact-head re-review, guarded merge/landed receipt and new release/rollout/live checks if service inputs changed; root completion remains false until no unresolved required finding or unqualified deployment exists]
- [ ] T-GR-PROD.3 Author complete canonical production dossier candidate  Owner: luna-evidence  Est: TBD  kind: agent  stage: implement  lane: agent  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.2]  acc: [Owned sanitized docs/gates include truthful M2a/M2b/M3/domain/AWS/provider/ops receipts, full source/digest/config/build/check counts and residual accepted constraints; existing historical IDs/evidence preserved; public hostname is https://gist.sire.run; no secret/account/customer raw data; PR exact base/head handed off]
- [ ] T-GR-PROD.4 Verify final dossier links hashes milestone schemas and closure criteria  Owner: luna-evidence-check  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.3]  acc: [Installed registry contract/evidence/eval/receipt checks and case-correct original M2a/M2b/M3 gate requirements pass on actual candidate; source hashes and running AWS/URL observations agree, original and new tasks are covered, no missing environment/client outcome recast as pass]
- [ ] T-GR-PROD.5 Independent final production evidence code and domain review  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.4]  acc: [Independent non-author reviewer records exact base/head and authentic current production evidence with findings/dispositions; every code/config/script edit has required changed-behavior quality evidence; no terminal acceptance based only on design/fixture/merge/health]
- [ ] T-GR-PROD.6 Merge exact reviewed production dossier  Owner: coordinator  Est: TBD  kind: agent  stage: merge  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.5, T-GR-SCOPE.8]  acc: [Required exact-head approval/policy/checks verified through gh, guarded owner merge records landed SHA; evidence-only commit does not force an unnecessary app rebuild, and source/provenance distinction remains correct]
- [ ] T-GR-PROD.7 Verify landed dossier and latest running service alignment  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.6]  acc: [Remote main contains accepted dossier and actual gate checks pass on landed evidence source; latest accepted service/IaC digest is running on AWS, and canonical HTTPS/authenticated smoke agrees after delivery; relevant intervening code/config changes invalidate stale receipts and create follow-on work]
- [ ] T-GR-PROD.8 Independently verify complete production predicate  Owner: independent-luna-ops  Est: TBD  kind: agent  stage: review  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-PROD.7]  acc: [Independent final checker proves all required task/gate receipts, canonical DNS/TLS/origin/OAuth metadata, running AWS service/digest/health, authenticated catalog and real caller/provider behavior, full M2a/M2b/M3 acceptance, no preview leaks, qualified ops/recovery and no unresolved blocking findings; no public registry.sire.run alias is accepted as the requested target]
- [ ] T-GR-PROD.9 Complete delivery only while gist.sire.run is running qualified AWS production  Owner: coordinator  Est: TBD  kind: agent  stage: verify  verifies: [infrastructure]  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.23, T-GR-PROD.8]  acc: [All accepted required registry/caller tasks and final independent predicate pass with current timestamps and immutable source/deploy/evidence receipts; https://gist.sire.run serves the qualified authenticated service on AWS now, Treg/Composio caller journeys and operations are evidenced; coordinator reports the actual URL and states with no required work remaining, otherwise keeps plan open and progresses/records concrete blockers]

The preserved [optional gateway planning record](plans/E-GR-GATEWAY-optional-managed-gateway.md) and [original gateway plan](plans/gateway-buildout.md) are archived historical input; ADR014 brings RFC-003 into Gist scope. PR #47's public head and preserved dirty worktree remain distinct and must retain their evidence. The founder approved the revised M-G1 direction on 2026-10-11; exact RFC/ADR/threat-model reconciliation and independent review feed the G0.4 record and full integrated plan. The specific credential-intake mode remains an owner decision and blocks only secret-ingress implementation; it does not block unrelated RFC-002 work or non-secret gateway contract/guidance design.

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
| RFC-002 registry terminal | PROD.9 | Requested AWS origin currently running the fully qualified registry/caller scope with independent final evidence |
| RFC-003 design admission | GATEWAY.4 to GATEWAY.9 | Reviewed and owner-approved design, followed by a conformance-checked full SDLC task graph |
| Whole-project delivery terminal | Hosted registry PROD.9 plus the new Zati library consumer terminal | Independently qualified hosted registry and real Zati library consumer outcomes; RFC-003 remains conditional under ADR016 |

## Required validation and evidence

Implementation verification names actual package tests, including real HTTP status/body assertions for changed API routes, real PostgreSQL transactions/restart/replica behavior, OAuth browser golden/denial cases, exact artifact/digest/version fixtures and one meaningful regression per defect. Use GOWORK=off for Go. Integration runs both module tests/vet, changed-package golangci-lint, owned gofmt/goimports, Python registry unit tests and `python3 scripts/registry/check.py contracts --freeze-check`; one lease-qualified race lane covers concurrency changes. Live rows use the existing live-tag workload/wiring/runtime suites and actually pinned client/runtime builds. Required environment absence is not success or a skip waiver.

Each receipt records task, owner, full base/head/landed/source/image/config/contract digests, environment/client/runtime identifiers, commands/test counts, independent reviewer, findings/dispositions and evidence location. Private credentials, account/host/user/customer identifiers and raw production traces stay in access-controlled records. Public channel summaries are sanitized and ignored in this public repository. Source maps: app/identity/oauth/storage/events/resolution/rest/remotemcp, catalog/registry, contracts/registry/v1, scripts/registry and registry CI workflows.

Capability selection was run in planning: baseline/delivery/go profiles; CLI go/gh/aws/golangci-lint/kazi/composio available. This reports installed binaries, not auth, live permissions or runtime interoperability. Use gh for GitHub, aws CLI for AWS and configured Cloudflare MCP only if an in-scope CF need arises. No fresh code-graph tool binding was available; direct source reads are the qualified fallback. Profile suggestions do not activate plugins. The generic parser supports the declared preflight/implement/verify/review/merge/verify-landed markers; qualified agent-owned operational release/deployment rows deliberately use no unsupported stage: deploy/release token; the coordinator routes each named workflow to actual authorized AWS/Cloudflare bindings, not a generic code executor. No native controller JSON, approval, enrollment or runtime grant is produced.

Stable architecture remains in docs/design.md and RFC/ADRs. Accepted design changes amend their existing owner records during reviewed delivery; events and debugging go to docs/devlog.md. This plan links the earlier proposed integration packet and provider assessment for technical detail rather than duplicating their complete fixtures. Future stage execution must revalidate any stale permission, auth, CI, remote-main or deployment claim.

## Open decisions and risks

1. RFC-002 enrollment mechanism is approved: permissions-free GitHub App sign-in verifies a stable numeric account ID, invited access is limited to one private workspace, GitHub tokens are not stored, and Gist issues bounded workload grants. The concrete workspace and operator account are pending. Treg/Composio are separate provider connections. For RFC-003, KMS envelope encryption is selected but stored-only versus one-shot caller-supplied credential intake remains unresolved; do not build a credential-ingress path until the owner records it. The shared signing-key source and any human/workload credential custodian still need qualification.
2. M-G1 names GitHub REST and a controlled MCP fixture, but no live GitHub account/repository, exact operations, permissions, call cap, or expiry is approved; local HTTP/MCP fixtures can qualify contract behavior, while live GitHub calls remain blocked. Exact Treg endpoint/target and Composio action/account/version/license/price plus numerical call and dollar envelopes are also unqualified; live provider calls stay blocked separately.
3. Consumer readiness, existing required CI availability/policy and current deployment/tool binding: production scope is now explicit and applicable standing grants must be reused; local composition does not qualify those boundaries. Do not change OAuth restrictions or required checks to gain progress.
4. Caller revocation has a check-to-send race; unknown outcomes and already accepted effects remain honest. Memory, business hierarchy and friend-approved marketing publication are not registry authority.

## Planning handoff

Original planning handoff (superseded by authorized October4/5 ship execution): this was a draft refinement only. All new checkboxes remain open, historical checkboxes and authored evidence are retained, and no fixes/builds/tests/reviews/merges/releases/deployments/worker starts have been performed by /plan. The executing coordinator reads this file, revalidates SCOPE.1/.6/.8/.11 and keeps scheduling/refining dependency-ready in-scope stages until T-GR-PROD.9 is true; no routine proceed-confirmation at local merge, rollout or NEXT planning checkpoint. External missing choices remain coordinator-routed. Planning validation is parser/graph/coverage and artifact consistency, not application acceptance.

Planning validation (2026-10-04): installed parser accepted 77 open tasks in 11 epics with all six supported code-stage markers; every task has owner, acceptance and wave assignment. Additional graph checks found no missing dependency, cycle, duplicate local ID, malformed acceptance, unguarded coding merge or TOC count mismatch. Exactly two outline epics have one trigger planning task each. Hash checks confirm preexisting plan/design contents remain byte-identical prefixes and the gateway plan is untouched. Whitespace checks passed; no application checks were run. Parser IDs namespace the preserved local IDs for display; they are not minted lifecycle or claim IDs.

## Production completion and continuation contract — 2026-10-04 refinement

At the 2026-10-04 checkpoint, the founder explicitly confirmed registry plus caller-owned integrations and selected https://gist.sire.run on AWS. `registry_production_done` is true only when T-GR-PROD.9 has fresh independent evidence of all required registry/caller milestones, canonical external DNS/TLS/health/origin/OAuth boundary, expected live AWS service/task/image/config/private storage, authenticated publish/discover/exact-get/resolve/events, real bounded Treg/Composio caller/runtime receipts, all required original M2a/M2b/M3 clients/metrics, complete preview cleanup and qualified monitoring/recovery/ownership. A local pass, PR, merge, initial rollout, website200, drafted plan or unsupported client is insufficient. Under ADR016, whole-project delivery additionally requires the new library-first Zati consumer terminal; gateway execution and hosted-guide endpoints are not required absent a separate accepted client trigger.

The executing coordinator persists task outcomes and receipt dependencies after every lane boundary. Within already accepted scope it refills Luna slots, invokes dependency-triggered planning refinements, assigns accepted findings to full fix/verification/review/merge/landed chains, rebuilds and redeploys changed service inputs, reruns affected live acceptance and continues. It does not stop just because one epic is done or because ordinary Markdown lacks a deploy stage token. Real denied tools/credentials, absent operator/provider budget, rejected auto-approval or material scope changes are recorded concrete blockers; they are not silently bypassed. No worker can accept its own code or production findings.

Known AWS, M2b/M3 and final production obligations are now decomposed to fulfill the user's explicit complete-SDLC request; every preflight must revalidate actual immutable inputs before dispatch. User/domain decisions, cloud inventory and numerical limits are not invented. The earlier77-task validation is historical evidence of that earlier revision only; validate this new graph and counts before handing it off.

Official deployment references used for this planning refinement: ALB HTTPS requires a matching certificate ([AWS listener documentation](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/create-https-listener.html)); ECS rolling deployment supports failure detection/rollback and image-digest consistency ([AWS deployment documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-ecs.html)). These support the task design, not an assertion that the deployed service already meets it.

Tooling observation for this refinement: aws, pulumi and gh binaries are available; no Cloudflare MCP function was exposed in the current tool inventory. T-GR-SCOPE.11 must qualify the configured binding before DNS writes. This is a real execution-capability prerequisite, not permission to use a generic connector/browser or change organization OAuth restrictions. No AWS authentication, stack apply or DNS mutation was attempted here.

Historical planning-only validation (2026-10-04, before shipping started): 128 open active tasks in 14 epics; default docs/plan.md parses those tasks once alongside 18 unchanged historical completed tasks (146 total). All 127 other active tasks are ancestors of terminal T-GR-PROD.9. Unique IDs, resolved acyclic dependencies, owner/acceptance/wave coverage, all six supported code stages, independent-review dependencies, guarded merges, split TOC counts and whitespace checks passed. All 77 prior recovery IDs/statuses are retained: 76 remain active and the unchanged optional gateway trigger is preserved in its deferred file. Prior plan/design/devlog contents remain byte-identical prefixes; ADR008 and the original gateway plan are unchanged. These are planning-artifact checks only; no application test or execution was run.

## Shipping checkpoint — 2026-10-04

User invoked /ship. T-GR-SCOPE.1/.2/.6/.8 have source-only receipts in docs/receipts; component contracts await independent T-GR-SCOPE.3 disposition before engineering admission. Four slots are occupied by one coordinator and three explicitly requested GPT-6-Luna workers, with isolated external-SSD ownership. Engineering builds are held while one-minute host load exceeds 10; no build lease is bypassed. Production bindings are partially qualified but blocked by AWS session/stack-account mismatch and absent configured Cloudflare MCP. Enrollment and provider targets/caps are pending a decision brief requested by the founder. No production_done, code/lifecycle admission, CI pass, release or live acceptance is asserted. Read-only preflights are not application acceptance.

Historical shipping validation: 128 active registry production tasks at the prior checkpoint; this count excludes the reopened RFC-003 gateway epic. Founder identity/pilot choices remain pending. Consumer owner reports its actual runtime/header/funding qualifications are not yet admitted; T-GR-SCOPE.7 stays blocked rather than accepting its planning PR as runtime evidence. Exact source review findings R1 (open-PR accuracy) and R2 (historical/current count distinction) were corrected in this candidate; fresh independent review is still required.


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

## 2026-10-10 AWS and DNS binding preflight refresh

The 2026-10-10 initial preflight is historical. A later read-only binding refresh found an authenticated AWS STS identity that does not match the registry-aws Pulumi stack account; Cloudflare confirmed the sire.run zone is active and no gist.sire.run record exists. These observations qualify read visibility only, not deployment or DNS writes. See [the current binding refresh](receipts/2026-10-10-production-binding-refresh.md); the [earlier preflight receipt](receipts/2026-10-10-production-binding-preflight.md) is retained as superseded history.

### E-GR-GATEWAY -- Retired execution ledger; catalog provenance retained

fidelity: executable
Acceptance: Execution rows are retired history under ADR016; there is no activation trigger or gateway terminal. Only T-GR-GATEWAY.31 and .35 survive as catalog provenance obligations in batch PROVENANCE, retaining their original IDs and acceptance. Their implementation/review/release gates are the independent PROVENANCE chain.

- [x] T-GR-GATEWAY.0 Reconcile preserved RFC-003 source, PR and threat-model work  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  verifies: [infrastructure]  acc: [Inventory exact main and PR heads, dirty worktrees, RFC-003/ADR009/ADR010 revisions, private threat-model revision and preserved PR47 amendment; identify conflicting and unique changes with owners and evidence; do not overwrite or adopt unreviewed work]  Done: 2026-10-10  Evidence: [RFC-003 in-flight work reconciliation](receipts/2026-10-10-rfc003-work-reconciliation.md)
- [x] T-GR-GATEWAY.1 Author a coherent revised RFC-003, technical ADR and threat model  Owner: coordinator  Est: TBD  kind: agent  stage: author  blocked-by: [T-GR-GATEWAY.0]  verifies: [infrastructure]  acc: [Align caller-owned approval without Gist approval page/JWS, separate OpenAPI-generated gateway MCP, dynamic-guide contract and evaluation, selected KMS encryption, credential-intake decision gate, workspace/connection binding, token scope, idempotency, egress/SSRF, result injection, revocation, retention, GitHub REST plus controlled MCP fixture, and M-G1/M-G2 boundaries across public RFC/ADRs and private threat model; record unresolved owner decisions without inventing them]  Done: 2026-10-11  Evidence: [RFC-003 design reconciliation](receipts/2026-10-11-rfc003-design-reconciliation.md)
- [ ] T-GR-GATEWAY.2 Independently review the complete revised gateway design  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.1]  verifies: [infrastructure]  acc: [A non-author reviewer records exact base/head and findings across RFC, ADR, private threat model and RFC-002 boundary; specifically test caller-owned approval limits, generated REST/MCP parity, credential-intake hold, guide isolation/freshness, KMS custody, egress, result injection, revocation and M-G1/M-G2 separation; every finding is resolved or explicitly held before G0.4 owner recording]
- [ ] T-GR-GATEWAY.3 Correct and re-review accepted gateway design findings  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.2]  verifies: [infrastructure]  acc: [All accepted design findings have tracked corrections and focused verification; independent exact-head review approves the complete corrected design; unresolved technical authority remains a named gate]
- [ ] T-GR-GATEWAY.4 Migrate the complete gateway SDLC graph into this authoritative plan  Owner: coordinator  Est: TBD  kind: agent  stage: author  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.3]  verifies: [infrastructure]  acc: [RFC-003 M-G1 and M-G2 design, implementation, meaningful verification, independent review, findings correction, guarded merge, landed verification, release, AWS deployment, bounded live acceptance and operations are decomposed into stable task IDs below; each task has owner, dependencies and testable acceptance; provider-specific expansion is an explicit pre-implementation gate; preserve RFC-002 task IDs and make docs/plan.md the single authoritative source]
- [ ] T-GR-GATEWAY.5 Independently review the integrated gateway task graph  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.4]  verifies: [infrastructure]  acc: [Pinned Wazi schema, semantic validator, Markdown reader, task graph, links and historical status comparison pass on exact candidate; independent reviewer approves the gateway scope, blockers and full SDLC coverage; the current candidate conformance evidence is recorded in [the plan validation receipt](receipts/2026-10-11-gist-plan-conformance.md) and does not complete the independent review]
- [ ] T-GR-GATEWAY.6 Correct plan findings and revalidate exact gateway graph  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.5]  verifies: [infrastructure]  acc: [Accepted plan findings receive evidence-backed fixes; task IDs, ownership, dependencies, acceptance, stages and preserved RFC-002 status are rechecked; independent exact-head re-review passes]
- [ ] T-GR-GATEWAY.7 Record owner G0.4 sign-off on the exact revised design and task graph  Owner: founder  Est: TBD  kind: human  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.6]  verifies: [infrastructure]  acc: [Bind the owner approval from 2026-10-11 to exact independently reviewed RFC, ADR, private threat-model revisions, and complete integrated M-G1/M-G2 task graph; preserve the still-open credential-intake choice and no-provider-spend/AWS/DNS boundaries; do not request repeat approval of the already approved direction]
- [ ] T-GR-GATEWAY.8 Guarded merge of the approved gateway design and plan  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.7]  verifies: [infrastructure]  acc: [Only the exact independently approved candidate merges under unchanged protections; unavailable hosted CI is distinguished from local validation; actual merged SHA and tree are recorded]
- [ ] T-GR-GATEWAY.9 Verify landed design and plan before admitting implementation  Owner: coordinator  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.8]  verifies: [infrastructure]  acc: [Fresh remote main matches the reviewed design and complete task graph; pinned schema, semantic validator and Markdown reader pass on landed SHA; G0.4 evidence is present; only then may dependency-ready RFC-003 implementation tasks be dispatched]

- [ ] T-GR-GATEWAY.10 Settle the M-G1 credential-intake mode  Owner: founder  Est: TBD  kind: human  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.9]  verifies: [infrastructure]  acc: [Record whether credential intake is stored-only or caller-supplied per request, with exact accepted payload, retention, revocation, caller/workspace binding and failure semantics; this decision gates only the corresponding credential-ingress implementation and live qualification; no provider calls or secrets are needed to decide]
- [ ] T-GR-GATEWAY.11 Preflight M-G1 contract, source write set, test fixtures and release boundary  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.9]  verifies: [infrastructure]  acc: [Reconcile landed RFC-002 interfaces, exact OpenAPI and implementation files, migrations, KMS/IAM boundary, fixture targets, generated MCP tooling, test commands, source ownership, deployment prerequisites and credential-intake dependency; list unresolved owner inputs without starting provider traffic]
- [ ] T-GR-GATEWAY.12 Implement gateway OpenAPI contract and deterministic generated MCP surface  Owner: luna-contract  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.11]  verifies: [infrastructure]  acc: [Standalone gateway OpenAPI is authoritative for REST and allowlisted MCP operations; pinned generation produces stable gist_invoke/status/guide tools with identical input schemas, authorization, errors and bounded outputs; RFC-002 v1 tools remain unchanged]
- [ ] T-GR-GATEWAY.13 Implement caller-bound authorization, execution lifecycle and durable idempotency  Owner: luna-execution  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.12]  verifies: [infrastructure]  acc: [Execution derives tenant/principal from verified execution:invoke grant, checks exact capability/binding/connection and current revocation, durably records admitted/running/succeeded/failed/unknown outcomes and returns original result on identical replay; changed binding conflicts; unknown mutating outcomes never auto-retry; fingerprint uses the versioned unambiguous ADR-010 encoding]
- [ ] T-GR-GATEWAY.14 Implement KMS-backed credential custody and exact connection binding  Owner: luna-custody  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.10, T-GR-GATEWAY.12]  verifies: [infrastructure]  acc: [Workspace-bound envelope encryption uses least-privilege KMS identity, ciphertext-only persistence, bounded in-memory plaintext, rotation/revocation and no-secret response/log guarantees; intake behavior matches the recorded owner choice; no credential workflow is enabled before that decision]
- [ ] T-GR-GATEWAY.15 Implement outbound egress, SSRF, result-screening and revocation controls  Owner: luna-boundaries  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.13]  verifies: [infrastructure]  acc: [HTTPS policy validates and pins resolved IPv4/IPv6 destinations, rejects private/loopback/link-local/metadata targets, revalidates redirects, bounds time/bytes and screens/labels provider results as untrusted data; current grant, binding, connection and revocation are rechecked immediately before send]
- [ ] T-GR-GATEWAY.16 Implement the bounded GitHub REST proving adapter  Owner: luna-github-adapter  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.14, T-GR-GATEWAY.15]  verifies: [infrastructure]  acc: [Only the approved GitHub REST operations, repository binding and least-privilege scopes are callable; arbitrary URL/method/path, repository substitution, widened mutations and unbounded responses are rejected; the local fixture proves declared effects without live GitHub credentials]
- [ ] T-GR-GATEWAY.17 Implement the allowlisted controlled remote-MCP proving fixture  Owner: luna-mcp-adapter  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.12, T-GR-GATEWAY.15]  verifies: [infrastructure]  acc: [Fixture origin, operations, schemas, timeout and response limits are fixed; arbitrary host/tool selection is denied; hostile fixture output remains bounded untrusted data; no customer MCP endpoint or provider credential is required]
- [ ] T-GR-GATEWAY.18 Implement evaluated dynamic tool-use guides  Owner: luna-guides  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.12, T-GR-GATEWAY.13]  verifies: [infrastructure]  acc: [Guide output is grounded in validated contracts, maintained recipes and current authorized resolution, carries provenance/freshness, excludes secrets and unsupported actions, invalidates on revocation, and grants no execution authority; evaluation records correctness and provider-discovery turns against an approved baseline]
- [ ] T-GR-GATEWAY.19 Verify M-G1 contract, persistence, security and end-to-end behavior  Owner: luna-acceptance  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.13, T-GR-GATEWAY.14, T-GR-GATEWAY.15, T-GR-GATEWAY.16, T-GR-GATEWAY.17, T-GR-GATEWAY.18]  verifies: [infrastructure]  acc: [Real HTTP and generated MCP plus PostgreSQL/KMS/controlled fixtures cover authorization, tenant isolation, exact binding, replay/conflict, crash/restart, unknown outcome, revocation, secret non-disclosure, egress/redirect denial, REST/MCP parity, hostile results and guide freshness/evaluation; all required cases execute without skipped-as-pass]
- [ ] T-GR-GATEWAY.20 Independently review the complete M-G1 implementation  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.19]  verifies: [infrastructure]  acc: [Non-author reviewer records exact base/head and examines contract parity, tenant/principal binding, credential custody, idempotency encoding, egress, result trust, revocation, fixture limits and guide evaluation; every finding has an evidence-backed disposition]
- [ ] T-GR-GATEWAY.21 Correct M-G1 findings and obtain exact-head re-review  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.20]  verifies: [infrastructure]  acc: [Every accepted finding has a scoped fix, affected tests and independent final-head re-review; no open blocker or stale review remains]
- [ ] T-GR-GATEWAY.22 Guarded merge of the reviewed M-G1 gateway implementation  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.21]  verifies: [infrastructure]  acc: [Current policy, independent approval and available checks are assessed at exact base/head; merge uses the approved method, records immutable landed SHA/tree, and separates billing-blocked CI from actual code failures]
- [ ] T-GR-GATEWAY.23 Verify M-G1 implementation on fresh remote main  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.22]  verifies: [infrastructure]  acc: [Reviewed source is reachable from fresh remote main and affected contract, HTTP/MCP, storage, security and compatibility acceptance passes on the recorded landed SHA; source merge alone is not release or production acceptance]
- [ ] T-GR-GATEWAY.24 Prepare immutable M-G1 release and rollback inputs  Owner: luna-release-operator  Est: TBD  kind: agent  stage: release  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.23]  verifies: [infrastructure]  acc: [Release pins source/image digests, OpenAPI/generator/runtime versions, migrations, KMS/config references, monitoring/redaction, backup/restore, rollback and bounded cost envelope; no secret bytes or unapproved provider target enter the artifact]
- [ ] T-GR-GATEWAY.25 Deploy the approved M-G1 artifact through qualified AWS workflow  Owner: luna-production-operator  Est: TBD  kind: agent  stage: deploy  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.24, T-GR-AWS.0, T-GR-SCOPE.11]  verifies: [infrastructure]  acc: [Exact reviewed immutable gateway artifact is deployed in the approved AWS account/region/stack with health, configuration, migration, KMS/IAM, rollback and cost receipts; no manual DNS/policy bypass or unbounded provider call]
- [ ] T-GR-GATEWAY.26 Qualify bounded M-G1 live caller and provider acceptance  Owner: luna-live-acceptance  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.25, T-GR-GATEWAY.10]  verifies: [infrastructure]  acc: [An authorized named caller uses the production gateway against exact approved GitHub REST and controlled MCP targets within owner-set call/cost/expiry limits; approval remains caller-owned, exact grants/revocation/unknown outcomes are observed, and the real result is independently captured; unresolved intake mode or limits block the affected live path]
- [ ] T-GR-GATEWAY.27 Accept M-G1 operations handoff  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.26]  verifies: [infrastructure]  acc: [Named operator accepts alarms, redacted traces, execution/unknown-outcome reconciliation, grant and key rotation/revocation, KMS audit, backup/restore, incident response, rollback and bounded observation; independent operations check records the M-G1 terminal without implying M-G2 completion]
- [ ] T-GR-GATEWAY.28 Preflight M-G2 provider expansion, billing and provenance authority  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.9]  verifies: [infrastructure]  acc: [Inventory RFC-002's admitted provider families and current gateway M-G1 interfaces; document candidate-specific contracts, exact targets, owners, auth routes, support states, cost units, upper bounds and no-call qualification steps; unknown account/endpoint/limits remain explicit blockers]
- [ ] T-GR-GATEWAY.29 Approve exact M-G2 provider targets and numerical limits  Owner: founder  Est: TBD  kind: human  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.28]  verifies: [infrastructure]  acc: [Record each selected provider/account/endpoint/repository, owner-approved maximum calls, dollar cap, expiry, credential route and permitted effects; selection is per-provider, no blanket account grant is inferred, and no live calls start from this decision alone]
- [ ] T-GR-GATEWAY.30 Define the utility billing ledger and enforcement contract  Owner: coordinator  Est: TBD  kind: agent  stage: author  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.28]  verifies: [infrastructure]  acc: [Specify billable unit, reservation/settlement/refund/unknown semantics, idempotency, per-workspace quotas, append-only evidence, compensating entries, abuse controls and enforcement-before-send; billed routes stay disabled until implementation and acceptance land]
- [x] T-GR-GATEWAY.31 Define publisher signing and provenance verification  Owner: coordinator  Est: TBD  kind: agent  stage: author  blocked-by: [T-CATALOG.0]  verifies: [infrastructure]  acc: [Specify key custody, publisher identity, canonical signed bytes, artifact/version binding, rotation/revocation and consumer verification; public-distribution claims remain blocked until the exact scheme is implemented and independently qualified]
- [ ] T-GR-GATEWAY.32 Expand the approved M-G2 inventory into stable per-provider task graphs  Owner: coordinator  Est: TBD  kind: agent  stage: author  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.29, T-GR-GATEWAY.30, T-GR-GATEWAY.31]  verifies: [infrastructure]  acc: [For every owner-approved provider, add stable tasks for contract/schema, implementation, golden fixtures, support state, security/cost tests, independent review, fixes, merge, landed verification, release, deployment, bounded live acceptance and operations; every task has a named provider target, owner, dependencies and acceptance before any M-G2 provider code or call]
- [ ] T-GR-GATEWAY.33 Implement the append-only utility billing ledger and route enforcement  Owner: luna-billing  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.29, T-GR-GATEWAY.30]  verifies: [infrastructure]  acc: [Ledger obeys the reviewed unit/reservation/settlement/refund/unknown contract, append-only database privileges, compensating entries and per-workspace quotas; route admission enforces limits before spend; no billed route is live without approved numerical budgets]
- [ ] T-GR-GATEWAY.34 Implement adapters from the approved M-G2 per-provider task graphs  Owner: luna-provider-adapters  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.29, T-GR-GATEWAY.32]  verifies: [infrastructure]  acc: [Only provider child tasks created by .32 can authorize adapter code; each selected target has its own contract, provenance, golden fixture, support-state and bounded scope; aggregate coverage never substitutes for per-provider qualification]
- [x] T-GR-GATEWAY.35 Implement publisher signing and consumer verification  Owner: luna-provenance  Est: TBD  kind: agent  stage: implement  blocked-by: [T-GR-GATEWAY.31]  verifies: [infrastructure]  acc: [Signed artifact bytes bind publisher, immutable artifact/version, schema and provenance; key custody, rotation, revocation and independent consumer verification pass; RFC-002's distribution block changes only through its separately reviewed landing]
- [ ] T-GR-GATEWAY.36 Verify M-G2 billing, provider and provenance boundaries  Owner: luna-acceptance  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.33, T-GR-GATEWAY.34, T-GR-GATEWAY.35]  verifies: [infrastructure]  acc: [Real HTTP/MCP/storage fixtures verify append-only ledger privileges, limit enforcement before send, duplicate/unknown settlements, per-provider schemas/support/fixtures, signed-byte tamper detection, key rotation/revocation and no unauthorized cost or credential leakage]
- [ ] T-GR-GATEWAY.37 Independently review the complete M-G2 implementation  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.36]  verifies: [infrastructure]  acc: [Non-author reviewer covers exact provider matrix and each generated child graph plus billing, provider boundaries, provenance, custody, effect limits and test evidence; findings are tracked by provider and no aggregate green status hides a failing provider]
- [ ] T-GR-GATEWAY.38 Correct M-G2 findings and obtain exact-head re-review  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.37]  verifies: [infrastructure]  acc: [Every accepted finding has a scoped provider/ledger/provenance fix, affected checks and independent exact-head re-review; no unsupported provider or unresolved cost boundary is represented as qualified]
- [ ] T-GR-GATEWAY.39 Guarded merge of reviewed M-G2 source  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.38]  verifies: [infrastructure]  acc: [Exact reviewed source satisfies unchanged repository protections and policy; approved merge method records immutable landed SHA/tree and preserves provider-specific review/fixture evidence]
- [ ] T-GR-GATEWAY.40 Verify M-G2 implementation on fresh remote main  Owner: luna-landed  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.39]  verifies: [infrastructure]  acc: [All reviewed ledger, provider and provenance source is reachable from fresh main; provider-specific suites and production-disabled cost controls pass on the landed SHA; no local branch or stale receipt substitutes]
- [ ] T-GR-GATEWAY.41 Prepare immutable M-G2 release, migration and rollback  Owner: luna-release-operator  Est: TBD  kind: agent  stage: release  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.40]  verifies: [infrastructure]  acc: [Artifact pins landed source, each provider contract/fixture/support state, ledger schema and signing/provenance keys by reference; release notes, monitoring, backup/restore, rollback and per-provider cost ceilings are independently reviewed]
- [ ] T-GR-GATEWAY.42 Deploy approved M-G2 release through qualified AWS workflow  Owner: luna-production-operator  Est: TBD  kind: agent  stage: deploy  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.41, T-GR-AWS.0, T-GR-SCOPE.11]  verifies: [infrastructure]  acc: [Exact immutable M-G2 artifact and migrations deploy under approved AWS account, role, stack and provider allowlist; health, ledger enforcement, KMS, cost, alarms and rollback are recorded; no manual or unbounded provider route is enabled]
- [ ] T-GR-GATEWAY.43 Qualify bounded M-G2 live provider and billing acceptance  Owner: luna-live-acceptance  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.42, T-GR-GATEWAY.29]  verifies: [infrastructure]  acc: [Each selected provider independently passes named live behavior, exact support state, pre-send limit enforcement, idempotent billing/provenance checks and owner-approved calls/dollars/expiry; any unapproved target remains disabled and is not counted as coverage]
- [ ] T-GR-GATEWAY.44 Accept M-G2 operations and whole-gateway terminal  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-RETIRED.0, T-GR-GATEWAY.43, T-GR-GATEWAY.27]  verifies: [infrastructure]  acc: [Named operator accepts provider health/support, per-provider cost and failure alerts, ledger reconciliation, signing-key rotation, credential revocation, incident response, recovery and bounded observation; independent check proves M-G1 and M-G2 terminals with no unresolved production gate]
### E-GR-GUIDE -- Dynamic caller tool-use guidance

fidelity: executable
Acceptance: A task-specific guide is derived from validated capability contracts, maintained recipes and the caller's current authorized resolution; its provenance, freshness, failure behavior, security boundary and measured outcome are qualified in production without changing frozen registry v1 or granting execution authority.

- [ ] T-GR-GUIDE.0 Define the dynamic-guide contract and security boundary  Owner: coordinator  Est: TBD  kind: agent  stage: author  blocked-by: [T-CATALOG-HOLD.0, T-GR-SCOPE.31]  verifies: [UC-004, UC-008]  acc: [Design defines request and response contract, validated capability/recipe/resolution inputs, provenance and freshness, stale/revoked-resolution handling, error behavior, guide selection/assembly, authored-craft separation, credential exclusion and non-authority semantics without changing frozen registry v1]
- [ ] T-GR-GUIDE.1 Independently review the guide design and threat cases  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.0]  verifies: [infrastructure]  acc: [Exact-head review covers cross-tenant leakage, stale capability/recipe, revoked binding, malformed or adversarial metadata, unsupported action invention, credential exposure, cache invalidation and guide/invocation authority separation; findings have explicit dispositions]
- [ ] T-GR-GUIDE.2 Correct and re-review accepted guide design findings  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.1]  verifies: [infrastructure]  acc: [Accepted design findings receive evidence-backed edits and focused validation; independent reviewer approves the exact corrected contract and threat cases]
- [ ] T-GR-GUIDE.3 Record owner approval of guide contract and release placement  Owner: founder  Est: TBD  kind: human  stage: verify  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.2]  verifies: [infrastructure]  acc: [Owner signs off exact guide contract, relation to registry resolution and gateway, version/freshness behavior and release placement; record exact reviewed revision and keep any unapproved capability out of implementation scope]
- [ ] T-GR-GUIDE.4 Decompose the guide implementation and full delivery graph  Owner: coordinator  Est: TBD  kind: agent  stage: author  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.3]  verifies: [infrastructure]  acc: [Stable task IDs separately cover source resolution, safe guide assembly, provenance/freshness/invalidation, endpoint or transport integration, metrics, security and correctness tests, independent review, fixes, merge, landed checks, release, AWS rollout, live acceptance and operations; every task has owner, dependency and testable acceptance]
- [ ] T-GR-GUIDE.5 Independently review the complete guide SDLC plan  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.4]  verifies: [infrastructure]  acc: [Reviewer confirms the authoritative plan contains the complete dependency graph and preserves RFC-002 v1, task identity and existing caller/provider authority gates]
- [ ] T-GR-GUIDE.6 Correct and revalidate the guide SDLC plan  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.5]  verifies: [infrastructure]  acc: [Accepted plan findings are fixed and the complete plan passes pinned Wazi schema, owning semantic validator, Markdown reader, link and historical-status checks; exact-head independent re-review passes]
- [ ] T-GR-GUIDE.7 Guarded merge of approved guide design and plan  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.6]  verifies: [infrastructure]  acc: [Exact approved design and plan candidate merges under unchanged protections; hosted CI failure/unavailability is separated from local evidence and actual merge SHA is recorded]
- [ ] T-GR-GUIDE.8 Verify landed guide design and implementation subtask graph  Owner: coordinator  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.7]  verifies: [infrastructure]  acc: [Fresh main contains the reviewed design and complete implementation subtask graph; pinned Wazi and affected docs checks pass on landed SHA; implementation is admitted only for dependency-ready tasks]
- [ ] T-GR-GUIDE.9 Implement grounded guide request, selection and assembly  Owner: coordinator  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.8]  verifies: [UC-004, UC-008]  acc: [The accepted contract returns only task-specific guidance grounded in validated capabilities, maintained recipes and the caller's current authorized resolution; unsupported operations are omitted rather than invented; guide output grants no execution authority]
- [ ] T-GR-GUIDE.10 Implement guide provenance, freshness and invalidation  Owner: coordinator  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.9]  verifies: [UC-004, UC-008]  acc: [Guide provenance names the validated contract, recipe and resolution inputs; stale, expired, revoked or cross-tenant inputs cannot produce reusable unauthorized guidance; no credential or private secret is emitted]
- [ ] T-GR-GUIDE.11 Implement reproducible correctness and discovery metrics  Owner: coordinator  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.10]  verifies: [UC-004, UC-008]  acc: [The test/measurement harness records guidance completeness, model/provider discovery calls and turns against the approved task set and baseline without claiming unmeasured savings or including secrets]
- [ ] T-GR-GUIDE.12 Verify guide correctness, isolation and invalidation  Owner: coordinator-verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.11]  verifies: [UC-004, UC-008]  acc: [Meaningful tests cover complete capability selection, invented/unknown operations, stale or revoked resolution, cross-tenant isolation, malformed/adversarial metadata, no-credential output and unchanged invocation authorization; measured discovery-call/turn fixtures and correctness baselines are reproducible]
- [ ] T-GR-GUIDE.13 Independently review the exact guide implementation  Owner: independent-luna-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.12]  verifies: [infrastructure]  acc: [Independent exact-head review checks contract parity, tenant isolation, freshness, credential exclusion, non-authority boundary, test coverage and measured claims; every finding is dispositioned]
- [ ] T-GR-GUIDE.14 Fix guide implementation findings and re-review  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.13]  verifies: [infrastructure]  acc: [Accepted code findings have focused fixes, affected tests and independent exact-head re-review; source and test identities are recorded]
- [ ] T-GR-GUIDE.15 Guarded merge of approved guide implementation  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.14]  verifies: [infrastructure]  acc: [Only exact reviewed implementation merges through current repository protections; unavailable hosted CI is not described as success and merge SHA/tree parity is recorded]
- [ ] T-GR-GUIDE.16 Verify landed guide implementation and frozen v1 parity  Owner: coordinator  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.15]  verifies: [UC-004, UC-008]  acc: [Fresh main contains the reviewed guide implementation; all affected tests and the frozen v1 contract/parity checks pass on landed SHA; no local-only code remains]
- [ ] T-GR-GUIDE.17 Prepare immutable guide release inputs  Owner: luna-release-operator  Est: TBD  kind: agent  stage: release  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.16]  verifies: [infrastructure]  acc: [Release source, image digest, contract/recipe revisions, compatibility notes, rollback and monitoring inputs are immutable, reviewed and tied to landed SHA; no unqualified provider capability or secret is embedded]
- [ ] T-GR-GUIDE.18 Deploy approved guide release through the authorized AWS workflow  Owner: luna-production-operator  Est: TBD  kind: agent  stage: deploy  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.17, T-GR-SCOPE.11, T-GR-AWS.0]  verifies: [infrastructure]  acc: [Exact reviewed immutable artifact is deployed at the canonical AWS origin through the approved workflow with pre/post health, configuration parity, rollback and cost/change scope receipts; no manual policy or DNS bypass]
- [ ] T-GR-GUIDE.19 Verify live guide behavior and measured outcomes  Owner: luna-live-acceptance  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.18, T-GR-TREG.6, T-GR-COMPOSIO.6, T-GR-SCOPE.5]  verifies: [UC-004, UC-008]  acc: [Named production caller workflows return complete grounded guides, reflect current authorization/revocation, expose no credentials, and meet owner-approved correctness/discovery-turn metrics using only bounded qualified provider targets; failures create scoped repair, re-verification and independent re-review work]
- [ ] T-GR-GUIDE.20 Record operations handoff and guide production terminal  Owner: luna-ops  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG-HOLD.0, T-GR-GUIDE.19]  verifies: [infrastructure]  acc: [Monitoring, freshness/invalidation alerts, ownership, incident response, rollback, cost and evidence retention are accepted; independent review verifies the live receipt and the result is recorded as the dynamic-guidance terminal, not whole-project completion]
#### Wave 10: Non-dispatchable disposition sentinels

- [ ] T-CATALOG-RETIRED.0 Retired execution scope; never dispatch  Owner: coordinator  Est: none  kind: human  stage: retired  acc: [Owner retired Gist hosted execution on 2026-10-16; this sentinel is permanently non-dispatchable and is never marked complete to revive old tasks]
- [ ] T-CATALOG-HOLD.0 Paused hosted expansion and unselected guide work  Owner: coordinator  Est: none  kind: human  stage: hold  acc: [Retain unfinished historical obligations without dispatch; a future scoped repair or existing-consumer obligation must record its exact task selection and authority before revising dependencies]

### Implementation resumption — 2026-10-16

After PR62 landed, the owner invoked ship with batch-size10 and selected a read-only inbox briefing. The owner clarified that Zati has no Gmail MCP configured and is waiting for Gist's tool/skill library. This supersedes the earlier requirement for an already-configured consumer as an implementation prerequisite: build the real skill and integration together, then qualify actual binding/setup separately. Preserve the original acceptance ledger; no absent connection is described as admitted. The prior docs-only delivery is complete; this invocation authorizes code, review and merge, not release publication, provider actions or deployment. Gmail secrets stay outside source and Gist custody.

First skill: `zati/inbox-briefing@1.0.0`, including `assets/briefing-template.md`. Required semantic capability: `mail/inbox/read@1.0.0`; provider route will be the actual admitted Gmail MCP connection when configured, never a guessed tool name. Read-only workflow lists recent unread messages and reads selected content, bounded to20 messages, without sending, modifying labels or marking mail read. Controlled adapters can prove source behavior but not live Gmail acceptance.

Current ship invocation (2026-10-16) resumes the library-to-Zati outcome. Current-session inherited model/reasoning; requested member ceiling10. Source foundation batch covers T-CATALOG.9 and .10 (actual2 implementation/verification members), one implementer, coordinator owns plan/skill artifacts; separate independent review .11, merge .12, landed .13. Consumer batch T-CATALOG.1/.2 follows producer landing through .13, with existing .3-.6 gates. Public signing stays separate. API reuses domain implementations under catalog/{ports,packages,resolution,discovery} with hosted compatibility wrappers; caller supplies storage and current authorization/binding state. Package consumption must check externally trusted pins and retain every asset. Zati integration owner serializes root dependency/specification changes.

Checks: root public catalog package tests and root compatibility suite; hosted packages/ports/discovery/resolution/contract and frozen wire checks; Zati skills import/archive, execution tool-selection and real application integration. Tests run under qualified host/lease capacity. Cross-component denial, tamper, missing/revoked/stale capability and current authorization cases are required. Exact live Gmail connection/account/operations and any provider limits remain separate acceptance inputs; never infer access from OAuth client settings.

Source foundation evidence: [2026-10-16 catalog library verification](receipts/2026-10-16-catalog-library-source.md). T-CATALOG.0 selects the maintained inbox skill and private trusted-source policy; its additive public interfaces and explicit missing-provider behavior are exercised by the public consumer test. Zati base35ebe4df owns import, configuration admission, current connections, permissions and effects. T-CATALOG.9/.10 are source/check complete only; independent review, PR66 rebase merge and landed verification subsequently completed at9e870c0a7b93990b166071ec061a27206afbda88. The reviewed tree matches fresh main; landed public consumer and frozen-source checks pass. Zati source integration and controlled checks are recorded in [the consumer receipt](receipts/2026-10-16-zati-catalog-consumer.md); independent review, correction and Zati PR41 landing are complete at aabfdc5e0d483c2196d32d95f7696ff9b1dc7b63, with exact reviewed-tree parity and passing landed consumer checks. Release and installed/live Gmail acceptance remain open.

### E-CATALOG -- Reusable catalog library and real Zati consumer

fidelity: executable
Acceptance: Public additive library APIs provide complete authorized packages and tool resolution, demonstrated by a real Zati consumer with existing execution ownership and no gateway dependency. This is planned work, not source or runtime qualification.

Parallel Work: two outcome batches, CATALOG (priority) and PROVENANCE (public-distribution qualification), with Gist and Zati owners coordinating API and consumer boundaries; no new fleet or paid worker authority. Use the current session executor and inherited model/settings when implementation is separately started. Capability label: local documentation now; repository-specific qualified Go checks and Zati runtime later. Missing exact consumer/check recipes are explicit preflight inputs. Existing task owner labels are historical assignments, not permission to change model routing.

#### Wave 11: Library-first consumer delivery

- [x] T-CATALOG.0 Select real consumer and freeze additive API contract  Owner: coordinator-with-Zati  Est: TBD  kind: agent  stage: preflight  acc: [Record selected real skill identity with asset, required capability and explicit connection/admission prerequisites, Gist/Zati revisions, public API/storage/auth ownership, selected catalog provider inventory and exact verification commands; an unconfigured Gmail connection blocks live acceptance rather than source implementation under the owner clarification of 2026-10-16; preserve Gist v1.2.0 ranking and root API compatibility; unresolved consumer selection blocks downstream implementation, not this document delivery]
- [x] T-CATALOG.1 Extract supported reusable catalog APIs and integrate selected Zati slice  Owner: Gist-with-Zati  Est: TBD  kind: agent  stage: implement  blocked-by: [T-CATALOG.0, T-CATALOG.13]  acc: [Reuse existing domain code for authorized discovery, exact complete package retrieval, digest/provenance verification and explicit tool-capability resolution through additive public APIs; real Zati adapter uses its existing permissions/connections/executor; no hosted/internal import, duplicated registry, token vault or invocation endpoint]
- [x] T-CATALOG.2 Verify actual consumer and catalog security boundaries  Owner: consumer-verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG.1]  acc: [Selected real skill and asset pass retrieval/resolution and authorized external execution with caller evidence; missing tool, wrong tenant, tampered bytes, expired/revoked binding and permission denial fail closed; package digest/provenance tests and public API compatibility pass for the selected trust model; workspace-private trusted-source import does not require public-distribution signing; task correctness and discovery/context baseline results recorded without invented savings]
- [x] T-CATALOG.3 Independently review catalog and consumer candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG.2]  acc: [Non-author reviews exact base/head across T-CATALOG.1/.2; records security/API/consumer findings and disposition, authentic tests and scope with no pooled credentials]
- [x] T-CATALOG.4 Correct accepted findings and obtain exact-head re-review  Owner: Gist-with-Zati  Est: TBD  kind: agent  stage: fix  blocked-by: [T-CATALOG.3]  acc: [Accepted findings have focused corrections, affected checks and independent exact-head approval across both repositories where changed]
- [x] T-CATALOG.5 Merge independently approved library and adapter changes  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-CATALOG.4]  acc: [Exact reviewed heads merge under each repository policy with immutable SHA and checks recorded; dependency order respects verified landing]
- [x] T-CATALOG.6 Verify landed code and reusable consumer qualification  Owner: consumer-verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-CATALOG.5]  acc: [Fresh remote main reaches reviewed Gist and Zati source; affected checks pass and real consumer receipt binds exact revisions; report source acceptance separately from installed release/runtime]
- [ ] T-CATALOG.7 Publish compatible library release and qualified Zati adoption  Owner: release-owner  Est: TBD  kind: agent  stage: release  blocked-by: [T-CATALOG.6]  acc: [Separately authorized immutable library release and Zati dependency update follow their reviewed workflows with rollback, no hosted deployment and no new paid provider authority; actual installed consumer revision is recorded]
- [ ] T-CATALOG.8 Accept library-first operational outcome  Owner: independent-reviewer  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG.7]  acc: [Actual released consumer uses catalog APIs with measured correctness and failure receipts, ownership, revocation refresh and rollback instructions; independent evidence distinguishes library qualification from paused hosted M2/M3 and never requires a Gist gateway]


#### Wave 13: Reusable source foundation and first real skill

- [x] T-CATALOG.9 Expose reusable catalog domain and inbox briefing package  Owner: catalog-implementer  Est: TBD  kind: agent  stage: implement  acc: [One shared implementation serves additive public APIs and hosted wrappers; exact verified package bytes and assets, object-shaped required capabilities and current authorization are enforced; real read-only inbox briefing skill and template declare semantic requirements without invented Gmail binding or credentials]
- [x] T-CATALOG.10 Verify public library and hosted compatibility  Owner: coordinator  Est: TBD  kind: agent  stage: verify  blocked-by: [T-CATALOG.9]  acc: [Public external-package consumer tests cover discovery to resolution to verified package consumption, denied/tampered/revoked/missing inputs and complete assets; hosted compatibility and frozen wire checks plus root API regressions pass; controlled source evidence is separate from live Gmail]
- [x] T-CATALOG.11 Independently review source foundation  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-CATALOG.10]  acc: [Non-author reviews exact base/head including shared extraction, hosted adapters, skill assets and plan; accepted findings corrected and affected checks plus exact-head re-review pass]
- [x] T-CATALOG.12 Merge approved source foundation  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-CATALOG.11]  acc: [Guarded rebase merge of exact reviewed source under current policy with immutable source and checks recorded]
- [x] T-CATALOG.13 Verify landed source foundation  Owner: coordinator  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-CATALOG.12]  acc: [Fresh main reaches independently reviewed catalog/skill tree and affected checks pass; consumer may now pin actual landed revision, no release or live Gmail acceptance inferred]

### E-PROVENANCE -- Retained publisher verification lifecycle

fidelity: executable
Acceptance: Retained T-GR-GATEWAY.31/.35 specify and implement catalog publisher signing without execution custody or billing. Public distribution remains blocked until this independently reviewed chain qualifies it; trusted workspace-private Zati import uses its selected provenance policy and is not blocked on this chain.

Current ship invocation 2026-10-16 uses the current inherited model/settings and requested batch ceiling10. One coherent source assignment has three author/implementation/verification members: T-GR-GATEWAY.31, T-GR-GATEWAY.35 and T-PROVENANCE.0; independent review, corrections, merge and landed verification remain T-PROVENANCE.1-.4. One implementer owns additive catalog provenance source, ADR and consumer tests; coordinator owns this plan and receipts. Native Zati qualification is independent and preserves the unrelated desktop preview. Existing Mac build leases and bounded qualified Linux checks govern resource use. Token/cost telemetry is unavailable; no acceleration claim is inferred from count.

Acceptance per source member remains its existing row. Freeze caller-owned signing and authoritative trust policy in an ADR, bind domain-separated canonical statements to immutable package/manifest/closure digests, publisher, version, schema, workspace and source; reject tampering, missing/unknown/revoked/stale trust and expired statements. No hosted identity-token key reuse, default network trust discovery or private-key storage. Frozen v1/v2 and operator-asserted import remain unchanged. T-PROVENANCE.5 stays pending for separately authorized public release and real trust enrollment, custody, rotation and recovery qualification. Split the assignment if review size or an external prerequisite requires it.

Source checkpoint da9dfc51321827250682330e96ef6cf590b4a66d implements ADR019 and real external consumer verification. Bounded Linux ARM64 full root/catalog tests, vet and tidy-diff pass; frozen contract checker and12Python regressions pass. Independent source review against16245c67a667447ee3a5d139e7d9a04f16a91ba6 approved with no blocking findings; no correction required. Final exact-head independent approval covers8f63a139b39ebf1428434d90c80ff5e611e2c70f. PR69 rebase-landed e0182846ff48e69acf25c8b48820bb2ae53ece06 with exact tree equality and fresh-main reachability; actual landed provenance/package consumer tests, frozen checker and404-task Wazi validation pass. T-PROVENANCE.5 remains pending. Evidence: [source receipt](receipts/2026-10-16-catalog-provenance.md).

#### Wave 12: Public-distribution provenance qualification

- [x] T-PROVENANCE.0 Verify publisher identity and lifecycle semantics  Owner: catalog-verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-GR-GATEWAY.35]  acc: [Meaningful signed-package consumer tests verify canonical bytes, tamper detection, publisher identity, rotation, revocation, stale trust and cross-tenant denial with no provider credential custody]
- [x] T-PROVENANCE.1 Independently review exact provenance candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PROVENANCE.0]  acc: [Non-author exact base/head review covers retained .31/.35 and verification; all findings have explicit disposition and no public-distribution claim precedes qualification]
- [x] T-PROVENANCE.2 Correct findings and obtain exact-head re-review  Owner: catalog-owner  Est: TBD  kind: agent  stage: fix  blocked-by: [T-PROVENANCE.1]  acc: [Accepted signing and verification findings have fixes, affected checks and independent exact-head approval]
- [x] T-PROVENANCE.3 Merge reviewed provenance implementation  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-PROVENANCE.2]  acc: [Exact reviewed head merges under current repository policy and immutable SHA is recorded]
- [x] T-PROVENANCE.4 Verify landed provenance behavior  Owner: catalog-verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-PROVENANCE.3]  acc: [Fresh main reaches reviewed signing source and authentic consumer verification checks pass with exact revision evidence]
- [ ] T-PROVENANCE.5 Release and qualify public-distribution trust  Owner: release-owner  Est: TBD  kind: agent  stage: release  blocked-by: [T-PROVENANCE.4]  acc: [Separately authorized compatible release has independently verified real consumer signatures and revocation plus publisher-key custody/rotation/recovery ownership and rollback evidence; no hosted rollout or public publication occurs without its own scope; public-distribution block lifts only for the qualified release]

### E-PREP -- Task preparation and semantic retrieval evaluation

fidelity: executable
Acceptance: Independently reviewed, reproducible evidence determines whether optional semantic retrieval and task-specific guides reduce discovery overhead without sacrificing whole-task correctness or authority boundaries. No benchmark result or model adoption is asserted by this plan.

Authority: the owner requested evaluation planning, ADRs and RFC updates on 2026-10-16 after selecting inbox briefing and library implementation. This document delivers that planning artifact. Execution follows existing ship scope where applicable; paid models/providers, real mail access, default enablement and release require their named gates. ADR017/018 and docs/evaluations/tool-preparation.md define the experiment. No gateway or hosted expansion is revived.

Parallel Work: original evaluation planning proposed source batch2 members T-PREP.1/.2 once their external inputs were qualified; coordinator owns API/data freeze, one inherited implementer owns harness/preparation. Verification .3 and independent review .4 are shared gates, not padded member count. Corrections .5 and re-review .6 preserve findings. Corpus preparation may proceed while the library foundation lands; .2 waits on T-CATALOG.13. Results batch .9/.11 is split at live-provider authority .10. Current-session inherited model/effort; counts/cost unavailable until execution, not estimated savings. Separate independent reviewer; host load/build leases bound concurrency. Split if a provider prerequisite, cross-repo ownership or practical review size requires it. Current owner ship invocation resumes source work with requested ceiling10, actual4 outcome members T-PREP.0-.3 in one retained implementation context; .0 freezes the API/protocol before .1 runtime/corpus work and .2 harness implementation. One source implementer owns preparation, adapter and harness; an independent corpus author and non-author label reconciler preserve held-out separation. Coordinator owns plan and receipts; independent full-candidate review remains .4-.6, then guarded merge .7 and landed verification .8. Native Mac qualification is an independent capacity-bound check, preserving unrelated previews. Current inherited model/reasoning, no overrides; exact model/token/cost telemetry unavailable.

Source batch preflight: existing library foundation and consumer source are landed. Model metadata reports public ungated Apache2 model google/embeddinggemma-2 at914f7f89142e33e77833254d9c9b90c3cef7303b, but installed Transformers5.3.0 lacks its architecture. Authorize isolated pinned runtime installation and bounded synthetic-only model download/qualification on existing Linux hardware, no shared runtime replacement, new cloud or paid/private-data calls. Resource envelope: at most8GiB downloads,24GiB peak resident memory,4CPU threads,one GPU job,20minute process deadline and45minute qualification total; stop and report exact incompatibility if exceeded. Actual model/runtime revisions, hashes and cold/warm evidence must precede .1 completion. Corpus author seals held-out inputs/labels outside implementation-accessible source until candidate freeze; two-annotator disagreement reconciliation is recorded before ranking scores. Public artifacts contain synthetic data and hashes, no credentials/private hosts. Runtime inability or incomplete labels keep .1/.2 incomplete; no fabricated semantic arm. Split at a material incompatibility or impractical review boundary without weakening existing acceptance. T-PREP.9 held-out runs and .10 paid/private-provider authority remain separate.

Source preflight checkpoint: ADR020 contract frozen at f2bf083 before implementation; real pinned EmbeddingGemma2 BF16/FP32 by256/768 text-only qualification passes with trusted pre-load file integrity and corrected model-version binding at46060fa. Independent bounded review approved PREP-R1. Corpus v3 is independently annotated/reconciled and reviewed at75ecc3b, metadata closure throughc43ab90;120cases60/60, actual44development/35held-out construction clusters, six strata per split. Failed corpus revisions remain recorded. Only2cases use the maintained inbox skill: this source pilot cannot establish product coverage,2pp noninferiority or adoption. T-PREP.2/.3 source/check acceptance passes at691cced; independent review issued PREP-R6-R9, corrections and fresh aggregate checks pass. Independent complete-candidate approval covered1e32629 against76d9c5f. PR71 rebase merged ate12b8a23a8479737fbc58ab5cf790e2ff130359e; fresh main has exact reviewed-tree parity and landed public API/loader/report tests plus dry-run pass. T-PREP.0-.8 source lifecycle is complete, with no benchmark/adoption/release implication. T-PREP.9 requires compatible independently reviewed catalog inventory and caller task-admission policy; the original default admits two literal smoke requests; the subsequently landed .22 adapter accepts an explicit hash-bound evaluation policy, and .31-.32 qualify a separate positive closed-menu cohort without admitting general caller authority. Richer inbox coverage remains required before positive adoption. No held-out scores or provider comparison are claimed. See [source receipt](receipts/2026-10-16-task-preparation.md).

Historical GUIDE coverage: T-GR-GUIDE.0-.9 design/contracts and .10-.16 implementation/evaluation/review are preserved as held history; selected library equivalents are T-PREP.0-.8 with ADR017 boundaries and .9-.13 measured results. Their gateway/REST/MCP route assumptions do not transfer. T-GR-GUIDE.17-.20 hosted release/deploy/production remain held. T-PREP.14-.20 own any separately selected library rollout. Original IDs/status/evidence remain available; no unimplemented guide is marked complete.

#### Wave 14: Protocol and source candidate

- [x] T-PREP.0 Freeze corpus protocol and additive preparation contract  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  acc: [ADR017/018 protocol is bound to exact source; additive Go request/result and optional inference interface specify auth-before-exposure, complete assets/schemas, explicit non-readiness and deterministic guide provenance; dataset splits, primary comparisons, thresholds, target resource budgets and no-provider dry run are frozen before held-out evaluation]
- [x] T-PREP.1 Build labeled corpus and qualify local embedding adapter  Owner: evaluation-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.0]  acc: [At least120 cases with disjoint60 development and60 held-out family splits have independently reconciled labels and strata; real inbox workflow is distinguished from synthetic probes; pinned text-only EmbeddingGemma2 adapter, official prefixes, dimensions and BF16/FP32 run on named qualified hardware with cold/warm memory and latency receipts; no hidden remote inference]
- [x] T-PREP.2 Implement preparation and reproducible comparison harness  Owner: evaluation-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.1, T-CATALOG.13]  acc: [L0/L1/E0/H0/H1 and documented C0/C1 adapters, deterministic guides, exact capability closure, bounded fallback, source/runtime manifests, call/cost estimator, redacted per-attempt records and deterministic report commands are exercised; unsupported access returns NOT_RUN, no live call or published performance claim is implied]
- [x] T-PREP.3 Verify harness and preparation behavior  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.2]  acc: [Real public API tests exercise task intent to selected complete skill/assets/schema/guide and explicit missing connection; adversarial isolation, revocation, tamper, truncation, cache/model version changes and fallback pass; metric accounting includes failed/retried/cold runs and all model tokens; frozen wire and existing root compatibility gates pass]
- [x] T-PREP.4 Independently review preparation candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PREP.3]  acc: [Non-author reviews exact base/head for .0-.3, data leakage, baseline parity, authorization and metric accounting; records all findings with stable IDs and disposition; this review records findings and does not claim approval while blockers remain]
- [x] T-PREP.5 Correct accepted preparation findings  Owner: evaluation-implementer  Est: TBD  kind: agent  stage: fix  blocked-by: [T-PREP.4]  acc: [After .4 has issued its findings, each accepted finding has a focused correction and affected checks; no-op requires a recorded zero-finding review, and fixes never wait on successful approval]
- [x] T-PREP.6 Re-review exact corrected preparation head  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-PREP.4, T-PREP.5]  acc: [Independent exact-head review verifies every blocking finding resolved or records no-change approval; complete candidate and protocol are covered]
- [x] T-PREP.7 Merge reviewed preparation source  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-PREP.6]  acc: [Exact approved head merges through guarded rebase under repository policy with authentic check provenance, no required-check bypass]
- [x] T-PREP.8 Verify landed preparation and harness  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-PREP.7]  acc: [Fresh main contains reviewed source; affected public API and accounting checks and protocol hashes match the run manifest; no benchmark or deployment acceptance is inferred]

#### Evaluation input qualification source batch (2026-10-16)

Owner ship invocation with batch ceiling25 resumes executable prerequisites identified in the readiness receipt, without reopening sealed held-out data or choosing an adoption outcome. Actual source batch3 outcome members .21-.23: freeze a bounded evaluation-only policy/config contract, implement explicit caller-policy injection, and expose the preregistered development k grid. One inherited implementer retains this assignment; separate independent review covers the complete candidate. Coordinator owns plan/receipt. Current attended executor; no model overrides, no paid/private-data/provider actions, no release or default changes. No new model download/inference is needed for this source batch; existing qualified Linux Go tooling can verify tracked-only source in isolated directories with4CPU threads and10minute command deadlines. Mac multi-package work requires current capacity and its owned build lease. Requested25 is a ceiling, not a task quota; token/cost telemetry unavailable. Representative cohort and inventory qualification remain .9 prerequisites, not inferred from new adapters. Source34c03b0 passed full-root Go tests/vet/build, affected races/lint,16 Python regressions and dependency/format checks after EVAL-R1/R2 corrections; independent full-candidate approval coveredf2ad4d4 against7ece68b, PR74 rebase landed8ef1969 with exact reviewed-tree parity and passing landed Go/Python/CLI checks. T-PREP.21-.29 source lifecycle is complete; .9 and owner adoption/release gates remain pending. See [adapter receipt](receipts/2026-10-16-evaluation-input-adapters.md).

The frozen additive driver contract: optional external task-policy file requires its exact SHA256, bounded regular-file single-snapshot parsing, strict schema and coherent caller scope. Positive entries bind exact intent digest, principal/runtime, immutable maintained skill pin, required capability contracts and recipe; the policy independently binds the base catalog SHA so versioned capability references cannot silently inherit different bytes; labels, expected statuses, query-derived catalog synthesis and permission inference are excluded. The loaded policy is an evaluation fixture, not a live authority service. No external policy retains the explicit two-literal source smoke behavior with an identifiable policy version. Every attempt and held-out admission/report binds the actual policy identity. Development accepts k4/8/16 with fixed floor0.30/RRF60; held-out runs require exact frozen k, model, source, policy, catalog, input and recipe identities before input/label access. CLI/report regressions must reject tampering, unsupported config, mixed identities and gold-as-policy. No benchmark quality or admission is supplied by configuration flags.

- [x] T-PREP.21 Freeze evaluation policy and configuration adapter requirements  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  acc: [Bounded additive evaluation-only contract above is frozen before code; source admission remains separate from real caller authority, held-out access and adoption; existing corpus and scope preserved]
- [x] T-PREP.22 Implement hash-bound caller task-policy adapter  Owner: evaluation-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.21]  acc: [Explicit independently supplied policy manifest enters the existing TaskAdmitter port with bounded strict digest-bound parsing and exact intent/principal/runtime/skill/capability scope; every attempt and held-out guard records policy identity; defaults preserve source smoke only, no gold or provider execution]
- [x] T-PREP.23 Implement frozen development configuration driver  Owner: evaluation-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.21]  acc: [CLI exposes development k4/8/16 under fixed cosine floor and RRF; exact config is emitted and bound to held-out admission/report checks; examples demonstrate qualified independent inventory/policy use without changing ranking thresholds or opening test data]
- [x] T-PREP.24 Verify evaluation policy and configuration behavior  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.22, T-PREP.23]  acc: [Meaningful public CLI and SDK-path tests cover external admitted preparation, denied/mismatched scope, file tamper/symlink/size/schema failures, k grid and mixed frozen policy/config refusal before held-out input or label access; affected Go/Python checks and compatibility pass]
- [x] T-PREP.25 Independently review evaluation input adapters  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PREP.24]  acc: [Non-author exact base/head review covers .21-.24 and records reachable findings without claiming approval while blockers remain]
- [x] T-PREP.26 Correct evaluation adapter findings  Owner: evaluation-implementer  Est: TBD  kind: agent  stage: fix  blocked-by: [T-PREP.25]  acc: [Accepted findings have focused corrections and affected checks; zero findings receive explicit no-op disposition]
- [x] T-PREP.27 Re-review corrected evaluation adapters  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-PREP.26]  acc: [Non-author approves exact corrected candidate after all blocking findings resolve and current checks pass]
- [x] T-PREP.28 Merge reviewed evaluation input source  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-PREP.27]  acc: [Exact approved head merges through guarded rebase under unchanged repository policy with authentic check provenance]
- [x] T-PREP.29 Verify landed evaluation input adapters  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-PREP.28]  acc: [Fresh main matches reviewed source and affected policy/config/public CLI checks pass; no held-out evaluation, live authority or adoption is inferred]

#### Controlled inbox development qualification (2026-10-16)

Owner ship ceiling25 admits four outcome members .30-.33: freeze this secondary study, qualify independently authored catalog/caller-policy inputs, independently reconcile a closed-menu inbox development cohort, and run bounded offline development configurations. One catalog/cohort author, a separate blind annotator/reviewer, and one runner preserve source/data ownership; inherited session settings, no overrides. Coordinator owns plan and evidence. This is development-only conformance coverage, not a new held-out study or replacement for the120-case pilot. All positive intents are explicit synthetic caller-menu requests for the same maintained read-only inbox recipe, independently authored after exporting a fixed catalog, without skill IDs in query text. No policy is generated from gold or expected outputs. The author freezes menu/policy hashes before independently labeling derived cases. A non-author labels inputs from frozen artifacts without original labels or scores, then disagreements reconcile before scoring. Use one conservative construction-family cluster for this one-workflow menu rather than claiming independent intent families; no generalization or2pp noninferiority inference. The sealed original held-out data stays untouched. This deliberately qualifies representable positive preparation first; existing safety regressions remain separate, negative-case/product/task coverage cannot be inferred.

Freeze30 admitted menu intents/cases, actual fixed catalog bytes and complete composite-read closure, input/label/policy hashes, source/runtime/recipe/configuration and grading before running. Compare L0/L1/E0/H0/H1; C0/C1 NOT_RUN. BF16 dimensions256/768 and k4/8/16, floor0.30/RRF60, fixed65536-byte budget. Report every actual attempt, coverage/readiness/errors/unknown usage, source-cold/source-warm timings excluding inference, with local embedding load/RSS/timing separately. Primary development selection maximizes complete positive preparation rate then correct exact closure, ties choose256 then k4; without eligible evidence no selection. This selects a development candidate only, never a Zati default. One family means uncertainty/generalization remain unavailable or inconclusive under the frozen margin; no pseudo-replication. Existing qualified Linux CPU runtime only:4threads,24GiB RSS hard cap,1200seconds per embedding command,45minute aggregate, no new download/cloud/paid/private/provider calls. Stop on pinned-file/runtime mismatch, preserve failures, never rewrite gold after scores. No Zati/SDK behavior change; .9 remains pending actual held-out and full protocol qualification.

Controlled grid checkpoint: frozen source3676aa1 produced2520 retained records (1800 local,720 unavailable C0/C1) across256/768 and k4/8/16. Each process pass has30 unique cases in ONE family: E030/30 ready, H0/H126/30 ready plus4 requires_selection, L0/L123/30 ready plus3 no_match and4 unsupported. All configurations tie on H1; declared development tie-break selects256/k4, not a default. H1 coverage86.7percent is below the proposed95percent target even here; no adoption/whole-task/generalization claim. Independent result review approved15eb7a1 with all71 hashes and12 reports/selection reproduced byte-identically; .35 records zero-finding no-op. Full-candidate approval covered1401039 against68eac7c; PR76 rebase landeddbab099 with exact reviewed-tree parity and successful fresh-main reproduction of2520 records,12 reports and selection. T-PREP.30-.38 are complete for this restricted development slice, without closing .9 or adoption gates. See [controlled development receipt](receipts/2026-10-16-controlled-inbox-development.md).

- [x] T-PREP.30 Freeze controlled inbox development qualification  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  blocked-by: [T-PREP.29]  acc: [Secondary development-only protocol above freezes scope, complete artifacts, caller menu and independent blind labeling, single-family estimand, grid/selection rule and bounded runtime before scores; original held-out and adoption gates preserved]
- [x] T-PREP.31 Qualify fixed inbox catalog and caller-policy menu  Owner: cohort-author  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.30]  acc: [Actual fixed exported catalog and complete immutable skill/composite capability closure are pinned independently of queries;30 explicitly synthetic caller-menu intents bind exact policy scope and catalog SHA before labels; reviewed artifacts are executable by landed driver without skill-ID hints or gold-derived admission]
- [x] T-PREP.32 Independently reconcile positive inbox development cohort  Owner: cohort-author-with-independent-annotator  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.31]  acc: [30 ready positive menu cases retain one conservative construction-family cluster and fixed source fixture; two independent annotations reconcile complete skill/tool/capability closure and rubric before scores; public hashes/provenance clearly exclude negative coverage, held-out generalization and live Gmail]
- [x] T-PREP.33 Verify inputs and run bounded development grid  Owner: evaluation-runner  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.32]  acc: [Real pinned local BF16 vectors256/768 and k4/8/16 exercise all local arms with complete all-attempt records, separate embedding/source timing and memory, explicit unavailable provider/usage and deterministic post-selection grading; selection rule applied without gold edits or held-out access, all failures retained]
- [x] T-PREP.34 Independently review controlled development evidence  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PREP.33]  acc: [Non-author reproduces report from exact frozen records and checks caller-policy/data independence, one-family inference limits, grid parity, failures and no unsupported adoption claims; stable findings recorded]
- [x] T-PREP.35 Correct controlled evaluation findings  Owner: cohort-author-with-runner  Est: TBD  kind: agent  stage: fix  blocked-by: [T-PREP.34]  acc: [Accepted artifact/analysis defects corrected without gold retuning; affected checks and explicit superseding run identities retained, zero-finding no-op recorded]
- [x] T-PREP.36 Re-review corrected controlled evidence  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-PREP.35]  acc: [Independent exact-artifact approval covers frozen inputs, all-attempt results and disclosed limitations after fixes]
- [x] T-PREP.37 Merge reviewed controlled development artifacts  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-PREP.36]  acc: [Exact approved candidate merges through guarded rebase under unchanged policy without required-check bypass]
- [x] T-PREP.38 Verify landed controlled evidence  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-PREP.37]  acc: [Fresh main matches reviewed artifacts and deterministic report reproduction verifies hashes/results; no new inference, held-out result, adoption, Zati integration or release is inferred]

#### Native Go embedding runtime trial (2026-10-16)

Owner explicitly approved the proposed Go/native runtime path with ship after the Python reference and controlled cohort results. This admits an experimental native trial and source delivery, not default semantic adoption, mailbox execution, release, installation or hosted expansion. Actual three implementation/qualification outcomes .40-.42 under default ceiling3: one retained Gist adapter/runtime qualifier and one isolated Zati integration owner, with separate inherited independent reviewer. Coordinator owns plan/receipts and producer-first landing; current attended executor, no model/effort overrides, token/cost telemetry unavailable. Preserve all existing adoption and authority gates. Zati has no current model-process supervisor; use an explicit `embedding probe` command with one start/readiness/bounded synthetic embedding/shutdown lifetime, rather than starting an unused server from normal serve. No UI changes. Zati source scaffolding may proceed against the frozen adapter contract while qualification runs; positive composed qualification still requires .41 and .43.

Freeze upstream llama.cpp v0.7.0 commit620d2225cc436ab0b001e3200e590e3b286ef9ab and BF16 GGUF revisionbfcd298762cc34d0357ece5ebdd31791a3a374d8, file SHA25668bae29d62fb8c7d23e98d21fd4662753ddd636e6b62b8a70dfe059a9844f216 (557950176bytes). Downloads total<=2GiB, build/runtime artifacts<=10GiB,4CPU threads/build jobs,24GiB charged-memory limit/no swap,1200seconds per inference/build command and45minutes aggregate qualification. Use existing qualified Linux host first, pinned Python reference and fixed35 corpus embedding inputs; no old held-out/private/provider input or paid infrastructure. Mac runtime requires its actual current capacity/build lease and separate evidence. Separately admit bounded Mac CPU-only source/probe verification after Linux parity: fresh load<=10 and owned build lease,4jobs/threads,10GiB external artifacts,4GiB aggregate owned-process RSS watchdog and180seconds per probe/1200seconds build. Mac has16GiB total RAM and no per-job cgroup/no-swap isolation; report RSS enforcement and observed pressure honestly, stop below2GiB available headroom, never change system swap/signing/Keychain. Reuse identical pinned public source/GGUF; scoped pinned Go1.26.2 download<=200MiB if absent. A Mac probe alone does not claim full Mac tokenizer/vector parity; any such parity must use the same frozen39 inputs and thresholds against retained reference data without installing Python inference. Pin actual binary hash after qualified deterministic build; do not trust an alias echoed by server as attestation.

Gist owns a caller-selected native HTTP client implementing existing Embedder, never subprocesses or downloads. Restrict initial adapter to explicit numeric-loopback HTTP, no proxy/redirect/retry, context/deadline and bounded bodies/batches/text. Query/document prefixes are applied exactly once; tokenization and input framing must match qualified native route. Refuse overflow/silent truncation. Require exact input-index bijection, finite nonzero768-vectors and valid response shape; normalize768, or truncate256 then renormalize. ModelVersion binds binary/GGUF/config/prefix/pooling/projection/output identity supplied by trusted caller process custody; response alias is a consistency check only. Zati verifies explicit immutable executable/model hashes before launch, protects temporary API key material outside argv/logs, launches only an owned loopback child, confirms authenticated readiness against expected settings/child lifecycle, and terminates/reaps only that child on all exits. No port/name-based killing, automatic installation or hidden network fallback. Errors never include task text, key or response bodies. Protect qualified files from concurrent writers; hash checks are not filesystem isolation.

Native qualification is frozen BEFORE native outputs: exact tokenizer IDs on35 fixed query/document inputs plus special-token/Unicode/empty/overflow probes; native768 and normalized/truncated256 finite unit norms within1e-5 and cosine>=0.9999 versus corresponding Python reference. Match pooling/projection/prefix/EOS and ordering; reject mismatch rather than weaken thresholds or silently substitute another runtime. Rerun the frozen30-case cohort using Go adapter/native vectors and compare readiness/closure with Python results; retain failures and any downstream differences. Numerical parity is required before describing native behavior as reference-qualified. A mismatch blocks native qualification and dependent positive trial claims; source scaffolding may remain experimental with explicit failure. This does not qualify quantization, target-Mac performance or whole-task/adoption outcomes.

- [x] T-PREP.39 Freeze native runtime and Go adapter trial contract  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  acc: [User approved native optional-trial direction; pinned artifact/resource/parity/API/lifecycle contract above is frozen before build/inference/source implementation, ownership and existing adoption boundaries preserved]
- [x] T-PREP.40 Implement bounded Go native embedding adapter  Owner: native-runtime-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.39]  acc: [Existing Embedder receives explicit caller-owned loopback native endpoint and runtime identity; exact prefixes/token bounds/order/dimensions/normalization and auth/context/response limits fail closed with meaningful HTTP regressions, no SDK authority or automatic network/process change]
- [x] T-PREP.41 Qualify pinned native runtime against Python  Owner: native-runtime-implementer  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.39]  acc: [Actual pinned binary/GGUF identities, enforced bounds, tokenizer/vector/corpus parity and all failures are recorded reproducibly against frozen Python reference before positive qualification; no threshold relaxation or unsupported Mac/adoption claim]
- [x] T-PREP.42 Implement explicit Zati-owned native probe lifecycle  Owner: Zati-integration-owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.39]  acc: [Explicit private config and synthetic probe start exact trusted child, authenticate bounded readiness, exercise real Gist native Embedder and shut down/reap on every outcome; negative integrity/occupied-port/cancellation/foreign-response tests and actual bounded native probe pass; spec/dependency/entrypoint ownership follows Zati guidance, default serve and mailbox permissions unchanged]
- [x] T-PREP.43 Verify composed Go/native trial  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.40, T-PREP.41, T-PREP.42]  acc: [Actual Go adapter and Zati probe exercise qualified native process plus lifecycle denials, affected tests/races/vet/lint/spec/dependency checks pass with baseline distinctions; producer/consumer candidate identities and platform evidence retained]
- [x] T-PREP.44 Independently review native trial source and parity  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PREP.43]  acc: [Non-author exact base/head review covers both repositories, trusted process/model identity, input fidelity, runtime limits, all parity artifacts and source tests; reachable findings recorded without approval while blockers remain]
- [x] T-PREP.45 Correct accepted native trial findings  Owner: native-runtime-with-Zati  Est: TBD  kind: agent  stage: fix  blocked-by: [T-PREP.44]  acc: [Accepted defects receive focused corrections and affected checks, retained failures and immutable superseding evidence; no-op only after explicit zero-finding review]
- [x] T-PREP.46 Re-review exact corrected native trial  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-PREP.45]  acc: [Independent exact-head approval covers both corrected repositories and current authentic parity/check evidence]
- [x] T-PREP.47 Merge reviewed native producer then consumer  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-PREP.46]  acc: [Guarded rebase merges exact approved Gist source first, then refreshes Zati immutable dependency pin with affected checks/re-review before its guarded merge; no required-check bypass]
- [x] T-PREP.48 Verify landed native Go trial  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-PREP.47]  acc: [Fresh producer/consumer main matches reviewed trees and actual affected Go/native probe checks pass with exact artifacts and owned-process cleanup; no installed/default/release/production or full-evaluation implication]

#### Controlled inbox followthrough — approved 2026-10-16

The owner approved diagnosing the four unresolved cases, completing the controlled Zati inbox path and measuring full-attempt discovery overhead before any adoption. Fresh bases: Gist23ba27a, Zatifab1de07. Read-only diagnosis found four exact RRF60 rank-swap ties (menu14/28/29/30), not numerical or authority failures. Preserve generic requires_selection and every frozen policy/gold/result. Zati already resolves an explicitly selected immutable inbox skill and exact separate list/read mappings before model dispatch. Reuse that existing path; do not introduce an unused pinned-preparation API or substitute the older composite-tool fixture for Zati compatibility.

Current attended engineering execution, inherited settings, no model overrides. Default ceiling3, three outcomes .50-.52: Gist diagnostic report; Zati real controller/model-fixture/Effects/TLS-MCP/final-reply slice; paired full-attempt measurement. Two retained owners have disjoint repos/write sets; coordinator owns plan and receipts; independent reviewer covers exact full candidates. Actual source/runtime evidence and blockers remain separate. Model/token/cost telemetry for agent work is unavailable; no acceleration claim. Producer diagnostic and consumer source have separate reviewed merge boundaries; any actual Gist dependency change waits producer landing and serialized consumer pin/lock checks.

Acceptance freezes complete exact skill/template/schema context, caller-selected workflow information, synthetic mailbox state, identical allowed list/read operations (maximum20 messages), executor/settings/rubric and per-attempt budgets across eligible comparison arms. Include equivalent direct-tool configuration as a strong baseline; the current selected Zati path already avoids model discovery. Any controlled meta-discovery arm is explicitly synthetic, never current Zati or Composio. Measure every model call/turn, input/output/cached tokens where actual usage exists, recovery, failures and elapsed time through stored final reply. Fixture-script counters or schema bytes are not real-model/billed-token evidence. Unknown usage/cost remains unknown. Freeze cases/rubric/arm order/run manifest before actual main-model outputs, keep raw traces private and publish redacted source-bound records. A bounded development smoke cannot establish held-out quality, adoption or general superiority.

Owner selected local inference only and subsequently explicitly approved starting cached pinned Qwen for this synthetic test, at most60 main-model requests,10minutes per attempt and one-hour total model-service lifetime, followed by stopping only the service started here. No downloads or paid calls. The documented6CPU/84Gi profile does not fit last reported free allocation; admit one NEW bounded4CPU/80Gi trial profile with exact target-only model arguments/revision/template preserved except pod-local loopback binding. This is not historical runtime qualification. Before launch require reviewed exact manifest/driver/watchdog, fresh capacity plus actual host memory/swap observations, full trusted cached loader-file integrity, same exact cached model/sidecar/infra image evidence, completed benchmark binary and frozen balanced cases/settings/rubric. No published ports, restart Never, per-container and external one-hour deadlines, exact owned cleanup, no tenant interruption or automatic retuning/restart after failure. Missing cached assets stops; retain actual no-download evidence without claiming unavailable atomic engine pull-never. The first owned startup failed before loading on absent ancillary .gitattributes, with zero inference and confirmed owned cleanup. Admit a reviewed preloader correction separating fourteen required loader files from eight ancillary documents, preserving all required full hashes/index closure/custom-code refusal and verified-only overlay. A corrected launch needs explicit coordinator go, fresh gates and the ORIGINAL 2026-10-17T07:20:47Z absolute deadline; no new hour, model retuning or erased failure. Actual missing/mismatched required assets still stops. No paid profile or embedding model substitutes for Qwen. Startup failure, missing assets or unqualified route remains explicit NOT_RUN rather than a fabricated comparison. Gmail/Composio calls, private mailbox data, new model downloads, default changes, installation, release, hosting and deployment are excluded. Existing adoption tasks .9-.20 remain intact. Deterministic controller/provider fixtures and independent source work proceed under the recorded scope; real-model execution waits current checks, reviewed launch mechanics and fresh runtime gates. Unavailable arms remain NOT_RUN with explicit reasons, never wins.

Joined-path RED and independent source diagnosis confirmed two existing production defects: INBOX-R1 reuses the entire turn ID as every external proposal Effect source, so a distinct second action conflicts; INBOX-R2 loses artifact purpose when publishing MCP request-audit plus result, leaving no unique result for subsequent model context. Admit the same serialized Zati integration owner to repair scoped execution interpret/effects preparation/controller delivery code and focused regressions, plus a separate authored spec maintenance assignment in tools/specgen/packages.py with rendered/checkable generated prompts. Preserve immutable per-proposal replay/conflict, callback-turn worker ownership, trusted unique tool_result selection and other adapter behavior; no authority relaxation, fixture result injection, public shared schema or root dependency change is admitted. Actual list→read→stored-reply acceptance must pass through these real owners. Existing claims are preserved; narrower execution inbox-loop claim is held. Retained race verification subsequently exposed INBOX-R6: an external effect can become visible before its proposal index and be incorrectly journaled as a model callback. Admit a scoped controller routing/turn-discovery/journal recovery repair with deterministic and race regressions. Require exact authoritative operation ownership, defer unresolved callbacks and recover persisted misroutes without repeating physical effects; preserve public contracts and authority fences. Model launch remains held until corrected source checks and independent review pass.

Source verification uses existing Linux capacity after a fresh preflight, tracked-only SHA-verified archives, four CPU/build threads,24GiB/no-swap bounded units,600-second command and1200-second aggregate deadlines, at most two heavy lanes and one race suite. No Mac heavy build is admitted in this batch. Use existing module/tool caches and no credential/env-store transfer. Diagnose with stdlib only. Temporary fixture servers/processes/data are owner-scoped and cleaned up on success/error/cancel. Any live model run needs its separate current capacity and exact manifest before execution.

- [x] T-PREP.49 Freeze controlled inbox followthrough contract  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  acc: [Owner-approved scope, original frozen evidence, exact-tie diagnosis, existing selected Zati route, baseline fairness, synthetic-only authority and real-model pending gate are explicit before writes or inference]
- [x] T-PREP.50 Reproduce and explain four generic preparation ties  Owner: diagnostic-owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.49]  acc: [Stdlib source/hash-bound diagnosis reproduces four exact rank-swap ties and retained abstention across configurations without labels, inference or frozen-data edits; no forced selection or new unused public API]
- [x] T-PREP.51 Complete controlled two-role Zati inbox workflow  Owner: Zati-integration-owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.49]  acc: [Existing selected immutable skill path traverses real controller, bounded model fixture, current Effects/TLS-MCP list and read tools and stored final briefing; exact context, source grounding, missing/stale/revoked authority, adversarial mail, forbidden-effect refusals and cleanup are meaningfully checked without live Gmail or changed defaults]
- [x] T-PREP.52 Measure paired full-attempt overhead with honest eligibility  Owner: Zati-integration-owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.51]  acc: [Frozen fair development manifest and harness retain full attempts/turns/recovery/latency and actual usage or explicit unknowns, include equivalent direct-tools baseline with no forced discovery and disclose message-packaging differences; actual main-model runs require owner route/budget authority, unavailable arms recorded NOT_RUN, fixture counts never substitute for model token or superiority evidence]
- [x] T-PREP.53 Verify controlled inbox followthrough outcomes  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.50, T-PREP.51, T-PREP.52]  acc: [Diagnostic reproduction and actual end-to-end controlled path pass on exact source with affected tests/races/vet/lint/spec/dependency gates and baseline distinctions; comparison provenance, all attempts and pending actual-model evidence remain explicit]
- [x] T-PREP.54 Independently review inbox slice and measurement  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PREP.53]  acc: [Non-author reviews exact full producer and consumer candidates including plan, source, safety/fairness/counters and evidence; concrete reachable findings and actual versus unavailable comparisons recorded]
- [x] T-PREP.55 Correct accepted inbox followthrough findings  Owner: implementers  Est: TBD  kind: agent  stage: fix  blocked-by: [T-PREP.54]  acc: [Accepted findings corrected with retained failure and affected checks, or explicit zero-finding no-op; no threshold/gold/baseline manipulation]
- [x] T-PREP.56 Re-review corrected inbox followthrough heads  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-PREP.55]  acc: [Independent approval binds exact corrected source, measurement eligibility and honest evidence limits in both repositories]
- [x] T-PREP.57 Merge reviewed controlled inbox followthrough  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-PREP.56]  acc: [Exact approved PR heads guarded-rebase merge against fresh bases under unchanged repository policy, with actual dependency order where relevant and no required-check bypass]
- [x] T-PREP.58 Verify landed controlled inbox followthrough  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-PREP.57]  acc: [Fresh mains match reviewed trees and bounded actual diagnostic/end-to-end acceptance pass; route/budget blockers or unknown model usage remain explicit, no installation/release/adoption/provider or production implication]

#### Wave 15: Evaluation and adoption decision

- [ ] T-PREP.9 Run frozen offline retrieval and preparation evaluation  Owner: evaluation-runner  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.8, T-PREP.29, T-COMPOSE.21]  acc: [Development selects one primary configuration before held-out access; pinned offline runs produce complete records, family-clustered uncertainty, strata, abstention, coverage, memory and cold/warm metrics; unavailable or inconclusive evidence is explicit and no live-provider advantage is claimed]
- [ ] T-PREP.10 Admit bounded model and provider comparison  Owner: owner-with-coordinator  Est: TBD  kind: human  stage: preflight  acc: [Run manifest records explicit authority, same executor/catalog pairing, allowed read-only account and operations, enforceable call/dollar/expiry limits, host capacity, retention and stop conditions before any paid or private-data run; alternatively record offline-only disposition and unavailable arms as NOT_RUN without inventing account or spending authority]
- [ ] T-PREP.11 Run paired whole-task and Composio comparisons  Owner: evaluation-runner  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.9, T-PREP.10]  acc: [Frozen tasks repeat3times per arm with identical model/settings/executor/state/budgets where possible; full attempt tokens, latency, cost and success include preparation and recovery; C0 native discovery and C1 preregistered direct tools are preserved; confounds, unavailable arms, failed runs and paired denominators are disclosed; an offline-only disposition records unavailable comparisons NOT_RUN and forwards available evidence for review, not a fabricated completed run]
- [ ] T-PREP.12 Independently review results and reproducibility  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PREP.11]  acc: [Non-author reproduces aggregate report from source-bound redacted records, checks blind grading, family split, baseline fairness, uncertainty and all proposed gates; accepted findings trigger corrected analysis and fresh exact-artifact review without retuning test data]
- [ ] T-PREP.13 Decide adoption or retain current default  Owner: owner  Est: TBD  kind: human  stage: preflight  blocked-by: [T-PREP.12]  acc: [Explicit evidence-bound decision selects lexical preparation, hybrid, scoped use or no change; unsupported superiority claims rejected, remaining uncertainty and operational budgets recorded; no default is changed by a benchmark alone; offline-only evidence may conclude retain current default, while positive adoption requires adequate whole-task evidence and no unavailable arm supports superiority claims]

#### Wave 16: Conditional consumer delivery and operations

- [ ] T-PREP.14 Integrate selected preparation into actual Zati path  Owner: Gist-with-Zati  Est: TBD  kind: agent  stage: implement  blocked-by: [T-PREP.13, T-CATALOG.6]  acc: [Only the approved outcome is integrated through Zati-owned current bindings and context assembly; missing Gmail remains explicit, complete assets and exact schemas survive, runtime selection is caller-owned and lexical rollback changes no permissions; if no adoption is selected record non-dispatch disposition rather than fabricated implementation]
- [ ] T-PREP.15 Verify selected consumer and performance bounds  Owner: consumer-verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.14]  acc: [Actual Zati application tests prove prepared context and existing effect admission, unavailable-model fallback, stale-index/revocation denials and measured target memory/startup/latency bounds; source and live Gmail evidence remain distinct]
- [ ] T-PREP.16 Independently review consumer integration  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-PREP.15]  acc: [Non-author exact base/head review covers both changed repositories, defaults, permissions, measured claims and rollback; accepted findings have tracked correction/check/re-review before merge approval]
- [ ] T-PREP.17 Merge approved consumer integration  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-PREP.16]  acc: [Exact reviewed heads merge in dependency order with repository policies and all blocking findings resolved]
- [ ] T-PREP.18 Verify landed consumer preparation  Owner: consumer-verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-PREP.17]  acc: [Fresh producer/consumer main and affected runtime checks bind the reviewed revisions, including actual fallback and current authorization; no installed acceptance inferred]
- [ ] T-PREP.19 Release selected preparation under explicit authority  Owner: release-owner  Est: TBD  kind: agent  stage: release  blocked-by: [T-PREP.18]  acc: [Separately authorized immutable library/consumer releases identify model/index/recipe compatibility, integrity, target runtime, upgrade and rollback; no hosted deployment or public package trust expansion is inferred]
- [ ] T-PREP.20 Observe actual selected consumer and close decision  Owner: consumer-verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-PREP.19]  acc: [Version-bound installed consumer observations cover cold/warm latency, memory, misses, invalidation, fresh revocation and exercised lexical rollback over a predeclared observation window; independently reviewed report distinguishes actual Gmail acceptance from controlled checks]

## Non-authoritative source archive

Only docs/plan.md is authoritative. Former split-plan paths now hold pointers; each original file is byte-preserved under docs/plans/archive/20261011/ with its source hash. Existing receipts keep working through the pointers, while task dispatch and current status resolve only through this canonical graph.

| Original source | SHA256 |
| --- | --- |
| [docs/plan.md](plans/archive/20261008/plan.md.source) | 79bbcb800c438431f59b85575ff83514f8641ac41bfa0090a9cafd606c1c98af |
| [docs/plans/E-GR-AUTH-enrollment-and-grants.md](plans/archive/20261011/E-GR-AUTH-enrollment-and-grants.md.source) | 03a25a2ca9f07cec205c77b5419077f9c1508f3ad7f330ca6edb89f1696acfec |
| [docs/plans/E-GR-AWS-aws-origin-and-infrastructure.md](plans/archive/20261011/E-GR-AWS-aws-origin-and-infrastructure.md.source) | 62109b991866804e749820fb3a2f2ef751df5fcfdda7cb8ef3aaed992489f826 |
| [docs/plans/E-GR-COMPOSIO-composio-action-artifacts.md](plans/archive/20261011/E-GR-COMPOSIO-composio-action-artifacts.md.source) | 8c084bf43c9e3592c301b6d53a002f21fe77fa76e7b1df29291b52f5eeab14b2 |
| [docs/plans/E-GR-CORE-startup-and-event-composition.md](plans/archive/20261011/E-GR-CORE-startup-and-event-composition.md.source) | 0f85fdc7cf24b4ce832b8fbc3b3708f8d1b6450ec5437d915b2e651837c88da2 |
| [docs/plans/E-GR-EVENT-durable-revocation-feed.md](plans/archive/20261011/E-GR-EVENT-durable-revocation-feed.md.source) | 5223897bddfb4a1dad8813f448936f03c7208ff1d32317f2ba5afea5a93e637a |
| [docs/plans/E-GR-GATEWAY-optional-managed-gateway.md](plans/archive/20261011/E-GR-GATEWAY-optional-managed-gateway.md.source) | 08ef5204636a1845d3d30bac62abcda77dfb2caf7d7fc2fdae14c1af12489ff3 |
| [docs/plans/E-GR-INTEGRATE-composition-and-interoperability.md](plans/archive/20261011/E-GR-INTEGRATE-composition-and-interoperability.md.source) | 67aa355220bd02c4910c62ed44b6bec97a139f984263ee9b5eb2d492ab2da77c |
| [docs/plans/E-GR-INTERFACE-shared-component-interfaces.md](plans/archive/20261011/E-GR-INTERFACE-shared-component-interfaces.md.source) | 5e1e41e3bba077c20c9a444f9919085e7d371f456cb6726710d98d6ef2299c29 |
| [docs/plans/E-GR-KEYS-persistent-signing-keys.md](plans/archive/20261011/E-GR-KEYS-persistent-signing-keys.md.source) | c50248ce4fced0b6ef006451bd3c539b58957b18cacc5c20edb7562273afc0b9 |
| [docs/plans/E-GR-M2B-oauth-preview-and-production.md](plans/archive/20261011/E-GR-M2B-oauth-preview-and-production.md.source) | 92a84ea8ab7269ad41c6310e5e72afddc4da51dc1dbb4cd0b73e0fe21c4b26d1 |
| [docs/plans/E-GR-M3-clients-runtime-and-production-metrics.md](plans/archive/20261011/E-GR-M3-clients-runtime-and-production-metrics.md.source) | 29e8db76a90d68184619fe416f7bf1445eec51159be5a6e954dad8fe8721deb6 |
| [docs/plans/E-GR-NEXT-remaining-registry-milestones.md](plans/archive/20261011/E-GR-NEXT-remaining-registry-milestones.md.source) | 2b849d6760194dd9e1c3c5309e373d9df9aa30f794ff947c3fd846ce7fd6cfc1 |
| [docs/plans/E-GR-PROD-terminal-production-delivery.md](plans/archive/20261011/E-GR-PROD-terminal-production-delivery.md.source) | fb903764ba01f0de7100c9e7b6f0f6c51787c24e5052f76c67a1a61db821cc2d |
| [docs/plans/E-GR-PUBLISH-canonical-publication.md](plans/archive/20261011/E-GR-PUBLISH-canonical-publication.md.source) | b151eaf88c59c2ffd5d70961880cf425b760c1db8fa167fd34595f63d9ba3dbe |
| [docs/plans/E-GR-RELEASE-release-acceptance-and-operations.md](plans/archive/20261011/E-GR-RELEASE-release-acceptance-and-operations.md.source) | bd6d7099349a98d21bf53194e7da79c1e9ad4661d0f6552254b1fd8544d37dd2 |
| [docs/plans/E-GR-SCOPE-requirements-and-preflight.md](plans/archive/20261011/E-GR-SCOPE-requirements-and-preflight.md.source) | 0f0749ec3975af8ee98a4635494f12e79ebedc5e061b0a7349db73adbc1780f6 |
| [docs/plans/E-GR-TREG-treg-action-artifacts.md](plans/archive/20261011/E-GR-TREG-treg-action-artifacts.md.source) | b3761ceb8750f7083f3b1d8552a289474278474f456d2ba6eacc8c0238edc1f5 |
| [docs/plans/E-GR-WIRE-canonical-registry-wire.md](plans/archive/20261011/E-GR-WIRE-canonical-registry-wire.md.source) | 2fc8d621a0bd2b56d42eb892ffe689d536339e6512300c91f9d43a1a3b250358 |
| [docs/plans/gateway-buildout.md](plans/archive/20261011/gateway-buildout.md.source) | 194e79f387bc10fb68bb2bd21cd0036460975532bd1cad4cf76822d725ef03e6 |
| [docs/plans/registry-buildout.md](plans/archive/20261011/registry-buildout.md.source) | cd554d10705c8f2da5a29bba8f9001e8103a4cc47c9cd55ea3d111a4705bf85e |
| [docs/plans/registry-recovery.md](plans/archive/20261011/registry-recovery.md.source) | e4bd42c96843fa3975fb1a5ad4ab91801b44ba0240b01b510b4a2571e5be274e |


Publication final review assignments (PUBLISH.4): three independent nonauthor Luna leaves examine the same exact candidate/base read-only. The lead covers full diff, plan, configuration and end-to-end wiring with app/REST/authority focus; the decoder reviewer covers strict grammar, schema/ZIP/evidence/digest and source-lock compatibility; the storage reviewer covers SQL/RLS/transactions/idempotency/owned object operations/cleanup and direct consumers. Coordinator owns finding dispositions and fixes; all scopes require exact final-head approval before PUBLISH.5. Their worktrees are isolated and their start turns use the shared fair dispatch bucket. PUBLISH.1/2/3 source completion does not imply review or landing. Five preexisting full-hosted lint findings are retained and compared against the unchanged base; no introduced finding is accepted as baseline.

Publication source closure: PR60 actual landed eb1bd86596b61bd423cc3f75757a2bdd38682c24 is reachable from fresh remote main and exactly matches final reviewed tree e3211491965e3c5d18cd302de639b4933cb3de44. Actual landed HTTP/store and wire/plan gates pass. PUBLISH.1-.6 and accepted .15-.20 recovery are now source-complete; this does not activate INTEGRATE, AUTH, TREG, COMPOSIO or production. Readiness scan finds remaining externally blocked enrollment/custody, live environment/provider numerical authority, consumer receipt and AWS/DNS binding preflights; no unblocked coding task is skipped in favor of a production claim. Default AWS STS has no credentials; the DGX resolver cannot resolve gist.sire.run. Retained original check failures and negative reviews stay linked in source verification and landing receipts.


### E-MARKET -- Usable Gist-backed marketplace in Zati

fidelity: executable
Acceptance: A human browses the module-pinned Gist inbox skill in Zati, inspects its requirements and actual connected tools, explicitly chooses list/read roles without hashes or JSON, and stages reviewed activation and worker assignment. Missing connections, stale choices and unsupported worker composition produce actionable refusals. Gist supplies package content; Zati retains all connection, policy, approval and execution ownership.

Owner ship invocation 2026-10-16, batch-size5, resumes this bounded library-consumer usability slice. Scope follows the accepted RFC-002/ADR016 direction: Zati marketplace first; hosted expansion remains paused. When asked about hosted resumption, the owner requested the existing plan/RFC/ADR position; no change to that direction was given. This source delivery authorizes implementation, independent review, merge and landed checks, not publication, deployment, mailbox calls or paid providers. Marketplace here means discovery/distribution; commerce and seller payments remain undecided. Existing T-CATALOG release/operational gates and T-PREP evaluation tasks remain unchanged.

Selected contract: additive authenticated installation-scoped skill.catalog.list and human-only phased skill.catalog.import. The catalogue comes from the actual module-pinned maintained inbox package with complete manifest and previews. Import accepts exact displayed package pins, current connection/tool identities and explicit read-semantics acknowledgment. The Skills owner derives schema hashes from the Connections owner and rechecks selections at commit; clients never compute pins or infer semantics. Existing skill.import remains compatible. One reviewed configuration draft contains skill activation, explicit bindings and worker assignment. Existing skilled workers are refused for this first setup flow rather than overwritten. No fake hosted source, duplicated registry, arbitrary package fetch, provider credential custody or authority grant is introduced.

Parallel Work: current attended executor, explicitly selected engineering work profile; existing desktop session tools retained. Requested ceiling5, actual4 member outcomes T-MARKET.1-.4 in one coherent cross-component candidate; one inherited implementer owns all Zati source/specification/UI and coordinator owns Gist plan/receipts. Independent reviewer is a separate inherited session after verification. No model/reasoning override. Exact model identifier, token/cost and stage timing telemetry unavailable unless exposed by the harness. Shared source/API ownership serializes implementation within this batch; no artificial parallel edits. No new infrastructure. Qualified Linux Go checks use existing bounded host, native Flutter checks/preview use Mac capacity and build lease. Preserve the unrelated running Flutter preview.

| Batch | Members and outcome | Dependencies | Verification and gates | Split condition |
| --- | --- | --- | --- | --- |
| Zati marketplace | T-MARKET.1/.2/.3/.4; catalogue admission, browse/role selection, reviewed activation/recovery, end-to-end qualification | T-MARKET.0 and landed T-CATALOG.6 | .5 independent review; .6 corrections/re-review; .7 guarded merge; .8 landed checks | Unresolved hosted scope, unsafe contract ownership, external provider prerequisites or impractical review size |

- [x] T-MARKET.0 Freeze bounded marketplace contract and ownership  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  blocked-by: [T-CATALOG.6]  acc: [Fresh Zati aabfdc5e and Gist d929055 sources, current claims, service/configuration/review/recovery interfaces and selected additive operations inspected; one source owner and complete-member write scope recorded; no existing dirty work or unrelated preview overwritten]
- [x] T-MARKET.1 Expose maintained catalogue and server-derived admission  Owner: marketplace-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-MARKET.0]  acc: [Public authenticated catalogue lists real pinned package previews/requirements; human exact-scope import derives actual schema pins through Connections, reuses bounded package LocalIO and ordinary draft staging, rechecks freshness at commit, and rejects wrong pins/scope/actor/connection/stale or duplicate tools; authored contracts/generated mirrors remain consistent]
- [x] T-MARKET.2 Deliver browse and explicit role-selection experience  Owner: marketplace-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-MARKET.1]  acc: [Zati Services exposes searchable maintained skills and connected tools with clear descriptions/requirements/schema inspection; user chooses exact connection/list/read roles and acknowledges read semantics without JSON/hashes; empty/missing/incompatible controller or provider states give a usable setup path]
- [x] T-MARKET.3 Deliver reviewed activation and recoverable worker assignment  Owner: marketplace-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-MARKET.2]  acc: [One configuration draft stages bindings, imported skill and exact worker assignment preserving existing state; human can reach required review and explicitly apply; unsupported existing skill composition refuses; original commands, draft/ref/stage progress survive uncertain acknowledgment/reopen and reconcile without automatic duplicate import or unauthorized apply]
- [x] T-MARKET.4 Qualify actual marketplace path and rewrite setup guide  Owner: coordinator-with-implementer  Est: TBD  kind: agent  stage: verify  blocked-by: [T-MARKET.3]  acc: [Actual assembled public operations import/draft/review/apply/context checks and Flutter wire/widget success plus empty/stale/refusal/recovery cases pass; spec renderer/Python tests/dependency lock/affected Go tests/vet/lint and Dart format/analyze pass or independently verified unchanged baseline findings remain explicit; Mac rendered-widget visual review and attempted isolated native preview under available capacity, with unavailable native builds explicit rather than claimed as passed; complete plain-language user steps recorded separately from live Gmail/release/performance]
- [x] T-MARKET.5 Independently review complete marketplace candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-MARKET.4]  acc: [Non-author exact base/head review covers both repository diffs, server/client trust, LocalIO freshness, draft/review recovery, real tests, guide and plan; concrete findings and dispositions recorded]
- [x] T-MARKET.6 Correct findings and obtain exact-head approval  Owner: marketplace-implementer-with-coordinator  Est: TBD  kind: agent  stage: fix  blocked-by: [T-MARKET.5]  acc: [Accepted blockers receive tracked fixes, affected checks and independent exact-head re-review; no author self-approval or invented CI/provider evidence]
- [x] T-MARKET.7 Merge reviewed source and delivery record  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-MARKET.6]  acc: [Current policy and base/head rechecked immediately before guarded GitHub rebase merge, immutable candidate and merge SHAs recorded; enforced required checks remain binding; billing fallback never changes protection]
- [x] T-MARKET.8 Verify landed marketplace source and handoff  Owner: coordinator  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-MARKET.7]  acc: [Fresh main reaches exact reviewed source trees, affected consumer behavior/spec/plan checks pass and local Zati main reconciles without discarding authored work; final handoff gives usable UI steps and honest missing-provider/release/installed qualification state]

Material limitations: only the maintained inbox skill is qualified initially. Gmail MCP service provisioning/OAuth and live mailbox acceptance are separate; existing Services setup remains the connection path. Public publisher trust, commerce, model evaluation, library/consumer release, installation and hosted rollout are outside this batch. A UI may report assigned/configured but cannot claim a working mailbox or ready execution without actual current authorization and provider evidence.

Marketplace source/check evidence: [2026-10-16 marketplace delivery](receipts/2026-10-16-zati-marketplace.md). Non-author exact-head review approved source; Zati PR42 landed aa379b6dbc3da65f5a8ae896a1984524a8497e11 with identical reviewed tree and passing landed API/context/spec checks. The documentation record follows its own reviewed merge/landed verification. Installation, full native build and a live mailbox remain unqualified.

### E-COMPOSE -- Reusable capability composition and scale
fidelity: executable
Acceptance: New immutable atomic inbox contracts compose safely in the Gist
library and Zati; exact legacy behavior remains available; independently reviewed
synthetic 10k/100k results establish the measured envelope with no unsupported
provider, model, adoption, installed-runtime or production claim.

Authority: ordinary repository delivery; executor current/attended, inherited
session selection and independent review. Go, Python, Git and gh are available;
actual compiler/host capacity, build lease, dependencies and checks are qualified
at implementation preflight. No Kazi/Foundry route is admitted by this plan.
Requested scope is design and planning documents now; every task below remains
pending implementation. Scope references are ADR021 and RFC-002 §19. Consumer
rows name Zati deliverables and landing evidence here, not a second executable
scheduler for any enrolled Zati lifecycle; consumer preflight reconciles the
owning repository's contract before dispatch.

Use cases: UC-COMP-1 selected inbox preparation with reusable list/read contracts;
UC-COMP-2 safe exact resolution under missing/ambiguous/stale bindings;
UC-COMP-3 discover skills from a large authorized catalog within explicit budgets;
UC-COMP-4 migrate old admitted inbox skills without changing immutable records.
Manual discovery inspected Gist resolution/preparation/package APIs and Zati
skills admission/execution consumers. Existing inbox safeguards are reused.

Batches are outcomes, not task-count quotas. Producer precedes consumer. Index
work can follow producer landing independently of consumer migration. Independent
review covers all implementation and verification rows of its named batch on an
exact base/head. Findings create appended fix, verification and re-review rows;
merge never passes an unresolved review. Preserve historical IDs/evidence.

| Batch | Rows | Observable result |
| --- | --- | --- |
| Producer | .1–.7 | Strict additive composition API with immutable fixtures and compatibility |
| Consumer | .8–.14 | Zati accepts new skill versions through ordinary authority/effects |
| Index source | .15–.16, .29–.32 | Independently delivered bounded discovery API |
| Scale and evaluation | .17–.21 | Frozen reproducible scale and composition evidence |
| Operational handoff | .22–.25 | Explicit release, installation, acceptance and support gates |

Shared verification: Gist focused resolution/preparation/package and new CLI
fixture checks, affected Python report tests, and repository-required checks;
Zati focused skills/execution/controller plus joined list/read workflow/race,
revocation/schema-drift cases and specgen checks. Preflight freezes exact commands
and required environment from current CI and maintained receipts. Documentation
uses Wazi reader/schema/semantic validation and link/history checks. Synthetic
scale workloads are new data; original sealed held-out inputs stay closed.
Numerical latency/RSS/index-size targets are frozen in .15 from measured baseline
and target-host capacity before scoring, never invented here or tuned afterward.

- [x] T-COMPOSE.1 Freeze producer schemas, API and verification contract  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  acc: [ADR021 requirements become canonical composition and capability schemas with digest rules, statuses, limits, compatibility cases and exact verification commands; no provider calls or wire changes; requirements and design reviewed before coding]
- [x] T-COMPOSE.2 Implement typed composition validation and exact resolution  Owner: gist-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-COMPOSE.1]  acc: [Strict bounded DAG and JSON Pointer validation, capability-indexed alternatives, connection-group intersection, per-capability and composition evidence, scoped freshness and complete-schema budgets implement UC-COMP-1 and UC-COMP-2 without Cartesian search or authority grants]
- [x] T-COMPOSE.3 Publish local immutable fixtures and migration contracts  Owner: gist-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-COMPOSE.1]  acc: [New list/read contracts and inbox package/recipe fixtures have pinned digests and conformance examples; old bytes remain unchanged; this is repository fixture publication only, not registry upload or a module release]
- [x] T-COMPOSE.4 Verify producer behavior and compatibility  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.2, T-COMPOSE.3]  acc: [Current required checks plus missing optional/required capability, duplicate roles/keys, cycles, type mismatch, bounded iteration, partial results, same-connection/account mismatch, ambiguity, schema drift, revocation, cancellation and insufficient context pass; unsupported consumers and legacy paths covered]
- [x] T-COMPOSE.5 Independently review producer composition candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-COMPOSE.4]  acc: [Independent non-author reviews .1-.4 exact base/head with source and checks; findings/dispositions recorded and all blockers resolved]
- [x] T-COMPOSE.6 Merge reviewed producer candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-COMPOSE.5, T-COMPOSE.28]  acc: [Exact reviewed head rebases through GitHub with current base and required checks; no bypass and landed SHA recorded]
- [x] T-COMPOSE.7 Verify landed producer composition  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-COMPOSE.6]  acc: [Fresh main reaches reviewed tree and affected additive API, immutable fixtures and legacy checks pass]
- [x] T-COMPOSE.8 Freeze Zati migration and owning lifecycle bindings  Owner: zati-coordinator  Est: TBD  kind: agent  stage: preflight  blocked-by: [T-COMPOSE.7]  acc: [Current Zati contract and plan authority reconciled; exact producer revision, new skill version and typed readiness mapping selected; legacy fallback and verification recipes fixed without executing an enrolled lifecycle from this ledger]
- [x] T-COMPOSE.9 Implement generic Zati admission and recipe enforcement  Owner: zati-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-COMPOSE.8]  acc: [Remove inbox-only restriction for negotiated new version; validate all required capabilities and affinity, bind typed dataflow and enforce bounded actual calls/output; preserve ordinary permissions, schema pins, effect recovery and legacy list/read mapping]
- [x] T-COMPOSE.10 Migrate marketplace and consumer documentation  Owner: zati-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-COMPOSE.8]  acc: [Explicit new-version admission and assignment flow preserves old selections; incompatible consumers refuse readiness; user guidance explains connection setup, partial briefing, upgrade and rollback without claiming live Gmail]
- [x] T-COMPOSE.11 Verify joined Zati composition path  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.9, T-COMPOSE.10]  acc: [Actual scripted controller to tool dispatch to persisted briefing succeeds for new and legacy paths; mixed mailbox, denied/revoked tool, stale schema, hostile mail, missing capability and bounded partial failure refuse or report correctly; source/race/spec checks pass without claiming real-model/provider acceptance]
- [x] T-COMPOSE.12 Independently review consumer migration candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-COMPOSE.11]  acc: [Independent non-author reviews .8-.11 exact Zati base/head and producer pins with findings resolved and ownership boundaries intact]
- [x] T-COMPOSE.13 Merge reviewed Zati migration  Owner: zati-coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-COMPOSE.12, T-COMPOSE.38]  acc: [Owning repository permits exact-head guarded rebase with current checks and recorded landed SHA; no parallel lifecycle advancement]
- [x] T-COMPOSE.14 Verify landed Zati migration  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-COMPOSE.13]  acc: [Fresh consumer main verifies reviewed source, exact producer dependency and joined legacy/new behavior; installation and provider qualification remain distinct]
- [x] T-COMPOSE.15 Freeze indexed discovery workload and scale thresholds  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  blocked-by: [T-COMPOSE.7]  acc: [Synthetic 10k/100k mixtures, skew, candidate and fan-out budgets, target host, baseline, latency/RSS/update thresholds, independent labels and no-leak/false-ready invariants fixed before runs; resource limits and exact commands recorded without paid calls or held-out access]
- [x] T-COMPOSE.16 Implement bounded indexing and generation-aware retrieval  Owner: gist-implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T-COMPOSE.15]  acc: [Capability lookup and authorized shortlist indexes avoid per-request full schema/vector materialization; updates/rebuild/rollback, cache isolation/invalidation, cancellation and incomplete-budget outcomes preserve current policy and exact readiness; optional semantic route does not become default]
- [x] T-COMPOSE.17 Qualify new composition inventory and evaluation cohort  Owner: evaluation-author  Est: TBD  kind: agent  stage: implement  blocked-by: [T-COMPOSE.14, T-COMPOSE.15, T-COMPOSE.32, T-COMPOSE.41]  acc: [Independent fixed inventory and caller-policy manifest use new immutable skills and actual two-tool contracts; non-author labels reconcile before scores; primary configuration and split frozen; old composite pilot remains unchanged and original held-out access ungranted]
- [x] T-COMPOSE.18 Verify scale and composition evaluation evidence  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.16, T-COMPOSE.17, T-COMPOSE.32]  acc: [Declared synthetic 10k/100k runs report all attempts, latency percentiles, RSS/index/update cost, recall/coverage, false readiness, exposure, token availability and cold/warm phases; thresholds and failure dispositions retained; full safety and compatibility checks pass without post-score retuning]
- [x] T-COMPOSE.19 Independently review index and evaluation candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-COMPOSE.18]  acc: [Independent non-author reviews .15-.18 exact base/head, label independence, reproducibility, all failures and limited claims; blockers resolved before merge]
- [x] T-COMPOSE.20 Merge reviewed scale and evaluation candidate  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-COMPOSE.19]  acc: [Exact approved source and artifacts rebase through unchanged GitHub policy with landed SHA and check provenance]
- [x] T-COMPOSE.21 Verify landed scale evidence and publish handoff  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-COMPOSE.20]  acc: [Fresh main matches reviewed source and deterministic report regeneration; producer and consumer compatibility recorded; results state measured envelope and remaining bottlenecks, never adoption or provider acceptance]
- [ ] T-COMPOSE.22 Authorize composition release and installation scope  Owner: owner-with-coordinator  Est: TBD  kind: human  stage: preflight  blocked-by: [T-COMPOSE.14, T-COMPOSE.21]  blocked: Separate release and installation authority required  acc: [Explicit module/consumer release versions, target platform, signing/custody, rollout and rollback authority recorded; no semantic-default, hosted or provider authority inferred]
- [ ] T-COMPOSE.23 Release library and install approved consumer  Owner: release-owner  Est: TBD  kind: agent  stage: release  blocked-by: [T-COMPOSE.22]  acc: [Qualified release/installation binding publishes only approved artifacts, records signatures and dependency pins, verifies installed version and rollback; generic dispatch stays blocked until route and authority are qualified]
- [ ] T-COMPOSE.24 Accept installed workflow within approved provider scope  Owner: owner-with-verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.23]  acc: [Actual installed UI and workflow evidence recorded; synthetic-only versus live provider scope explicit; live inbox requires separate read-only account/call/cost/expiry authority, otherwise remains unqualified]
- [ ] T-COMPOSE.25 Record operational handoff and recovery ownership  Owner: maintainer  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.24]  acc: [Runbook covers index generations, cache invalidation, connection revocation, unsupported upgrades, rollback and incident ownership; installed evidence and unresolved provider limits recorded; no hosted rollout or semantic adoption implied]

#### Composition producer ship preflight — 2026-10-17

Owner invoked plan and ship with ceiling25. Actual first assignment is three
implementation/verification members T-COMPOSE.2/.3/.4 under preflight .1 and
review/merge/landed .5/.6/.7. One inherited current-session implementer owns the
producer worktree; coordinator owns plan/claims and independent review. Concrete
schema/API freeze precedes code and is recorded in docs/contracts/composition-v1.md.
External dependency: landed ADR021 at d013516. Consumer and scale batches await
producer landing. Source work may proceed while verification host is qualified;
no heavy local build under observed Mac load above10. Existing qualified Linux
source-check route is preferred; no model runtime or paid infrastructure starts.
Actual model/effort inherit this session, without overrides; token/cost telemetry
is unavailable. Split if contract incompatibility or check capacity prevents
complete acceptance; never mark pending rows complete to fit a batch.

T-COMPOSE.1 preimplementation freeze: independent composition_doc_audit approved
contract SHA256380a85c246b7bba5cb83bd829823542b382886dec613f17f4d9f60c6329e20a4.
Review corrected exact provider reference/pin semantics and added hard parsing,
schema and candidate ceilings before coding. This is design approval only;
source/check/review/landing rows remain pending.

Producer review remediation (2026-10-17): initial candidate5cb0491 is superseded
before aggregate acceptance. Fixture self-review found missing frozen capability
metadata; independent review found decimal-to-float integer validation could
accept fractional values. These are tracked corrections, not accepted delivery.

- [x] T-COMPOSE.26 Correct producer compatibility and safety findings  Owner: gist-implementer  Est: TBD  kind: agent  stage: fix  blocked-by: [T-COMPOSE.2, T-COMPOSE.3]  acc: [All accepted producer review findings have precise fixes; frozen schema conformance, exact numeric validation and any required package/runtime preservation corrections retain regression coverage and finding dispositions]
- [x] T-COMPOSE.27 Verify corrected producer behavior  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.26]  acc: [Affected regression checks and aggregate required candidate checks pass or retain explicit inherited failure comparison; exact head and source evidence recorded]
- [x] T-COMPOSE.28 Independently re-review corrected producer candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-COMPOSE.27]  acc: [Non-author approves exact corrected base/head covering all accepted findings and complete producer source before merge; no unresolved blocking findings remain]

Producer source checkpoint: 51e9785 implements the additive composition package
and new inbox v2 fixture; legacy bytes/wire formats remain unchanged. Corrections
cover frozen schema compatibility, exact bounded integer validation, retained
package instructions/assets and all-result context bounds. Focused Go tests and
two actual frozen-schema fixture checks pass; aggregate verification and final
independent approval remain pending. Source rows do not imply merged delivery.

Producer aggregate evidence at51e9785: qualified Linux Go1.26.1 full root
integration/race, vet/build/binary, frozen contracts, registry12 tests and tidy
pass; new fixture schema checks2 pass. Full lint remains failing on exactly the
same5 unchanged baseline findings, zero introduced. Independent source re-review
resolved COMPOSE-R2/R3/R4; exact final documentation approval and guarded landing
remain gates. See receipts/2026-10-17-composition-producer.md.

Producer delivery: independent composition_doc_audit approved final b91764e
against d013516; PR83 guarded rebase landed d0e97da with exact tree equality.
Fresh landed race tests for composition/new inbox/legacy inbox and fixture-schema
checks pass. Hosted CI could not start due existing billing lock; no required
status checks configured, unchanged authorized local fallback used. .1-.7 and
.26-.28 are source delivery complete, not release or consumer acceptance.

Next admitted assignments under ceiling25: consumer .8-.11 (three implementation/
verification members, one integration owner in isolated Zati worktree) and scale
preflight/index .15-.16 (one implementer, separate Gist worktree). They depend on
verified producer landing, now complete. Scale cohort .17 and aggregate .18 still
wait for consumer .14; index source may proceed independently. Coordinator owns
this canonical plan and its claim. Each concrete contract/preflight is reviewed
before implementation. Source checks use existing bounded Linux capacity;
consumer shared spec and root dependency updates serialize within its owner.
No new release, deployment, model or provider authority is supplied.

Index preimplementation freeze: coordinator reviewed composition-scale.md at
SHA2566594ae3f52311fc9d2c49daa2dba8bcf828f2e3e2bb175979f3c224dd82fdbe9.
10k/100k sizes are total logical entries, with private partitions and a separate
single-scope worst case. Numerical limits are engineering targets, not measured
baseline results. Copy-on-write top-level O(N) cost and current-policy rollback
semantics are explicit. .17/.18 remain gated on consumer landing and independent
cohort qualification; source tests do not establish scale performance.

Consumer preimplementation freeze: independent composition_doc_audit approved
Zati ADR019 digest e0570c8f0b55e7c8bb5bce4f8fa042886f63a5916f77636a21e340a1a2c6c4e1.
V2 initially requires identity-compatible canonical structured tool schemas;
arbitrary legacy MCP shapes stay v1 until a separately admitted typed adapter.
Private authenticated output evidence and transactional call/dataflow fences
are required, not model instruction compliance. Necessary Connections metadata
propagation exposes retained structured output schemas separately, preserving
legacy MCP envelope schema pins and conservative effect classification.

Index source is a separate coherent delivery boundary from independently authored
consumer evaluation data. Preserve .17-.21 unchanged in scope; add source-only
verification/review/merge/landed obligations and make cohort/scoring wait for
actual index landing. No workload results or acceptance thresholds are weakened.

- [x] T-COMPOSE.29 Verify bounded index source  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.16]  acc: [Focused lexical/exact, isolation, current policy, incomplete budget, ownership and concurrent publication tests pass on exact source; aggregate checks retain explicit comparison for unchanged inherited failures with zero introduced lint findings; no 10k/100k evaluation result inferred]
- [x] T-COMPOSE.30 Independently review bounded index candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-COMPOSE.29]  acc: [Non-author exact base/head review covers .15-.16 source and .29 evidence; concrete findings resolved without implying scale or consumer readiness]
- [x] T-COMPOSE.31 Merge reviewed bounded index source  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-COMPOSE.30, T-COMPOSE.35]  acc: [Exact approved source merges via guarded rebase under unchanged checks and policy, with landed SHA recorded]
- [x] T-COMPOSE.32 Verify landed bounded index source  Owner: verifier  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-COMPOSE.31]  acc: [Fresh main reaches reviewed tree and affected index/composition compatibility checks pass; .17-.21 qualification remains pending]

Index candidate16699bd source implementation is complete; aggregate checks found
a test fixture tagged its unrelated calendar document as mail, invalidating the
expected two-hit assertion, and one new De Morgan lint finding. Actual failure
is retained; verification/review/merge remain pending correction. All other
root suites/vet/build/contracts/registry/tidy passed; inherited5 lint findings
remain distinct from the new finding.

- [x] T-COMPOSE.33 Correct index source verification findings  Owner: gist-index-implementer  Est: TBD  kind: agent  stage: fix  blocked-by: [T-COMPOSE.16]  acc: [Incorrect lexical fixture and introduced lint finding corrected without weakening rank/isolation/budget behavior; failure retained and exact corrected head recorded]
- [x] T-COMPOSE.34 Verify corrected bounded index source  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.33]  acc: [Affected discovery race suite and lint delta pass; unchanged aggregate evidence remains source-bound, no scale result inferred]
- [x] T-COMPOSE.35 Independently re-review corrected index candidate  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-COMPOSE.34]  acc: [Non-author exact corrected head approval covers fixes and final plan/evidence, with no remaining blockers before merge]

Index correction e4697fe fixes the fixture without weakening ranking assertions
and removes the introduced lint finding. Exact-source discovery race tests pass;
full lint still exits1 with exactly5 inherited findings and zero introduced.
All unchanged passing aggregate checks remain tied to16699bd. Receipt:
receipts/2026-10-17-composition-index.md. Independent corrected-source review found
no blockers; final full documentation-head review and landing remain pending.

Index source delivery: independent composition_doc_audit APPROVED final
8e4c868 againstd0e97da; PR84 guarded rebase landedab6b6c6 with exact tree equality.
Fresh landed discovery/composition/new inbox/legacy inbox race packages passed.
Archive SHA256c943d75eccf793a00b29f8cdf8e54e4b2b5bf950b4f4ff424088e6169cdb8cf3.
Hosted checks remain unavailable under billing lock; repository protections were
unchanged. .15–.16 and .29–.35 are source delivery complete. No 10k/100k score,
consumer acceptance, release or deployment follows from this checkpoint.

Consumer WIP5dd18c8 compiled five owning packages but execution test compilation
failed on a changed internal helper signature. Early independent source review
also found exact-integer mapping rounding, oversized result recovery failure and
missing explicit unread/pending coverage. These are accepted corrections, not
consumer completion. Joined source verification and final approval remain gates.

- [x] T-COMPOSE.36 Correct consumer compile and runtime review findings  Owner: zati-implementer  Est: TBD  kind: agent  stage: fix  blocked-by: [T-COMPOSE.8]  acc: [Restore helper compatibility; preserve exact JSON numbers through mapped inputs; verify oversized artifacts without truncated-digest loops and settle bounded failure; derive expected/completed/pending/failed coverage and resolve all accepted consumer review findings with regressions]
- [x] T-COMPOSE.37 Verify corrected consumer boundaries  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.36, T-COMPOSE.11]  acc: [Exact corrected source passes affected numeric/output/recovery/coverage regressions and joined consumer checks; all failures and inherited baseline differences remain documented without model/provider claims]
- [x] T-COMPOSE.38 Independently re-review corrected consumer source  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-COMPOSE.37]  acc: [Independent non-author approves final consumer base/head and all accepted findings plus evidence before merge; no unresolved blockers remain]

Consumer source checkpoint09817654 implements .9–.10, including explicit desktop
v1/v2 selection and public import, authenticated runtime evidence, persistent
call/dataflow bounds and generic whole-loop dependency completion. Nine marketplace
widget tests passed on the pinned Flutter SDK; source owner/race/joined aggregate
checks remain pending and .11 is not complete. Review corrections additionally
cover HTML serialization expansion and whole-loop dependency ordering. The direct
import schema-equality report was retracted after checking the exact revision;
existing validation already enforced it. Source tests also exposed shared schema
mirror and allowlist fixture errors, now corrected for affected verification.
No consumer landing, installed app, provider or performance claim is made here.

Consumer delivery candidate: Zati source5c0f5e28 passed full five-owner non-race,
focused composition race, joined controller/Responses/Effects/TLS-MCP/persisted
reply checks for complete/partial v2, missing binding, legacy workflow and reviewed
marketplace activation. Widget tests9 passed. Vet/spec/mirrors passed. Broad race
attempt timed out and is not counted as passing; lint189 is unchanged baseline.
Checksum-only a10e5ba1 verifies modules and leaves tidy byte-identical to baseline
four Narrate removals. Accepted fixes include immutable byte transport and
canonical complete plan identity across actual command boundaries.

Independent composition_doc_audit APPROVED exact final Zati cee4f395 against
f7baa9fa. Zati PR47 guarded rebase landedfc68df88, exact reviewed tree verified.
Hosted CI could not start due billing restriction; unchanged authorized local
fallback applied. Fresh landed behavior checks remain
T-COMPOSE.14's gate; .17 remains blocked until that gate passes. Full receipt:
Zati docs/receipts/2026-10-17-gist-composition.md. No release, installation or live
provider acceptance follows from source delivery.

Fresh consumer landing verification: fc68df88 passed composition race checks in
Skills/Connections/Execution, new complete/partial plus legacy joined workflow
race tests, specification generation and runtime mirrors. Exact archived source
SHA25651fe43171ef5e343209eebae8754914ed691f17b2cc4a2eb58552f544c5e3759;
bounded service completed successfully and stopped. T-COMPOSE.8–.14 and .36–.38
are source delivery complete. Composition cohort .17 is now eligible, followed
by .18 scoring only after inventory, independent labels and primary configuration
are frozen. Existing thresholds and original held-out restrictions remain intact.

Evaluation pre-score inventory review found C16 used a connection-group key for
ConnectionSelections despite the published requirement-key contract. No scored
run had occurred. Correct the fixture to the intended second-account selection,
regenerate immutable inventory, preserve independent labels and review the exact
freeze before measuring. Expected readiness and thresholds are unchanged.

- [x] T-COMPOSE.39 Correct evaluation fixture and freeze findings  Owner: evaluation-author  Est: TBD  kind: agent  stage: fix  blocked-by: [T-COMPOSE.14, T-COMPOSE.32]  acc: [Accepted pre-score fixture and freeze findings corrected against published contracts; C16 uses list/read connection selections; original attempts and unchanged scenarios/thresholds retained]
- [x] T-COMPOSE.40 Verify corrected evaluation inventory and harness  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-COMPOSE.39]  acc: [Exact source unit checks and deterministic inventory regeneration pass; independent labels reconcile before scores and no implementation-produced outcome is used as gold]
- [x] T-COMPOSE.41 Independently review corrected evaluation freeze  Owner: independent-reviewer  Est: TBD  kind: agent  stage: rereview  blocked-by: [T-COMPOSE.40]  acc: [Non-author approves exact runner/config/inventory/labels/report manifest before scored execution, with accepted findings resolved and no outcome or threshold tuning]

Pre-score source checks at817d084 passed full root integration/race, vet, builds,
contracts, registry tests, tidy, report tests and inventory regeneration. Lint
found one introduced unchecked output.Close alongside five inherited findings.
Independent freeze review at5941596 also found early failed scale attempts could
crash the report. Both are corrected under .39; .40/.41 remain pending refreshed
checks and exact-freeze approval. No scored attempt, label or threshold changed.

Evaluation freeze approved by independent composition_doc_audit at
afceb5b770528e56cf8f5fc07c83b7c6d27f8d3a against ab6b6c6. Corrected runner
race, six report tests and deterministic inventory comparison pass; full lint
matches exactly five inherited findings, zero introduced. Earlier full root
integration/race/vet/build/contracts/registry/tidy evidence at817d084 remains
valid for unchanged source. Freeze manifest SHA256
a285988ebedc0a2edd681bd81b8e4c7d0f8c1f9e6b2377a502376bc0e3d7390e
binds 78 files. Both label authors agree 19 outcomes and exact selected IDs.
.17 and .39-.41 close before .18 scoring; measurements remain unobserved here.

Scored qualification at the approvedafceb5b freeze: all19 composition outcomes
match independent labels (4/4 exact ready closures,15refusals,0false-ready and
0private exposures); all four10k/100k partitioned/single-scope runs pass frozen
targets. Every first scored attempt is retained; no retries or retuning. Actual
100k peakRSS is~1GiB including retained generations; broad searches over budget
refuse explicitly. See docs/receipts/2026-10-17-composition-qualification.md.
.18 closes; .19-.21 candidate review/delivery remain pending. Source-qualified
Zati and synthetic index scores do not authorize release or live acceptance.

Evaluation delivery complete: PR85 reviewed66537d98e530c5697cdfb8aa56a52373a2ed80e8
againstab6b6c66ff82201309723ace1ac711ff6734119c; independent composition_doc_audit
approved exact source, reproducible report and all retained outcomes. Guarded
GitHub rebase landed3889cbefa7c93466699e1c958af65f1d0c64ff79 with tree equality
and main reachability. GitHub jobs did not start due to the account billing lock;
required checks/protections were unchanged and the established local fallback
was used. Fresh landed runner/discovery/composition/new-inbox/legacy-inbox race
tests, six report tests, inventory byte equality,78freeze hashes, all result
checksums and byte-identical report regeneration passed. Archive SHA256
224b4b81d85899d9180ba6cebb4879fd8e705e566f1d849166c93eae125a2411
binds that fresh landed check; bounded service exited successfully and is inactive.

All E-COMPOSE source, consumer, index and evaluation delivery rows .1-.21 and
tracked repairs .26-.41 are complete. Next .22 remains an explicit human release
and installation gate; .23-.25 cannot proceed from a source ship alone. No module
release tag, installed application, live provider or model-default decision is
claimed. Prior held-out/model evaluation authority is not reopened by these scores.

### E-HOSTED -- Deploy the existing AWS catalog at the canonical Gist origin

fidelity: executable
Acceptance: Reviewed IaC preserves the existing registry account/database/bucket,
deploys an exact source/image at gist.sire.run, verifies TLS and authenticated
catalog/MCP behavior, and records rollback plus unresolved client/provider limits.

The owner has supplied deployment authority, AWSCLI and CloudflareMCP are
authenticated, and the existing member-account deployment role is assumable.
Existing service is healthy on the old registry origin with an older image.
The canonical hostname has no DNS record. New source requires durable signing
and enrollment configuration absent from the old task. The owner supplied a company-wide Sire Run GitHubApp credential binding; its
private key verified ownership and permission-free identity metadata through
GitHub. Secret values remain outside source. Initial private Gist workspace
and the authenticated founder account are explicit deployment configuration;
no identity is seeded before actual GitHub verification. No credential values enter source, logs or public receipts.

The existing Pulumi stack in sirerun/foundation owns AWS resources. Cloudflare
records follow its certificate/ALB outputs with an idempotent reconciliation
record and read-only drift check. Stage new certificate alongside the old one;
activate canonical origin/audience together only after issuance and source/image
qualification. Preserve old image/configuration for rollback and all existing data.
Mac load prevents heavy local builds; qualified bounded Linux capacity supplies
checks and a cross-compiled Pulumi program without credential transfer.

A cold-start defect may require explicit enrollment-only bootstrap: preconfigured
founder identity is verified by GitHub; catalog access and grants remain denied
until normal persisted maintainer authority and maintenance readiness succeed.
Default strict startup remains. No fake membership seeding or asserted identity.
Publication v2 stays disabled until a real trusted production binding verifier
is supplied; do not infer fixture-only conformance is production qualification.

- [x] T-HOSTED.1 Reconcile hosting authority and current infrastructure  Owner: coordinator  Est: TBD  kind: agent  stage: preflight  acc: [Current source, dirty worktrees, stack outputs, AWS role, existing ECS revision, DNS, secret names and protected data identified without secret disclosure; narrow owner-authorized scope recorded]
- [x] T-HOSTED.2 Implement staged canonical origin and runtime IaC  Owner: infrastructure-owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T-HOSTED.1]  acc: [Foundation owns staged certificate/TLS, exact image, strict origin/audience, Secrets Manager references, least-privilege roles, correct parent-module container build, migration task and rollback without data replacement]
- [x] T-HOSTED.3 Correct required hosted startup bootstrap  Owner: registry-owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T-HOSTED.1]  acc: [Explicit first-enrollment mode solves cold-start only through real verified founder enrollment; all catalog and grants fail closed until current maintainer readiness; default startup and revocation behavior retain meaningful regressions]
- [ ] T-HOSTED.4 Verify exact source and infrastructure changes  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-HOSTED.2, T-HOSTED.3, T-HOSTED.14, T-HOSTED.15]  acc: [Relevant Pulumi mocks/buildspec checks and hosted unit/integration/race/security checks pass; credential-free remote build provenance and actual preview changes retained; no destructive or unrelated resource changes]
- [ ] T-HOSTED.5 Independently review final IaC and runtime candidates  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-HOSTED.4, T-HOSTED.16]  acc: [Non-author reviews exact base/head across both repositories, staged migration, first-start authority, credential custody, data protection and verification; accepted findings corrected and re-reviewed before merge]
- [ ] T-HOSTED.6 Merge and verify landed deployment inputs  Owner: coordinator  Est: TBD  kind: agent  stage: merge  blocked-by: [T-HOSTED.5, T-HOSTED.17]  acc: [Guarded rebase preserves repository policies, exact approved trees and immutable source provenance; fresh landed checks verify affected behavior and plan conformance]
- [ ] T-HOSTED.7 Stage canonical certificate and reconcile validation DNS  Owner: deployment-owner  Est: TBD  kind: agent  stage: deploy  blocked-by: [T-HOSTED.6]  acc: [Merged preview matches exact existing-stack apply; new ACM certificate and Cloudflare validation records are owned, idempotent and issued while old serving configuration remains available]
- [ ] T-HOSTED.8 Qualify enrollment and durable secret configuration  Owner: coordinator-with-owner  Est: TBD  kind: agent  stage: preflight  blocked-by: [T-HOSTED.1]  acc: [Existing or explicitly provisioned GitHubApp client reference, canonical callback, exact founder numeric identity/workspace, persistent signing/flow/CSRF/session custody and maintenance target recorded; no unrelated secrets borrowed or secret values published]
- [ ] T-HOSTED.9 Build and pin the reviewed registry image  Owner: deployment-owner  Est: TBD  kind: agent  stage: release  blocked-by: [T-HOSTED.6]  acc: [Existing CodeBuild/ECR produces ARM64 image from exact merged source and reviewed Containerfile, retains build/digest and failure receipts; no moving image tags]
- [ ] T-HOSTED.10 Run owned versioned migrations and deploy canonical service  Owner: deployment-owner  Est: TBD  kind: agent  stage: deploy  blocked-by: [T-HOSTED.7, T-HOSTED.8, T-HOSTED.9]  acc: [Version-aware migration workflow preserves existing production data; merged exact image/config preview applies only reviewed stack resources and canonical origin/audience together; ECS stabilizes and rollback remains available]
- [ ] T-HOSTED.11 Verify external HTTPS and authenticated catalog journey  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-HOSTED.10]  acc: [Canonical DNS/TLS/issuer/resource metadata, real operator enrollment, workload authorization, exact artifact retrieval/MCP and unauthenticated/wrong-audience/tenant/revocation denials observed; health alone cannot close acceptance]
- [ ] T-HOSTED.12 Independently accept deployed service and operational handoff  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-HOSTED.11]  acc: [Source/image/config/live receipts agree, bounded recovery/backup/rotation/monitoring/ownership and actual remaining client/provider/publication limits are documented without claiming all historical RFC milestones complete]
- [ ] T-HOSTED.13 Record and verify landed hosting handoff  Owner: coordinator  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T-HOSTED.12]  acc: [Independently reviewed evidence and canonical plan merge with source-bound production receipt; fresh main and running revision align, no ongoing watcher or unsupported marketplace completion is claimed]

#### Hosted execution notes

Execution: current-session inherited model/effort, one infrastructure implementer
and one registry implementer with disjoint isolated worktrees; coordinator owns
cloud changes/plan and non-author review is a separate gate. No agent changes
AWS or DNS concurrently. Requested outcome is deployed service; missing real
identity configuration blocks only its dependent activation, not reviewable IaC.

#### Hosted migration and identity qualification

- [x] T-HOSTED.14 Implement an auditable one-shot database migrator  Owner: registry-owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T-HOSTED.1]  acc: [Default status reads schema metadata only; explicit apply holds a PostgreSQL advisory lock and records exact SQL hashes transactionally; pre-existing untracked schema refuses automatic replay; adoption requires explicit reviewed prefix plus manifest/schema fingerprints and recovery evidence; no down migration or fabricated identity]

- [ ] T-HOSTED.15 Correct database privilege separation before activation  Owner: registry-and-infrastructure-owners  Est: TBD  kind: agent  stage: fix  blocked-by: [T-HOSTED.14]  acc: [Current app receives only a separate non-owner runtime database URL; isolated migration task retains owner authority; explicit role provisioning refuses unsafe attributes or ownership and grants only required runtime objects while excluding the migration ledger]
- [ ] T-HOSTED.16 Verify the accepted database review correction  Owner: verifier  Est: TBD  kind: agent  stage: verify  blocked-by: [T-HOSTED.15]  acc: [Pulumi mocks prove current application and execution IAM exclude owner secret; PostgreSQL integration proves runtime role attributes, non-ownership, tenant isolation and denied schema or ledger mutation; exact checks and limitations retained]
- [ ] T-HOSTED.17 Independently re-review database correction and deployment authority  Owner: independent-reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T-HOSTED.16]  acc: [Non-author reviews exact correction and checks; production activation additionally requires live role, ownership and RLS evidence before the application is given runtime credentials]

#### Database correction and retained evidence

Independent review accepted staged certificate and secret-container creation,
but blocked activating the new app with the existing RDS owner credential.
The correction separates runtime and migration secrets and adds explicit
non-owner role provisioning. Naming a secret does not prove least privilege;
actual database privileges and tenant behavior remain activation checks.

The owner considered GitHub-backed public distribution and requested design
audit before dropping sign-in. Audit confirms authenticated publication, private
workspaces, OAuth/workload grants, principal-bound resolutions and revocation
are implemented functionality worth preserving. Company-wide GitHubApp setup
now supplies the identity prerequisite. Keep those controls; a public catalog
projection can be added independently after explicit visibility, trust and
freshness contracts. No static-only replacement or removal of auth is shipped.

Candidate checks retain actual failures: initial Foundation offline dependency
cache missed pinned SDK modules; a bounded retry downloaded checksum-verified
program dependencies and passed mocks/race/vet plus cross-compilation. Initial
full hosted integration/race passed internal/cmd suites but named-client
acceptance failed because no real pinned client build was configured. That
external acceptance remains unqualified; do not invent client evidence.

Hosted source qualification: the bounded Linux run of the explicit internal,
command, wiring, retrieval and runtime integration suites passed with the race
detector, followed by vet and build. Named external-client acceptance remains
unqualified. Database privilege correction requires its own changed-source checks.
