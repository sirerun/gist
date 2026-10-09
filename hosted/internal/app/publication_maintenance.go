package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
)

const publicationCleanupBatch = 20

func (a *App) startPublicationJanitor(parent context.Context, service *publicationV2) error {
	ctx, cancel := context.WithTimeout(parent, a.cfg.RequestTimeout)
	if err := a.cleanupPublicationTargets(ctx, service); err != nil {
		cancel()
		return err
	}
	cancel()
	runCtx, stop := context.WithCancel(parent)
	a.publicationJanitorCancel = stop
	a.publicationJanitorDone = make(chan struct{})
	interval := a.cfg.PublicationV2.CleanupInterval
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		defer close(a.publicationJanitorDone)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				job, cancel := context.WithTimeout(runCtx, a.cfg.RequestTimeout)
				err := a.cleanupPublicationTargets(job, service)
				cancel()
				a.maintenanceMu.Lock()
				a.publicationMaintenanceErr = err
				a.maintenanceMu.Unlock()
			}
		}
	}()
	return nil
}

func (a *App) cleanupPublicationTargets(ctx context.Context, service *publicationV2) error {
	var failures []error
	for _, target := range service.config.MaintenanceTargets {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		principal, err := a.maintenancePrincipal(ctx, &postgresPolicy{pool: a.pool}, target)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		err = service.store.Cleanup(ctx, principal, publicationCleanupBatch, func(ctx context.Context, tx pgx.Tx) error {
			if err := currentMaintenanceAuthority(ctx, tx, principal); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("app: publication cleanup: %w", err))
		}
	}
	return errors.Join(failures...)
}

func currentMaintenanceAuthority(ctx context.Context, tx pgx.Tx, p ports.Principal) error {
	var role string
	var active bool
	var memberGeneration, workspaceGeneration uint64
	var scopes []string
	err := tx.QueryRow(ctx, `SELECT wm.role,wm.active,wm.policy_generation,wm.scopes,w.policy_generation FROM workspace_memberships wm JOIN workspaces w ON w.id=wm.workspace_id WHERE wm.workspace_id=$1 AND wm.issuer=$2 AND wm.subject=$3 FOR SHARE OF wm,w`, p.WorkspaceID, p.Issuer, p.Subject).Scan(&role, &active, &memberGeneration, &scopes, &workspaceGeneration)
	if err != nil || !active || role != "maintainer" || memberGeneration != workspaceGeneration || memberGeneration != p.PolicyGeneration || !containsScopes(scopes, []string{string(ports.ActionPublish)}) {
		return rest.ErrPublicationDenied
	}
	return nil
}
