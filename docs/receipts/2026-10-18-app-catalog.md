# Application catalog and local brand artwork

The owner authorized completing and deploying the harvested application catalog,
compact cards, public-copy cleanup and brand-preserving logo conversion.

## Delivered behavior

- Application browsing contains 3,380 listings with category filters, bounded
  search/pagination, app details and local brand artwork.
- The existing 127 admitted MCP descriptors remain distinct. An app links to a
  connector only by exact identity or one of 15 reviewed aliases. Missing routes
  are explicitly unavailable; a source listing does not create an MCP server.
- Generated artwork comprises 1,267 vector silhouettes over explicit brand
  colors, 2,077 PNGs preserving opaque/multicolor artwork and 36 colored initials
  where source images were missing, denied, timed out or unsuitable for tracing.
  No remote image requests are needed at runtime. Per-asset intake diagnostics
  remain outside the public catalog.
- The generic repeated glyph experiment is superseded. White logo tracing uses
  normalized alpha coverage and B/W spline fitting. Background and foreground
  are separate SVG geometry and colors. The conversion CLI supports explicit
  background/foreground colors and corner radius.
- Public HTTP, MCP and saved-bookmark responses omit upstream intake, probes
  and qualification evidence. Required v1 descriptor fields remain compatible
  with strict clients: provenance identifies the Gist catalog, skill integrity
  references remain, and connector icons use a separate small local manifest.
- Compact centered cards keep the existing palette, reveal useful copy on
  hover/focus and retain connector bookmark controls. App cards have no invented
  connection or bookmark capability. Existing private records are untouched.

## Verification before release

Observed passing: 27 Python intake tests, 13 JavaScript behavior tests, registry
contract freeze check and 14 registry checker tests; Go directory, hosted app,
directory MCP and embedded web UI package tests and vet. The directory CLI also
compiles under its package test. Whole-snapshot public descriptors strictly
decode and validate against the existing Record contract. The bookmark response
privacy regression has its own behavioral test.

All 3,380 referenced assets exist; generated SVGs parse and have matching brand
backgrounds without scripts or external image references. Asset source/private
path scans and diff whitespace checks pass. The first 30 brand tiles were
visually inspected together. No provider execution, account authorization,
database reset/migration, paid model calls or Featured workflow qualification
was performed. Full PostgreSQL integration was not rerun: this release changes
metadata projection and embedded browsing rather than persistence or grants.

This source receipt records pre-release verification. Exact GitHub review,
landed revisions, immutable AWS build/image, constrained IaC activation and live
browser/API acceptance are recorded by the release coordinator after those
operations; this document does not substitute for those observations.

This release supersedes the historical public display of intake/operator/probe
labels recorded in T-DIR.63; its original dated completion remains historical.
