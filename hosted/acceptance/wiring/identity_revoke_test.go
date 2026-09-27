//go:build integration

package wiring

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/storage"
)

const revokeWorkspace = "q3-identity-revoke"

// revokeMember adds subject to revokeWorkspace with role and scopes, and
// returns a token for it carrying exactly those scopes.
func (f *fixture) revokeMember(t *testing.T, prefix, role string, scopes ...string) (string, string) {
	t.Helper()
	subject := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	if err := storage.WithTenant(context.Background(), f.pool, revokeWorkspace, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1) ON CONFLICT (id) DO NOTHING`, revokeWorkspace); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,$4,$5,1)`, revokeWorkspace, f.baseURL, subject, role, scopes)
		return err
	}); err != nil {
		t.Fatalf("add member %s: %v", subject, err)
	}
	tok, err := f.app.MintWorkloadToken(context.Background(), identity.WorkloadRequest{Subject: subject, WorkspaceID: revokeWorkspace, Scopes: scopes, ParentSubject: subject, ParentScopes: scopes, TTL: 2 * time.Minute})
	if err != nil {
		t.Fatalf("mint %s: %v", subject, err)
	}
	return subject, tok
}

func (f *fixture) revokeIdentity(t *testing.T, token, subject string) (int, string) {
	t.Helper()
	resp := f.do(t, "POST", "/v1/identities/revoke", token, fmt.Sprintf(`{"issuer":%q,"subject":%q}`, f.baseURL, subject))
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode, string(raw)
}

// identity:revoke without the maintainer role is denied by the real
// postgresPolicy.Decide, not a scope-only double, and revokes nothing.
func TestIdentityRevokeRequiresMaintainerRole(t *testing.T) {
	f := requireFixture(t)
	_, readerTok := f.revokeMember(t, "reader", "reader", "catalog:read", "identity:revoke")
	target, targetTok := f.revokeMember(t, "target", "reader", "catalog:read")
	if status, body := f.revokeIdentity(t, readerTok, target); status != 403 {
		t.Fatalf("reader revoke status=%d body=%s, want 403", status, body)
	}
	if count, revoked := f.storedIdentity(t, revokeWorkspace, target); count != 1 || revoked {
		t.Fatalf("target rows=%d revoked=%v after a denied revoke", count, revoked)
	}
	if got := f.discoverStatus(t, targetTok); got != 200 {
		t.Fatalf("target status=%d after a denied revoke, want 200", got)
	}
}

// A second revoke keeps the original revoked_at.
func TestIdentityRevokeTwiceKeepsOriginalRevokedAt(t *testing.T) {
	f := requireFixture(t)
	target, _ := f.revokeMember(t, "twice", "reader", "catalog:read")
	store, err := storage.NewPostgres(f.pool)
	if err != nil {
		t.Fatal(err)
	}
	revokedAt := func() time.Time {
		var at time.Time
		if err := storage.WithTenant(context.Background(), f.pool, revokeWorkspace, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT revoked_at FROM workload_identities WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, revokeWorkspace, f.baseURL, target).Scan(&at)
		}); err != nil {
			t.Fatalf("read revoked_at: %v", err)
		}
		return at
	}
	if err := store.Revoke(context.Background(), revokeWorkspace, f.baseURL, target); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	first := revokedAt()
	// now() is the transaction start, so a later transaction differs.
	time.Sleep(20 * time.Millisecond)
	if err := store.Revoke(context.Background(), revokeWorkspace, f.baseURL, target); err != nil {
		t.Fatalf("second revoke: %v", err)
	}
	if second := revokedAt(); !second.Equal(first) {
		t.Fatalf("revoked_at moved from %v to %v on a repeat revoke", first, second)
	}
}

// End to end: a maintainer revokes an identity over HTTP, and that identity's
// next request is 401.
func TestMaintainerRevokeMakesIdentityUnauthorized(t *testing.T) {
	f := requireFixture(t)
	_, maintainerTok := f.revokeMember(t, "maintainer", "maintainer", "catalog:read", "identity:revoke")
	target, targetTok := f.revokeMember(t, "victim", "reader", "catalog:read")
	if got := f.discoverStatus(t, targetTok); got != 200 {
		t.Fatalf("target before revoke status=%d, want 200", got)
	}
	if status, body := f.revokeIdentity(t, maintainerTok, target); status != 200 {
		t.Fatalf("maintainer revoke status=%d body=%s, want 200", status, body)
	}
	if got := f.discoverStatus(t, targetTok); got != 401 {
		t.Fatalf("target after revoke status=%d, want 401", got)
	}
	if status, body := f.revokeIdentity(t, maintainerTok, target); status != 200 {
		t.Fatalf("repeat revoke status=%d body=%s, want 200", status, body)
	}
}
