package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// ErrRevoked reports that a pinned catalog version exists but was revoked.
// Callers map it to 409 artifact_revoked after authorization (ADR 005).
var ErrRevoked = errors.New("registry version revoked")

// Revocation is the immutable notice recorded for one revoked version.
type Revocation struct {
	Ref       ports.ArtifactRef
	RevokedAt time.Time
	// Created is true when this call revoked the version and false when it
	// was already revoked; a repeat returns the original notice unchanged.
	Created bool
}

// RevokeVersion moves one published or deprecated catalog version to the
// terminal revoked state and records its revocation time. It never inserts a
// catalog version. Revoking an already revoked version returns the existing
// notice with its original time. A missing, foreign, or never-published
// version returns ErrNotFound.
func (s *Postgres) RevokeVersion(ctx context.Context, ref ports.ArtifactRef) (Revocation, error) {
	return s.revokeVersion(ctx, Tenant{WorkspaceID: ref.WorkspaceID}, ref)
}

// RevokeVersionForPrincipal persists the authenticated policy generation in
// the revocation event. Application adapters should prefer this method when
// they have the authorized principal available.
func (s *Postgres) RevokeVersionForPrincipal(ctx context.Context, principal ports.Principal, ref ports.ArtifactRef) (Revocation, error) {
	if principal.WorkspaceID == "" || principal.WorkspaceID != ref.WorkspaceID {
		return Revocation{}, errors.New("storage: revocation principal workspace mismatch")
	}
	return s.revokeVersion(ctx, tenantFromPrincipal(principal), ref)
}

func (s *Postgres) revokeVersion(ctx context.Context, tenant Tenant, ref ports.ArtifactRef) (Revocation, error) {
	if s == nil || s.pool == nil {
		return Revocation{}, errors.New("storage: unavailable postgres revocation store")
	}
	if err := validateRef(ref); err != nil {
		return Revocation{}, err
	}
	out := Revocation{Ref: ref}
	err := WithTenantPrincipal(ctx, s.pool, tenant, func(ctx context.Context, tx pgx.Tx) error {
		var state string
		var revokedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT state, revoked_at FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 FOR UPDATE`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&state, &revokedAt); err != nil {
			return err
		}
		switch state {
		case "revoked":
			if revokedAt != nil {
				out.RevokedAt = revokedAt.UTC()
				return nil
			}
			// A revoked row without a time predates this path; stamp it once.
			return tx.QueryRow(ctx, `UPDATE catalog_versions SET revoked_at=clock_timestamp() WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 RETURNING revoked_at`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&out.RevokedAt)
		case "published", "deprecated":
			out.Created = true
			if err := tx.QueryRow(ctx, `UPDATE catalog_versions SET state='revoked', revoked_at=clock_timestamp() WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 RETURNING revoked_at`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&out.RevokedAt); err != nil {
				return err
			}
			return insertEvent(ctx, tx, ports.Event{
				WorkspaceID:      ref.WorkspaceID,
				Type:             ports.EventVersionRevoked,
				Subject:          ref,
				PolicyGeneration: tenant.PolicyGeneration,
			})
		default:
			return pgx.ErrNoRows
		}
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Revocation{}, ErrNotFound
	}
	if err != nil {
		return Revocation{}, fmt.Errorf("revoke catalog version: %w", err)
	}
	out.RevokedAt = out.RevokedAt.UTC()
	return out, nil
}
