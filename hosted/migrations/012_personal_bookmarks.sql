-- Personal preferences, separate from global curation.
CREATE TABLE directory_bookmarks (
 workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 issuer text NOT NULL,
 subject text NOT NULL,
 entry_id text NOT NULL CHECK (octet_length(entry_id) BETWEEN 1 AND 512),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (workspace_id, issuer, subject, entry_id)
);
ALTER TABLE directory_bookmarks ENABLE ROW LEVEL SECURITY;
ALTER TABLE directory_bookmarks FORCE ROW LEVEL SECURITY;
CREATE POLICY directory_bookmarks_tenant ON directory_bookmarks
 USING (registry_workspace_visible(workspace_id))
 WITH CHECK (registry_workspace_visible(workspace_id));
