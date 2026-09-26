# Launch-provider selection and demand scoring

**Task:** D1  
**Decision status:** Proposed for David's approval  
**Prepared:** 2026-09-25  
**Scope:** M1 curated catalog; catalog capture only, not provider execution

## Decision requested

Approve the following three launch routes for M1:

1. **Aggregator:** Composio (aggregator-class route).
2. **MCP server:** GitHub MCP Server (`github/github-mcp-server`).
3. **Direct API:** Slack Web API.

This is a deliberately small, evidence-led starting set. The aggregator and
direct API must be captured as separate provider records and adapter routes;
the direct Slack route is not treated as a Composio-backed route. No selection
grants execution authority. D2 must capture offline provider metadata,
versioned schemas, effects, credential metadata, and safe golden fixtures
before any record is considered resolvable. Any provider requiring an
execution gateway to demonstrate registry correctness is out of scope for M1.

**David's ruling:** [ ] Approve  [ ] Reject  [ ] Request a re-score  
**Ruling note / date:** ______________________________________________

## Evidence boundary and capture date

The repository contains no measured production workload or provider-usage
telemetry. “Demand” below is therefore a declared proxy: P0/P1 use cases in
the buildout plan, the taxonomy's Level-4 activity density, and the breadth of
the provider's documented contract surface. It is not a claim of market share,
availability, or successful integration. Scores are a planning instrument and
must be revisited after D2 capture and M3 runtime acceptance.

Public first-party sources were identified for offline review on **2026-09-25**;
the source pages themselves are not runtime dependencies:

| Candidate | Primary source |
| --- | --- |
| Composio | [Composio documentation](https://docs.composio.dev/docs/intro) |
| GitHub MCP Server | [GitHub MCP Server repository](https://github.com/github/github-mcp-server) |
| Slack Web API | [Slack Web API](https://api.slack.com/web) and [method reference](https://api.slack.com/methods) |
| Google Drive API | [Google Drive API overview](https://developers.google.com/drive/api/guides/about-sdk) |
| Nango | [Nango documentation](https://nango.dev/docs) |

These links establish public documentation and an inspectable contract surface;
they do not replace byte capture, license review, or conformance testing.

## Scoring method

Each criterion is scored from 1 (poor or unproven) to 5 (strong and
inspectable). Weighted score is the weighted average, reported out of 100.
“Maintenance burden” is reverse-scored: 5 means lower burden. A score below 3
on license/redistribution or immutable capture is a launch hold even when the
total is high.

| Criterion | Weight | What earns a high score |
| --- | ---: | --- |
| Early-workload demand | 25 | Direct fit to P0 workflows and dense seeded families |
| License / redistribution | 15 | Clear terms for offline metadata and fixture storage |
| Immutable schema capture | 15 | Stable, serializable schemas and explicit operation identity |
| Safe fixture availability | 10 | Synthetic, non-credential fixtures can cover positive/negative cases |
| Explicit versions | 10 | API/schema/server revisions can be pinned and digested |
| Effects, cost, credential metadata | 10 | Read/disclosure/mutation, quotas/cost, and auth scopes are inspectable |
| Semantic portability | 10 | Operations map cleanly to Gist capability contracts |
| Maintenance burden | 5 | Small adapter surface and predictable change management |

Demand is intentionally not allowed to erase safety or provenance gaps. Scores
are based on public documentation plus repository demand proxies, not a claim
that a provider has already passed Gist conformance.

## Scored comparison

| Candidate / route | Demand | License | Capture | Fixtures | Versions | Metadata | Portability | Maintenance | Weighted score | Disposition |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| **Composio / aggregator** | 5 | 3 | 4 | 4 | 4 | 4 | 4 | 3 | **81** | **Select** |
| Nango / aggregator | 4 | 3 | 4 | 4 | 4 | 4 | 4 | 3 | 76 | Reserve; re-score after license/capture review |
| **GitHub MCP Server / MCP** | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 | **80** | **Select** |
| **Slack Web API / direct** | 5 | 4 | 4 | 5 | 4 | 5 | 4 | 4 | **89** | **Select** |
| Google Drive API / direct | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 4 | 80 | Reserve; document-first alternative |

The totals are calculated as `sum(score × weight) / 5`; they are not a ranking
of provider quality. GitHub MCP Server and Google Drive API tie numerically,
but GitHub is preferred for the required distinct MCP route and a compact
fixture surface. Slack is preferred as the direct route because its documented
method/scopes/response surface supports communication plus explicit effect and
credential metadata. Composio is selected for breadth and fast demand coverage,
but its lower license and maintenance scores require a strict offline capture
review before approval becomes an ingest decision.

## Demand and capability-family shortlist

The taxonomy seed is a naming and prioritization input, not an identity
namespace. The ranking combines Level-4 density with the P0 use-case manifest;
it does not import every family merely to achieve coverage.

| Rank | Family | Demand signal | Initial contract candidates | Launch-route fit |
| ---: | --- | --- | --- | --- |
| 1 | `communication` | Cross-cutting notification/send work; UC-003, UC-010 | `communication/channel.invite`, `communication/message.send`, `communication/notification.send` | Slack direct; Composio; GitHub MCP |
| 2 | `document` | Parse/extract/generate work across finance, HR, and IT | `document/parse`, `document/extract`, `document/generate`, `document/archive` | Composio; Google Drive reserve |
| 3 | `identity` | 33 account activities; onboarding/offboarding and access management | `identity/user.create`, `identity/account.update`, `identity/access.grant` | Composio; GitHub MCP identity metadata |
| 4 | `case` | 38 support activities; UC-003 discovery and UC-005 resolution patterns | `case/ticket.create`, `case/ticket.resolve`, `case/escalation.open` | Composio; later direct API |
| 5 | `report` | 41 report activities, the densest verb cluster | `report/generate`, `report/analyze` | Composio; Google Drive reserve |
| 6 | `record` | 19 record + 19 audit activities; UC-004 exact retrieval | `record/track`, `record/audit`, `record/retain` | GitHub MCP; Composio |
| 7 | `approval` | 17 approve + 9 approval activities across HR, finance, IT | `approval/request.submit`, `approval/decision.record` | Composio; later direct API |
| 8 | `payment` | 16 pay activities plus invoicing/billing | `payment/invoice.create`, `payment/payment.process`, `payment/refund.issue` | Composio candidate only; requires stricter mutation review |
| 9 | `schedule` | 17 schedule activities across strategy, HR, IT | `schedule/event.create`, `schedule/meeting.plan` | Composio candidate only |
| 10 | `deploy` | 18 IT deploy activities; mutation-heavy and gateway-sensitive | `deploy/service.provision`, `deploy/change.execute` | GitHub MCP candidate only; defer execution claims |

The first-contract recommendation is **identity, communication, and document**,
as requested by the plan, because they combine broad early-workload demand with
lower proof risk than payment or deploy mutations. The shortlist preserves all
ten taxonomy families for later demand evidence; it does not make the lower
ranks launch commitments.

## Selection rationale and rejection conditions

### Selected routes

- **Composio:** gives the catalog an aggregator-shaped route for broad
  capability discovery and lets D2 test whether a normalized operation can be
  captured without confusing aggregation with execution. Reject at D2 if
  redistribution terms, version pinning, or safe fixture capture cannot be
  evidenced.
- **GitHub MCP Server:** provides a genuine MCP transport/server route with a
  public repository, inspectable tool definitions, and a bounded synthetic
  fixture surface. Keep the MCP server route distinct from any GitHub REST
  capture. Reject if the server's tool schemas cannot be pinned and digested.
- **Slack Web API:** provides a direct HTTP API route with documented methods,
  scopes, response shapes, and rate/credential metadata. Use synthetic channel
  and message fixtures only. Reject if license or fixture terms prohibit the
  offline evidence needed by the registry.

### Rejected or reserved alternatives

- **Nango** remains a reserve aggregator: credible breadth, but no reason in
  the checked-in demand proxies to add a second aggregator to a three-route
  M1. Reconsider if Composio fails its D2 evidence gate.
- **Google Drive API** remains a reserve direct route: it is a strong document
  fit, but adding it alongside Slack would expand M1 before the first catalog
  capture is proven. Reconsider if document demand or fixture review shows
  that Drive is a better first direct route.
- Any provider that is only a gateway/execution path, exposes credentials,
  requires live account access for basic capture, or cannot provide immutable
  version evidence is rejected for D1/M1 regardless of demand.

## D1 outcome and handoff

**Outcome:** propose Composio, GitHub MCP Server, and Slack Web API as the
three-provider M1 launch set; prioritize `identity`, `communication`, and
`document` capability contracts; reserve Nango and Google Drive API as scored
alternatives. This outcome is pending David's approval or rejection above.

If approved, D2 must create offline captures and binding goldens for the three
selected routes, record exact digests and licenses, and mark every provider
catalog-only until conformance passes. If rejected, David should name the
replacement route or scoring change; the same criteria and evidence boundary
continue to apply.

## Approval record

**Approved as selected** (operator ruling, 2026-09-25): Composio (aggregator),
GitHub MCP Server (MCP server), and Slack Web API (direct API) proceed to D2
as the three-provider M1 launch set, with `identity`, `communication`, and
`document` as the initial capability-contract families. Nango and Google
Drive API remain scored reserves. D2 must still satisfy every D2 acceptance
criterion before anything is treated as more than catalog-only.
