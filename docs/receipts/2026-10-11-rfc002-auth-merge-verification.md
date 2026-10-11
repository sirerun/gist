# RFC-002 enrollment review and landed verification

Date: 2026-10-11
Pull request: https://github.com/sirerun/gist/pull/63
Reviewed base: c02811e00c76794f854173463a03a22594b1705a
Reviewed head: e943603bf26e1e14f02938fde5ce31f04da50fa9
Landed main SHA: 1e52ff959f8538e07c40b768956f3e37ab10d329

## Independent review

A nonauthor independent reviewer inspected the exact PR head and base. Coverage included permissionless GitHub App sign-in, numeric account-ID proof, OAuth state and PKCE, replay handling, host-only encrypted flow cookie, one-time founder bootstrap, invitation hash/account/workspace binding, atomic redemption, role and scope enforcement, tenant isolation, migration constraints, and invitation row security. The reviewer found no actionable security or behavior defects. The reviewer did not run tests.

## Verification and merge

The component test receipt records `GOWORK=off go test -tags=integration ./... -count=1`, scoped vet/lint, and owned-file formatting on tested code source SHA `f74e5f1a461d5cb804a0085017be292d71632c5a`. Git comparison from that source SHA to the exact PR head and landed main showed only `docs/receipts/2026-10-11-rfc002-auth-component.md`; no Go or migration source changed after the tested source. The review and source test evidence therefore cover the landed executable bytes. No tests were rerun at the merge SHA.

GitHub PR #63 merged by rebase at landed main SHA `1e52ff959f8538e07c40b768956f3e37ab10d329`. Existing branch protection had no required status checks and zero required approving reviews. The hosted job failed before steps started because of the repository billing lock; it is not recorded as a CI pass. No protection or billing setting changed.

## Boundary

This lands the isolated enrollment component. The application startup and routes still do not register it, and this component does not issue or refresh Gist OAuth workload tokens. Full enrollment-to-consent-to-bounded-token behavior, revocation linkage, and composed HTTP/MCP acceptance remain in T-GR-INTEGRATE.1/.2. No live GitHub App, operator, workspace, provider call, or production deployment was qualified by this receipt.

## Acceptance on the landed SHA

On 2026-10-11, the full hosted Go module integration suite passed on exact remote main SHA `1e52ff959f8538e07c40b768956f3e37ab10d329` with Go 1.26.1 and an isolated PostgreSQL 16 cluster restricted to a task-local Unix socket (`listen_addresses` disabled). Command: `GOWORK=off go test -tags=integration ./... -count=1` from `hosted/`. `GIST_DATABASE_URL` named the local socket-only database; the three offline client contract fixtures used the documented `x@local-fixture` build labels. Every package passed, including enrollment, storage migrations, application wiring, and client/retrieval/runtime fixtures. The cluster was stopped after the test and its task-local data retained.

An initial run without the required client fixture labels failed the explicit `TestWorkload` precondition. A second run with a socket URL lacking the fixture's required URL host failed URL validation. The corrected command above then completed successfully. These fixture labels verify local HTTP/MCP compatibility only; they do not qualify real Codex/Claude/policy-runtime builds, a live GitHub App, production enrollment, or production acceptance. The hosted CI job remained billing-locked and is not counted as passing.

## Follow-up: enrollment OAuth return flow (PR #64)

PR #64 was reviewed at head `f1694858fab491ebabfc0385f57ffdc6ce5e7b90` against base `1e52ff959f8538e07c40b768956f3e37ab10d329` and merged by rebase at `d12fd22b7d059629698a2d1c03ed0a0b69fb1860`. GitHub reports the reviewed head tree and landed tree are both `a85f9444f702e8d5fa98d12a316bef9d4b0380f3`, so the landed source bytes match the tested and reviewed tree.

On the reviewed tree, the hosted Go module checks passed: `GOWORK=off go test ./... -count=1`, `GOWORK=off go vet ./...`, `GOWORK=off go test -race ./internal/enrollment -count=1`, and `GOWORK=off go test -tags=integration ./... -count=1`. The integration run used a task-local PostgreSQL fixture and synthetic client build labels. No test was rerun at the merge commit itself. Hosted GitHub jobs failed before executing any steps; they are not recorded as CI passes. The independent exact-head reviewer found no actionable security or behavior issue and did not run tests.

This follow-up restores the pending same-origin `/oauth/authorize` request after successful enrollment and renders the consent page. It is partial preparatory integration evidence only. The full `T-GR-INTEGRATE.1` acceptance and rows `.0-.6` remain open for persistent signing, canonical REST/MCP serialization, exact provider closure, durable revocation, broader composed denial coverage, independent integration review, and final landed acceptance. No live GitHub sign-in, real operator/workspace, provider call, AWS/DNS mutation, deployment, or production acceptance is qualified.
