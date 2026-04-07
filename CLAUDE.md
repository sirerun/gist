
## Shared Docs Repo

Cross-repo planning and documentation lives in a dedicated git repo: `github.com/sirerun/docs`, checked out at `/Users/dndungu/Code/sirerun/docs/`. This is the single source of truth for `plan.md` (cross-repo execution plan used by /plan and /apply), `adr/` (cross-repo ADRs), `devlog.md` (investigations/benchmarks), `usecases.md`, `design.md`, and `content-classification.md`.

Work in this project is typically cross-repo. Always read/update the plan in the shared docs repo, not a per-repo copy. Commit docs changes via PR to `sirerun/docs` independently from code PRs.

## Gist Context Management

Use the gist MCP tools (gist_index, gist_search, gist_stats) to manage context efficiently:

- When reading files over 5KB or receiving tool output over 5KB, index the content with gist_index (set a descriptive source label like the file path).
- Instead of re-reading indexed files, use gist_search to retrieve only the relevant snippets.
- When exploring a codebase (reading multiple files, grepping across directories), index results into gist and search across them.
- After completing a task, call gist_stats and briefly report bytes saved (e.g., "Gist: indexed 48.2 KB, returned 3.1 KB, saved 93.6%").
