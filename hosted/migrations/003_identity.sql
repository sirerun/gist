ALTER TABLE workload_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE workload_identities FORCE ROW LEVEL SECURITY;
ALTER TABLE oauth_authorization_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_authorization_codes FORCE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_families FORCE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_refresh_tokens FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS identity_workspace ON workload_identities;
CREATE POLICY identity_workspace ON workload_identities
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));
DROP POLICY IF EXISTS code_workspace ON oauth_authorization_codes;
CREATE POLICY code_workspace ON oauth_authorization_codes
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));
DROP POLICY IF EXISTS refresh_family_workspace ON oauth_refresh_families;
CREATE POLICY refresh_family_workspace ON oauth_refresh_families
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));
DROP POLICY IF EXISTS refresh_token_family_workspace ON oauth_refresh_tokens;
CREATE POLICY refresh_token_family_workspace ON oauth_refresh_tokens
    USING (EXISTS (
        SELECT 1
        FROM oauth_refresh_families AS f
        WHERE f.family_id = oauth_refresh_tokens.family_id
          AND registry_workspace_visible(f.workspace_id)
    ))
    WITH CHECK (EXISTS (
        SELECT 1
        FROM oauth_refresh_families AS f
        WHERE f.family_id = oauth_refresh_tokens.family_id
          AND registry_workspace_visible(f.workspace_id)
    ));
