# Production binding refresh — T-GR-SCOPE.11

Date: 2026-10-06. Outcome: **BLOCKED for production writes; read-only bindings refreshed.**

## Source and policy

- Gist `main` was freshly read as `b91ec511153c5e2ed57569be532e0401ab8121a8`. The receipt is based on that exact source commit.
- The separately mapped private IaC checkout is dirty/stale and cannot qualify current source. Its private identity and revision are intentionally omitted; no IaC file was changed.
- GitHub ruleset 13902341 is active for the protected branch. It prohibits deletion and non-fast-forward updates, allows rebase merges only, requires no approving review, and lists no required status checks.
- The latest observed Actions run on the Gist source commit ended with failed and skipped job statuses. Its failure annotations say the failed jobs were not started because the account is locked by a billing issue. This is unavailable hosted validation, not a code or test failure. No workflow was retried and no paid runner was used.

## Current bindings

- The registered Pulumi `registry-aws` stack's `accountId` output was readable without selecting a stack, refreshing state, or requesting secret outputs. Its displayed last update is 2026-09-28. The value is intentionally omitted.
- STS identity reads for the ambient AWS session and one explicitly configured profile succeeded; neither account matched the Pulumi target (`false`). The default profile had no usable identity. No target-account role was assumed or otherwise tested. Account identifiers, principals, and ARNs are omitted.
- The current session exposed no Cloudflare DNS MCP capability. Discovery returned Sites tools only; these do not grant DNS authority. No browser or generic API fallback, OAuth/configuration change, or DNS action was used.

## Decisions and limits still open

- Existing provider proposals remain proposals: neither candidate has an approved exact action and target, provider account, spending bound, or call authorization. Provider artifacts remain `catalog_only`; no credentials were read, provider actions called, or credits spent.
- Operator and target-account qualification, identity/workspace and credential custody, the configured DNS owner/binding, and consumer runtime acceptance remain open. Any publication action still requires its separate exact-content, destination, schedule, and human approval.

No infrastructure, application source, account, role, DNS record, provider connection, or deployment was changed. This receipt does not qualify deployment, DNS, live service, release, or production acceptance.
