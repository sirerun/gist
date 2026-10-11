# RFC-002 auth readiness and live identity gate

The approved enrollment design is ready for source implementation with synthetic identities and controlled GitHub API fixtures. The owner has not supplied the exact operator account or private workspace, so those real values cannot be used for local tests or live onboarding.

The plan now separates those concerns:

- `T-GR-AUTH.0` can preflight from the accepted lane contract without the real workspace or operator ID.
- `T-GR-AUTH.1` explicitly uses synthetic invited-operator fixtures; source tests imply no live account, invitation, credential, or GitHub App registration.
- `T-GR-PROD.0` depends on `T-GR-SCOPE.4`, preserving the owner-provided operator/workspace record as a prerequisite to production acceptance.

This changes only the dependency and acceptance text; the plan still contains 317 tasks. Plan source digest changed from `sha256:d77a873fe5cdc85d5df7c56f5e898acd7dee1cb540ebd0eb573bbc3db2f7f52f` to `sha256:dbed4c3dbbb6a79b26aec4a9b989bcad3e9c9b5fa166516688557b121b2b1d39`.

Validation on this candidate passed the pinned Wazi 0.0.1 Markdown reader, schema, and semantic gate. The contract digest was `sha256:7582512f122d2f2a9c4461facc7541c9887053f137260d6ebe9c6dea611d039d`; all 60 contract fixtures passed. `git diff --check` passed.

This is an unreviewed plan candidate. It is not implementation, independent review, merge, deployment, or production acceptance evidence. Hosted checks remain unavailable because the repository account is billing-locked.
