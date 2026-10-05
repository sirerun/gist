package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// PutPinnedResolution persists every immutable canonical pin. The frozen legacy
// Put/Get methods remain separate and retain their original port types.
func (s *postgresResolutionStore) PutPinnedResolution(ctx context.Context, r ports.PinnedResolution) error {
	if s == nil || s.pool == nil {
		return errors.New("resolution store unavailable")
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode pinned resolution: %w", err)
	}
	p := r.Principal
	return storage.WithTenantPrincipal(ctx, s.pool, storage.Tenant{Issuer: p.Issuer, Subject: p.Subject, Audience: p.Audience, WorkspaceID: p.WorkspaceID, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO resolutions(id,workspace_id,issuer,subject,audience,principal_hash,policy_generation,skill_id,skill_version,expires_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,to_timestamp($10),$11)`, r.ID, p.WorkspaceID, p.Issuer, p.Subject, p.Audience, ports.PrincipalHash(p), p.PolicyGeneration, r.Skill.ID, r.Skill.Version, r.ExpiresAt, raw)
		return err
	})
}

func (s *postgresResolutionStore) GetPinnedResolution(ctx context.Context, c ports.Cursor) (ports.PinnedResolution, error) {
	if s == nil || s.pool == nil || c.WorkspaceID == "" || c.PrincipalHash == "" {
		return ports.PinnedResolution{}, storage.ErrNotFound
	}
	var raw []byte
	err := storage.WithTenant(ctx, s.pool, c.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT payload FROM resolutions WHERE id=$1 AND workspace_id=$2 AND principal_hash=$3 AND expires_at > clock_timestamp()`, c.ID, c.WorkspaceID, c.PrincipalHash).Scan(&raw)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.PinnedResolution{}, storage.ErrNotFound
	}
	if err != nil {
		return ports.PinnedResolution{}, fmt.Errorf("read pinned resolution: %w", err)
	}
	var r ports.PinnedResolution
	if err := json.Unmarshal(raw, &r); err != nil {
		return ports.PinnedResolution{}, fmt.Errorf("decode pinned resolution: %w", err)
	}
	if r.Principal.WorkspaceID != c.WorkspaceID || ports.PrincipalHash(r.Principal) != c.PrincipalHash || r.ID != c.ID || r.ExpiresAt <= time.Now().Unix() {
		return ports.PinnedResolution{}, storage.ErrNotFound
	}
	return r, nil
}

var _ ports.PinnedResolutionStore = (*postgresResolutionStore)(nil)
