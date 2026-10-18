//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrivateDirectoryRestrictedRLSAndRevocation(t *testing.T) {
	f := newCompositionPostgres(t)
	f.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cfg := compositionConfig(f.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	instance, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Shutdown(context.Background())
	cookie, err := instance.IssueOAuthSession(compositionSubject, []string{compositionWorkspace})
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, target, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req.AddCookie(cookie)
		req.Header.Set("Origin", compositionIssuer)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gist-CSRF", token)
		w := httptest.NewRecorder()
		instance.Handler().ServeHTTP(w, req)
		return w
	}
	session := request("GET", "/app/api/session", "", "")
	var sess struct {
		CSRF string `json:"csrf_token"`
	}
	if session.Code != 200 || json.Unmarshal(session.Body.Bytes(), &sess) != nil || sess.CSRF == "" {
		t.Fatalf("session%d %s", session.Code, session.Body)
	}
	raw, _ := json.Marshal(map[string]any{"record": customRecord()})
	post := request("POST", "/app/api/directory", string(raw), sess.CSRF)
	var entry privateDirectoryEntry
	if post.Code != 201 || json.Unmarshal(post.Body.Bytes(), &entry) != nil {
		t.Fatalf("create%d %s", post.Code, post.Body)
	}
	if w := request("GET", "/app/api/directory", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "localhost") {
		t.Fatalf("private list%d %s", w.Code, w.Body)
	}
	public := request("GET", "/api/directory?q=Private%20local%20server", "", "")
	if public.Code != 200 || strings.Contains(public.Body.String(), "localhost") || strings.Contains(public.Body.String(), "custom/example") {
		t.Fatalf("public leak%d %s", public.Code, public.Body)
	}
	var unscoped int
	if err := instance.pool.QueryRow(ctx, `SELECT count(*) FROM private_directory_entries`).Scan(&unscoped); err != nil || unscoped != 0 {
		t.Fatalf("unscoped RLS %d %v", unscoped, err)
	}
	if err := storage.WithTenant(ctx, instance.pool, "another-workspace", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM private_directory_entries`).Scan(&unscoped)
	}); err != nil || unscoped != 0 {
		t.Fatalf("foreign RLS %d %v", unscoped, err)
	}
	// Actual SQL authority is checked again, independently of the browser session.
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET active=false WHERE workspace_id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	store := postgresPrivateDirectory{pool: instance.pool}
	principal := ports.Principal{Issuer: compositionIssuer, Subject: compositionSubject, WorkspaceID: compositionWorkspace, PolicyGeneration: 1}
	items, err := store.list(ctx, principal, "")
	if err != nil || len(items) != 0 {
		t.Fatalf("revoked SQL read %d %v", len(items), err)
	}
	if _, err := store.create(ctx, principal, customRecord()); err == nil {
		t.Fatal("revoked SQL create allowed")
	}
	if err := store.remove(ctx, principal, entry.ID); err == nil {
		t.Fatal("revoked SQL delete allowed")
	}
	if w := request("DELETE", "/app/api/directory?id="+entry.ID, "", sess.CSRF); w.Code != 401 {
		t.Fatalf("revoked HTTP delete%d", w.Code)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET active=true,role='reader' WHERE workspace_id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "/app/api/directory", string(raw), sess.CSRF); w.Code != 403 {
		t.Fatalf("reader mutation%d %s", w.Code, w.Body)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET role='maintainer' WHERE workspace_id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	if w := request("DELETE", "/app/api/directory?id="+entry.ID, "", sess.CSRF); w.Code != http.StatusNoContent {
		t.Fatalf("delete%d %s", w.Code, w.Body)
	}
	// The per-workspace cap stays exact under concurrent requests.
	if _, err := f.adminPool.Exec(ctx, `INSERT INTO private_directory_entries(workspace_id,record,created_by) SELECT $1,'{}'::jsonb,'fixture' FROM generate_series(1,999)`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := store.create(ctx, principal, customRecord()); results <- err }()
	}
	success, full := 0, 0
	for range 2 {
		switch err := <-results; {
		case err == nil:
			success++
		case errors.Is(err, errDirectoryFull):
			full++
		default:
			t.Fatalf("capacity race error %v", err)
		}
	}
	if success != 1 || full != 1 {
		t.Fatalf("capacity race success=%d full=%d", success, full)
	}

}
