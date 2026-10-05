# Independent CORE composition review

Date: 2026-10-05. Verdict: **PASS** for the bounded CORE source and fixture composition at PR #55 head `d746051323e570de77befc5040011c7eb9bb392c`, base `d75ac183fe8b949a4f1381e8ebca3226f0c0e282`.

I did not author or coauthor the CORE source, fixtures, R11/R12 changes, or candidate receipts. My prior EVENT storage contribution is already in the reviewed base. I reviewed the startup/configuration, event and maintenance paths, fixture adaptations, owner-decision boundary, and exact candidate graph.

R1–R8 fixes preserve explicit signing-key custody, current-policy tenant maintenance, response budgeting, transactional catalog/outbox behavior, and bounded cancellation/shutdown. R9/R10 fixtures add only a dedicated acceptance maintainer and ephemeral test signing key, preserving reader/client grants while adapting requests to the frozen schema. These tests do not establish external enrollment or named-client qualification. R11 qualifies cross-epic task references; R12 removes only receipt trailing spaces.

Maintenance refreshes current membership, generation, and publish authority before bounded tenant purge. Startup is deadline-bound; backlog and target failures remain visible. The PostgreSQL shutdown regression blocks an active maintenance query and verifies cancellation before pool close. Publication budget is qualified before staging and catalog/outbox share a tenant transaction. A private unbound blob can remain after database rollback; no compensation or garbage-collection guarantee is made, and publication staging ownership/retention/reconciliation remain production gates.

The exact candidate graph has 251 total tasks and 233 active tasks (109 done, 17 open, 107 blocked). It has no duplicate IDs, unresolved dependencies, cycles, undefined waves, or ambiguous waves; all active tasks reach `T-GR-PROD.9`. CORE reports 28/43, and CORE.34 becomes open once its implementation prerequisites are done. Comparing the prior graph with the reviewed graph preserves all existing dependency semantics apart from the intended new CORE merge dependencies on `.39` and `.42`. The optional gateway is unchanged.

Owner decisions remain proposals, not approvals. Enrollment/key custody, provider targets/caps, AWS account binding, and configured Cloudflare DNS capability remain unresolved or unavailable. No provider call, credential read, deployment, or production acceptance is claimed.

Independent checks:

- On source tree `e5a794f`, the eight selected CORE app PostgreSQL regressions passed with `-race` (3.468s), covering rollback, tenant/policy denial, bounded maintenance, startup deadline, and active-query shutdown cancellation.
- `go test -count=1 -p=2 ./internal/app` passed (0.622s); `go test -count=1 -p=2 ./cmd/registry` passed (0.291s).
- At exact review head `d746051`, integration-tagged `golangci-lint` on `./internal/storage` passed with 0 issues, including the retention probe added by CORE.
- The hosted production and test files are byte-identical from `e5a794f` to `d746051`; the complete `git diff --check d75ac183fe8b949a4f1381e8ebca3226f0c0e282..d746051323e570de77befc5040011c7eb9bb392c` passed.

Broader local source/fixture checks and the exact default-plan parser run are recorded in [CORE author verification](2026-10-05-core-author-verification.md), [CORE fixture verification](2026-10-05-core-fixture-verification.md), and [CORE root and dispatcher verification](2026-10-05-core-root-and-dispatch-verification.md). These are local checks, not hosted CI or external acceptance.
