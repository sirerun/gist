# RFC-003 design reconciliation receipt

Date: 2026-10-11
Status: Authored candidate; independent review and exact G0.4 recording remain open.

## Founder direction recorded

The founder approved the revised M-G1 direction for planning and implementation:
caller-owned user approval without a Gist approval page or approval JWS; gateway
MCP generated from a separate OpenAPI contract; dynamic task-specific guidance
with an evaluation; KMS-encrypted provider credential custody; GitHub REST and a
controlled MCP fixture. Billing and broad provider expansion remain M-G2.

The founder separately approved permissions-free GitHub App sign-in for invited
RFC-002 operators, stable numeric account identity, invitation to one private
workspace, no stored GitHub token, and Gist-issued bounded workload grants.
Treg and Composio are provider connections, not identity providers.

## Candidate coverage

The candidate updates ADR009/010/014/015, RFC-002, RFC-003, and the authoritative
plan. It also revises the private M-G1 threat model to cover the caller-owned
approval boundary, REST/MCP contract parity, guide isolation/freshness, KMS
custody, egress, result injection, revocation, GitHub REST, and the controlled
MCP fixture. The detailed threat model remains in the private foundation
repository.

Source file digests for this authored candidate:

- ADR009: `62ca5515076ea822a1d81760e78fdf920bbbe34996216de5e8c05eee9cbf919f`
- ADR010: `af447c7dc3cdfd90535f39d95f1b5754b7b5e0ae6afe7a5bc5b3620baf522a7a`
- ADR014: `691e8c586ba3343f8d0901173d2befae22a852e6a6fdc72d74fee99880ef6d06`
- ADR015: `b96524ba6fce6204b0763fca0c230801e976ba4ff7617d55e3b6a81b6eedd708`
- RFC-002: `cc12c77f88515e6e37ee1221c323e8ce79deeb30b9f9e0b3c954a8cb17edc59a`
- RFC-003: `92791e1c686f2bc622a20dc180643ad0836a404565815cf2df46661c625ef436`
- Private threat-model digest: `a9242fead3074792c6b4cb4a78cea1253771e13e08c6331d6e407c6860c0c087`

## Remaining gates

Stored-only versus one-shot caller-supplied provider credential intake remains
unresolved; no secret-ingress route is approved until that decision is recorded.
The specific operator workspace and numeric account are pending. GitHub REST
live target, exact operations, permissions, call cap, and expiry are unqualified;
fixture-only verification may proceed. No provider execution, AWS deployment,
DNS mutation, or production acceptance was performed.

This is an authoring receipt only. It does not claim independent review, G0.4
exact-revision sign-off, implementation, merge, deployment, or production
acceptance.

## Validation evidence

The authored plan source digest is `sha256:d77a873fe5cdc85d5df7c56f5e898acd7dee1cb540ebd0eb573bbc3db2f7f52f`. The pinned Wazi 0.0.1 contract, owning semantic validator, and Markdown reader gate returned `valid=true`, `schemaValid=true`, `semanticValid=true`, `readerCompatible=true`, and `taskCount=317`. The verifier reports `authorityAuthenticated=false`; this is a plan conformance result, not a source of execution or approval authority. All 60 pinned validator fixtures passed. This is local DGX validation, not hosted CI or application acceptance.

## Exact authored revisions

The Gist design candidate is commit `4a3e32075392783701ba8fc4884045a5512ace1e`, based on `3f650f20b33dcf09b673c3d2093dab1babdb1578`. The aligned private threat-model candidate is commit `71281deac75298edf7c1cd0ebc0abd5e3dea6585`, based on `9a813396c6be54875d1f9d8dfbc4caaa395e699f`; its file digest is recorded above. Both commits remain unmerged and require independent exact-revision review.
