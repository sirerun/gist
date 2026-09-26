CREATE TABLE IF NOT EXISTS event_outbox (
    event_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    workspace_id text NOT NULL,
    event_type text NOT NULL CHECK (event_type IN ('version_published', 'version_revoked')),
    artifact_kind text NOT NULL,
    artifact_id text NOT NULL,
    artifact_version text NOT NULL,
    policy_generation bigint NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz
);
CREATE INDEX IF NOT EXISTS event_outbox_workspace_id ON event_outbox (workspace_id, event_id);
CREATE TABLE IF NOT EXISTS event_cursors (
    id text PRIMARY KEY,
    workspace_id text NOT NULL,
    issuer text NOT NULL,
    subject text NOT NULL,
    principal_hash text NOT NULL,
    policy_generation bigint NOT NULL,
    last_event_id bigint NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE event_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE event_outbox FORCE ROW LEVEL SECURITY;
ALTER TABLE event_cursors ENABLE ROW LEVEL SECURITY;
ALTER TABLE event_cursors FORCE ROW LEVEL SECURITY;
CREATE POLICY event_tenant_isolation ON event_outbox USING (registry_workspace_visible(workspace_id)) WITH CHECK (registry_workspace_visible(workspace_id));
CREATE POLICY cursor_tenant_isolation ON event_cursors USING (registry_workspace_visible(workspace_id)) WITH CHECK (registry_workspace_visible(workspace_id));

CREATE OR REPLACE FUNCTION purge_registry_events(retention interval DEFAULT interval '7 days')
RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE removed bigint;
BEGIN
    DELETE FROM event_outbox WHERE occurred_at < now() - retention;
    GET DIAGNOSTICS removed = ROW_COUNT;
    RETURN removed;
END $$;
