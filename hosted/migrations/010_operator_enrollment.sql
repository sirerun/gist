CREATE TABLE github_login_flows (
    state_hash bytea PRIMARY KEY CHECK (octet_length(state_hash) = 32),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);
CREATE INDEX github_login_flows_expiry ON github_login_flows(expires_at);

CREATE TABLE workspace_invitations (
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL CHECK (octet_length(token_hash) = 32),
    github_account_id text NOT NULL CHECK (github_account_id ~ '^[1-9][0-9]{0,18}$'),
    role text NOT NULL CHECK (role IN ('reader','maintainer')),
    scopes text[] NOT NULL CHECK (cardinality(scopes) > 0),
    created_by text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, token_hash)
);
ALTER TABLE workspace_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspace_invitations FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_invitations_tenant ON workspace_invitations
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));
CREATE INDEX workspace_invitations_expiry ON workspace_invitations(workspace_id, expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE founder_bootstrap (
    singleton smallint PRIMARY KEY CHECK (singleton = 1),
    workspace_id text NOT NULL UNIQUE,
    issuer text NOT NULL,
    subject text NOT NULL CHECK (subject ~ '^[1-9][0-9]{0,18}$'),
    completed_at timestamptz NOT NULL DEFAULT now()
);
