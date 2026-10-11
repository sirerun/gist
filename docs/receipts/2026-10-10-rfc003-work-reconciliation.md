# RFC-003 in-flight work reconciliation

Date: 2026-10-10
Status: Preflight inventory only; no worktree adopted, cleaned, reviewed, or merged.

## Remote source and pull requests

- Gist remote main is `c02811e00c76794f854173463a03a22594b1705a`.
- PR #47, `bdd7d3e4f2186cfcd7aba8b708b8099539bf99e6`, is open against historical base `84e14128565058419a9b90619f27fa517ba4f494`. It changes documentation only and has no recorded review decision.
- PR #62, `812d5b4d37d5814f54bfc315f4350039907a9092`, is open against current base `c02811e00c76794f854173463a03a22594b1705a`. It changes documentation and a preflight receipt only and has no recorded review decision.
- No open RFC-003 implementation or gateway-contract code pull request was found.

## Preserved local work

- The linked checkout for PR #47 has uncommitted edits to RFC-003 and ADR009/ADR010 plus untracked draft support files. Its caller-owned approval/OpenAPI-generated MCP proposal is distinct from the published PR head. The checkout remains untouched.
- The repository has 94 linked worktrees. Dirty production, recovery, provider, and gateway work remains preserved; no cleanup or branch mutation was performed.
- The private M-G1 threat-model draft still describes Gist-hosted approval pages, JWS evidence, and authority-key handling. It does not yet cover the proposed caller-owned approval boundary. Its owner sign-off remains pending.
- A registry authorization patch already landed through PR #36 and is present in current main. The separate general library-retrieval patch is outside the RFC-002/003 hosted implementation graph.

## Disposition

RFC-003 scope is reopened by ADR014. PR #47's amendment remains a design proposal, not accepted implementation authority. RFC-003 code remains gated on a coherent revised RFC/ADR/threat model, independent review, G0.4 owner sign-off, and a conformance-checked first-class task graph. RFC-002 remains separately active. This inventory authorizes no provider call, spend, AWS change, DNS change, release, or deployment.
