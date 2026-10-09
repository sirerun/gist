# Publication v2 source verification

Initial tested changed tagged source: `fb45bea5e416fdc8e66ba941afb81f2cdc856532`. Base: `8a24c72a596a757e13426945911d87446cc3a4e0`. Execution: isolated DGX worktree, Go1.26.1, PostgreSQL16.15, owned filesystem store and actual TLS HTTP. No Mac build, credential migration, AWS deployment, live S3 or external provider execution.

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

## PUBLISH60-R1 lifetime correction and refreshed source

Original independent lead, decoder and storage reports approved exact
head864b4927270e0fee4ae93f4add566e209950b8e8 at the same base.
They remain historical: coordinator subsequently observed two actual runtime REDs
at that source: successful owned-object read after completed shutdown and root
descriptor count1to2 after denied construction. Corrected code releases the pinned
root exactly once after HTTP and both janitors finish; failure construction releases
newly owned resources and joins actual errors. Existing nil-store shutdown fixtures
remain compatible; no S3 deletion or credential action is introduced.

Final changed source is `56b3236af2ed853566db4f3d8fa0c480db450074`.
The first correction's full checks exposed a test field typo and the existing
nil-store fixture panic; these were corrected, and original R4 failed receipts are
retained. The original two-case attempt that could not compile is not behavior RED;
`publication-lifetime-both-actual-red.log` is the actual two-case runtime RED.
R5 full hosted unit, complete app/storage real-store integration, wiring/retrieval,
full hosted race, publication real-store race, vet/build and delta lint all pass
at the corrected SHA. Linux descriptor-observation test executes here; it does
not qualify non-Linux descriptor behavior. Required new final-head independent
review remains PUBLISH.17, with merge still gated. Root full race also passes on
unchanged root-module bytes at original candidate864b492.

R5 receipt integrity: `sha256:364e4ad21f8e3c3dbb431b6598b206d473348e295898921faeeed30d17ab22cb`.
Actual lifetime RED integrity: `sha256:d5e8a6b9876d20797749e893188c19fabcdea9aef8acbbfe7738a15a1bec4f8e`.
Root race log integrity: `sha256:0afaab55ca5df6c83c06cf1943c12a6a07460c9c9747ce9400c3e5378a1665b7`.

## Accepted LEAD-R1 active-handler drain

The independent corrected-lifetime lead requested changes at0006a12:
Server.Close cancels connections but does not wait for active handlers.
The original report is preserved in publication-drain-review.md.
A controlled actual TLS handler using the real owned object reader observed
cleanup completion while it was still admitted. The initial negative fixture
had teardown ordering that blocked release; only the corrected, completed
`publication-drain-completed-red.log` is the behavior RED.

At corrected source `a0df51a038544f907dc2b90e97cb4586de4e21be`, a mutex-bound admission fence stops
new request admission before a WaitGroup drain. After forced HTTP connection
close, cleanup keeps the object root and SQL pool until admitted handlers and
both janitors have actually returned. The shutdown caller still returns on its
deadline; repeated callers observe the retained actual HTTP drain error.
The controlled reader finishes with identical real stored bytes, new admission
denies503 and final cleanup closes the root. Shutdown errors use bounded canonical
JSON and a generated request ID, without echoing incoming identity metadata.

R6 full hosted unit, complete real app/storage integration, wiring/retrieval,
full hosted race, publication real-store race, vet/build and delta lint all
pass at this exact source. Full candidate diff check also passes, including
normalization of one trailing blank line in the retained original review report.
Root-module source and both v1/v2 wire inventories remain unchanged from base.
PUBLISH.18/.19 are qualified; independent corrected-head PUBLISH.20 review plus
PUBLISH.4/.17, guarded merge and actual landed proof remain open.

R6 receipt integrity: `sha256:0425267665fc9939047f1cdea595964e05af448cae6c562731aec53922acfd2f`.
Completed active-drain RED integrity: `sha256:44ec1b4fd1623bbd8cc74e94a3bc6ff41d4fe7ecf59386d6b03415373d9a7121`.
