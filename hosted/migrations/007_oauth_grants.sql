-- Reference authorization server (I4). oauth_clients gains the scope set a
-- client registered with, so an authorization request can be held to it
-- (reject, never downgrade). A refresh family records the hash of the code it
-- was redeemed from, so a replayed code can revoke every grant made from it.
-- Only hashes of codes and refresh tokens are ever stored.
ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS scope text[] NOT NULL DEFAULT ARRAY[]::text[];
ALTER TABLE oauth_refresh_families ADD COLUMN IF NOT EXISTS code_hash bytea;
CREATE INDEX IF NOT EXISTS oauth_refresh_families_code_hash ON oauth_refresh_families (code_hash);
CREATE INDEX IF NOT EXISTS oauth_refresh_tokens_family ON oauth_refresh_tokens (family_id);
