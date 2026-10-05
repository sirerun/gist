// Package testfixtures holds explicit, test-owned startup fixtures shared by
// the acceptance suites.
package testfixtures

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/app"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/storage"
)

const maintenanceSubject = "acceptance-fixture-event-maintainer"

// SigningKeyConfig returns ephemeral Ed25519 key material solely for a local
// acceptance-test app instance. Production configuration remains operator-owned.
func SigningKeyConfig() ([]byte, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate acceptance signing key: %w", err)
	}
	raw, err := json.Marshal(identity.KeySetConfig{Current: identity.KeyConfig{
		KID:       "acceptance-fixture",
		Algorithm: "EdDSA",
		Private:   private,
		Public:    public,
	}})
	if err != nil {
		return nil, fmt.Errorf("marshal acceptance signing key: %w", err)
	}
	return raw, nil
}

// MaintenanceTargets returns one dedicated maintenance principal per
// configured workspace. It does not alter any existing acceptance principal.
func MaintenanceTargets(workspaceIDs []string) []app.MaintenanceTarget {
	targets := make([]app.MaintenanceTarget, 0, len(workspaceIDs))
	seen := make(map[string]struct{}, len(workspaceIDs))
	for _, workspaceID := range workspaceIDs {
		if _, ok := seen[workspaceID]; ok {
			continue
		}
		seen[workspaceID] = struct{}{}
		targets = append(targets, app.MaintenanceTarget{WorkspaceID: workspaceID, Subject: maintenanceSubject})
	}
	return targets
}

// SeedMaintenanceTargets creates only the dedicated fixture maintainer rows
// required by app.New's startup purge. The workspace and membership writes use
// the same tenant-scoped transaction helper as the acceptance catalog seeds.
func SeedMaintenanceTargets(ctx context.Context, pool *pgxpool.Pool, issuer string, targets []app.MaintenanceTarget) error {
	for _, target := range targets {
		if target.WorkspaceID == "" || target.Subject != maintenanceSubject {
			return fmt.Errorf("invalid acceptance maintenance target")
		}
		if err := storage.WithTenant(ctx, pool, target.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1) ON CONFLICT (id) DO NOTHING`, target.WorkspaceID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,'maintainer',$4,1) ON CONFLICT (workspace_id,issuer,subject) DO NOTHING`, target.WorkspaceID, issuer, target.Subject, []string{"catalog:publish"})
			return err
		}); err != nil {
			return fmt.Errorf("seed acceptance maintenance target for workspace %s: %w", target.WorkspaceID, err)
		}
	}
	return nil
}
