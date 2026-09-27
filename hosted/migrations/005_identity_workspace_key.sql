-- A subject can hold memberships in several workspaces (workspace_memberships
-- is keyed by workspace_id, issuer, subject), and each mint records its
-- identity under the token's workspace. Key workload_identities the same way,
-- so minting one subject in a second workspace inserts its own row instead of
-- colliding with a row that the second workspace's RLS scope cannot see.
ALTER TABLE workload_identities DROP CONSTRAINT IF EXISTS workload_identities_pkey;
ALTER TABLE workload_identities ADD CONSTRAINT workload_identities_pkey PRIMARY KEY (workspace_id, issuer, subject);
