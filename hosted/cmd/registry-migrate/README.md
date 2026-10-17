# Registry schema release tool

The image contains `/registry-migrate` separately from `/registry`. The server
never invokes it. Supply the release-task database through `GIST_DATABASE_URL`;
connection strings and SQL errors are not printed.

- No arguments, or `--status`: repeatable-read, read-only catalog inspection.
  Reports embedded migration manifest SHA256, current public-schema definition
  SHA256, relation names, applied ledger and pending versions. No row values,
  credentials, DDL, adoption or migration execution.
- `--apply`: take the shared session advisory lock, verify the complete applied
  prefix against embedded exact SQL hashes, then execute each pending migration
  and its ledger row in one transaction. A failed migration rolls both back.
  Already applied versions are not replayed. No down migrations are supported.
- `--adopt-through=N --expected-manifest-sha256=HEX
  --expected-schema-sha256=HEX`: under the same lock, record an explicitly
  operator-audited existing prefix without executing that prefix's SQL. Both
  exact hashes must match fresh status. Existing ledger entries, empty schemas,
  unknown prefixes and mismatches refuse adoption. Apply the suffix separately.

A pre-existing public schema without a ledger refuses automatic application.
Before adoption the release owner must audit which exact migration prefix the
schema implements, confirm grants/runtime role separation, and qualify its
backup/recovery point. Matching hashes detect drift; they do not prove that the
chosen prefix is correct or that a backup exists. Keep that audit in the release
receipt. Never select a prefix merely from the newest table's name.

The fingerprint covers public relation definitions, columns, constraints,
indexes, policies, triggers, functions, custom types/enums, sequence parameters,
ACLs and extension versions. It intentionally excludes the ledger and all row
contents. It is an audit fingerprint of this database, not a portable semantic
schema comparison or proof against uncoordinated administrative DDL. Release
operators must serialize all schema changes through this tool. The default
five-minute deadline is configurable with `--timeout` up to fifteen minutes.

Do not use the application runtime role for schema mutation. The one-shot task
needs the approved migration owner and network boundary; it needs no GitHub App,
OAuth, signing or provider credentials.

## Runtime database role

After the audited migration ledger is fully applied, run the separate explicit
`--provision-runtime-role` operation. It requires `GIST_RUNTIME_DATABASE_URL`
pointing at the exact same host, port and database with a distinct lowercase role
name and a strong printable-ASCII password of at least 32 bytes. The application
receives this URL; only the migration task retains the schema owner's URL.

Provisioning creates a LOGIN role with NOSUPERUSER, NOBYPASSRLS, NOCREATEDB,
NOCREATEROLE, NOINHERIT and NOREPLICATION. Unsafe existing role attributes,
memberships, ownership, DDL powers or unreviewed grants refuse provisioning.
Only a salted SCRAM verifier enters role SQL. Passwords and connection strings
are never emitted in status or errors. Re-running this explicit operation can
rotate that role's password.

The dedicated registry database's PUBLIC CREATE/TEMP defaults and public-schema
CREATE/default function EXECUTE are revoked transactionally. The source declares
specific table, sequence and function grants; it gives no migration ledger or
DDL access. Namespace reservation and trusted review tables are read-only. V2
publication staging tables receive no runtime grant while that publisher remains
disabled. New migrations that need new runtime access require an explicit source
change and provisioning run; blanket future-object grants are not installed.

## Read-only deployment qualification

`--verify-runtime-role` authenticates through `GIST_RUNTIME_DATABASE_URL` and
returns a small JSON attestation containing only the runtime username and boolean
observations. The complete operation uses a repeatable-read READ ONLY transaction:
role attributes, memberships, nonownership, required/forbidden grants, forced RLS,
and reads with empty and freshly generated nonexistent workspace context. Scope
settings are transaction-local; no rows, roles or definitions are changed. Row
hiding can be vacuous on an empty production database, so seeded integration
qualification remains separate evidence. A failed check emits a sanitized error
and no successful attestation. The migration-owner URL is used only to bind the
expected host, port, database and distinct identity.

`--schema-definitions` is an explicit read-only audit mode for legacy-baseline
review. It reports sorted public-schema definition metadata and the exact SHA256
used by `--status`. It includes columns, constraints, indexes, policies, functions
and ACL metadata, never application row contents. Treat this diagnostic as an
operator audit artifact. Compare those actual definitions with the selected
source prefix before authorizing adoption; relation names alone are insufficient.
The default status output remains unchanged and excludes full definitions.
