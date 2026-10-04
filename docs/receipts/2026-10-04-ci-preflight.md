# CI and merge-policy preflight

Date: 2026-10-04
Task: T-GR-SCOPE.8
Repository: `sirerun/gist`
Inspected source: `main` at `84e14128565058419a9b90619f27fa517ba4f494` (matches assigned worktree base).

## Policy

- Active ruleset `main` (`13902341`) targets the default branch. It denies deletion and non-fast-forward updates and requires pull requests. Its allowed merge method is **rebase only**. The ruleset requires 0 approving reviews, no code-owner review, no last-push approval and no conversation resolution. It has no `required_status_checks` rule.
- Legacy `main` branch protection likewise reports 0 required approvals and no required status checks. Signatures, linear history, conversation resolution, admin enforcement and force-pushes are not enabled there; deletion is disabled. The active ruleset still controls merge method and non-fast-forward/deletion protections.
- Therefore, **there are currently no GitHub-required check contexts**. Repository workflows exist and report check results, but those results are not configured as merge-blocking required checks. The effective merge path is a PR, followed by rebase merge; independent review remains required by the delivery plan's own acceptance, even though GitHub's configured approval count is zero.

## Current remote and open work

- `main` currently resolves to `84e14128565058419a9b90619f27fa517ba4f494`.
- The only open PR is #47, `docs: bank dynamic tool-use skill direction`, head `bdd7d3e4f2186cfcd7aba8b708b8099539bf99e6`, base `84e14128565058419a9b90619f27fa517ba4f494`, merge state `UNSTABLE`. It is unrelated to this preflight lane. Its CI and E2E check rollups fail.

## CI startup evidence

The latest runs on current `main` are all failed: CI run `36513485365`, E2E Context Savings run `36513485319`, and Release run `36513485481` (created 2026-09-29 02:37:44 UTC). They completed within roughly 3–5 seconds. The CI `Test` job record has no runner name/group and an empty `steps` array. The latest CI and E2E runs on another branch (run IDs `36847582638` and `36847582551`, created 2026-10-01 10:11:43 UTC) likewise fail within seconds; CI's `Lint` job has no runner identity and zero steps. GitHub has no failed-step log for these jobs (`gh run view --log-failed` returns `log not found`). Thus Actions checks are **not passing and no job steps ran**; there is no positive hosted-CI evidence for the current source. ADR008 records the GitHub Actions account as billing-locked and unavailable. The observed zero-step/no-runner failures are consistent with that recorded limitation, though GitHub's current run records do not expose a more specific billing error message.

## Accepted local-validation alternative

ADR008 §12 explicitly says GitHub Actions is unavailable and changes are merged on local validation; ADR009 §7 also records GitHub Actions unavailability by reference to ADR008. This is the previously accepted local-validation route and remains applicable to this repository's guarded PR delivery under the accepted scope. Current rules do not require any CI status check, so no required check remains unsatisfied. Preserve the existing ruleset and branch protections. Each implementation/review lane must still run and record its applicable local checks before merge. Local results are local evidence only and must never be described as GitHub CI evidence. The present receipt is a read-only policy preflight, not a validation of application changes.

## Decision

**GO for guarded PR development and local-validation workflow; no GitHub policy change is needed.** **CI evidence remains unavailable:** do not claim a hosted CI pass. Recheck policy and run startup immediately before an actual merge because these remote settings and run records can change.

Sources inspected read-only with `gh`: ruleset `13902341`, `main` branch protection, current-main workflow runs/jobs, and open PR list/details. No workflow was triggered and no GitHub settings were changed.
