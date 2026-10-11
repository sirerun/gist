# RFC-002 operator enrollment preflight

## Base and decisions

The coordinator completed this read-only preflight against exact Gist main/base revision c02811e00c76794f854173463a03a22594b1705a, ADR015, and RFC-002 section 10. It does not enroll a person, create a GitHub App, handle real credentials, or perform an external request.

The owner-approved boundary is permissionless GitHub App user sign-in to prove the stable numeric account ID, exact invitation to one private workspace, no stored GitHub access or refresh token, and separately bounded Gist workload grants. The concrete operator and workspace remain owner inputs required for live onboarding.

## Current source findings

- The hosted app constructs oauth.CookieSessions and the reference OAuth authorization server. The session issuer accepts an explicit subject/workspace list and issues Secure, HttpOnly, SameSite=Lax cookies.
- requestContext dispatches OAuth routes, /mcp, health/readiness, and REST. There are no GitHub sign-in, callback, operator invitation, or bootstrap handlers.
- app.Config has an optional OAuthLoginURL, but the registry command does not populate it. There are no GitHub App client settings or enrollment routes in the command wiring.
- workspace_memberships stores workspace, issuer/subject, role, scopes, active state, and policy generation under forced row-level security. The OAuth consent path checks membership before issuing a Gist authorization code. Enrollment should create membership only after verified numeric-ID invitation or exact one-time owner bootstrap; GitHub login alone must never issue workload authority.
- Hosted migrations 001-009 exist; 009 is publication-v2. Migration 010 is the next candidate slot on this base, but availability must be rechecked against fresh main and active branches immediately before implementation.
- The component can use Go standard net/http, crypto, and testing packages plus the existing PostgreSQL layer. No additional dependency or third-party license is required.

## Implementation boundary and interface

The auth lane owns a new hosted/internal/enrollment package, its unit tests, a narrow hosted/internal/storage enrollment adapter, and the allocated invitation/login-attempt migration. App startup, shared route dispatch, config/environment wiring, and joined HTTP acceptance stay with the coordinator's T-GR-INTEGRATE ownership. Reuse oauth.CookieSessions; do not fork MCP OAuth or workload-token issuance.

The component interface needs fixed GitHub App code exchange and one GET /user identity lookup, an invitation store binding the numeric ID to one workspace and redeeming atomically, and a session issuer returning only the verified Gist subject/workspaces. The callback remains exactly https://gist.sire.run/auth/github/callback; reject wildcard/dynamic callbacks and external return destinations.

The source contract also needs: (1) one-time initial-owner bootstrap limited to the exact configured numeric ID and workspace, disabled after success; (2) an authenticated maintainer operation to issue expiring, single-use invitations bound to an exact numeric ID and workspace; and (3) a multi-replica, one-use login-state mechanism with S256 PKCE and state bound to the initiating browser. Store only short-lived state hash/expiry for atomic consumption; keep the verifier transient in a Secure, HttpOnly, SameSite=Lax flow cookie. Never persist authorization code or GitHub tokens. Missing bootstrap configuration, wrong identity/workspace, repeated bootstrap, invalid/replayed invitation or state, PKCE mismatch, and return-path mismatch fail closed. No manual SQL or public bootstrap path is permitted.

The exact owner values remain runtime configuration, not source fixtures. Tests use synthetic identities/workspaces, httptest GitHub endpoints, and the repository PostgreSQL integration harness; they never contact GitHub or create a real App. Required denial coverage is listed in T-GR-AUTH.2. Exact runtime route wiring and full HTTP journey remain in T-GR-INTEGRATE.

## Verification boundary

This preflight used source inspection only; no Go build or test was run and no source file changed. The plan task is complete as preflight evidence, not implementation or live acceptance. Production remains gated on owner-provided workspace/operator inputs and separately tracked AWS/DNS authority. Recheck the migration slot and shared-file allocations at implementation start.
