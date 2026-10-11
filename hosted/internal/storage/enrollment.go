package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sirerun/gist/hosted/internal/enrollment"
)

type EnrollmentStore struct {
	store  *Postgres
	issuer string
}

func (s *Postgres) Enrollment(issuer string) (*EnrollmentStore, error) {
	if s == nil || issuer == "" {
		return nil, errors.New("storage: enrollment requires postgres and issuer")
	}
	return &EnrollmentStore{store: s, issuer: issuer}, nil
}

func (e *EnrollmentStore) CreateFlow(ctx context.Context, hash []byte, expires time.Time) error {
	now := time.Now()
	if len(hash) != 32 || !expires.After(now) || expires.After(now.Add(10*time.Minute)) {
		return errors.New("storage: invalid enrollment flow")
	}
	_, err := e.store.pool.Exec(ctx, "DELETE FROM github_login_flows WHERE state_hash IN (SELECT state_hash FROM github_login_flows WHERE expires_at < $1 ORDER BY expires_at LIMIT 500)", now)
	if err != nil {
		return fmt.Errorf("prune expired GitHub login flows: %w", err)
	}
	_, err = e.store.pool.Exec(ctx, "INSERT INTO github_login_flows(state_hash,expires_at) VALUES($1,$2)", hash, expires)
	if err != nil {
		return fmt.Errorf("create GitHub login flow: %w", err)
	}
	return nil
}

func (e *EnrollmentStore) ConsumeFlow(ctx context.Context, hash []byte, now time.Time) error {
	if len(hash) != 32 {
		return enrollment.ErrReplay
	}
	var consumed time.Time
	err := e.store.pool.QueryRow(ctx, "UPDATE github_login_flows SET consumed_at=$2 WHERE state_hash=$1 AND consumed_at IS NULL AND expires_at>$2 RETURNING consumed_at", hash, now).Scan(&consumed)
	if errors.Is(err, pgx.ErrNoRows) {
		return enrollment.ErrReplay
	}
	if err != nil {
		return fmt.Errorf("consume GitHub login flow: %w", err)
	}
	return nil
}

func (e *EnrollmentStore) CompleteLogin(ctx context.Context, workspaceID, subject string, invitationHash []byte, founder, workspaceName string, bootstrapScopes []string, now time.Time) error {
	if workspaceID == "" || subject == "" || founder == "" || workspaceName == "" {
		return errors.New("storage: incomplete enrollment")
	}
	return WithTenant(ctx, e.store.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var role string
		var active bool
		err := tx.QueryRow(ctx, "SELECT role,active FROM workspace_memberships WHERE workspace_id=$1 AND issuer=$2 AND subject=$3", workspaceID, e.issuer, subject).Scan(&role, &active)
		if err == nil {
			if !active {
				return enrollment.ErrDenied
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("load existing workspace membership: %w", err)
		}
		if len(invitationHash) > 0 {
			if len(invitationHash) != 32 {
				return enrollment.ErrDenied
			}
			var invitedRole string
			var scopes []string
			err := tx.QueryRow(ctx, "UPDATE workspace_invitations SET consumed_at=$3 WHERE workspace_id=$1 AND token_hash=$2 AND github_account_id=$4 AND consumed_at IS NULL AND expires_at>$3 RETURNING role,scopes", workspaceID, invitationHash, now, subject).Scan(&invitedRole, &scopes)
			if errors.Is(err, pgx.ErrNoRows) {
				return enrollment.ErrDenied
			}
			if err != nil {
				return fmt.Errorf("redeem workspace invitation: %w", err)
			}
			_, err = tx.Exec(ctx, "INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,active,policy_generation) VALUES($1,$2,$3,$4,$5,true,1)", workspaceID, e.issuer, subject, invitedRole, scopes)
			if err != nil {
				return fmt.Errorf("create invited workspace membership: %w", err)
			}
			return nil
		}
		if subject != founder {
			return enrollment.ErrDenied
		}
		_, err = tx.Exec(ctx, "INSERT INTO founder_bootstrap(singleton,workspace_id,issuer,subject,completed_at) VALUES(1,$1,$2,$3,$4)", workspaceID, e.issuer, subject, now)
		if err != nil {
			if isUniqueViolation(err) {
				return enrollment.ErrDenied
			}
			return fmt.Errorf("claim founder bootstrap: %w", err)
		}
		tag, err := tx.Exec(ctx, "INSERT INTO workspaces(id,name) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", workspaceID, workspaceName)
		if err != nil {
			return fmt.Errorf("create initial workspace: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return enrollment.ErrDenied
		}
		_, err = tx.Exec(ctx, "INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,active,policy_generation) VALUES($1,$2,$3,'maintainer',$4,true,1)", workspaceID, e.issuer, subject, normalizeScopes(bootstrapScopes))
		if err != nil {
			return fmt.Errorf("create founder membership: %w", err)
		}
		return nil
	})
}

func (e *EnrollmentStore) CreateInvitation(ctx context.Context, workspaceID, actor, accountID, role string, scopes []string, tokenHash []byte, expires time.Time) error {
	now := time.Now()
	if workspaceID == "" || actor == "" || accountID == "" || (role != "reader" && role != "maintainer") || len(tokenHash) != 32 || len(scopes) == 0 || !expires.After(now) || expires.After(now.Add(7*24*time.Hour)) {
		return errors.New("storage: invalid workspace invitation")
	}
	scopes = normalizeScopes(scopes)
	return WithTenant(ctx, e.store.pool, workspaceID, func(ctx context.Context, tx pgx.Tx) error {
		var actorRole string
		var actorScopes []string
		var active bool
		if err := tx.QueryRow(ctx, "SELECT role,scopes,active FROM workspace_memberships WHERE workspace_id=$1 AND issuer=$2 AND subject=$3 FOR SHARE", workspaceID, e.issuer, actor).Scan(&actorRole, &actorScopes, &active); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return enrollment.ErrDenied
			}
			return fmt.Errorf("load invitation operator: %w", err)
		}
		if !active || actorRole != "maintainer" || !scopeSubset(scopes, actorScopes) {
			return enrollment.ErrDenied
		}
		var exists bool
		err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workspace_memberships WHERE workspace_id=$1 AND issuer=$2 AND subject=$3)", workspaceID, e.issuer, accountID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check invited membership: %w", err)
		}
		if exists {
			return enrollment.ErrDenied
		}
		_, err = tx.Exec(ctx, "INSERT INTO workspace_invitations(workspace_id,token_hash,github_account_id,role,scopes,created_by,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)", workspaceID, tokenHash, accountID, role, scopes, actor, expires)
		if err != nil {
			return fmt.Errorf("create workspace invitation: %w", err)
		}
		return nil
	})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func normalizeScopes(scopes []string) []string {
	out := append([]string(nil), scopes...)
	sort.Strings(out)
	return out
}
func scopeSubset(requested, allowed []string) bool {
	allowedSet := make(map[string]bool, len(allowed))
	for _, scope := range allowed {
		allowedSet[scope] = true
	}
	seen := make(map[string]bool, len(requested))
	for _, scope := range requested {
		if scope == "" || seen[scope] || !allowedSet[scope] {
			return false
		}
		seen[scope] = true
	}
	return true
}

var _ enrollment.Store = (*EnrollmentStore)(nil)
