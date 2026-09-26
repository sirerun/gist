# Q3 composition wiring record

Status: source composition complete; live integration fixture blocked in the
2026-09-25 sandbox.

The app owns composition in `hosted/internal/app`. `New` opens and pings the
real pgx pool, creates the filesystem-backed object store, verifies workload
tokens with `identity.WorkloadIssuer`, adapts the PostgreSQL policy and
resolution stores, installs the discovery service before lexical results reach
REST, and exposes REST plus remote MCP from one HTTP server. A configured HTTPS
broker is used as an opaque connection initiator; no provider secret is stored
or forwarded. With no broker configured, connection routes return the existing
explicit `service_unavailable` response.

Limits are passed from deployment configuration into `app.Config` and then
`rest.Limits`; they are not production constants. `/healthz` is liveness and
`/readyz` reports PostgreSQL readiness separately. REST and MCP requests receive
an `X-Request-ID` when one was not supplied.

## Port deviation / amendment

No frozen port bytes were changed. The frozen `ports.LexicalSearcher` and
`ports.CatalogRecord` do not expose discovery capability metadata, resolution
status, or disclosures. The app therefore retains the richer discovery and
resolution APIs inside their owning packages and adapts discovery candidates to
the existing REST page shape. This is a wiring limitation, not an authorization
grant: policy is still evaluated before discovery ranking, limiting, cache use,
and conditional package responses. If richer metadata must become a typed
cross-lane contract, propose a C-owned port amendment and renew the M1 handoff.

## Environment result

`pg_isready -h 127.0.0.1 -p 5432` reported an accepting server, but a direct
connection to the configured `registry` database failed because that database
does not exist. No object-store or service fixture was provided. The
integration tests use the plan's required fail-closed behavior: absent
`REGISTRY_*` fixture variables fail the test rather than skip. The static IaC
review remains the only deployment evidence; Q5 owns live deployment evidence.
