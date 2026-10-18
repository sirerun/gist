-- Descriptors only. No account credentials, execution or upstream probes.
CREATE TABLE private_directory_entries (
    workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    id uuid NOT NULL DEFAULT pg_catalog.gen_random_uuid(),
    record jsonb NOT NULL CHECK (jsonb_typeof(record)='object' AND octet_length(record::text)<=65536),
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id,id)
);
ALTER TABLE private_directory_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE private_directory_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY private_directory_entries_tenant ON private_directory_entries
    USING (registry_workspace_visible(workspace_id))
    WITH CHECK (registry_workspace_visible(workspace_id));
