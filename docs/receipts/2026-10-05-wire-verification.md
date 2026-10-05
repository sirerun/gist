# WIRE verification receipt

Source: `d815edf3814f57ac9372288065a62f2cdcabd6aa` on the WIRE branch, rebased onto landed `origin/main` `8d11f53d3d94b5f4ba20545354c375708838edcf`.

The resolver now persists a `ports.PinnedResolution` through the additive `PutPinnedResolution` store interface. The frozen `Resolution`, `Finding`, and `ResolutionStore` declarations remain intact; coordinator-owned frozen files `policy.go` and `events.go` match `contracts/registry/v1/lock.json` byte-for-byte. Additive pin types preserve the root skill pin, binding pin, full closure, provenance, and requiredness.

Local checks on this source:

- `go test -p 2 ./internal/app` passed.
- `go test -p 2 ./internal/resolution` passed.
- `go test -race -p 2 ./internal/app` passed.
- `go test -race -p 2 ./internal/resolution` passed.
- `go vet -p 2 ./internal/app` and `go vet -p 2 ./internal/resolution` passed.
- `golangci-lint run --config /Volumes/BuildOffload/worktrees/gist-recovery-20261005-fixtures/lint-config/.golangci.yml ./internal/app ./internal/resolution` reported 0 issues.
- `go test -tags integration -p 2 ./internal/app -run '^TestPostgresPinnedResolutionRoundTripAndPrincipalBinding$' -count=1 -v` passed against local PostgreSQL. The fixture creates a `NOSUPERUSER NOBYPASSRLS` role and verifies full pin/closure roundtrip, wrong-principal and wrong-workspace denial, and expiry denial.

The real REST and MCP handler parity fixtures exercise the same canonical resolver for success, malformed casing, malformed runtime identifiers, ambiguous bindings, foreign-workspace binding candidates, revoked binding pins, and gateway-required bindings. No provider or cloud calls were made. These are local checks, not hosted CI or provider/runtime acceptance evidence.

## R6 transport follow-up

R6 regression tests were added at `dc8adc6` and failed on the prior WIRE head `779276051d14f60f542e00fb530c158e4cff13de`: REST and MCP both accepted duplicate `skill_ref`, runtime `id`, and `max_bytes` keys. Coordinator fixes `3ba74a0` preserve raw canonical request JSON through REST/MCP, and `7c2e9d5` makes private session-index assertions fail closed. These checks passed on final source commit `7c2e9d55f6828118d480b739a964e2b487f813a7`:

- `go test -p 2` and `go test -race -p 2` passed for `./internal/app`, `./internal/rest`, and `./internal/remotemcp` individually.
- `go vet -p 2` passed for those three packages individually.
- Configured golangci-lint v2 across all three packages reported 0 issues.
- Duplicate keys and trailing JSON are rejected by REST; duplicate keys are preserved through MCP argument routing and rejected by the same canonical resolver.

## R7 PostgreSQL fixture isolation

Final source: `5406596760e77b03ca85562704e16ce1eadbf137`, rebased onto landed `origin/main` `f162015238a94fbc930865f70bef014865f553d4`.

The integration helper now generates independent 16-hex cryptographic suffixes for each database and restricted role. Its names stay below PostgreSQL's 63-byte identifier limit. Cleanup is registered immediately after successful creation and targets only those generated resources; role cleanup runs before database cleanup and closes restricted connections first. No existing global role or shared fixture is dropped. Repeating the integration package twice completed successfully, covering repeated creation and cleanup without stale-name collisions.

Final local checks, each passed at this source revision:

- `go test -count=2 -tags=integration -p=2 ./internal/app` against the local PostgreSQL fixture.
- `go test -count=1 -p=2 ./internal/app`.
- `go test -race -count=1 -p=2 ./internal/app`.
- `go vet -p=2 ./internal/app`.
- Configured golangci-lint v2 for `./internal/app` reported 0 issues.

Every Go/lint command was launched through the fresh `gated-command.py` load gate; commands held at load above 10 were retried only after a fresh reading was below 10. All caches and temporary files were on the external SSD. These results are local verification, not hosted CI or runtime acceptance.
