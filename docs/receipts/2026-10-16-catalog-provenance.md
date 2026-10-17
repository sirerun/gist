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

All passed; tidy produced no diff. Supplementary changed-package race passed
(1.151s), full root and explicit cmd/gist builds passed, changed-file gofmt is
clean. Exact CI golangci-lint2.14.0 Linux ARM64 official archive checksum was
verified. Candidate full lint reports five findings, reproduced identically on untouched
base16245c6: three errcheck, one staticcheck and one unused. Coordinator inspected
both complete outputs; no introduced finding. Provenance-only lint passes.
Whole-repository lint is not reported green; no exclusions or policy weakened.
Supplementary logs are retained on the qualified host under the candidate's
`verification-logs` directory. Frozen contract check and all12Python registry
regressions passed locally. No dependencies, frozen wire or hosted behavior changed.
Non-author independent review approved exact source base/head with no blocking
findings. No correction was necessary. Final exact-head independent review approved8f63a139b39ebf1428434d90c80ff5e611e2c70f with no blocking findings.

GitHub PR69: candidate hosted CI failed to start because the account is locked
by billing (check117274948325 annotation confirmed). The repository has no
required status checks and zero required approving reviews; no policy was
changed. The owner-authorized local/qualified-host fallback is used, with
independent review and exact-head merge retained. Hosted CI is unavailable,
not passed.

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
Controller binary SHA256:
`cc786bf170c6a764814bf86226df6daf802f989a9bad2a41af76e24c014ff508`.
No build lease remains held; native qualification claim released while
capacity-held. The unrelated existing native Flutter preview and installed controller remain
unchanged.

## Merge and landed verification

PR69: https://github.com/sirerun/gist/pull/69
Reviewed base/pre-merge main: `16245c67a667447ee3a5d139e7d9a04f16a91ba6`.
Reviewed head: `8f63a139b39ebf1428434d90c80ff5e611e2c70f`.
Guarded GitHub rebase landed `e0182846ff48e69acf25c8b48820bb2ae53ece06`.
Fresh main reaches that SHA; complete reviewed/landed tree comparison is equal.
No required checks/approval or unresolved review thread was bypassed. Hosted CI
remains billing-unavailable; source fallback evidence and baseline lint limits
above apply.

Actual landed tracked archive SHA256 matched local/remote:
`609ec87941c774a52a1ed1bdca11fdfab6b5d6e97cea5414143a929810cb889c`.
On the qualified Linux host, bounded `go test -p 4 ./catalog/provenance
./catalog/packages ./catalog/integrity` passed (integrity has no standalone tests).
Frozen checker and Wazi404-task validation passed against the identical landed
plan/source. Retained source members and review/fix/merge/landed gates are complete.
No release, enrolled production publisher or public-distribution qualification is
claimed. Native desktop build/visible acceptance and live Gmail stay pending.
