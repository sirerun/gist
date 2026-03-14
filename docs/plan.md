# Plan: Gist -- Multi-Tool Setup, Byte Savings Visibility, and README Update

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

- [x] T1.1 Define tool adapter types and registry  Owner: task-T1.1  Est: 30m  Done: 2026-03-14
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
  - Acceptance: compiles, `gist setup` prints tool list, `gist setup invalidtool` prints error.
  - Dependencies: none.

- [x] T1.2 Implement MCP config file manipulation  Owner: task-T1.2  Est: 30m  Done: 2026-03-14
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
  - Acceptance: function creates/updates/removes MCP entry correctly for both JSON and TOML formats.
  - Dependencies: none.

- [x] T1.3 Implement instructions file manipulation  Owner: task-T1.3  Est: 20m  Done: 2026-03-14
  - In cmd/gist/setup.go, implement `func configureInstructions(path string, sentinel string, uninstall bool, dryRun bool) (changed bool, err error)`.
  - If path is empty, return changed=false (tool has no instructions file).
  - Read existing file. If absent, start with empty string.
  - On install: if sentinel string not found in content, append a blank line + the gist instructions section. Return changed=true. If found, return changed=false.
  - On uninstall: if sentinel found, remove from sentinel line through the next `## ` heading (exclusive) or end of file. Trim trailing blank lines. If not found, return changed=false.
  - If dryRun, print what would be written to stderr but do not write.
  - Create parent directories with `os.MkdirAll`.
  - Acceptance: function appends/removes instructions section correctly.
  - Dependencies: none.

- [x] T1.4 Wire adapter, MCP, and instructions into cobra RunE  Owner: task-T1.4  Est: 15m  Done: 2026-03-14
  - In cmd/gist/setup.go, implement the cobra RunE function.
  - Look up the tool adapter from the registry using the first arg.
  - Determine target paths: if `--project`, use ProjectMCPPath and ProjectInstPath. Otherwise, use GlobalMCPPath and GlobalInstPath. Expand `~` to `os.UserHomeDir()`.
  - Call configureMCP and configureInstructions.
  - Print results to stderr: "Configured <DisplayName> MCP at <path>", "Added gist instructions to <path>", "Already configured (no changes)", "Removed gist from <path>".
  - Acceptance: `gist setup claude` creates both files. `gist setup claude --uninstall` removes both entries.
  - Dependencies: T1.1, T1.2, T1.3.

### E2: Setup Subcommand Tests

- [x] T2.1 Write MCP config manipulation tests  Owner: task-T2.1  Est: 30m  Done: 2026-03-14
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
  - Acceptance: `GOWORK=off go test ./cmd/gist/ -run TestConfigureMCP -race -v` passes.
  - Dependencies: T1.2.

- [x] T2.2 Write instructions file manipulation tests  Owner: task-T2.2  Est: 20m  Done: 2026-03-14
  - In cmd/gist/setup_test.go.
  - Test cases:
    - Fresh install: no file -> creates with gist section.
    - Existing file without gist -> appends gist section.
    - Already has gist section -> no change (idempotent).
    - Uninstall -> removes gist section, preserves other content.
    - Uninstall when not present -> no change.
    - Content after gist section -> preserved on uninstall.
    - Empty path (codex) -> no change, no error.
  - Acceptance: `GOWORK=off go test ./cmd/gist/ -run TestConfigureInstructions -race -v` passes.
  - Dependencies: T1.3.

- [x] T2.3 Write end-to-end setup tests  Owner: task-T2.3  Est: 20m  Done: 2026-03-14
  - In cmd/gist/setup_test.go.
  - Test the full flow for each tool adapter: fresh setup, idempotent re-run, uninstall.
  - Use t.TempDir() as home directory substitute.
  - Verify both files are created/modified/cleaned correctly for each tool.
  - Acceptance: `GOWORK=off go test ./cmd/gist/ -run TestSetupE2E -race -v` passes.
  - Dependencies: T1.4.

### E3: Byte Savings Visibility

- [x] T3.1 Add BytesUsed to SearchResult  Owner: task-T3.1  Est: 15m  Done: 2026-03-14
  - In search.go, add `BytesUsed int` field to SearchResult struct with json tag `bytes_used`.
  - In convertMatches(), set BytesUsed = len(snippet) for each result.
  - Acceptance: compiles, go vet clean.
  - Dependencies: none.

- [x] T3.2 Update CLI stats command with human-readable formatting  Owner: task-T3.2  Est: 15m  Done: 2026-03-14
  - In cmd/gist/stats.go, add a formatBytes helper function that returns "X B", "X.Y KB", or "X.Y MB" depending on magnitude.
  - Apply formatBytes to BytesIndexed, BytesReturned, BytesSaved output lines.
  - Keep SavedPercent, SourceCount, ChunkCount, SearchCount as plain integers.
  - Acceptance: `gist stats` output shows human-readable byte values.
  - Dependencies: none.

- [x] T3.3 Add savings summary to MCP gist_search response  Owner: task-T3.3  Est: 20m  Done: 2026-03-14
  - In mcp/tools.go handleSearch(), after calling g.Search(), compute total bytes used across results (sum of BytesUsed from each SearchResult).
  - Wrap the search response in a struct that includes the results array and a `bytes_used` total field.
  - Update the gist_search tool description in ToolDefinitions() to mention bytes_used.
  - Acceptance: gist_search MCP response includes bytes_used per result and total.
  - Dependencies: T3.1.

### E4: Test Updates for Savings Visibility

- [x] T4.1 Add SearchResult.BytesUsed tests  Owner: task-T4.1  Est: 15m  Done: 2026-03-14
  - In search_test.go, add a test verifying BytesUsed = len(snippet) for each result.
  - In gist_test.go TestSearch, verify BytesUsed > 0 on results.
  - Acceptance: `GOWORK=off go test -run TestSearch -race -v` passes.
  - Dependencies: T3.1.

- [x] T4.2 Add CLI stats formatting test  Owner: task-T4.2  Est: 10m  Done: 2026-03-14
  - In cmd/gist/stats_test.go (new file), table-driven tests for formatBytes: 0 -> "0 B", 512 -> "512 B", 1024 -> "1.0 KB", 1536 -> "1.5 KB", 1048576 -> "1.0 MB", 1572864 -> "1.5 MB", negative -> "0 B".
  - Acceptance: `GOWORK=off go test ./cmd/gist/ -run TestFormatBytes -v` passes.
  - Dependencies: T3.2.

- [x] T4.3 Add MCP search savings test  Owner: task-T4.3  Est: 15m  Done: 2026-03-14
  - In mcp/tools_test.go, add a test that indexes content, searches, and verifies bytes_used total in response.
  - Update TestToolsCallSearch to verify the wrapped response structure.
  - Acceptance: `GOWORK=off go test ./mcp/ -race -v` passes.
  - Dependencies: T3.1, T3.3.

### E5: README Update

- [x] T5.1 Update README with agentic tool integration section  Owner: task-T5.1  Est: 30m  Done: 2026-03-14
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
  - Dependencies: none (README content can be written before the setup code exists; the README documents the planned interface).
  - Acceptance: README contains "Agentic Tool Integration" section with install commands, supported tools table, and flag documentation.

- [x] T5.2 Update README MCP Server section  Owner: task-T5.2  Est: 10m  Done: 2026-03-14
  - In README.md, update the existing "MCP Server" section.
  - Replace the manual JSON config example with a note that `gist setup <tool>` handles configuration automatically.
  - Keep the manual JSON example as a "Manual configuration" subsection for users who prefer manual setup or use unsupported tools.
  - Remove `--dsn` from the MCP example since in-memory is the default and sufficient for agentic tool sessions.
  - Acceptance: MCP Server section references `gist setup` and provides both automatic and manual configuration paths.

- [x] T5.3 Update README CLI Usage section  Owner: task-T5.3  Est: 10m  Done: 2026-03-14
  - In README.md, add `gist setup <tool>` to the CLI Usage code block.
  - Add a one-line description: "# Configure gist for your agentic coding tool".
  - Place it after the existing `gist serve` example.
  - Acceptance: CLI Usage section includes `gist setup` example.

### E6: Quality Gates

- [x] T6.1 Run linter and fix findings  Owner: lead  Est: 10m  Done: 2026-03-14
  - `GOWORK=off go vet ./...`
  - `GOWORK=off go build ./...`
  - Acceptance: zero errors, zero warnings.
  - Dependencies: all of E1-E5.

- [x] T6.2 Run full test suite  Owner: lead  Est: 10m  Done: 2026-03-14
  - `GOWORK=off go test -race -count=1 ./...`
  - Acceptance: all tests pass.
  - Dependencies: T6.1.

## Parallel Work

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
