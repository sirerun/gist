package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// HasExpiredEvents qualifies whether a bounded purge left work without deleting
// an extra row. It retains the same principal-bound forced-RLS transaction and
// database clock predicate as PurgeEvents.
func (s *Postgres) HasExpiredEvents(ctx context.Context, tenant Tenant) (bool, error) {
	if s == nil || s.pool == nil {
		return false, errors.New("storage: unavailable postgres event store")
	}
	if tenant.WorkspaceID == "" {
		return false, errors.New("storage: invalid event retention scope")
	}
	var remaining bool
	err := WithTenantPrincipal(ctx, s.pool, tenant, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM event_outbox WHERE workspace_id=$1 AND occurred_at < clock_timestamp() - interval '7 days')`, tenant.WorkspaceID).Scan(&remaining)
	})
	if err != nil {
		return false, fmt.Errorf("probe tenant event retention: %w", err)
	}
	return remaining, nil
}
