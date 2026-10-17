package app

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// firstEnrollmentEligible distinguishes a genuinely new configured workspace
// from a revoked actor, a failed purge, an unavailable database or a previously
// completed bootstrap. It does not create any identity, membership or policy.
func (a *App) firstEnrollmentEligible(ctx context.Context) (bool, error) {
	var eligible bool
	err := storage.WithTenant(ctx, a.pool, a.cfg.GitHubEnrollment.WorkspaceID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM workspaces WHERE id=$1) AND NOT EXISTS (SELECT 1 FROM founder_bootstrap WHERE singleton=1)`, a.cfg.GitHubEnrollment.WorkspaceID).Scan(&eligible)
	})
	return eligible, err
}

func (a *App) bootstrapAdmission() error {
	if !a.cfg.FirstEnrollment {
		return nil
	}
	return a.maintenanceReady()
}
