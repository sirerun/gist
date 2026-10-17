# Gist-backed Zati marketplace delivery

Scope: T-MARKET.1-.4, requested batch ceiling5, actual4 outcome members under one
retained source implementer. Coordinator owns integration, plan and evidence;
non-author review and guarded rebase/landed gates follow .5-.8. Current session
model/reasoning inherited without overrides. Exact model/token/cost and per-stage
timing telemetry are unavailable; no batching performance claim is made.

## Accepted boundary and preflight

Fresh source bases: Gist `d9290553cefa290e12a50f1b35f8e3512e65516e`, Zati
`aabfdc5e0d483c2196d32d95f7696ff9b1dc7b63`. Existing dirty original Gist checkout
and unrelated running Zati Flutter preview were preserved. New task worktrees
and exact task/source claims isolate this assignment.

When asked whether to resume hosted expansion, the owner asked what the existing
plan/RFC/ADRs say. RFC002 and ADR016 prioritize Zati with Gist as a library;
hosted expansion remains paused except repairs/current consumer obligations.
This batch follows that scope and supplies no deployment or release authority.

The existing desktop Services/configuration/review/secure-command-recovery path
is reused. New controller catalogue/import operations supply actual maintained
Gist packages and derive tool schema pins through the existing Connections owner.
Human choices explicitly assert list/read semantics. Zati retains all activation,
credentials, permissions, budget, approvals and execution. No inferred provider
conformance, automatic permission grant or fabricated hosted catalogue.

## Qualified checks and capacity

Existing Linux ARM64 host: Go1.26.1 installed with module Go1.26.2 toolchain,
load0.81,1.5TiB available storage; bounded Go commands use four workers, no new
infrastructure or credentials transferred. Native Flutter checks use the Mac and
shared build lease when capacity permits. Pinned Flutter3.32.1/Dart3.8.1 matches
the installed SDK. Locked pub resolution completed without dependency updates.
The unrelated running Flutter source is not this task's preview.

Baseline actual Zati main reports157 uncapped golangci-lint2.11.3 findings:
136errcheck,1ineffassign,14staticcheck,6unused. Full-tree gofmt names only unchanged
`internal/configuration/schemas.go`. These are baseline findings, not passing
whole-repository lint/format results. Candidate deltas will be checked exactly.

Implementation, behavior checks, visual observations, review and landing evidence
remain pending. No Gmail connection, live mailbox result, measured performance,
release, installation or hosted-production qualification is asserted here.

## First implementation checkpoint

Tracked-source checkpoint74d60fe6 archive SHA256 matched local/remote:
`800ba3abe11ae74055ba4fe718bcd27df1677b736f97b5168f2ff41bfe4affb9`.
The first narrow Go run correctly failed: the new import used pointer/value
forms incorrectly in two places, and three registry tests still expected
revision45 rather than intentional46. Connections compiled; no unmatched test
selection is counted as behavior acceptance. Findings returned to the source
owner for correction before aggregate qualification.

Mac load subsequently exceeded10 (observed31.39), so native heavy checks are held
under the shared runtime rule. Existing preview remains unchanged. This is a
capacity limitation, not a native test pass or an installed-runtime claim.

Checkpointcbf54eee archive SHA256 matched:
`465a0fd75ffc794545c2e0584284bf861cf9472983351f19668018d4f6ab472d`.
Selected Skills/catalog, Registry/OpenAPI and CLI import tests passed; Connections
and Application compiled with no matching tests in that narrow selection.
Flutter analysis on an immutable extracted source snapshot found14 issues:
two missing disposition-type imports and12 required control-flow brace lints.
These were returned to the implementer. Analysis used pinned SDK and a held
shared build lease. The lease cleanup initially ran outside a Git repository;
coordinator immediately released the exact owned token from the task worktree.
No lease remains held by that attempt. Native build/visual acceptance is pending.

Checkpoint8d8a0494 archive SHA256 matched:
`6d93a5adc074af5cfb714dda6275f77fda00bd572a2e02ddb2a2ffd9261a79cf`.
New Skills owner cases passed, including stale selection at Finish and rejected
uninspected/unsupported selections. The assembled marketplace path failed with
`registered local IO operation skill.catalog.import has no owning LocalIO service`.
This exposed missing application routing beyond descriptor registration. The
source owner received the concrete failure; this checkpoint is not accepted.

## Aggregate source checks and corrections

Checkpointe88249f1 archive matched:
`a6ff589eb73832e33e82edd8edc9c072246b4ab311b738a3bd4695b7edf63364`.
Full Skills, Connections, Effects, MCP and CLI suites passed. Application and
Registry tests exposed old227-operation assumptions; intentional revision46
contains229public operations. Three Registry assertions and one Application
assertion were updated without changing accepted operation behavior.
Vet, dependency lock, module verification and tidy passed. An unused testfield
was the sole introduced uncapped lint finding; it was removed.

Checkpointe7b797df archive matched:
`47738d8b25867ba94e52ae9868ecb724fa131310a43cf784578ac5dcaff36a26`.
Full Application, Registry and Skills suites passed. Renderer and all42Python
specification tests passed:43scopes,351operations,156sourceblocks,152cases.
The new realHTTP test first failed because its transport credential had not
been provisioned. Correction uses the existing real fixture credential owner,
not an authentication bypass. A subsequent test erroneously treated the new
positive scenario as a negative case; that classification was corrected.

Checkpointe7711e4a archive matched:
`d43cb023e7c2816acf2c9462e61af2ece44cd3ec62c706cb3b1de18811f81c24`.
Dart analysis passed with no issues. Initial widget run passed empty, skilled
worker refusal, settled refusal and disposal/import recovery scenarios; positive
setup tested too early using a fixed delay, and existing Services fixture/viewport
assumptions needed adaptation. These failures remain recorded, not counted green.
Single-worker Dart checks used the shared lease and no Xcode build; the lease
was released. Heavy native builds remain capacity-gated.

Checkpointd86a7a72 archive matched:
`aeefdef10ca5fd5514079a4f23156b4ffef308f7c88c632988298d11833de03e`.
The actual HTTP-envelope catalogue/import test, including denied false semantics
assertion, passed through real credential verification and one configuration
draft containing connection/tool bindings, imported skill and worker update.
Required review/apply and actual instruction/template context followed; retained
worker instructions and exact skill/binding closure were asserted. Focused test
passed in2.981s. Full vet passed; uncapped lint exactly matches the157 baseline
filename/linter/message multiset, no introduced finding. This proves controlled
source behavior, not a live mailbox or installed application.

## UI regression qualification

Checkpointd90b83ff tracked archive SHA256:
`8dcde148b3bb163a7818955bbdb7407df95031e90e41337405c5786f7c2630d5`.
All12Marketplace/Services widget cases passed in65s after correcting the fixture
query routing and semantic field selectors. Existing disposal/recovery assertions
were retained. Earlier service API and operation-catalog parity tests also passed.
Positive multistage checks now wait for observed owner stages and settled UI
rather than assuming a fixed delay proves completion.

Final UI audit added a journal-write lifetime fence: disposing a panel cannot
release setup ownership until its in-flight secure-storage operation completes.
Checkpoint487d6a52 tracked archive SHA256:
`b005b70566e8d81c57d198e4c3873a589a58f6cd6c9dfb0edff30ddfc444bea0`.
Its delayed-storage regression and rendered-widget preview are being checked.
All changed Go formatting is clean; nine changed Dart files have zero format
changes. Ordinary whole-tree Go formatting still has the unchanged baseline file.

## Final source and visual review

Final code snapshot2045fde1 archive SHA256:
`70d78a49352adcec41db003c44abca145173a0dec4265298747563ec4cbe32f9`.
Dart analysis passed with no issues. All six Marketplace cases, including the
new delayed secure-write/disposal race, passed. A separate temporary visual
fixture rendered the actual MarketplacePanel against synthetic UnixHTTP data;
all seven checks passed in85s. The fixture is outside production source and uses
memory-only credentials and no account/provider access. No running unrelated
preview was changed.

The initial Flutter test screenshot used Ahem glyph blocks and was unsuitable
for reading. A repeat of only the visual fixture loaded a system text font and
Material icons; it passed and produced [the readable image](2026-10-16-zati-marketplace.png).
Coordinator inspected that image: maintained Inbox Briefing, search, package
inspection and service selection are visible with no overflow. This is rendered
widget evidence with a fixture theme/font, not a full installed application
screenshot or the final native host styling.

Fresh full native builds were held because Mac load exceeded10, reaching25.62.
The plan now names the actual rendered-widget review and records unavailable
native build evidence rather than upgrading it into a pass. Release/installation
remains outside scope; no production window, live Gmail or native full-build
acceptance is claimed. Existing preview and signing/Keychain state were preserved.

Final Zati source `3d8ff25bbf2714fa504ff294caba9167f17aaffd` differs from checked
2045fde1 only in documentation/generated context: it reconciles the old blanket
marketplace exclusion with the maintained-catalogue exception and gives exact
Connectors → Connections → Services navigation. Renderer and42Python checks
passed after that documentation change; functional source checks remain valid.
T-MARKET.1-.4 are source/check complete. Independent review, correction/re-review,
merge and landed verification remain open and are recorded below when observed.
