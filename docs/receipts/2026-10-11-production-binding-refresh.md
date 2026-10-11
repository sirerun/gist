# Production binding read-only refresh

Date: 2026-10-11. Scope: refresh AWS, DNS and current-origin bindings for the selected Gist production destination. Read-only checks only.

- The AWS CLI authenticated as a user principal in the AWS Organizations management account. The `registry-aws` Pulumi stack `accountId` resolves to an ACTIVE Organizations member account named `gist-registry`; this matches the documented cross-account stack topology. The different account IDs are expected and do not by themselves indicate a mismatch. IDs and profile names are omitted. Three CLI profiles are configured, two authenticate, and none directly selects the member account.
- The Foundation `registry-aws` runbook on main documents the management/payer account, member-account role assumption, and stack workflow. No STS role assumption, Pulumi preview/apply, or current deployment permission was tested. The DGX environment has no AWS CLI or Pulumi CLI.
- The Cloudflare MCP returned HTTP 200 for the exact `sire.run` zone lookup; the zone is active. Exact DNS reads returned HTTP 200: `registry.sire.run` has one CNAME and `gist.sire.run` has zero records. Record content is omitted. This confirms read access, not DNS write authority.
- A public HTTPS `HEAD /` request to `https://gist.sire.run/` failed because DNS could not resolve the hostname. A public request to `https://registry.sire.run/healthz` returned HTTP 200 with `status: ok`; this does not prove Gist acceptance at the canonical hostname.
- Public authorization-server metadata at the existing registry origin advertises `https://registry.sire.run` as its issuer and uses that origin for authorization and token endpoints. This is direct evidence that the current OAuth origin has not yet been migrated to the selected Gist hostname.
- No STS role assumption, stack preview/apply, DNS write, certificate request, provider action, deployment, or cost-incurring operation was attempted.

Remaining: verify the permitted member-account role and deploy workflow, the current CodeBuild/ECR artifact path, canonical issuer/redirect migration, owner-approved numerical change/cost limits, and Cloudflare DNS write authority. T-GR-SCOPE.11 and all production/deployment gates remain open.
