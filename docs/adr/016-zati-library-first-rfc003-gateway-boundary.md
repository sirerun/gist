# ADR016 -- Zati library-first integration and catalog-only Gist and retired RFC-003 gateway

Date: 2026-10-16
Status: Accepted owner direction; API and consumer plan remain to be designed and verified. This change is documentation-only.

## Context

Zati is the flagship product, and the owner has decided Gist must be used in
Zati as a library. Zati already pins `github.com/sirerun/gist v1.2.0` and uses
the root package to shortlist tools among candidates Zati has already
authorized. That does not yet provide Zati with the hosted registry's complete
skill catalog, exact package assets, or capability-resolution implementation;
those pieces are currently under `hosted/internal` and are not a public Go
consumer API.

Zati owns skill lifecycle, tenant and caller authorization, context limits,
credentials, provider bindings, approvals, effect records, execution, and
unknown-outcome handling. RFC-003's hosted gateway would duplicate those
controls for Zati. External clients use their own execution machinery or an integration provider; that need does not expand Gist into an executor.

## Decision

1. Treat Zati as the flagship Gist consumer and make the in-process Gist
   library path a first-class delivery goal. Preserve the existing root
   package API and compatibility.
2. Plan public, additive catalog/domain APIs only for capabilities the
   accepted Zati slice needs: authorized discovery, exact immutable-version
   retrieval with complete assets and digest verification, and explicit
   capability resolution. Keep the API transport- and storage-agnostic.
   Hosted and Zati adapters would supply authorization, storage, current
   binding and revocation state; do not import `hosted/internal` into Zati or
   create a second registry implementation. This documentation change does
   not authorize source-code implementation.
3. Zati remains the authority for whether a skill is eligible, which provider
   bindings are current, how skill content is evaluated/promoted, and whether
   an effect may execute. Gist metadata, instructions, and discovery results
   grant no permission. Recheck authorization, binding, and revocation at
   consumption and immediately before execution.
4. Retire RFC-003's standalone execution gateway. Gist remains a reusable
   catalog for skills and tools: discovery, exact packages, schemas, provenance
   and resolution. Invocation, provider-token custody, execution approvals,
   execution journals and utility billing belong elsewhere. Preserve registry
   authentication, publisher signing and catalog support evidence. Existing
   local-executor compatibility is outside this documentation change. A future
   reversal requires a new architectural decision, not an automatic trigger.
5. Pause hosted RFC-002 work except for repairs and existing consumer obligations while prioritizing the Zati
   library path. Keep the hosted service running and preserve its deployment;
   this decision does not authorize shutdown or new hosted deployment. Hosted
   M2/M3 gaps do not block a future local library consumer slice, and a local
   consumer slice does not claim hosted production acceptance.
6. The first consumer acceptance must use a real Zati skill that has both a
   package asset and an admitted required tool capability. Catalog fixtures
   alone do not meet acceptance. If no such pair is identified, keep the
   consumer integration waiting at that product-selection boundary rather
   than inventing one.
7. This accepted change revises documentation only. Do not implement the public
   library API or Zati adapter until the documentation/plan is independently
   reviewed and the owner separately starts that implementation scope.

## Consequences

RFC-002's library and Zati consumer plan become the near-term priority. Hosted
RFC-002 implementation pauses except for repairs and existing consumer obligations while the service continues
to run. The public catalog interface and exact Zati adapter/test ownership
require design and coordination. The gateway graph is retained as non-dispatchable history, with catalog provenance obligations transferred to the library graph. No gateway terminal remains active.

This change covers documentation only. It authorizes no library/API source
implementation, new hosting, paid provider calls, credential migration,
service shutdown, or gateway deployment.

This decision supersedes the execution scope of ADR009, ADR010 and ADR014. ADR013 remains applicable to independent consumers; its optional-adoption premise does not negate Zati-first prioritization.
