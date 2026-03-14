# ADR 003: Add `gist setup <tool>` Subcommand for Agentic Tool Integration

## Status
Accepted

## Date
2026-03-14

## Context

Integrating gist with an agentic coding tool (Claude Code, Gemini CLI, Cursor, VS Code Copilot, Codex CLI) requires two manual steps per tool:

1. Add a gist MCP server entry to the tool's MCP config file.
2. Append gist usage instructions to the tool's system instructions file.

Each tool has its own config location, JSON schema, and instructions file format. The setup is easy to get wrong and requires knowledge of each tool's internals. A `gist setup <tool>` subcommand can automate both steps for any supported tool.

Alternative approaches considered:
- **Single `gist setup` with auto-detect**: Rejected because multiple tools can be installed simultaneously and users should explicitly choose which to configure.
- **`gist setup --all`**: Considered but deferred. Can be added later once the per-tool adapters exist.
- **Interactive wizard**: Rejected because it is not automatable.

## Decision

Add a `gist setup <tool>` subcommand where `<tool>` is one of: `claude`, `gemini`, `cursor`, `copilot`, `codex`. Each tool is defined by a adapter struct that specifies:

- MCP config file path (global and project-level)
- MCP config format (JSON with `mcpServers` key, JSON with `servers` key, or TOML)
- Instructions file path (global and project-level)
- Instructions file sentinel (heading marker for idempotent insert/remove)

The subcommand supports:
- `gist setup claude` -- configure for Claude Code
- `gist setup gemini` -- configure for Gemini CLI
- `gist setup claude --project` -- configure for current project only
- `gist setup claude --uninstall` -- remove gist config
- `gist setup claude --dry-run` -- preview changes

Running `gist setup` without a tool name prints the list of supported tools.

The adapter pattern makes adding new tools trivial: define the struct, register it. No new code paths needed.

## Consequences

**Positive:**
- One-command setup per tool: `gist setup claude`, `gist setup gemini`.
- Adding new tools requires only a new adapter definition (paths + format), not new logic.
- Idempotent and reversible.
- Dry-run support.
- Works for both global and per-project scopes.

**Negative:**
- Must track config file locations for multiple tools, which may change.
- TOML support for Codex CLI adds a small amount of format-specific code.
- Instructions content may need to differ per tool (e.g., tool-specific MCP tool names).
