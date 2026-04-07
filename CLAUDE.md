
## Shared Docs Repo

Cross-repo planning and documentation lives in a dedicated git repo: `github.com/sirerun/docs`, checked out at `/Users/dndungu/Code/sirerun/docs/`. This is the single source of truth for `plan.md` (cross-repo execution plan used by /plan and /apply), `adr/` (cross-repo ADRs), `devlog.md` (investigations/benchmarks), `usecases.md`, `design.md`, and `content-classification.md`.

Work in this project is typically cross-repo. Always read/update the plan in the shared docs repo, not a per-repo copy. Commit docs changes via PR to `sirerun/docs` independently from code PRs.

## Staging Environment — HIBERNATED

`sire-staging.run` is temporarily hibernated (E3 in the shared docs/plan.md) to reduce cloud costs until funding closes. Do not deploy or test against staging. Tests target production using dedicated `qa+bot@sire.run` accounts in sandboxed workspaces. Hibernated (deleted): staging Cloud SQL, Redis, GKE. Preserved: secrets, Artifact Registry, DNS, KMS, IAM, Pulumi state. Revival: revert E3 gates and `pulumi up --stack staging`.

## No Manual DevOps — IaC + Release Pipeline Only

Production and staging are managed exclusively through IaC and the CI/CD release pipeline. Banned: `kubectl set/edit/scale/patch/delete` and `kubectl apply` against staging/prod, `gcloud secrets create/add/delete` and other imperative `gcloud` mutations, direct prod DB writes, hot-patching pods, re-tagging or force-pushing. Required path: edit IaC → PR → CI → rebase merge → tag release → deploy workflow → verify via workflow checks. Read-only diagnostics (`kubectl get/describe/logs`, `gcloud ... list/access`, `gh run view`) are fine. Agents: never run mutating commands against live infra; open a PR.

## Gist Context Management

Use the gist MCP tools (gist_index, gist_search, gist_stats) to manage context efficiently:

- When reading files over 5KB or receiving tool output over 5KB, index the content with gist_index (set a descriptive source label like the file path).
- Instead of re-reading indexed files, use gist_search to retrieve only the relevant snippets.
- When exploring a codebase (reading multiple files, grepping across directories), index results into gist and search across them.
- After completing a task, call gist_stats and briefly report bytes saved (e.g., "Gist: indexed 48.2 KB, returned 3.1 KB, saved 93.6%").
