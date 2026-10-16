# Zati catalog consumer source qualification

Scope: T-CATALOG.1/.2, with independent review, corrections, guarded merge and
landed verification tracked by .3-.6. Requested batch ceiling10, actual2 members;
one retained implementer owns Zati production/specification source, coordinator
owns the controlled transport test and cross-repository evidence, and a separate
non-author reviews the final candidate. Session model/settings are inherited;
model identifier, tokens, cost and stage timing telemetry are unavailable.

## Immutable inputs and behavior

Gist producer PR66 landed at `9e870c0a7b93990b166071ec061a27206afbda88`.
Zati baseline is `35ebe4dfcb309c3918379fd64c10dc84ce4943a2`; the consumer pins
`github.com/sirerun/gist v1.2.1-0.20261016191827-9e870c0a7b93`.

The CLI imports the maintained complete inbox package through normal artifact
upload and skill import. The package includes instructions and the briefing
asset. Immutable source/manifest digests and operator-asserted list/read mappings
are retained; activation still uses reviewed configuration. Installation-private
scope is supported; narrower catalog import scope is explicitly refused.

Gist package validation and existing lexical ranking are reused. Zati owns the
composite two-tool mapping and checks current connections, versions, schemas,
bindings and ordinary permissions. Gist's single-tool resolver is independently
qualified in the producer; the consumer does not manufacture a conformance
record to pass a composite mapping through it. Package pins establish trusted
private-source integrity, not public publisher signatures.

Actual model-context assembly loads complete pinned instruction and template
bytes outside transactions. The commit fence consults the Skills owner again.
MCP retains conservative mutation classification and normal Effects review;
operator assertions never grant permission or prove read-only provider behavior.

## Observable checks

Checks ran on the existing qualified Linux ARM64 host with Go1.26.2, bounded
commands and at most four Go workers. Only tracked-source archives were
transferred, with matching local/remote SHA256; no credentials or local settings.
Source `bed115c9` archive: `d2b90b32204f7ae7b98ec70aebbc12dfffc5319831b5b5fb13ac16f07cbd6da0`.

- Full CLI, Skills, Connections, Effects, Registry and MCP suites passed.
- Full Execution suite passed on `30fbbbe4` in196s. Later changed asset/unsupported
  runtime paths were checked by the focused catalog suite.
- Assembled Gist/MCP/catalog tests passed on `bed115c9` in25.3s. They prove actual
  import/upload/reviewed activation, exact instructions/template in model context,
  missing mapping/read binding, wrong schema and stale connection refusals.
- Unit cases cover wrong workspace, unauthorized/narrowed import, changed/missing
  asset bytes, unsupported local runtime and revocation between prepare/commit.
- The companion transport test uses a controlled TLS MCP server, actual adapter,
  controller, artifact callbacks and Effects review/dispatch/record. It observes
  exactly one list and one read call. This is real controlled network execution,
  separate from the context test; it is not an end-to-end live Gmail/model task.
- Generated specification check and all42 Python specgen tests passed. Contract
  revision45 is reflected in generated runtime schemas and registry assertions.
- Dependency lock and module verification passed; tidy's obsolete v1.2.0 checksums
  were removed. Full vet passed. Final lint comparison and review follow below.

Initial checks caught omitted generated mirrors, stale revision assertions,
installation/worker artifact-scope handling, a CLI fixture compilation error and
unchecked close/JSON errors; these were corrected, not counted as passes.
An invalid coordinator `-p4` test invocation ran no selected tests and was rerun
with `-p 4`. A120s baseline diagnostic timed out; it is not regression evidence.

The existing discovery test records bounded shortlist/authorized fallback
behavior. No comparable token, latency or task-quality experiment was run;
performance versus Composio and semantic retrieval remain unmeasured. No savings
claim is made. Gmail is not configured, no mailbox was contacted, no release tag
or installed application qualification is implied. Public publisher trust,
release/installation, live Gmail and evaluation remain separate plan gates.

See Zati `docs/guides/gist-inbox-briefing.md` for the exact import/mapping/setup
procedure and limitations. Missing provider setup fails closed.

## Final candidate checks

Candidate `bc9166803c8f1a216a74428fdeed515cde9aec46` archive SHA256:
`5dc3e59788f453c3a20f42e55be92f294423c0870fd138252ec0257a39e5c659`.
Focused CLI/catalog/assembled inbox and governed MCP tests passed after the final
close-error corrections; the exact CLI import test also passed. Full vet passed.
Uncapped golangci-lint2.11.3 reports157 findings on both untouched baseline and
candidate, with identical filename/linter/message multisets and no introduced
finding. Full-tree formatting reports only unchanged baseline
`internal/configuration/schemas.go`; changed source formatting is clean.
This is a qualified baseline comparison, not a claim that whole-repository lint
or formatting is green. Final independent review and merge are not yet asserted.

## Independent review

Non-author review of Zati `bc9166803c8f1a216a74428fdeed515cde9aec46` and Gist
documentation `d208efb89298e9251142449917d310da4fd5655a` accepted one P2 finding,
ZATI-CAT-R1: mixing catalog and ordinary active skills could silently remove the
ordinary skill's tools. The selected bounded correction explicitly refuses
unsupported mixed composition rather than widening tool exposure. Correction,
regression checks and exact-head re-review remain required before merge.
No other blocking finding was reported in the full candidate review.

Zati main Actions also cannot start because of the account billing lock, confirmed
from check117143352354's annotation. This prevents hosted CI evidence. Main is
reported unprotected; local fallback retains independent review and exact-head
merge guards, without altering billing or repository policy. Native multi-OS,
full race and Flutter qualification are not claimed by the source checks.

ZATI-CAT-R1 correction `054325383a15e7c19212fa171ad99e9ef563a9ab` moves the
composition check into the authoritative Skills owner before asset processing.
Both mixed orderings return `capability_unsupported`; ordinary-only selection
retains existing behavior. Regression tests include an ordinary skill with its
own tool requirement. Targeted Skills/Execution/catalog and assembled Gist/MCP
checks pass, as do renderer and all42 specification tests. Correction archive
SHA256 matches at both ends:
`cbd59bec757f2b610af2f68844c8316129e16703e9bcc3774853b988c0e23c02`.
