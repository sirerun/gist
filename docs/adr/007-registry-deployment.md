# ADR 007: Registry Deployment, Origin, and Issuer

## Status

Accepted

## Date

2026-09-25

## Context

The hosted registry needs a stable public resource boundary without making a
hostname part of artifact identity. RFC-002 section 5 defines the hosted
service boundary and deployment surface. It also needs one authorization model for
M2a workload access and M2b multi-tenant connector access. RFC-002 section 10
makes Gist a protected resource and permits either the built-in reference
authorization server or a conforming delegated issuer. RFC-002 sections 12,
14, and 16 require tenant-bound credentials, IaC-only deployment, a final
configurable origin, and a short-lived OAuth pre-production environment.

This ADR settles those deployment choices. Governance, authorization, and
hosted-module invariants remain in [ADR 004](004-registry-governance.md),
[ADR 005](005-registry-authorization.md), and
[ADR 006](006-hosted-module.md); this record does not restate them. RFC-003
remains outside this implementation: its gateway, provider egress, credential
custody, billing, and execution endpoints are not created by this decision.

## Decision

### 1. Resource ownership and deployment shape

The registry is one independently deployable Go service in the `hosted/`
nested module, deployed with the reviewed IaC and release pipeline prescribed
by RFC-002 section 14. The service owns HTTP transport, authorization checks,
catalog metadata, and mediated access to private immutable bytes. PostgreSQL
owns metadata and policy state; private object storage owns package bytes.
Neither storage system is directly exposed to clients.

The deployment configuration, not source code or an artifact manifest, owns
the public origin. The required configuration variable is
`GIST_PUBLIC_ORIGIN`; it must be one exact HTTPS origin with no path, query,
fragment, wildcard, or trailing slash. A deployment with a missing, malformed,
or changed origin fails readiness and does not mint or accept tokens. The
origin is used for canonical URLs, OAuth protected-resource metadata,
redirect allowlisting, and the resource audience. No hostname, origin, or
deployment identifier is included in package identity, artifact IDs, version
keys, digests, or manifest canonicalization. This preserves RFC-002's artifact
model and ADR 006's digest boundary.

The initial production deployment uses the configured production value of
`GIST_PUBLIC_ORIGIN`; documentation and test fixtures refer only to that
variable, never to a private deployment hostname. `GIST_RESOURCE_AUDIENCE`
may be set only to the exact `GIST_PUBLIC_ORIGIN` value; omission derives the
same value, and any mismatch fails startup. Audience migration is therefore
an explicit deployment change, not a wildcard acceptance rule.

### 2. M2a workload tokens and M2b human OAuth

Gist is the protected resource server. It validates issuer, signature and
algorithm, exact resource audience, expiry, scopes, workspace membership, and
policy generation before any catalog lookup, as required by RFC-002 sections
10 and 12 and implemented consistently with ADR 005.

For M2a, machine consumers use short-lived workload access tokens minted by
the selected authorization-server strategy. The default is the built-in
reference authorization server; it is also the mandatory fallback. A workload
token has a maximum five-minute lifetime, a unique `jti`, one exact
`GIST_RESOURCE_AUDIENCE`, and explicit `catalog:*` scopes. It is bound to its
workload subject, one workspace, the current policy generation, and a
parent-authority record. Its workspace and scopes must be a subset of the
parent bounds at mint time and at resource authorization time; a child token
cannot widen parent workspace, role, scope, or policy-generation authority.
No workload token is accepted as a provider credential or forwarded upstream.

For M2b, connector and human flows reuse one configured OAuth 2.1
authorization server as issuer while Gist remains only the protected
resource. The built-in reference AS is the production default until a
delegated issuer passes every RFC-002 section 10 probe. A deployment may
select one external issuer through `GIST_OAUTH_ISSUER` only after recording
the issuer metadata, JWKS, PRM, and conformance evidence. The external issuer
must pass RFC 8414 discovery, RFC 7591 HTTPS-only dynamic registration with
reject-not-downgrade scopes, authorization-code PKCE S256 with single-use
consent-bound codes, refresh rotation with family revocation on reuse, RFC
9728 PRM, and RFC 8707 resource/audience binding. Any failed, unavailable, or
partially configured check selects the built-in strategy before traffic is
accepted; it never creates a mixed-issuer session.

The selected issuer owns consent, grants, refresh/revocation, and signing
keys. Gist does not create a password database. Human login is an authenticated
deployment identity session with explicit workspace-membership and workspace
selection checks, CSRF protection, and session binding; switching workspace
requires re-consent and a newly issued token. Gist stores only the issuer
configuration and opaque authorization/connection state required by the
accepted contracts, never raw passwords, authorization codes, or refresh
tokens. Issuer or membership uncertainty fails closed with the ADR 005
`service_unavailable` behavior.

### 3. Origin and issuer migration

Origin, resource audience, issuer, redirect allowlist, and PRM metadata are a
single versioned deployment configuration. A migration provisions the new
origin and issuer metadata in parallel, publishes the new canonical metadata,
registers new redirect URIs, and requires fresh consent. Existing tokens are
not accepted under a different origin, audience, issuer, or redirect binding.
The old origin remains only for an explicitly time-bounded redirect handoff
that returns no credentials and cannot call catalog routes; after the recorded
cutover it is disabled. Rollback restores the prior complete configuration,
not a wildcard issuer or audience. Artifact IDs, package digests, and stored
workspace records do not change during migration.

### 4. Ephemeral OAuth preview

OAuth and connector acceptance uses a dedicated Pulumi stack named
`preview`, created from the registry IaC by the reviewed preview workflow.
Each run receives a unique temporary public origin, resource audience,
redirect allowlist, database, private object-store namespace, issuer client
registrations, and synthetic workspaces/accounts. The preview has no
production data, secrets, storage identities, DNS records, or audience in
common with production. Gist code does not provision external identity
infrastructure; the workflow configures only the selected issuer interfaces.

The workflow records the run owner, commit/configuration hash, origin,
audience, stack state, and expiry. The hard maximum lifetime is 24 hours. A
successful acceptance run must execute the reviewed destroy stage immediately
after evidence capture; failure paths invoke the same destroy stage. Preview
clients, tokens, redirects, DNS, state outputs, and test-secret references
are revoked or removed, while sanitized evidence retains no credential
material.

Teardown is fail-closed: a failed or incomplete destroy makes the workflow
failed, blocks M2b acceptance and any promotion dependent on that run, and
does not become a reusable staging environment. The cleanup controller may
retry the reviewed destroy operation and alert the owner, but it may not mark
the preview complete while any preview resource remains. A TTL cleanup job
handles abandoned runs and emits equivalent evidence. Production resource
identities and data are unchanged by preview creation or teardown.

### 5. Residency, subprocessors, and release evidence

The deployment records its configured data-residency region and the approved
subprocessor/service roles for the runtime, PostgreSQL, object storage, DNS,
and issuer. A release is blocked when a provider, region, issuer, or
subprocessor is absent from that reviewed configuration. Logs and evidence
contain workspace-safe IDs and hashes, not tokens, package secrets, raw OAuth
codes, or customer data. Backup, restore, and rollback evidence is produced
by the release workflow against isolated stores; no manual live mutation is a
release step.

## Alternatives

- Bake a hostname into artifact IDs or digests. Rejected because deployment
  origin is operational configuration and RFC-002 requires portable artifact
  identity.
- Accept any issuer, audience, or redirect belonging to a configured issuer.
  Rejected because it permits resource confusion and cross-resource replay;
  exact binding is required by RFC-002 section 10.
- Use a new Gist password database for M2b. Rejected because the selected AS
  owns consent and grants, while Gist is the protected resource.
- Make an external issuer mandatory. Rejected because RFC-002 requires the
  built-in reference AS as the default/fallback and external delegation is
  valid only after complete conformance evidence.
- Keep a permanent staging service. Rejected because RFC-002 section 14
  requires a short-lived public-origin environment and the preview must not
  become a production-data or credential dependency.

## Consequences

Every deployment must supply and review origin, audience, issuer, residency,
subprocessor, and key/configuration values before readiness. M2a can proceed
with workload tokens using the built-in AS; M2b can reuse an external AS only
after the complete section 10 checklist passes. Preview testing adds IaC,
temporary DNS/client registration, evidence, and cleanup work, but prevents
connector behavior from depending on an undeclared persistent staging system.
The gateway boundary in RFC-003 remains unchanged.

## Verification and falsifiable acceptance tests

### Origin mechanism

- Configuration tests accept exactly one valid HTTPS `GIST_PUBLIC_ORIGIN`,
  derive the same `GIST_RESOURCE_AUDIENCE`, and fail readiness for missing,
  path-bearing, wildcard, trailing-slash, or audience-mismatched values.
- An artifact identity test creates the same manifest under two configured
  origins and proves identical IDs, canonical digest, and package bytes.
- HTTP tests prove canonical links, PRM, OAuth redirect validation, and
  bearer challenges use the configured origin and exact audience; a token for
  the old origin receives `401 invalid_token` after migration.

### Issuer mechanism

- Workload-token vectors reject wrong issuer, signature algorithm/key, issuer,
  audience, expiry, workspace, policy generation, scope, and parent-bound
  claims; a child token with a wider scope or workspace never authorizes.
- OAuth conformance tests pass every RFC 8414, RFC 7591, PKCE, refresh-reuse,
  RFC 9728, and RFC 8707 probe for the selected issuer. Removing any one
  capability selects the built-in strategy and records the failed row; it
  never accepts the incomplete external issuer.
- Two synthetic workspaces and two principals prove workspace switching
  requires re-consent and reissue, foreign catalog access is uniform `404`,
  consent denial/CSRF fails, and issuer or membership outage fails closed.

### Preview mechanism

- The preview IaC diff contains a distinct origin, audience, database,
  storage namespace, and issuer registration, and an integration test proves a
  preview token cannot read production.
- Workflow tests prove create, OAuth/connector acceptance, evidence capture,
  and destroy execute; destroy removes preview resources and leaves production
  resource IDs unchanged.
- Injecting a destroy failure leaves the workflow failed, prevents the M2b
  acceptance job from passing, and emits an owner/expiry cleanup record. A
  separate TTL test destroys an abandoned run within the configured cleanup
  window. No test may pass while preview resources remain.

## References

- RFC-002 §§ 5, 8, 10, 12, 14, 15, and 16.
- RFC-003 §§ 1–2 and 9 for the explicit gateway scope boundary.
- ADR 004, ADR 005, and ADR 006.
