# KEYS source lane preflight

Baseline source revision: `235b1f3f86c66d6f943a910369688be46291e1d6`.
Admission/design commit applied: `87cec14` (working-tree commit `f58578c`).
Owned write set: `hosted/internal/identity/keyring.go`,
`hosted/internal/identity/keyring_test.go`, and this receipt.
No edits to `keys.go`, app/config wiring, frozen contracts, or shared plan rows.

## Contract and interface

`identity.LoadKeySet(data []byte, clock Clock) (*KeySet, error)` strictly loads
explicit JSON. `current` has `kid`, `algorithm` (`EdDSA`), `private_key`, and
`public_key`; `retired` is an optional array of `kid`, `algorithm`,
`public_key`, and RFC3339 `retired_at` entries. Key byte values use JSON's
standard padded base64 representation. Retired entries have public bytes only.
Unknown fields, malformed/trailing JSON, invalid algorithm or lengths,
private/public mismatch, missing retirement time, and duplicate/reused key IDs
fail closed. Error text does not wrap decoder errors or contain key bytes.
The loader does not generate keys. Existing startup wiring and actual operator
secret provisioning remain coordinator/operator responsibilities.

## New capability and negative coverage

The previous constructor path accepted a process-local generated identity, so
two independent instances could not interoperate after restart. This component
accepts one explicitly provisioned key source for independent KeySet instances.
The test fixtures exercise independent load and token verification, configured
old-public-key overlap/expiry, duplicate and reused `kid`, mismatched key
material, unsupported algorithm, malformed lengths and JSON, unknown fields,
trailing data, and errors that omit private key bytes.

## Capacity and verification boundary

At assignment the one-minute load was `11.28`, above the repository limit of
`10`; source authoring and gofmt were permitted, while Go checks were held. At
the subsequent check, `uptime` reported `5.79 9.76 9.28`. After coordinator
assigned the single-package slot, an immediate check reported `9.84 10.16
9.43`. `GOWORK=off` identity package tests passed using external SSD Go cache
and temp directories. The next load check reported `15.14 11.38 9.89`, so
scoped vet and race checks were held. No build/lint pass is claimed.
