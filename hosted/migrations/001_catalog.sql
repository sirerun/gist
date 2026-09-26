CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS catalog_versions (
    workspace_id text NOT NULL,
    kind text NOT NULL,
    artifact_id text NOT NULL,
    version text NOT NULL,
    state text NOT NULL CHECK (state IN ('draft', 'screening', 'approved', 'published', 'deprecated', 'rejected', 'revoked')),
    digest_algorithm text NOT NULL,
    digest_value text NOT NULL,
    manifest_digest_algorithm text NOT NULL DEFAULT 'sha256',
    manifest_digest_value text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    package_key text,
    owner_id text NOT NULL,
    trust text NOT NULL DEFAULT 'operator_asserted' CHECK (trust = 'operator_asserted'),
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    revoked_at timestamptz,
    PRIMARY KEY (workspace_id, kind, artifact_id, version)
);
ALTER TABLE catalog_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_versions FORCE ROW LEVEL SECURITY;
CREATE INDEX IF NOT EXISTS catalog_versions_lookup ON catalog_versions (workspace_id, kind, artifact_id, version);

CREATE TABLE IF NOT EXISTS catalog_payloads (
    workspace_id text NOT NULL,
    kind text NOT NULL,
    artifact_id text NOT NULL,
    version text NOT NULL,
    package_key text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    digest_algorithm text NOT NULL,
    digest_value text NOT NULL,
    manifest jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, kind, artifact_id, version),
    FOREIGN KEY (workspace_id, kind, artifact_id, version)
        REFERENCES catalog_versions(workspace_id, kind, artifact_id, version)
        ON DELETE RESTRICT
);
ALTER TABLE catalog_payloads ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_payloads FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS namespace_reservations (
    prefix text PRIMARY KEY,
    workspace_id text NOT NULL,
    owner_id text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'retired')),
    created_at timestamptz NOT NULL DEFAULT now(),
    retired_at timestamptz
);
ALTER TABLE namespace_reservations ENABLE ROW LEVEL SECURITY;
ALTER TABLE namespace_reservations FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS review_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id text NOT NULL,
    kind text NOT NULL,
    artifact_id text NOT NULL,
    version text NOT NULL,
    decision text NOT NULL CHECK (decision IN ('approved', 'rejected', 'held')),
    reviewer_id text NOT NULL,
    evidence jsonb NOT NULL,
    reviewed_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE review_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE review_records FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS taxonomy_editions (
    workspace_id text NOT NULL,
    taxonomy_id text NOT NULL,
    version text NOT NULL,
    attribution jsonb NOT NULL,
    license text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, taxonomy_id, version)
);
ALTER TABLE taxonomy_editions ENABLE ROW LEVEL SECURITY;
ALTER TABLE taxonomy_editions FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS taxonomy_nodes (
    workspace_id text NOT NULL,
    taxonomy_id text NOT NULL,
    version text NOT NULL,
    node_id text NOT NULL,
    parent_id text,
    label text NOT NULL,
    level integer NOT NULL CHECK (level >= 0),
    apqc_ref text,
    PRIMARY KEY (workspace_id, taxonomy_id, version, node_id),
    FOREIGN KEY (workspace_id, taxonomy_id, version)
        REFERENCES taxonomy_editions(workspace_id, taxonomy_id, version)
        ON DELETE RESTRICT
);
ALTER TABLE taxonomy_nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE taxonomy_nodes FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS workload_identities (
    issuer text NOT NULL,
    subject text NOT NULL,
    subject_type text NOT NULL CHECK (subject_type IN ('workload', 'human')),
    workspace_id text NOT NULL,
    scopes text[] NOT NULL DEFAULT ARRAY[]::text[],
    policy_generation bigint NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    PRIMARY KEY (issuer, subject)
);
CREATE TABLE IF NOT EXISTS oauth_clients (
    client_id text PRIMARY KEY,
    client_name text NOT NULL,
    redirect_uris text[] NOT NULL,
    audience text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code_hash bytea PRIMARY KEY,
    client_id text NOT NULL REFERENCES oauth_clients(client_id),
    subject text NOT NULL,
    workspace_id text NOT NULL,
    redirect_uri text NOT NULL,
    resource text NOT NULL,
    scope text[] NOT NULL,
    code_challenge text NOT NULL,
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);
CREATE TABLE IF NOT EXISTS oauth_refresh_families (
    family_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id text NOT NULL REFERENCES oauth_clients(client_id),
    subject text NOT NULL,
    workspace_id text NOT NULL,
    resource text NOT NULL,
    scope text[] NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    token_hash bytea PRIMARY KEY,
    family_id uuid NOT NULL REFERENCES oauth_refresh_families(family_id) ON DELETE CASCADE,
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);

CREATE OR REPLACE FUNCTION registry_workspace_visible(row_workspace text)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT row_workspace = current_setting('registry.workspace_id', true)
$$;

DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['catalog_versions','catalog_payloads','namespace_reservations','review_records','taxonomy_editions','taxonomy_nodes'] LOOP
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
        EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (registry_workspace_visible(workspace_id)) WITH CHECK (registry_workspace_visible(workspace_id))', t);
    END LOOP;
END $$;
