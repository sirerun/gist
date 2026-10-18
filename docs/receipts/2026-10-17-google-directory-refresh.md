# Official Gmail directory refresh — 2026-10-17

The official Google Gmail Developer Preview entry and immutable screening skill
links are live at https://gist.sire.run/library. The catalog contains 6,495
candidates: 3,914 MCP and 2,581 OpenAPI entries, five recommendations and zero
Featured workflows. Gmail's community OpenAPI/Mint alternative remains available.
This does not establish a connected account or runnable client capability.

## Reviewed source and build

Gist PR96 landed `d4de98cc81a8591782599bbfcd18c85b90ca5ad0`, tree-identical
to independently reviewed `76e94154d59ba25cf38741d01013165d0cb1ef49` against
`ffe0ebe2287a7aa8524534b4b1b0a0f0bb45b10d`. Four offline intake checks and
byte-identical regeneration pass; qualified Linux directory/essentials/CLI race
checks, CLI validation and vet pass. The original 6,494-entry production receipt
remains historical evidence and is not overwritten.

The first image attempt failed before compilation when Docker Hub returned
HTTP429 resolving the Go builder. Foundation PR287 landed
`f3fba0d4c3ef4fb45ecc2ece0fca811ab4aea1ef`: Docker's official ECR Public
Go1.26.2 index is pinned to
`sha256:b54cbf583d390341599d7bcbc062425c081105cc5ef6d170ced98ef9d047c716`.
Its multiarchitecture manifest and publisher annotations were checked; this is
not a claim of independent image-build or signature attestation. Focused
RegistryAWS race tests/vet and exact-source Darwin ARM64 Pulumi crossbuild pass.
Independent source and saved-preview review preceded a buildspec-only apply:
one CodeBuild update, 59 resources unchanged. No retry loop or new capacity was used.

CodeBuild `gist-registry-image:670d2ee1-2383-4d3c-8170-acded4caa3f3` succeeded
at exact Gist source `d4de98cc`. The immutable image is
`sha256:89d28725342f8d2740e572df9fa9f789d8fa321f9b73bf0112b2485ec29c1856`.
Foundation PR288 landed `caee935e229961134675c9452a1adbaaf7a7b760`, exactly
matching reviewed candidate `07d919ac1efce3e76ba86f32d33c916a2e75c427`.
Its independently reviewed saved preview replaced the app task definition and
updated the existing service; 58 resources stayed unchanged. SQL, migration
command, restricted-role sources, migrator image, IAM and secrets were unchanged.
No database mutation was required. Existing migration/RLS evidence remains valid.

## Actual runtime acceptance

ECS `gist-registry:6` is the sole completed deployment, desired/running1 and
pending0. Actual running task `57a4465bdf434dc9ace201db64e3a7fd` reports the
expected immutable image digest above. Health/readiness and live directory
search/get/export pass with snapshot
`sha256:933e80e8d3e39abf5fb919f03a8cb3d32548d4f78af9887cbf8eae2d1ee38a38`.
Official Gmail endpoint, the retained alternative, five screening references and
zero Featured status match the source.

Real public browser checks at widths1440,390,320 pass for home, library and the
exact Gmail detail: no overflow, page errors or unexpected requests. The actual
isolated Darwin ARM64 Zati consumer, unchanged binary
`sha256:2b0ecf6ffa167d3cd7ad9554ca5f732bd3501410be496a846947973ba1ed3da1`
from source `2a2b0324e9d8a2a16e2b9daffe91adc5538a99e5`, sees the new snapshot
and exact five skill IDs/URLs/package digests without rebuilding or reinstalling.
Normal owner-authorized CLI operations passed; connection counts remained0→0,
the original test-instance deadline and installed controller were preserved.
This is metadata acceptance, not installed GUI or provider execution acceptance.

Rollback is the previous app image
`sha256:31f3359b4d10d2e2cbf0bc935d53196bf18cb53ce40974284ce611439cb2e331`
through reviewed IaC. Keep the additive database schema and private data intact.
See the separate Gmail preflight receipt for authenticated provider metadata
observations. No Featured badge or model-quality claim follows from this release.
