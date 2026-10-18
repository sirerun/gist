//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

func TestPersonalBookmarksPostgresIsolationAndPersistence(t *testing.T) {
	f := newCompositionPostgres(t)
	f.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	_, err := f.adminPool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,active,policy_generation) VALUES($1,$2,'second','reader',ARRAY['catalog:read'],true,1)`, compositionWorkspace, compositionIssuer)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := New(ctx, compositionConfig(f.runtimeURL, t.TempDir(), compositionSigningConfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Shutdown(context.Background())
	first, err := instance.IssueOAuthSession(compositionSubject, []string{compositionWorkspace})
	if err != nil {
		t.Fatal(err)
	}
	second, err := instance.IssueOAuthSession("second", []string{compositionWorkspace})
	if err != nil {
		t.Fatal(err)
	}
	request := func(cookie *http.Cookie, method, target, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		r.Header.Set("Origin", compositionIssuer)
		r.Header.Set("X-Gist-CSRF", token)
		w := httptest.NewRecorder()
		instance.Handler().ServeHTTP(w, r)
		return w
	}
	csrf := func(cookie *http.Cookie) string {
		w := request(cookie, "GET", "/app/api/session", "")
		var s struct {
			CSRF string `json:"csrf_token"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &s) != nil || s.CSRF == "" {
			t.Fatalf("session %d %s", w.Code, w.Body)
		}
		return s.CSRF
	}
	token1, token2 := csrf(first), csrf(second)
	id := "mcp:io.github.github/github-mcp-server"
	target := "/app/api/bookmarks?id=" + url.QueryEscape(id)
	index, err := directory.Default()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := index.Get(id)
	if w := request(nil, "PUT", target, token1); w.Code != 401 {
		t.Fatal("anonymous write")
	}
	if w := request(first, "PUT", target, "bad"); w.Code != 403 {
		t.Fatal("CSRF accepted")
	}
	if w := request(second, "PUT", target, token1); w.Code != 403 {
		t.Fatal("cross-session CSRF")
	}
	if w := request(first, "PUT", "/app/api/bookmarks?id=missing", token1); w.Code != 404 {
		t.Fatal("unknown entry accepted")
	}
	for i := 0; i < 2; i++ {
		if w := request(first, "PUT", target, token1); w.Code != 204 {
			t.Fatalf("reader save %d %s", w.Code, w.Body)
		}
	}
	if w := request(first, "GET", "/app/api/bookmarks?view=entries&q=github&kind=mcp", ""); w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatalf("saved search %d %s", w.Code, w.Body)
	}
	for _, q := range []string{"?view=entries&offset=-1", "?id=other", "?view=bad", "?kind=invalid", "?view=entries&offset=201"} {
		if w := request(first, "GET", "/app/api/bookmarks"+q, ""); w.Code != 400 {
			t.Fatalf("accepted query %s %d", q, w.Code)
		}
	}
	if w := request(second, "GET", "/app/api/bookmarks", ""); w.Code != 200 || strings.Contains(w.Body.String(), id) {
		t.Fatal("cross-user leak")
	}
	if w := request(second, "DELETE", target, token2); w.Code != 204 {
		t.Fatal("idempotent remove failed")
	}
	p := ports.Principal{WorkspaceID: compositionWorkspace, Issuer: compositionIssuer, Subject: compositionSubject, PolicyGeneration: 1}
	// A fresh store instance reads durable rows; no browser state is involved.
	store := postgresBookmarks{pool: instance.pool}
	ids, err := store.list(ctx, p)
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("durability %v %v", ids, err)
	}
	if w := request(second, "PUT", target, token2); w.Code != 204 {
		t.Fatal("second user's own save failed")
	}
	if w := request(first, "DELETE", target, token1); w.Code != 204 {
		t.Fatal("remove failed")
	}
	if w := request(second, "GET", "/app/api/bookmarks", ""); w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatal("cross-user removal")
	}
	after, _ := index.Get(id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("global curation changed")
	}
	var count int
	if err := instance.pool.QueryRow(ctx, `SELECT count(*) FROM directory_bookmarks`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unscoped RLS %d %v", count, err)
	}
	if err := storage.WithTenant(ctx, instance.pool, "foreign", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM directory_bookmarks`).Scan(&count)
	}); err != nil || count != 0 {
		t.Fatalf("tenant RLS %d %v", count, err)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET active=false WHERE subject='second'`); err != nil {
		t.Fatal(err)
	}
	p.Subject = "second"
	ids, err = store.list(ctx, p)
	if err != nil || len(ids) != 0 {
		t.Fatal("revoked SQL read")
	}
	if err := store.set(ctx, p, id, true); !errors.Is(err, errDirectoryDenied) {
		t.Fatalf("revoked SQL write %v", err)
	}
	if w := request(second, "GET", "/app/api/bookmarks", ""); w.Code != 401 {
		t.Fatalf("revoked HTTP read %d", w.Code)
	}
	// Saved view pages the user's entire collection, with search independent of page.
	public, err := index.Search(directory.Query{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	savedIDs := []string{}
	for _, record := range public.Items {
		savedIDs = append(savedIDs, record.ID)
	}
	if _, err := f.adminPool.Exec(ctx, `INSERT INTO directory_bookmarks(workspace_id,issuer,subject,entry_id) SELECT $1,$2,$3,unnest($4::text[])`, compositionWorkspace, compositionIssuer, compositionSubject, savedIDs); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 3; page++ {
		endpoint := "/app/api/bookmarks?view=entries"
		if cursor != "" {
			endpoint += "&offset=" + cursor
		}
		w := request(first, "GET", endpoint, "")
		var result struct {
			Items []directory.Record `json:"items"`
			Next  string             `json:"next_cursor"`
			Total int                `json:"total"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Total != 50 {
			t.Fatalf("page %d %d %s", page, w.Code, w.Body)
		}
		for _, record := range result.Items {
			if seen[record.ID] {
				t.Fatal("duplicate saved page")
			}
			seen[record.ID] = true
		}
		cursor = result.Next
	}
	if len(seen) != 50 || cursor != "" {
		t.Fatalf("incomplete pages: %d %s", len(seen), cursor)
	}
	if _, err := f.adminPool.Exec(ctx, `DELETE FROM directory_bookmarks WHERE subject=$1`, compositionSubject); err != nil {
		t.Fatal(err)
	}

	// Fill only the first user's collection and verify its bound is independent.
	if _, err := f.adminPool.Exec(ctx, `INSERT INTO directory_bookmarks(workspace_id,issuer,subject,entry_id) SELECT $1,$2,$3,'fixture-'||n FROM generate_series(1,200) n`, compositionWorkspace, compositionIssuer, compositionSubject); err != nil {
		t.Fatal(err)
	}
	p.Subject = compositionSubject
	if err := store.set(ctx, p, "overflow", true); !errors.Is(err, errBookmarksFull) {
		t.Fatalf("capacity %v", err)
	}
	if err := store.set(ctx, p, "fixture-1", true); err != nil {
		t.Fatal("existing bookmark rejected at capacity")
	}
	if w := request(first, "PUT", target, token1); w.Code != 409 {
		t.Fatalf("HTTP limit %d %s", w.Code, w.Body)
	}
}
