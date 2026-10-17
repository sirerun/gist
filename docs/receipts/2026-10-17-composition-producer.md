# Composition producer source verification — 2026-10-17

Scope: T-COMPOSE.1–.4 and corrective .26–.27. Source candidate
`51e97853ab492e197a22f1659e0686dc29408d29`, base
`d01351634733ff37c8d65c216f7688d01cc9aaf5`. Documentation closeout may advance
candidate SHA without changing verified source. Final review/merge/landed evidence
is recorded by the PR and subsequent plan handoff; this receipt alone is not landing.

## Implemented source

`catalog/composition` parses and validates bounded typed composition, resolves
exact caller-indexed contracts/bindings/tools/providers, checks shared connection
and account identity, performs scoped fresh authority checks and returns complete
verified package files and schemas within the caller budget. It exposes runtime
value/accounting helpers but executes no tools. `catalog/skills/inboxcomposition`
provides the new immutable inbox2.0.0 fixture and separate list/read contracts.
Old inbox1.0.0 bytes, frozen wire contracts and existing preparation API are unchanged.
Zati migration, general indexed discovery and 10k/100k results remain pending.

## Verification

Exact-source archive SHA256:
805e1c918fa95c4d7f4bc14d3ea9ef5b16f8d4fc042f9cb559d967a004d15dd8.
Qualified existing Linux ARM64 host, Go1.26.1 and cached golangci-lint2.14.0.
Offline dependencies; four runtime workers, two package workers,24GiB memory
limit, no swap and1200second aggregate deadline. Owned PostgreSQL16 fixture
stopped after checks, with no remaining owned postmaster PID. The systemd
memory accounting reported256KiB despite Go builds; it is not a reliable peak-RSS
measurement and supports no resource-performance claim.

| Check | Observed result |
| --- | --- |
| Full root integration tests with race and coverage | PASS,21seconds |
| Full root vet/build and CLI binary build | PASS |
| Frozen registry contract check | PASS |
| Registry Python suite |12 PASS |
| Dependency tidy diff | PASS, no changes |
| New fixture frozen manifest/capability schema and examples |2 PASS |
| Full lint | FAIL,5 inherited findings;0 introduced |

Baseline `dc44b395ab5140dfba112998dad659f59f5ea88b` contains only planning docs
beyond base; same host/toolchain/full suite passes. Its five lint findings match
candidate file/line/linter/message: discovery/cache.go47 and64 unchecked type
assertions; packages/retrieve.go50 unchecked Close; integrity/digest.go69
De Morgan suggestion; discovery/index.go258 unused cacheKey. Full candidate
job exit1 preserves lint failure; it is not reported as all-green CI.

## Review dispositions

Independent reviewer `composition_doc_audit` reviewed initial5cb0491 and corrected
51e9785 againstd013516. All reported source blockers were resolved:

- COMPOSE-R2: fractional JSON numbers rounded into integers. Exact bounded decimal
  checking now precedes integer acceptance; adversarial exponent/precision tests.
- COMPOSE-R3: prepared context omitted verified skill instructions/assets. Complete
  digest-bound package content and exact instructions now survive preparation.
- COMPOSE-R4: early refusal results could exceed caller byte limits. One result
  guard covers every successful return; an unrepresentable refusal is an error.
- Implementer self-review corrected new fixtures to actual frozen registry schema
  fields; separate inventoried conformance metadata retains read-only semantics.

Design review additionally fixed exact provider reference semantics and hard
parser/schema/candidate ceilings before code. No remaining blocking source
findings were reported on51e9785; final candidate review remains separately bound.

Current attended executor and inherited model/effort throughout. Requested batch
ceiling25; actual3 implementation/verification members, with separately tracked
review corrections. Token/cost telemetry unavailable; no acceleration claim.
No model/provider calls, hosted deployment, release, installation or adoption.
