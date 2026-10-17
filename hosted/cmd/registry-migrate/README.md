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
