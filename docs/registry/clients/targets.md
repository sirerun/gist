# Named client and build targets

R1 target matrix, 2026-09-25. These are public product targets, not completed
compatibility claims. R3 pins an exact build, records the supported schema
subset/losses, and reruns every required negative case. No client receives
automatic skill installation or global configuration changes.

| Product target | Public build target | Registry path | R3 acceptance boundary |
| --- | --- | --- | --- |
| OpenAI Codex | Codex CLI stable release; pin the CLI package/version and host OS at R3 | Remote MCP when supported by the pinned build; otherwise the documented local adapter target | Authenticate, call `gist_discover`, fetch exact package/schema with `gist_get`, resolve, and surface explicit unsupported/connection errors. M2a policy-gated runtime target. |
| Anthropic Claude Code | `claude` CLI stable release; pin CLI version and host OS at R3 | Remote MCP via `claude mcp`; local configuration is an explicit test fixture, not a registry-side write | Same retrieval/error workflow; verify tool naming/schema subset and that a tool result does not silently install a skill. |
| Cursor | Cursor desktop stable channel and, where useful, the matching `cursor-agent` CLI; pin both build and OS at R3 | Streamable HTTP or SSE remote MCP with OAuth; stdio remains a local comparison only | Verify transport negotiation, OAuth, tool/schema subset, exact retrieval, and no mutation of `mcp.json` by Gist. |
| Claude.ai | Claude web app custom connector surface, not a local build; record plan/connector surface and test date at R3 | Public remote MCP over HTTPS with OAuth; requests originate from Anthropic infrastructure | Add/consent/retrieve/refresh/revoke using synthetic tenant data; no localhost or filesystem assumption. |
| Grok Bot | Local `Grok Bot.app` 0.58.0 is an identification artifact only; not an acceptance target unless the spike is later qualified | Unknown; custom schemes observed, but no supported remote MCP/OAuth evidence | Excluded from R3/M3 until a follow-up identifies vendor, transport, OAuth, and a supported integration path. |

## Contract rules for every target

- Pin product, build/version, OS, transport revision, auth mode, and registry
  fixture hash in the acceptance receipt.
- Treat retrieved instructions and schemas as lower-trust data. They cannot
  override host instructions, identity, permissions, or protected-effects
  policy.
- A client may register a schema as a native tool only when its own runtime
  permits that operation; otherwise it must report `unsupported_runtime` or
  `requires_gateway` rather than imply executability.
- Claude.ai's remote connector must be tested at a public HTTPS origin because
  the connection originates from Anthropic infrastructure. Cursor's public
  MCP documentation lists stdio, SSE, and Streamable HTTP; the selected build
  and transport are still pinned at R3.

## Public references

- [Codex CLI](https://github.com/openai/codex)
- [Claude Code CLI reference](https://docs.anthropic.com/en/docs/claude-code/cli-usage)
- [Cursor MCP](https://docs.cursor.com/context/model-context-protocol)
- [Claude custom connectors using remote MCP](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp)
