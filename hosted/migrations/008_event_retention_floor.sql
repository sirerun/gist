CREATE TABLE IF NOT EXISTS event_retention_floors (
    workspace_id text PRIMARY KEY,
    through_event_id bigint NOT NULL CHECK (through_event_id >= 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE event_retention_floors ALTER COLUMN updated_at SET DEFAULT clock_timestamp();

-- PostgreSQL now() is transaction-start time. Outbox events can be inserted
-- after waiting on a lock, so their durable retention timestamp must reflect
-- the actual insertion clock.
ALTER TABLE event_outbox ALTER COLUMN occurred_at SET DEFAULT clock_timestamp();

ALTER TABLE event_cursors
    ADD COLUMN IF NOT EXISTS cursor_order bigint GENERATED ALWAYS AS IDENTITY;
CREATE INDEX IF NOT EXISTS event_cursors_principal_order
    ON event_cursors (principal_hash, cursor_order);

ALTER TABLE event_retention_floors ENABLE ROW LEVEL SECURITY;
ALTER TABLE event_retention_floors FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS event_retention_floor_tenant_isolation ON event_retention_floors;
CREATE POLICY event_retention_floor_tenant_isolation ON event_retention_floors
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));

CREATE OR REPLACE FUNCTION purge_registry_events(retention interval DEFAULT interval '7 days')
RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE removed bigint;
BEGIN
    WITH purged AS (
        DELETE FROM event_outbox
        WHERE occurred_at < clock_timestamp() - retention
          AND registry_workspace_visible(workspace_id)
        RETURNING workspace_id, event_id
    ), floors AS (
        SELECT workspace_id, max(event_id) AS through_event_id
        FROM purged
        GROUP BY workspace_id
    ), saved AS (
        INSERT INTO event_retention_floors (workspace_id, through_event_id)
        SELECT workspace_id, through_event_id FROM floors
        ON CONFLICT (workspace_id) DO UPDATE
          SET through_event_id = GREATEST(event_retention_floors.through_event_id, EXCLUDED.through_event_id),
              updated_at = clock_timestamp()
        RETURNING workspace_id
    )
    SELECT count(*) INTO removed FROM purged;
    RETURN removed;
END $$;
