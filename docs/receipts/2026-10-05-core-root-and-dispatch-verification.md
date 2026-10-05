# CORE root compatibility and dispatcher verification

Local coordinator evidence for PR55 on base `d75ac183fe8b949a4f1381e8ebca3226f0c0e282`. This receipt does not assert hosted CI, release, deployment, or live provider acceptance.

## Root module compatibility

Root production and test module files are byte-identical across the named source heads; hosted acceptance and plan changes do not alter the root module.

- At `bd5d4d9c88cea35425a1d306cde5f424f6a42e90`, `GOWORK=off GOMAXPROCS=2 go test -count=1 -race -tags=integration -p=2 -timeout=5m -coverprofile=<external-SSD artifact> ./...` passed against the owned disposable PostgreSQL 16 database with pg_trgm. Four root packages passed: library 6.745s, CLI 1.433s, runtime 1.268s, MCP 1.342s. Fresh one-minute load was 4.58.
- Root `go build -p=2 ./...` passed at bd5 source, fresh load 5.18. An earlier attempt was held at load 11.32; no command ran and no pass was claimed for that attempt.
- Root `go vet -p=2 ./...` and `go build -p=2 -o<external-SSD artifact> ./cmd/gist/` passed at `2e9dc8fbbf42720cdbf700f32534f6159cb0e150`, fresh loads 3.10 and 5.07.
- Configured root integration-tagged golangci-lint passed with zero issues at `a2379fda6bcccc29ee2700c75e95596cfcb2310a`, fresh load 3.63. Installed v2 used a faithful migration of the repository v1 configuration, without disabling checks.
- Python registry unit discovery passed seven tests; frozen contract checks passed. CLI `--help` exited zero. Local Darwin CLI artifact SHA256: `a0a8c555576350701c7b976c5a5a74246bab72f71f7d68e8d8cf326a7759bdf3`.

Multipackage commands acquired the shared build lease and released only their own returned CAS identity immediately after completion. Caches, temporary files, coverage and CLI artifacts remained on the external SSD. The CLI artifact is not an AWS registry image or release artifact.

## CORE-R11 dispatcher correction

The installed default-plan parser resolves literal task IDs or same-epic namespaces. Bare references to a different epic could block tasks despite completed prerequisites. The correction at `e2be457a50273fa254fdda6175817ae13ced832a` qualifies active cross-epic dependencies without changing task IDs, owners, acceptance criteria or authored completion states. The optional managed gateway was left unchanged.

The coordinator invoked the actual parser with the absolute `docs/plan.md` source and `write_output=False`. A strict literal/same-namespace resolver verified all 230 active tasks, every dependency, no cycles and no duplicate or wave diagnostics. Every active task joins `E-GR-PROD.T-GR-PROD.9`. Comparing existing rows against `782ed45` preserved predecessor semantics on 223 rows; the only additional existing-row edge is the new R11 independent review prerequisite on CORE merge.

Before closing its implementation checkbox, CORE.34 parsed as open because CORE.1 and WIRE.6 were complete. SCOPE.4, SCOPE.5, SCOPE.7 and PUBLISH.8 remained authored open, and PROD.9 remained blocked. The separate CORE.38 verification checkbox records this executed audit, not the earlier metadata-only commit.

## Remaining boundaries

Private publication component rollback can leave a staged content-addressed blob; database catalog and outbox rollback are qualified separately. Canonical publication staging ownership, retention and reconciliation remain required before production. Fixture grants and programmatic clients do not prove external enrollment, named external runtime qualification, provider execution, or AWS deployment.
