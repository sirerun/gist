# Bounded composition index source verification — 2026-10-17

Scope: T-COMPOSE.15–.16, source verification .29 and corrections .33–.34.
Base `d0e97da28be399e2a29a7a9f8184f163fe7f70c7`; corrected source
`e4697fe4990b365fd97c01783bb8ab952b599639`. Final documentation review,
merge and landed verification are separately recorded in the PR and plan handoff.

## Implemented source

Additive `discovery.ScopedIndex` provides lexical postings, exact artifact lookup
and composition binding lookup under caller policy and principal/runtime scope.
It bounds query work, candidate counts and response bytes, reports exhausted
budgets explicitly, and fences concurrent content publication and current policy
before exposure. Updates and rollback publish monotonic generations; rollback
never restores authority. No result cache or default semantic route is added.

The existing discovery service is unchanged. This API does not itself execute
or prove readiness. Its caller must invalidate policy scope when connection
admission changes. Updates copy top-level maps in O(N); in-flight readers can
retain old snapshots. These costs are explicit in the frozen workload.

## Verification attempts

Aggregate candidate `16699bddde0ecfcf20ef30e062dca85dbc2cb09e`, archive SHA256
`11758c6454dabc2b67ce29432257c5bf75874b6e811209ed2fd049661817a21e`:
qualified Linux ARM64 worker, Go1.26.1, cached golangci-lint2.14.0,
offline dependencies, four runtime workers/two package workers, 24GiB limit,
no swap, 1200second watchdog and an owned PostgreSQL16 fixture.

- Full root integration/race suite failed the new lexical fixture assertion:
  the unrelated calendar entry inherited the helper's mail tag, correctly
  producing three matches instead of the asserted two. Other suites passed.
- Full root vet/build, CLI binary build, frozen contracts, 12 registry Python
  tests and unchanged dependency tidy check passed.
- Lint failed with one new De Morgan suggestion and five inherited findings.
  The aggregate service exited1; it is not an all-green result.

Correction `e4697fe` gives the calendar negative fixture its own calendar-only
metadata and keeps the exact top-ranked mail/read and two-hit assertions. The
hex predicate rewrite is logically equivalent and removes the new lint finding.
Corrected exact-source archive SHA256
`d1df781421433cacd39642220fb4fe003ae4b992e4aca88e1e6dbccb48ca2737`
passed the complete discovery race package (1.039seconds). Root lint still exits1
with exactly the same five inherited findings and zero introduced. Unchanged
aggregate checks above remain source-bound to16699bd; only the two documented
corrections changed executable/test source. The bounded correction service
finished in3.219seconds and is inactive, with no database or model started.
Independent reviewer composition_doc_audit found no blocking source findings on
e4697fe; final documentation-head approval remains a separate gate.

The five inherited lint findings match the previously qualified producer
baseline: discovery/cache.go47 and64 unchecked assertions;
packages/retrieve.go50 unchecked Close; integrity/digest.go69 De Morgan;
discovery/index.go258 unused cacheKey. No inherited lint cleanup is bundled.

## Evidence limits

This delivery contains source tests, not the frozen 10k/100k evaluation.
T-COMPOSE.17–.21 remain pending the landed Zati consumer and an independently
qualified inventory/cohort. No latency, memory, token-efficiency, readiness or
provider acceptance claim follows from source checks. Hosted CI cannot start
under the existing billing lock; unchanged authorized local verification fallback
applies only where repository protection allows it.

Current attended executor and inherited model/effort; requested batch ceiling25,
actual two implementation/verification members plus separately tracked fixes.
Token/cost telemetry unavailable; no acceleration claim. No release, deployment,
installation, model calls or provider calls.
