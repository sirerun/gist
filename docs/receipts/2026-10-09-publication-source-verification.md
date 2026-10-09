# Publication v2 source verification

Tested final changed tagged source: `fb45bea5e416fdc8e66ba941afb81f2cdc856532`. Base: `8a24c72a596a757e13426945911d87446cc3a4e0`. Execution: isolated DGX worktree, Go1.26.1, PostgreSQL16.15, owned filesystem store and actual TLS HTTP. No Mac build, credential migration, AWS deployment, live S3 or external provider execution.

## Observed behavior

Old-source actual TLS HTTP returned404 for all six v2 publication routes before implementation (retained publication-baseline-red log). The new actual composed app publishes all six kinds201, returns original document/package bytes and correct digests, and replays the exact durable receipt200. Only namespace/trusted synthetic review setup is seeded; catalog success records come from authenticated HTTP. Runtime PostgreSQL role is NOSUPERUSER/NOBYPASSRLS and attempts/identity tables force RLS.

Actual checks cover malformed/duplicate/escaped-duplicate/UTF8/surrogate/trailing JSON, byte budgets before mutation and lower configured request/ZIP/expansion caps; inactive/wrong-tenant/expired/revoked policy, membership, namespace, scope and review evidence; current-rights replay denial; restarted replica replay, changed-version identity conflicts and corrupt-object read denial; revoked binding dependencies, missing verifier, tampered retained fixture/result, absent required capability and exact document-pin compatibility. The test-only binding executor actually runs five retained input/output conversions against both schemas; this qualifies synthetic source behavior only.

Restricted-role real-store checks cover alias concurrency, atomic catalog/outbox rollback and retry, durable idempotency, cleanup deletion claims/tenant fences, failure fairness, permanent retired-key tombstones and late PUT/new-key retry. Filesystem path/link/symlink/corruption tests and fake conditional S3 tests pass. Actual revoked-row owned reads and unretained local-file schema loading each had a meaningful retained RED and corrected PASS.

## Check receipts

| Receipt | Revision | Result |
| --- | --- | --- |
| root | `c48899cf76d8714d9114bd8a1b37840f6d23139a` | go test ./...: exit0; go vet ./...: exit0; go build ./...: exit0; /home/ndungu/go/bin/golangci-lint run --config /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/lint-derived/.golangci.yml ./...: exit0; /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/openapi-venv/bin/python scripts/registry/check.py contracts --freeze-check: exit1; /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/openapi-venv/bin/python -m unittest discover -s scripts/registry -p *_test.py -v: exit0; /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/openapi-venv/bin/python contracts/registry/v2/validate.py: exit0 |
| hosted | `c48899cf76d8714d9114bd8a1b37840f6d23139a` | go test ./...: exit0; go test -tags=integration ./internal/app ./internal/storage -count=1: exit0; go test -tags=integration ./acceptance/wiring ./acceptance/retrieval -count=1: exit0; go test -race ./...: exit0; go test -race -tags=integration ./internal/app ./internal/storage -run TestPublicationV2 -count=1: exit1; go vet ./...: exit0; go build ./...: exit0; /home/ndungu/go/bin/golangci-lint run --config /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/lint-derived/.golangci.yml ./...: exit1 |
| r2-hosted | `1ce7ef81ce2bf5caa7ddeaf51b21440c6487bd53` | go test ./...: exit0; go test -tags=integration ./internal/app ./internal/storage -count=1: exit1; go test -tags=integration ./acceptance/wiring ./acceptance/retrieval -count=1: exit0; go test -race ./...: exit0; go test -race -tags=integration ./internal/app ./internal/storage -run TestPublicationV2 -count=1: exit1; go vet ./...: exit0; go build ./...: exit0; /home/ndungu/go/bin/golangci-lint run --config /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/lint-derived/.golangci.yml --new-from-rev=8a24c72a596a757e13426945911d87446cc3a4e0 ./...: exit0 |
| r3-hosted | `fb45bea5e416fdc8e66ba941afb81f2cdc856532` | go test -tags=integration ./internal/app ./internal/storage -count=1: exit0; go test -race -tags=integration ./internal/app ./internal/storage -run TestPublicationV2 -count=1: exit0; /home/ndungu/go/bin/golangci-lint run --config /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/lint-derived/.golangci.yml --new-from-rev=8a24c72a596a757e13426945911d87446cc3a4e0 ./...: exit0 |
| r3-root | `fb45bea5e416fdc8e66ba941afb81f2cdc856532` | /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/openapi-venv/bin/python scripts/registry/check.py contracts --freeze-check: exit0; /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/openapi-venv/bin/python -m unittest discover -s scripts/registry -p *_test.py -v: exit0; /home/ndungu/worktrees/gist-plan-ship-20261008-root/tooling/openapi-venv/bin/python contracts/registry/v2/validate.py: exit0 |

The first full hosted lint run found13 issues: eight introduced findings were fixed; the same qualified lint configuration at the actual base reports the remaining five (two cache type assertions, two existing reader-close checks, one existing digest simplification). Final v2 delta lint passes with `--new-from-rev=8a24c72a596a757e13426945911d87446cc3a4e0`. Root full lint passes. These are local checks with a tool-derived v2 config from the unchanged repository v1 config, not hosted CI success.

The R2 hosted tagged build failure was a coordinator fixture variable scoped in the wrong test, fixed and reverified by the R3 full app/storage suite and publication race suite. The original R2 failure remains in the table. Root Go checks from the first receipt reuse unchanged root-module bytes; R2 hosted unit/race/build/vet checks reuse unchanged production and untagged test bytes (the only subsequent hosted delta is the corrected tagged storage fixture). R3 refreshes the affected real-store and race acceptance at the final source SHA. Final contract freeze, Python9tests and v2 schema/OpenAPI gates pass at that SHA.

The unchanged v1 lock pins catalog.go as a source artifact. The explicit additive `contracts/registry/implementation-v2.json` amendment pins original/current source hashes and reconstructs the entire original source outside the approved new fields. It cannot override wire files. Both original v1 and v2 wire inventories and locks remain byte-identical to base.

Raw local command logs, durations, exit codes and source revisions remain in the retained task record. Their integrity hashes:

- publication-quality-root/result.json: `sha256:9a59f1233a36ad77fb2dc446ce158ce6e2f7d949d9e7402775a2be23b39427ad`
- publication-quality-hosted/result.json: `sha256:3334bd940bbf354278afd3e5591b3c8614ecc68a6f10c88e78c27ce1b8361be9`
- publication-quality-r2-hosted/result.json: `sha256:6f41092552d7c9e280c0cc833f124522e814337f3a346c5057533655bb55a09a`
- publication-quality-r3-hosted/result.json: `sha256:be2c50dcad8ff1f62e137ab1aad5486e2771066084458d22997d7c604ab3d4f7`
- publication-quality-r3-root/result.json: `sha256:97d9850395a9a7a493d1af097ed7e59ea0c4cb90fa52bd1ccb07a672bd15a6be`

## Gate disposition

PUBLISH.1/.2/.3 source implementation and verification are satisfied; PUBLISH.4 independent review, .5 guarded merge and .6 landed verification remain open. The full plan is not complete. This candidate adds only opt-in source composition; it does not select an operator issuer, provider workspace, credentials or production verifier, or enable v2 in a deployed service.

Default AWS CLI STS preflight returned NoCredentials (exit253). AWS account/role/stack binding, invited-operator enrollment, actual provider capture/license/account/target/numerical bounds, independent consumer and production release/deployment/live acceptance remain open. Existing GitHub hosted jobs are billing-unavailable; local verification is the authorized fallback, with unchanged protection/policy.
