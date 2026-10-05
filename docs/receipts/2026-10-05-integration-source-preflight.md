# Speculative integration source preparation

Coordinator owns shared ports/REST/app configuration, activation, publication adapters and joined fixtures. Candidate component APIs: identity.LoadKeySet; app.NewCanonicalResolver; storage.NewEventCursor and ReadEventPageForPrincipal, RevokeVersionForPrincipal. Source preparation is explicitly speculative under INTEGRATE; existing full-component landed/trust/provider dependencies remain open.

At aa9b7683992c68635af11bed65f062fdaf1bb323, scoped `go test -p2 ./internal/rest` and `go vet -p2 ./internal/rest` passed with external-storage caches/temp and fresh one-minute load8.68. These fixtures prove context-aware cursor selection, current principal and bounded request budgets, public413/409/503 error mapping, and private error redaction. No real-store source candidate or production behavior follows from these test doubles.

Installed golangci-lint is v2 and original repository configuration is v1; direct use is incompatible. An unchanged copy was migrated using the installed tool into task-local external storage. Scoped REST/ports lint found one pre-existing unchecked close in the read-only artifact path; the coordinator now explicitly ignores only this read-only stream close error, as Go conventions permit. A fresh final-source lint/test pass remains required; no lint success is claimed from migration alone.
