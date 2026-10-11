# RFC-002 auth and composition stage boundary

Date: 2026-10-11
Plan candidate: Gist PR #62, source revision fd75761f152b1a37c82c7acfd2922c80f78338dc before this receipt

## Decision

The enrollment component and its source-candidate verification are separate from the composed application journey. T-GR-AUTH.1-.3 now record the GitHub App sign-in, invitation/storage component, denial tests, formatting, and local package evidence. They do not claim that app startup is wired or that Gist OAuth access tokens are issued or refreshed through an enrolled operator. The full enrollment-to-consent-to-bounded-token issue/refresh, audience, membership revocation, and parent-scope denial remain explicit in T-GR-INTEGRATE.2, which depends on all required components landing. Independent auth review, guarded merge, and landed verification remain open.

This preserves the RFC-002 end state while making each component and composition acceptance executable at its own stage.

## Candidate evidence

- RFC-002 PR #63 base: c02811e00c76794f854173463a03a22594b1705a.
- Source code tested: f74e5f1a461d5cb804a0085017be292d71632c5a.
- PR #63 receipt-only head: e943603bf26e1e14f02938fde5ce31f04da50fa9; see [component receipt](https://github.com/sirerun/gist/blob/e943603bf26e1e14f02938fde5ce31f04da50fa9/docs/receipts/2026-10-11-rfc002-auth-component.md).
- The full integration-tagged Go suite, scoped vet, and pinned lint were recorded on the exact source head. DGX formatting verification with Go 1.26.1, goimports v0.52.0, and gofmt emitted no diff for all five changed Go files.
- GitHub Actions did not start because the account is billing-locked. This is unavailable hosted CI, not a pass. No live GitHub App, operator account, workspace, or external runtime was qualified.

## Plan conformance

The authoritative single-file docs/plan.md passed the pinned Wazi reader, schema, semantic, and Markdown compatibility gate on the candidate. Result: valid=true, schemaValid=true, semanticValid=true, readerCompatible=true, taskCount=317. Source digest: sha256:1e4117592c57a17f597e8a07504fb73fefd6ee2fced0eca15ecfc1b59ba96e5a. Reader digest: sha256:0b1292cbe93af31531647f3e2ae28c37f12decbe89023192729e010d6d7e6d67. Verifier digest: sha256:df729760ca1482b8539a6bfeeba92a00a6c8a225b7d64247bc51d66bd65f371e. Contract digest: sha256:7582512f122d2f2a9c4461facc7541c9887053f137260d6ebe9c6dea611d039d. The unchanged contract qualification passed all 60 fixtures (21 valid and 39 invalid).

## Remaining gates

T-GR-AUTH.4 independent exact-head security review, T-GR-AUTH.5 guarded merge, and T-GR-AUTH.6 landed verification remain open. T-GR-INTEGRATE.1-.6 remain open. The exact invited operator/workspace inputs remain owner-provided production gates under T-GR-SCOPE.4. No source, AWS, DNS, or provider mutation is claimed by this receipt.
