CREATE TABLE IF NOT EXISTS workspaces (
    id text PRIMARY KEY,
    name text NOT NULL,
    policy_generation bigint NOT NULL DEFAULT 1 CHECK (policy_generation > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS workspace_memberships (
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issuer text NOT NULL,
    subject text NOT NULL,
    role text NOT NULL CHECK (role IN ('reader', 'maintainer')),
    scopes text[] NOT NULL DEFAULT ARRAY[]::text[],
    active boolean NOT NULL DEFAULT true,
    policy_generation bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, issuer, subject)
);
CREATE TABLE IF NOT EXISTS policy_decisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issuer text NOT NULL,
    subject text NOT NULL,
    audience text NOT NULL,
    action text NOT NULL,
    artifact_kind text,
    artifact_id text,
    allowed boolean NOT NULL,
    policy_generation bigint NOT NULL,
    decided_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS cursors (
    id text PRIMARY KEY,
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issuer text NOT NULL,
    subject text NOT NULL,
    audience text NOT NULL,
    principal_hash text NOT NULL,
    query_hash text NOT NULL DEFAULT '',
    sort_version text NOT NULL DEFAULT '1',
    policy_generation bigint NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS resolutions (
    id text PRIMARY KEY,
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issuer text NOT NULL,
    subject text NOT NULL,
    audience text NOT NULL,
    principal_hash text NOT NULL,
    policy_generation bigint NOT NULL,
    skill_id text NOT NULL,
    skill_version text NOT NULL,
    expires_at timestamptz NOT NULL,
    payload jsonb NOT NULL
);
CREATE TABLE IF NOT EXISTS connections (
    id text PRIMARY KEY,
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issuer text NOT NULL,
    subject text NOT NULL,
    capability_id text NOT NULL,
    capability_version text NOT NULL,
    status text NOT NULL,
    connect_url text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['workspace_memberships','policy_decisions','cursors','resolutions','connections'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
        EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (registry_workspace_visible(workspace_id)) WITH CHECK (registry_workspace_visible(workspace_id))', t);
    END LOOP;
END $$;
