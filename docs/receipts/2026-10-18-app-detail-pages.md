# App discovery pages — 2026-10-18

Owner request: keep card hover to one category; replace query-string detail URLs with indexable paths; use richer app and workflow pages to explain Zati. The supplied Zati Vertical Sites design explores property operations, recruiting and software teams through worker ownership, selected apps and human review. Those designs are inspiration, not implemented vertical template evidence.

## Implementation

- Public app pages at `/apps/{slug}` render name, description, categories, workflow prompts, connection options, related apps and Zati links in initial HTML. Selected core apps have curated examples; remaining apps use a conservative planning prompt.
- A curated `/apps/openai-gpt-3/airtable` page explains a content review workflow, prerequisites and review points. No arbitrary app-pair pages or ready-made execution claims.
- Connector details use `/connectors/{id}`; existing app/connector query URLs permanently redirect after catalog lookup. Unknown entries return404. Existing connector bookmarks and descriptor export remain client features.
- Unique titles/descriptions, absolute canonical links, OpenGraph tags, public sitemap and robots discovery. XML URLs derive only from the admitted public catalog; no workspace/private entries.
- Cards contain no description paragraphs. One category/tag remains available on hover or keyboard focus, without growing the row.
- Gist continues catalog discovery and setup metadata. Zati or another client owns account connection, selected tools, execution and review. Workflow ideas do not establish account grants, provider operation coverage or qualified vertical automation.

## Verification

Local Node behavior tests17/17, Go webui tests/vet and desktop/mobile browser checks cover readable initial HTML, canonical URLs, permanent redirects, unknown routes, public sitemap, hover simplicity, no horizontal overflow and JavaScript-disabled app content. Browser widths1440/390/320. Public API data in the local browser harness is read from production; the page, assets and server routes are the candidate. This is not deployed acceptance.

Production evidence is recorded separately after the immutable build and reviewed IaC release.
