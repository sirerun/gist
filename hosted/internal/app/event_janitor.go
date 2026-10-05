package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// MaintenanceTarget identifies an existing maintainer in one workspace. It is
// trusted deployment input; neither requests nor tenant discovery can add targets.
type MaintenanceTarget struct {
	WorkspaceID string `json:"workspace_id"`
	Subject     string `json:"subject"`
}

const eventMaintenanceInterval = time.Minute

// purgeEventTargets verifies current membership, generation and publish rights
// before every bounded purge. Configuration alone cannot revive a revoked actor.
const maxEventMaintenanceBatches = 20

func (a *App) purgeEventTargets(ctx context.Context, catalog *storage.Postgres) error {
	var failures []error
	for _, target := range a.cfg.EventMaintenanceTargets {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := a.purgeEventTarget(ctx, catalog, target); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (a *App) purgeEventTarget(ctx context.Context, catalog *storage.Postgres, target MaintenanceTarget) error {
	policy := &postgresPolicy{pool: a.pool}
	for batch := 0; batch < maxEventMaintenanceBatches; batch++ {
		p, err := a.maintenancePrincipal(ctx, policy, target)
		if err != nil {
			return err
		}
		purged, err := catalog.PurgeEvents(ctx, storage.Tenant{Issuer: p.Issuer, Audience: p.Audience, Subject: p.Subject, WorkspaceID: p.WorkspaceID, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration}, storage.MaxEventPurgeBatch)
		if err != nil {
			return fmt.Errorf("app: purge expired events: %w", err)
		}
		if purged < storage.MaxEventPurgeBatch {
			return nil
		}
	}
	// A full last batch may have removed the final expired row. Recheck current
	// authority and probe without deleting beyond the bounded catch-up limit.
	p, err := a.maintenancePrincipal(ctx, policy, target)
	if err != nil {
		return err
	}
	remaining, err := catalog.HasExpiredEvents(ctx, storage.Tenant{Issuer: p.Issuer, Audience: p.Audience, Subject: p.Subject, WorkspaceID: p.WorkspaceID, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration})
	if err != nil {
		return fmt.Errorf("app: qualify event maintenance backlog: %w", err)
	}
	if remaining {
		return errors.New("app: event maintenance backlog exceeds bounded catch-up")
	}
	return nil
}

func (a *App) maintenancePrincipal(ctx context.Context, policy *postgresPolicy, target MaintenanceTarget) (ports.Principal, error) {
	policyState, err := policy.CheckWorkloadForIssuer(ctx, a.cfg.PublicOrigin, target.Subject, target.WorkspaceID, []string{string(ports.ActionPublish)}, 0)
	if err != nil || !policyState.Allowed {
		return ports.Principal{}, errors.New("app: event maintenance membership unavailable")
	}
	p := ports.Principal{Issuer: a.cfg.PublicOrigin, Audience: a.cfg.ResourceAudience, Subject: target.Subject, WorkspaceID: target.WorkspaceID, Scopes: []string{string(ports.ActionPublish)}, PolicyGeneration: policyState.PolicyGeneration}
	decision, err := policy.Decide(ctx, p, ports.ActionPublish, nil)
	if err != nil || !decision.Allowed {
		return ports.Principal{}, errors.New("app: event maintenance authority unavailable")
	}
	return p, nil
}

func (a *App) startEventJanitor(parent context.Context, catalog *storage.Postgres) {
	ctx, cancel := context.WithCancel(parent)
	a.janitorCancel = cancel
	a.janitorDone = make(chan struct{})
	go func() {
		defer close(a.janitorDone)
		ticker := time.NewTicker(eventMaintenanceInterval)
		defer ticker.Stop()
		a.runEventJanitor(ctx, catalog, ticker.C)
	}()
}

func (a *App) runEventJanitor(ctx context.Context, catalog *storage.Postgres, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
			runCtx, stop := context.WithTimeout(ctx, a.cfg.RequestTimeout)
			err := a.purgeEventTargets(runCtx, catalog)
			stop()
			a.maintenanceMu.Lock()
			a.maintenanceErr = err
			a.maintenanceMu.Unlock()
		}
	}
}

func (a *App) maintenanceReady() error {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	if a.maintenanceErr != nil {
		return errors.New("app: event maintenance unavailable")
	}
	return nil
}
