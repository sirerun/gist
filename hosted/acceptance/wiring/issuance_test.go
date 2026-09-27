//go:build integration

package wiring

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// memberSubject adds a fresh subject to each workspace, so a test that revokes
// it cannot disturb the shared workloads other suites mint for.
func (f *fixture) memberSubject(t *testing.T, prefix string, workspaces ...string) string {
	t.Helper()
	subject := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	for _, workspace := range workspaces {
		if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,'maintainer',$4,1)`, workspace, f.baseURL, subject, []string{"catalog:read", "catalog:publish"})
			return err
		}); err != nil {
			t.Fatalf("add %s to %s: %v", subject, workspace, err)
		}
	}
	return subject
}

func (f *fixture) discoverStatus(t *testing.T, token string) int {
	t.Helper()
	resp := f.do(t, "POST", "/v1/discover", token, `{"query":"fixture","max_bytes":4096}`)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func (f *fixture) storedIdentity(t *testing.T, workspace, subject string) (count int, revoked bool) {
	t.Helper()
	if err := storage.WithTenant(context.Background(), f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*), coalesce(bool_or(revoked_at IS NOT NULL), false) FROM workload_identities WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, workspace, f.baseURL, subject).Scan(&count, &revoked)
	}); err != nil {
		t.Fatalf("read stored identity: %v", err)
	}
	return count, revoked
}

// A minted token authenticates with no manual seeding: issuance records the
// stored identity that verification requires. One subject minted in two
// workspaces gets one row per workspace.
func TestMintRecordsIdentityAndAuthenticates(t *testing.T) {
	f := requireFixture(t)
	subject := f.memberSubject(t, "mint", "q3-tenant-a", "q3-tenant-b")
	for _, workspace := range []string{"q3-tenant-a", "q3-tenant-b"} {
		token := f.token(t, workspace, subject, "catalog:read")
		if got := f.discoverStatus(t, token); got != 200 {
			t.Fatalf("%s: minted token status=%d, want 200", workspace, got)
		}
		if count, revoked := f.storedIdentity(t, workspace, subject); count != 1 || revoked {
			t.Fatalf("%s: stored identity rows=%d revoked=%v, want 1 unrevoked", workspace, count, revoked)
		}
	}
	// Re-minting refreshes the row instead of adding one.
	f.token(t, "q3-tenant-a", subject, "catalog:read")
	if count, _ := f.storedIdentity(t, "q3-tenant-a", subject); count != 1 {
		t.Fatalf("re-mint rows=%d, want 1", count)
	}
}

// Revoking the stored identity rejects its outstanding token, and a revoked
// identity cannot mint again: issuance never clears revoked_at. Revocation is
// workspace-scoped, so the same subject still mints in another workspace.
func TestRevokedIdentityRejectsTokenAndRefusesMint(t *testing.T) {
	f := requireFixture(t)
	subject := f.memberSubject(t, "revoke", "q3-tenant-a", "q3-tenant-b")
	token := f.token(t, "q3-tenant-a", subject, "catalog:read")
	if got := f.discoverStatus(t, token); got != 200 {
		t.Fatalf("before revoke status=%d, want 200", got)
	}
	store, err := storage.NewPostgres(f.pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Revoke(context.Background(), "q3-tenant-a", f.baseURL, subject); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got := f.discoverStatus(t, token); got != 401 {
		t.Fatalf("after revoke status=%d, want 401", got)
	}
	again, err := f.app.MintWorkloadToken(context.Background(), identity.WorkloadRequest{Subject: subject, WorkspaceID: "q3-tenant-a", Scopes: []string{"catalog:read"}, TTL: 2 * time.Minute})
	if !errors.Is(err, identity.ErrUnauthorized) || again != "" {
		t.Fatalf("mint revoked identity token=%q err=%v, want no token and ErrUnauthorized", again, err)
	}
	if count, revoked := f.storedIdentity(t, "q3-tenant-a", subject); count != 1 || !revoked {
		t.Fatalf("after refused mint rows=%d revoked=%v, want 1 revoked", count, revoked)
	}
	other := f.token(t, "q3-tenant-b", subject, "catalog:read")
	if got := f.discoverStatus(t, other); got != 200 {
		t.Fatalf("other workspace status=%d, want 200", got)
	}
}
