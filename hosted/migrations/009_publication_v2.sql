-- Additive byte-domain and durable ownership fields for v2 publication.
ALTER TABLE catalog_versions
    ALTER COLUMN manifest_digest_algorithm DROP NOT NULL,
    ALTER COLUMN manifest_digest_value DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS document_digest_algorithm text,
    ADD COLUMN IF NOT EXISTS document_digest_value text,
    ADD COLUMN IF NOT EXISTS package_digest_algorithm text,
    ADD COLUMN IF NOT EXISTS package_digest_value text,
    ADD COLUMN IF NOT EXISTS artifact_bytes bytea,
    ADD COLUMN IF NOT EXISTS metadata_bytes bytea,
    ADD COLUMN IF NOT EXISTS object_key text,
    ADD COLUMN IF NOT EXISTS publication_receipt bytea;

CREATE TABLE publication_attempts (
    workspace_id text NOT NULL,
    attempt_id text NOT NULL CHECK (attempt_id ~ '^[0-9a-f]{32}$'),
    kind text NOT NULL,
    artifact_id text NOT NULL,
    version text NOT NULL,
    idempotency_key text NOT NULL,
    artifact_digest text NOT NULL CHECK (artifact_digest ~ '^[0-9a-f]{64}$'),
    object_key text NOT NULL,
    state text NOT NULL CHECK (state IN ('staged','committed','deleting','retired')),
    generation bigint NOT NULL DEFAULT 1,
    lease_until timestamptz NOT NULL,
    claim_until timestamptz,
    artifact_size bigint NOT NULL CHECK (artifact_size >= 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (workspace_id, attempt_id)
);
ALTER TABLE publication_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE publication_attempts FORCE ROW LEVEL SECURITY;
CREATE POLICY publication_attempts_tenant ON publication_attempts
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));
CREATE INDEX publication_attempts_expired ON publication_attempts(workspace_id, lease_until, attempt_id) WHERE state IN ('staged','deleting','retired');
-- Each physical object belongs to exactly one permanent attempt tombstone.
CREATE UNIQUE INDEX publication_attempts_object_key_unique ON publication_attempts(object_key);

CREATE TABLE publication_idempotency (
    workspace_id text NOT NULL,
    kind text NOT NULL,
    idempotency_key text NOT NULL,
    artifact_id text NOT NULL,
    version text NOT NULL,
    artifact_digest text NOT NULL CHECK (artifact_digest ~ '^[0-9a-f]{64}$'),
    attempt_id text NOT NULL,
    response_body bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (workspace_id, kind, idempotency_key),
    FOREIGN KEY (workspace_id, attempt_id) REFERENCES publication_attempts(workspace_id, attempt_id) ON DELETE RESTRICT
);
ALTER TABLE publication_idempotency ENABLE ROW LEVEL SECURITY;
ALTER TABLE publication_idempotency FORCE ROW LEVEL SECURITY;
CREATE POLICY publication_idempotency_tenant ON publication_idempotency
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));
CREATE INDEX publication_idempotency_version ON publication_idempotency(workspace_id,kind,artifact_id,version);

-- Preserve taxonomy source bytes and make taxonomy projections part of the
-- same catalog transaction as publication.
ALTER TABLE taxonomy_editions ADD COLUMN IF NOT EXISTS source_bytes bytea;
