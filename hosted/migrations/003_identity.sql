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

ALTER TABLE workload_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE workload_identities FORCE ROW LEVEL SECURITY;
ALTER TABLE oauth_authorization_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_authorization_codes FORCE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_families FORCE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY identity_workspace ON workload_identities USING (registry_workspace_visible(workspace_id)) WITH CHECK (registry_workspace_visible(workspace_id));
CREATE POLICY code_workspace ON oauth_authorization_codes USING (registry_workspace_visible(workspace_id)) WITH CHECK (registry_workspace_visible(workspace_id));
CREATE POLICY refresh_family_workspace ON oauth_refresh_families USING (registry_workspace_visible(workspace_id)) WITH CHECK (registry_workspace_visible(workspace_id));
CREATE POLICY refresh_token_family_workspace ON oauth_refresh_tokens USING (EXISTS (SELECT 1 FROM oauth_refresh_families f WHERE f.family_id = family_id));
