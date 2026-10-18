# ADR 022: Hosted Library web app

Status: Accepted by owner, 2026-10-17.

## Decision

Build the approved **Library** visual direction at `https://gist.sire.run`:
warm ivory, forest green, editorial typography, softly layered cards, and the
approved selected-page symbol and serif wordmark. The website explains Gist,
offers client/developer documentation, and gives invited operators a usable
authenticated catalog. It complements the Go library and MCP interface.

The public website contains explanatory examples explicitly labeled as examples.
It does not make private catalog records public. The private library reads live
workspace records after GitHub sign-in, with search, kind filters, bounded
pagination and exact version detail. Empty, unavailable, expired-session and
no-result states are first-class experiences. Real account setup and execution
stay in Zati or another client. No performance superiority is claimed.

Use the existing Go service with embedded HTML, CSS, JavaScript and SVG assets.
No separate frontend service, build toolchain, third-party scripts, analytics,
CDN fonts, or new infrastructure are required. The responsive browser interface
uses same-origin, read-only session APIs. The wordmark is vector outlines, so
its appearance is independent of local fonts. Body and headline fonts use
available system fonts.

## Identity and authorization

Keep the existing permission-free company GitHub App sign-in. The website uses
the existing Secure, HttpOnly, SameSite session cookie; browser code does not
receive or store bearer tokens. Every private read rechecks current membership,
workspace policy and read authority, under the configured workspace and tenant
RLS. Request parameters cannot select another workspace. Only an exact fixed
`/library` return target is added to the existing strict OAuth return-target
allowlist. Existing API callback JSON and OAuth authorization return behavior
remain compatible.

Public informational routes and assets can render while first enrollment is
pending. This exception does not permit private catalog, MCP, REST, OAuth
issuance or publication through the bootstrap admission gate. Session APIs
are no-store; sign-out is a same-origin POST. A restrictive content security
policy and text-only metadata rendering protect the authenticated interface.

## Delivery and limits

The existing AWS image pipeline and Foundation Pulumi stack own release and
deployment. Review and test the source, merge the exact reviewed revision,
build an immutable image, review its config-only pin, preview/apply it and
verify the live website. Preserve the existing database, credentials and
first-enrollment flag until actual founder sign-in and maintenance readiness
qualify its removal. Browser fixtures are not live GitHub acceptance.

Publishing administration, anonymous catalog projection, provider authorization,
and Gist-hosted execution are outside this web-app slice. Existing publication
APIs retain their own contracts and gates. RFC-003 remains retired.
