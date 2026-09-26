package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	settingIssuer           = "registry.issuer"
	settingSubject          = "registry.subject"
	settingAudience         = "registry.audience"
	settingWorkspace        = "registry.workspace_id"
	settingScopes           = "registry.scopes"
	settingPolicyGeneration = "registry.policy_generation"
)

type Tenant struct {
	Issuer, Subject, Audience, WorkspaceID string
	Scopes                                 []string
	PolicyGeneration                       uint64
}

func WithTenant(ctx context.Context, pool *pgxpool.Pool, workspace string, fn func(context.Context, pgx.Tx) error) error {
	return WithTenantPrincipal(ctx, pool, Tenant{WorkspaceID: workspace}, fn)
}

func WithTenantPrincipal(ctx context.Context, pool *pgxpool.Pool, tenant Tenant, fn func(context.Context, pgx.Tx) error) error {
	if pool == nil || tenant.WorkspaceID == "" || fn == nil {
		return errors.New("storage: invalid tenant transaction")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tenant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	settings := []struct{ name, value string }{{settingIssuer, tenant.Issuer}, {settingSubject, tenant.Subject}, {settingAudience, tenant.Audience}, {settingWorkspace, tenant.WorkspaceID}, {settingScopes, joinScopes(tenant.Scopes)}, {settingPolicyGeneration, fmt.Sprint(tenant.PolicyGeneration)}}
	for _, setting := range settings {
		if _, err := tx.Exec(ctx, `SELECT set_config($1, $2, true)`, setting.name, setting.value); err != nil {
			return fmt.Errorf("set transaction context %s: %w", setting.name, err)
		}
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant transaction: %w", err)
	}
	return nil
}

func joinScopes(scopes []string) string {
	var out string
	for i, s := range scopes {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}
