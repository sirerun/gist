# Gist plan authority and RFC-003 graph repair

Date: 2026-10-11
Reviewed pair before correction: Gist PR #62 head 3d995d1cdf6824347931a8bc5a067a5c7085e89a; Foundation PR #279 head 9ca4129e986c25203362bf47fcd7809343d6b3bd.

## Corrections

- docs/plan.md is the sole active RFC-002/RFC-003 delivery graph. The 21 tracked split-plan files were copied byte-for-byte to docs/plans/archive/20261011/*.source; their former paths are pointer stubs to the canonical plan and exact source snapshots. The plan archive table records each source digest. Current status is not read from archived checkboxes.
- Added 35 stable gateway task IDs, T-GR-GATEWAY.10 through .44, covering M-G1 and M-G2 contract, preflight, implementation, verification, independent review, findings correction, guarded merge, landed verification, immutable release, AWS deployment, bounded live acceptance, and operations. The full plan now contains 352 schema tasks, including dynamic-guidance delivery.
- M-G2 provider implementation remains gated on exact owner-approved targets and numerical limits. T-GR-GATEWAY.32 requires stable per-provider child SDLC rows before any M-G2 provider code or calls. Credential intake mode remains an explicit owner gate at .10; it blocks the corresponding credential path, not independent work.
- ADR-010 defines an unambiguous v1 idempotency frame: a domain/version separator, length-prefixed canonical UTF-8 identity fields, and a fixed-width 32-byte payload digest. Foundation PR #279's TM-02 control and required test vector now match this contract.

## Exact source and checks

- docs/plan.md SHA-256: f109376c22ece7d5d8d92589293724b8b90015a1c2eb72a0228e81094bfd5fee
- docs/adr/010-gateway-mg1-technical-design.md SHA-256: 8290f899b09039e2b55cef6604caefc3d64506d66cac8c3746af6c7fdf4e290d
- docs/rfc-002.md SHA-256: cc12c77f88515e6e37ee1221c323e8ce79deeb30b9f9e0b3c954a8cb17edc59a
- docs/rfc-003.md SHA-256: dd402eab10736f3a71e6585c7441adf7ec072985e5d8c0eda7a0bbc21883d765
- Private threat-model TM-02 candidate SHA-256: f40d2340494db87b57737825bad6d257e99d8f791b5251e4bfcad85ce66db811
- The updated 352-task plan passed Wazi schema, semantic, reader, repair-preview, and frozen-contract validation; see the exact conformance receipt. Historical crosswalk verification found 249 unique task IDs across 21 split-plan snapshots; all 249 exist in the canonical graph and 19 evidence-backed checkbox statuses differ from their historical copies: T-GR-AUTH.0-.6, T-GR-GATEWAY.0, T-GR-PUBLISH.0-.6, and T-GR-SCOPE.21-.23/.26.
- Markdown relative links across the canonical plan and former-path pointers: 0 broken.
- Changed public docs/archive privacy scan: 0 findings for local paths, private IPs/hosts, or credential-shaped values.
- git diff --check: passed before this receipt was written; rerun on the final candidate.

## Remaining gates

The plan graph and design corrections are not implementation or approval evidence. T-GR-GATEWAY.2-.9 (independent design review through landed plan verification) remain open. No G0.4 record, gateway code, provider call, AWS mutation, DNS mutation, or live acceptance is claimed. RFC-002 integration, provider qualification, and production gates remain open.
