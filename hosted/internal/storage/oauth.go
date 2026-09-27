package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/oauth"
)

// OAuthStore persists the reference authorization server. Clients are global
// (registration happens before any workspace is chosen). Codes, refresh
// families and refresh tokens are workspace-scoped and every access runs in
// WithTenant, so forced row-level security confines each call to the
// workspace the credential was issued in. Only SHA-256 hashes of codes and
// refresh tokens reach the database.
type OAuthStore struct{ s *Postgres }

// OAuth returns the authorization-server store on the same pool.
func (s *Postgres) OAuth() *OAuthStore { return &OAuthStore{s: s} }

func (o *OAuthStore) CreateClient(ctx context.Context, c oauth.Client) error {
	if c.ID == "" || len(c.RedirectURIs) == 0 || len(c.Scopes) == 0 || c.Audience == "" {
		return errors.New("storage: incomplete oauth client")
	}
	_, err := o.s.pool.Exec(ctx, `INSERT INTO oauth_clients(client_id,client_name,redirect_uris,audience,scope,created_at) VALUES($1,$2,$3,$4,$5,$6)`, c.ID, c.Name, c.RedirectURIs, c.Audience, c.Scopes, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("create oauth client: %w", err)
	}
	return nil
}

func (o *OAuthStore) GetClient(ctx context.Context, clientID string) (oauth.Client, error) {
	if clientID == "" {
		return oauth.Client{}, oauth.ErrNotFound
	}
	var c oauth.Client
	err := o.s.pool.QueryRow(ctx, `SELECT client_id,client_name,redirect_uris,audience,scope,created_at FROM oauth_clients WHERE client_id=$1`, clientID).Scan(&c.ID, &c.Name, &c.RedirectURIs, &c.Audience, &c.Scopes, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.Client{}, oauth.ErrNotFound
	}
	if err != nil {
		return oauth.Client{}, fmt.Errorf("get oauth client: %w", err)
	}
	return c, nil
}

func (o *OAuthStore) SaveCode(ctx context.Context, c oauth.AuthCode) error {
	if len(c.Hash) == 0 || c.WorkspaceID == "" {
		return errors.New("storage: incomplete authorization code")
	}
	return WithTenant(ctx, o.s.pool, c.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO oauth_authorization_codes(code_hash,client_id,subject,workspace_id,redirect_uri,resource,scope,code_challenge,issued_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, c.Hash, c.ClientID, c.Subject, c.WorkspaceID, c.RedirectURI, c.Resource, c.Scopes, c.Challenge, c.IssuedAt, c.ExpiresAt)
		if err != nil {
			return fmt.Errorf("save authorization code: %w", err)
		}
		return nil
	})
}

// RedeemCode consumes the code and opens its refresh family in one
// transaction, so a concurrent replay either sees an unconsumed code (and
// waits on the row lock) or sees a consumed code whose family already exists
// and revokes it. A failed check still commits the consumption: a code that
// has been presented with a wrong verifier, client or redirect URI is dead.
func (o *OAuthStore) RedeemCode(ctx context.Context, workspaceID string, hash []byte, check func(oauth.AuthCode) error, first oauth.RefreshToken) (oauth.AuthCode, string, error) {
	if workspaceID == "" || len(hash) == 0 || check == nil || len(first.Hash) == 0 {
		return oauth.AuthCode{}, "", oauth.ErrNotFound
	}
	var code oauth.AuthCode
	var familyID string
	var checkErr error
	reused := false
	err := WithTenant(ctx, o.s.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var consumed *time.Time
		err := tx.QueryRow(ctx, `SELECT code_hash,client_id,subject,workspace_id,redirect_uri,resource,scope,code_challenge,issued_at,expires_at,consumed_at FROM oauth_authorization_codes WHERE code_hash=$1 FOR UPDATE`, hash).
			Scan(&code.Hash, &code.ClientID, &code.Subject, &code.WorkspaceID, &code.RedirectURI, &code.Resource, &code.Scopes, &code.Challenge, &code.IssuedAt, &code.ExpiresAt, &consumed)
		if errors.Is(err, pgx.ErrNoRows) {
			return oauth.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("load authorization code: %w", err)
		}
		if consumed != nil {
			// Replay: revoke everything issued from this code, then commit.
			reused = true
			if _, err := tx.Exec(ctx, `UPDATE oauth_refresh_families SET revoked_at=COALESCE(revoked_at, now()) WHERE code_hash=$1`, hash); err != nil {
				return fmt.Errorf("revoke grants from replayed code: %w", err)
			}
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE oauth_authorization_codes SET consumed_at=now() WHERE code_hash=$1`, hash); err != nil {
			return fmt.Errorf("consume authorization code: %w", err)
		}
		if checkErr = check(code); checkErr != nil {
			// Commit the consumption; the redemption itself fails below.
			return nil
		}
		if err := tx.QueryRow(ctx, `INSERT INTO oauth_refresh_families(client_id,subject,workspace_id,resource,scope,code_hash) VALUES($1,$2,$3,$4,$5,$6) RETURNING family_id::text`, code.ClientID, code.Subject, code.WorkspaceID, code.Resource, code.Scopes, code.Hash).Scan(&familyID); err != nil {
			return fmt.Errorf("create refresh family: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO oauth_refresh_tokens(token_hash,family_id,issued_at,expires_at) VALUES($1,$2::uuid,$3,$4)`, first.Hash, familyID, first.IssuedAt, first.ExpiresAt); err != nil {
			return fmt.Errorf("create refresh token: %w", err)
		}
		return nil
	})
	switch {
	case err != nil:
		return oauth.AuthCode{}, "", err
	case reused:
		return oauth.AuthCode{}, "", oauth.ErrGrantReused
	case checkErr != nil:
		return oauth.AuthCode{}, "", checkErr
	}
	return code, familyID, nil
}

func (o *OAuthStore) RotateRefresh(ctx context.Context, workspaceID string, hash []byte, check func(oauth.RefreshFamily, oauth.RefreshToken) error, next oauth.RefreshToken) (oauth.RefreshFamily, error) {
	if workspaceID == "" || len(hash) == 0 || len(next.Hash) == 0 || check == nil {
		return oauth.RefreshFamily{}, oauth.ErrNotFound
	}
	var f oauth.RefreshFamily
	reused := false
	err := WithTenant(ctx, o.s.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var t oauth.RefreshToken
		var consumed, revoked *time.Time
		err := tx.QueryRow(ctx, `SELECT t.token_hash,t.issued_at,t.expires_at,t.consumed_at,f.family_id::text,f.client_id,f.subject,f.workspace_id,f.resource,f.scope,f.code_hash,f.revoked_at
FROM oauth_refresh_tokens t JOIN oauth_refresh_families f ON f.family_id=t.family_id
WHERE t.token_hash=$1 FOR UPDATE OF t, f`, hash).
			Scan(&t.Hash, &t.IssuedAt, &t.ExpiresAt, &consumed, &f.ID, &f.ClientID, &f.Subject, &f.WorkspaceID, &f.Resource, &f.Scopes, &f.CodeHash, &revoked)
		if errors.Is(err, pgx.ErrNoRows) {
			return oauth.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("load refresh token: %w", err)
		}
		if revoked != nil {
			return oauth.ErrFamilyRevoked
		}
		if consumed != nil {
			reused = true
			if _, err := tx.Exec(ctx, `UPDATE oauth_refresh_families SET revoked_at=COALESCE(revoked_at, now()) WHERE family_id=$1::uuid`, f.ID); err != nil {
				return fmt.Errorf("revoke reused refresh family: %w", err)
			}
			return nil
		}
		if err := check(f, t); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE oauth_refresh_tokens SET consumed_at=now() WHERE token_hash=$1`, hash); err != nil {
			return fmt.Errorf("consume refresh token: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO oauth_refresh_tokens(token_hash,family_id,issued_at,expires_at) VALUES($1,$2::uuid,$3,$4)`, next.Hash, f.ID, next.IssuedAt, next.ExpiresAt); err != nil {
			return fmt.Errorf("rotate refresh token: %w", err)
		}
		return nil
	})
	if err != nil {
		return oauth.RefreshFamily{}, err
	}
	if reused {
		return oauth.RefreshFamily{}, oauth.ErrGrantReused
	}
	return f, nil
}

func (o *OAuthStore) RevokeFamily(ctx context.Context, workspaceID, familyID string) error {
	if workspaceID == "" || familyID == "" {
		return oauth.ErrNotFound
	}
	return WithTenant(ctx, o.s.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE oauth_refresh_families SET revoked_at=COALESCE(revoked_at, now()) WHERE family_id=$1::uuid`, familyID)
		if err != nil {
			return fmt.Errorf("revoke refresh family: %w", err)
		}
		return nil
	})
}

func (o *OAuthStore) RevokeRefresh(ctx context.Context, workspaceID string, hash []byte, clientID string) error {
	if workspaceID == "" || len(hash) == 0 || clientID == "" {
		return oauth.ErrNotFound
	}
	var affected int64
	err := WithTenant(ctx, o.s.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE oauth_refresh_families f SET revoked_at=COALESCE(f.revoked_at, now()) FROM oauth_refresh_tokens t WHERE t.family_id=f.family_id AND t.token_hash=$1 AND f.client_id=$2`, hash, clientID)
		affected = command.RowsAffected()
		if err != nil {
			return fmt.Errorf("revoke refresh token: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return oauth.ErrNotFound
	}
	return nil
}

var _ oauth.Store = (*OAuthStore)(nil)
