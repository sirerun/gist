//go:build integration

package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/enrollment"
)

func TestEnrollmentBootstrapAndInviteAreAtomicAndBound(t *testing.T) {
	pool := searchTestPool(t)
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	enrollmentStore, err := store.Enrollment("https://gist.example")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := enrollmentStore.CompleteLogin(ctx, "private-a", "1001", nil, "1001", "Private A", []string{"catalog:read", "catalog:publish"}, now); err != nil {
		t.Fatalf("founder bootstrap: %v", err)
	}
	var role string
	var scopes []string
	err = WithTenant(ctx, pool, "private-a", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT role,scopes FROM workspace_memberships WHERE workspace_id=$1 AND issuer=$2 AND subject=$3", "private-a", "https://gist.example", "1001").Scan(&role, &scopes)
	})
	if err != nil || role != "maintainer" || len(scopes) != 2 {
		t.Fatalf("bootstrap membership role=%q scopes=%v err=%v", role, scopes, err)
	}
	if err := enrollmentStore.CompleteLogin(ctx, "private-b", "1001", nil, "1001", "Private B", []string{"catalog:read"}, now); !errors.Is(err, enrollment.ErrDenied) {
		t.Fatalf("second workspace founder bootstrap accepted: %v", err)
	}

	tokenHash := make([]byte, 32)
	tokenHash[0] = 9
	if err := enrollmentStore.CreateInvitation(ctx, "private-a", "1001", "2002", "reader", []string{"catalog:read"}, tokenHash, now.Add(time.Hour)); err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if err := enrollmentStore.CompleteLogin(ctx, "private-b", "2002", tokenHash, "1001", "Private B", nil, now); !errors.Is(err, enrollment.ErrDenied) {
		t.Fatalf("invitation redeemed in a different workspace: %v", err)
	}
	if err := enrollmentStore.CompleteLogin(ctx, "private-a", "2003", tokenHash, "1001", "Private A", nil, now); !errors.Is(err, enrollment.ErrDenied) {
		t.Fatalf("invitation accepted for a different GitHub ID: %v", err)
	}
	if err := enrollmentStore.CompleteLogin(ctx, "private-a", "2002", tokenHash, "1001", "Private A", nil, now); err != nil {
		t.Fatalf("redeem invitation: %v", err)
	}
	err = WithTenant(ctx, pool, "private-a", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT role,scopes FROM workspace_memberships WHERE workspace_id=$1 AND issuer=$2 AND subject=$3", "private-a", "https://gist.example", "2002").Scan(&role, &scopes)
	})
	if err != nil || role != "reader" || len(scopes) != 1 || scopes[0] != "catalog:read" {
		t.Fatalf("invited membership role=%q scopes=%v err=%v", role, scopes, err)
	}
	if err := enrollmentStore.CompleteLogin(ctx, "private-a", "2003", tokenHash, "1001", "Private A", nil, now); !errors.Is(err, enrollment.ErrDenied) {
		t.Fatalf("consumed invitation replay succeeded for another identity: %v", err)
	}
	if err := enrollmentStore.CompleteLogin(ctx, "private-b", "1001", nil, "1001", "Private B", []string{"catalog:read"}, now); !errors.Is(err, enrollment.ErrDenied) {
		t.Fatalf("second founder workspace bootstrap succeeded: %v", err)
	}
	var workspaceExists bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workspaces WHERE id='private-b')").Scan(&workspaceExists); err != nil || workspaceExists {
		t.Fatalf("failed bootstrap left a workspace: exists=%t err=%v", workspaceExists, err)
	}
}

func TestEnrollmentFlowIsSingleUseAndExpires(t *testing.T) {
	pool := searchTestPool(t)
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	enrollmentStore, err := store.Enrollment("https://gist.example")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	validHash := make([]byte, 32)
	validHash[0] = 11
	if err := enrollmentStore.CreateFlow(ctx, validHash, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := enrollmentStore.ConsumeFlow(ctx, validHash, now); err != nil {
		t.Fatalf("first flow consume: %v", err)
	}
	if err := enrollmentStore.ConsumeFlow(ctx, validHash, now); !errors.Is(err, enrollment.ErrReplay) {
		t.Fatalf("flow replay accepted: %v", err)
	}
	expiredHash := make([]byte, 32)
	expiredHash[0] = 12
	if err := enrollmentStore.CreateFlow(ctx, expiredHash, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := enrollmentStore.ConsumeFlow(ctx, expiredHash, now.Add(2*time.Second)); !errors.Is(err, enrollment.ErrReplay) {
		t.Fatalf("expired flow accepted: %v", err)
	}
}

func TestEnrollmentRejectsExpiredInvitation(t *testing.T) {
	pool := searchTestPool(t)
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	enrollmentStore, err := store.Enrollment("https://gist.example")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := enrollmentStore.CompleteLogin(ctx, "expired-invite", "1001", nil, "1001", "Expired Invite", []string{"catalog:read"}, now); err != nil {
		t.Fatalf("founder bootstrap: %v", err)
	}
	tokenHash := make([]byte, 32)
	tokenHash[0] = 7
	if err := enrollmentStore.CreateInvitation(ctx, "expired-invite", "1001", "2002", "reader", []string{"catalog:read"}, tokenHash, now.Add(time.Hour)); err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if err := enrollmentStore.CompleteLogin(ctx, "expired-invite", "2002", tokenHash, "1001", "Expired Invite", nil, now.Add(2*time.Hour)); !errors.Is(err, enrollment.ErrDenied) {
		t.Fatalf("expired invitation accepted: %v", err)
	}
}

func TestInvitationCannotWidenMaintainerScopes(t *testing.T) {
	pool := searchTestPool(t)
	store, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	enrollmentStore, err := store.Enrollment("https://gist.example")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := enrollmentStore.CompleteLogin(ctx, "scope-bounds", "1001", nil, "1001", "Scope Bounds", []string{"catalog:read"}, now); err != nil {
		t.Fatalf("founder bootstrap: %v", err)
	}
	tokenHash := make([]byte, 32)
	tokenHash[0] = 9
	if err := enrollmentStore.CreateInvitation(ctx, "scope-bounds", "1001", "2002", "reader", []string{"catalog:publish"}, tokenHash, now.Add(time.Hour)); !errors.Is(err, enrollment.ErrDenied) {
		t.Fatalf("maintainer widened invitation scope: %v", err)
	}
	var invitationExists bool
	err = WithTenant(ctx, pool, "scope-bounds", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workspace_invitations WHERE workspace_id=$1 AND token_hash=$2)", "scope-bounds", tokenHash).Scan(&invitationExists)
	})
	if err != nil || invitationExists {
		t.Fatalf("denied invitation persisted: exists=%t err=%v", invitationExists, err)
	}
}
