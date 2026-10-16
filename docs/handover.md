# Gist delivery handover

Updated 2026-10-16. This is the current handoff; the authoritative delivery graph
is [docs/plan.md](plan.md). [ADR016](adr/016-zati-library-first-rfc003-gateway-boundary.md)
supersedes the former execution-gateway scope and continuation instructions.

## Scope and current state

Gist remains reusable, with its Go library and Zati as the first priority.
It supplies skill/tool discovery, complete exact packages, schemas, provenance
and capability resolution. Execution, provider-token custody, execution
approvals/journals and utility billing belong to external executors.
**RFC-003 is retired, not deferred.** Do not resume M-G1/M-G2, request a gateway
credential-intake decision, or treat historical G0.4 approval as current authority.
Registry authentication and publisher provenance remain catalog obligations.
Existing root-library execution compatibility is unchanged by this docs work.

Zati already uses Gist v1.2.0 for authorized tool ranking. Full public catalog
APIs and actual skill-package/asset/capability consumer integration are planned,
not qualified by that existing ranking integration. The CATALOG batch starts
with joint selection of a real skill, asset and admitted capability and exact
consumer checks. Public-distribution signing has its own PROVENANCE lifecycle;
it does not block appropriately trusted workspace-private import.

Hosted expansion is paused except repairs and existing consumer obligations.
Keep the service and existing work intact. This decision authorizes no shutdown,
deployment, credential migration, provider spend or application implementation.
A bounded repair needs its precise scope recorded; it is not authority to resume
all paused production milestones. Retired/held plan rows retain their original
IDs, authored completion states and evidence without becoming completed work.

## Evidence and current review gates

- [PR #62](https://github.com/sirerun/gist/pull/62) carries the documentation
  rewrite. Independent review of the exact candidate, correction/re-review,
  guarded merge and landed verification are required for the documents.
- [The conformance receipt](receipts/2026-10-16-catalog-plan-conformance.md)
  records local Wazi reader/schema/semantic validation and verifier fixtures.
  [The coverage ledger](receipts/2026-10-16-catalog-plan-coverage.md) preserves
  the old graph and records every disposition. Neither is implementation or
  production acceptance.
- Hosted CI previously failed before running because of the account billing
  lock. Read current checks for the final candidate and follow the applicable
  documented local-validation fallback; never label unavailable CI passing.
- Preserve PR #47 and all unique historical work. The retired private gateway
  threat model is historical input, not a new prerequisite for catalog delivery.
- October11 DNS/health/AWS observations are dated history in the
  [exact prior handover](plans/archive/20261016/handover.md.source), not current
  availability claims. The selected hosted origin remains `https://gist.sire.run`
  under ADR011; no current production qualification is asserted here.

## Resume sequence

1. Finish independent Gist-session review of the frozen documentation candidate.
2. Correct accepted findings, rerun affected checks, obtain exact-head re-review,
   then merge and verify the landed documentation under repository policy.
3. Report document delivery separately from unexecuted library work. The
   CATALOG preflight records the actual consumer selection, revision, trust model
   and verification recipes before separately authorized implementation.
4. Keep external execution ownership, the hosted-expansion hold and public
   signing qualification boundary intact. Do not revive retired gateway tasks
   or dispatch paused hosted work through old milestone/continuation language.

This handoff is documentation only. It supplies no implementation, release,
production promotion or paid-provider authority.
