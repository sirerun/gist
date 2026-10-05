# CORE acceptance fixture verification

This receipt covers the fixture-only R9/R10 adaptation on source head
`7bddbe7f42a0f34311fceaa504be099e37aeda63`, based on `6243787610459815dec8240620d14076e54e61f9` and integrated for comparison at
`a2379fda6bcccc29ee2700c75e95596cfcb2310a`. Production app and package source
were not changed by this lane. The only difference between the owned client
fixture and the integrated candidate is an equivalent boolean expression used
to satisfy the configured linter.

## Fixture changes and regression evidence

- On the unadapted baseline, the tagged wiring startup test failed at
  `app.New`: `GIST_SIGNING_KEY_CONFIG is required and must be valid` (the
  fixture had no valid signing key configuration). This is the R9 startup RED.
- The adapted fixtures supply an ephemeral test Ed25519 key set and seed a
  dedicated tenant-scoped maintenance principal with only `catalog:publish`
  before `app.New`. Existing reader, client, and adversary grants remain
  unchanged. Retrieval, wiring, and client fixtures include migration 008 in
  their explicit migration lists where those lists are maintained manually.
- After startup was corrected, the old wiring request shape received HTTP 422.
  The corrected request uses the frozen canonical resolve fields. Resolve then
  returned HTTP 503 because the synthetic catalog metadata omitted the
  manifest version; the fixture now seeds the exact manifest bytes from its
  test package. The corrected wiring suite asserts successful `ready` output
  and revoked-artifact denial. The m2a fixture likewise checks the frozen
  response `aggregate` and retains its successful `ready` assertion.
- The client test exercises the programmatic MCP transport seam with synthetic
  build-label environment values. It does not qualify an installed CLI,
  external MCP process, or provider runtime.

## Checks run on the owned source

All Go and lint commands were individually gated on host load, used the local
PostgreSQL integration fixture where tagged, and used the configured v2 lint
configuration. The following passed:

| Package | Tagged PostgreSQL tests | Race | Vet | Configured lint |
| --- | --- | --- | --- | --- |
| `acceptance/wiring` | pass | pass | pass | pass, 0 issues |
| `acceptance/clients` | pass | pass | pass | pass, 0 issues |
| `acceptance/retrieval` | pass | pass | pass | one existing finding below |
| `acceptance/testfixtures` | n/a | n/a | n/a | pass, 0 issues |

The initial retrieval lint run reported the unowned finding
`acceptance/retrieval/metrics_test.go:306` (`ineffassign`: ineffectual
assignment to `unauthorized`). The coordinator removed the redundant
assignment in commit `dda50c947d14c009bd749273efbcc67ba86b7172`; after that
fix, retrieval and all acceptance packages passed configured lint as recorded
below.

## Follow-up verification after the lint fix

Source head: `0e250fb` (`test(retrieval): remove unused workspace denial
assignment`), following the owned fixture and receipt commits. I cherry-picked
the coordinator's one-line test-only cleanup; it does not change assertions or
production code.

On the updated source, these gated commands passed:

- `go test -count=1 -tags=integration -p=2 ./acceptance/retrieval`
- `go test -race -count=1 -tags=integration -p=2 ./acceptance/retrieval`
- `go vet -tags=integration -p=2 ./acceptance/retrieval`
- configured v2 `golangci-lint` on `./acceptance/retrieval`: 0 issues
- configured v2 `golangci-lint` on `./acceptance/...`: 0 issues

The acceptance-wide lint held the shared build lease and released it
immediately after the command. The retrieval integration and race commands
used the local PostgreSQL fixture. No other package source changed between
the prior package checks and this follow-up.

## Related evidence supplied by the coordinator

The CORE-R6 zero-budget regression was independently reproduced in an isolated
worktree at source `8f07de04c889138ac21b7d9ec33da41e8f7c565d`. The single
integration test file was taken from overlay commit
`fe8cda3486e02b8561ea7ceb06b98fa351d30c7b`; the test name was
`TestPublisherCatalogAndOutboxShareTransaction`. With the local PostgreSQL
fixture, the command
`go test -count=1 -tags=integration -run '^TestPublisherCatalogAndOutboxShareTransaction$' -p=2 ./internal/app`
failed as intended: `core_publication_transaction_integration_test.go:122:
response budget 0 did not reject before publication: <nil>`. Thus a zero-byte
response budget still allowed publication on the pre-fix source.

The corrected implementation is
`08f9b012967f251b434768c2c214512b1daad573`, with overlay
`fe8cda3486e02b8561ea7ceb06b98fa351d30c7b`. The coordinator reports that the
corrected PostgreSQL case passed twice and records its associated race, vet,
and lint results. In addition, I ran the same exact named test against the
corrected current source at `d6d65da87484a958031281438917ce8c7461c898`:
`go test -count=1 -tags=integration -run '^TestPublisherCatalogAndOutboxShareTransaction$' -p=2 ./internal/app`
passed against the local PostgreSQL fixture. The integration test file's git
blob is identical to the overlay file from `fe8cda3`. The RED and this
corrected-source PASS are direct results; the previously reported repeated
checks remain coordinator-attributed evidence.

The coordinator reports that its final physical-blob test on source
`bd5` passed all eight PostgreSQL tests. That result is separate from this
fixture lane and is recorded only as coordinator-supplied evidence.

## Scope boundary

These results qualify the named local fixture and package checks only. They do
not establish canonical HTTP publication, installed-client or external MCP
runtime acceptance, provider integration, hosted CI success, or production
readiness. No provider, cloud, credential, or keychain action was performed.
