# KEYS exact-head review R4

Review tasks: T-GR-KEYS.4, T-GR-KEYS.9, and T-GR-KEYS.12. Reviewer was not an author or coauthor of PR #49. Exact base: `235b1f3f86c66d6f943a910369688be46291e1d6`. Exact reviewed PR head: `da8bbf4959e2639512a108f022ddbee540c2a3b9`.

Scope inspected: `hosted/internal/identity/keys.go`, `hosted/internal/identity/keyring.go`, `hosted/internal/identity/keyring_test.go`, and the KEYS receipts. Static review traced key loading, key ID uniqueness, rotation, retired-key expiry, verification, and JWKS publication.

The prior accepted P2 finding is resolved at this head: `Rotate` now treats an identical current key as a no-op and rejects any KID already present in the key set, so a retired KID cannot be reactivated with a new key or have its expiry overwritten. Exact-boundary expiry is consistently denied by token verification and JWKS publication. Loader review confirmed exact Ed25519 private-key regeneration and full private/public consistency checks, duplicate and case-variant JSON-field rejection at nested levels, bounded retirement overlap, sanitized load errors, and public-only retired material.

Disposition: no additional source or security finding identified in this static exact-head review. Independent source-level verification reproduced the accepted regression and qualified the final source. At pre-fix head `04f9c0df04a8d09d08e95d8481aac8db626e0c22`, an isolated overlay test `TestReviewR4RejectsRetiredKIDReuse` failed as expected with `rotation reused a retired key ID`. At final source head `da8bbf4959e2639512a108f022ddbee540c2a3b9`, these commands passed on fresh one-minute load readings at or below 10:

- `GOWORK=off GOMAXPROCS=2 go test -p 2 -count=1 ./internal/identity`
- `GOWORK=off GOMAXPROCS=2 go test -race -p 2 -count=1 ./internal/identity`
- `GOWORK=off GOMAXPROCS=2 go vet -p 2 ./internal/identity`

All Go build, module, and temporary caches were on the external SSD. The author receipt separately records migrated-config golangci-lint with zero issues; I did not independently rerun lint. No merge or landed status is claimed.
