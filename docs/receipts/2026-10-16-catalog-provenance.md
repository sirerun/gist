# Catalog publisher provenance source delivery

Scope: retained T-GR-GATEWAY.31/.35 and T-PROVENANCE.0, with independent review,
corrections, merge and landed gates .1-.4. Requested ceiling10, actual three
source outcome members, one inherited-model implementer and a separate reviewer.
Exact model/token/cost telemetry is unavailable. No task-count speed claim.

Base: `16245c67a667447ee3a5d139e7d9a04f16a91ba6`. Existing dirty original checkout
is preserved; source and claims are isolated. Saved plan validates with404 tasks,
source digest `9f9bb531d2d79a53f0c7786b5cb007874731e2f83681fcc2c29691bf250cc54e`.

This implements caller-owned signing and consumer verification interfaces only.
It does not choose a production publisher, enroll a real key, custody private
keys, publish a release, resume hosted expansion or lift the public-distribution
block. T-PROVENANCE.5 remains pending.

## Source candidate and qualification

Candidate `da9dfc51321827250682330e96ef6cf590b4a66d` adds the detached Ed25519
attestation API, external-consumer tests, ADR019 and catalog usage documentation.
Three source members are complete. Caller policy selects exact immutable subject
and fresh authoritative trust. Revocation always rejects; historical retirement
requires explicit policy. All real ZIP assets are validated without changing
operator-asserted manifest trust. Source/schema claims remain publisher assertions,
not independently proven repository ownership or schema conformance.

Tracked archive SHA256 matched locally and remotely:
`8eae6d27f0444937f249e188bdf82553ede2f7c851c6f6a512570639decf4ddb`.
Qualified existing Linux ARM64 host, Go1.26.1, no new infrastructure or credentials:

```sh
GOMAXPROCS=4 GOWORK=off timeout 600 go test -p 4 ./catalog/...
GOMAXPROCS=4 GOWORK=off timeout 600 go test -p 4 ./...
GOMAXPROCS=4 GOWORK=off timeout 600 go vet -p 4 ./...
GOMAXPROCS=4 GOWORK=off timeout 120 go mod tidy -diff
```

All passed; tidy produced no diff. Frozen contract check and all12Python registry
regressions passed locally. No dependencies, frozen wire or hosted behavior changed.
Non-author independent review approved exact source base/head with no blocking
findings. No correction was necessary. Final plan/receipt exact-head review,
guarded merge and fresh-main landed verification remain pending.

## Independent native Zati qualification

Zati source: `aa379b6dbc3da65f5a8ae896a1984524a8497e11`. Pinned Go1.26.2
controller build passed with `GOMAXPROCS=4 go build -p 4 ./cmd/zati` on the Mac.
Flutter3.32.1 locked dependency resolution passed without tracked changes.
Native desktop configuration first failed because CocoaPods was absent. Installed
CocoaPods1.17.0 into an isolated BuildOffload tool directory; no system gem or
Keychain defaults changed. Build lease released after the failed attempt.
Desktop configuration retry passed with CocoaPods1.17.0 and no tracked changes.
Actual isolated Mac headless controller initialization and authenticated catalogue
listing returned completed with one maintained zati/inbox-briefing1.0.0 package;
owned process stopped and ephemeral key/state removed. Initial proof omitted
required installation scope and received400; corrected request succeeded. The
proof first asserted a wrong response wrapper; inspecting the saved response
confirmed the actual data.items shape and its package identity. No provider calls.
Native compilation remains held while one-minute load exceeds10. Visible acceptance,
installation, release and live Gmail acceptance are not yet qualified.
The unrelated existing native Flutter preview and installed controller remain
unchanged.
