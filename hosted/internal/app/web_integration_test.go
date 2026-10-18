//go:build integration

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestWebCatalogUsesRestrictedTenantRoleAndCurrentMembership(t *testing.T) {
	f := newCompositionPostgres(t)
	f.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	digest := strings.Repeat("a", 64)
	seed := func(workspace, id, state string) {
		t.Helper()
		metadata := fmt.Sprintf(`{"name":%q,"description":"Read-only inbox preparation","object_key":"private-storage-secret","credential":"do-not-expose"}`, id)
		_, err := f.adminPool.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_value,metadata,owner_id) VALUES($1,'skill',$2,'1.0.0',$3,'sha256',$4,$4,$5::jsonb,'fixture')`, workspace, id, state, digest, metadata)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30; i++ {
		seed(compositionWorkspace, fmt.Sprintf("skill-%02d", i), "published")
	}
	seed(compositionWorkspace, "revoked", "revoked")
	seed(compositionWorkspace, "draft", "draft")
	if _, err := f.adminPool.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES('foreign','Foreign')`); err != nil {
		t.Fatal(err)
	}
	seed("foreign", "foreign-secret", "published")
	cfg := compositionConfig(f.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	instance, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := instance.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	cookie, err := instance.IssueOAuthSession(compositionSubject, []string{compositionWorkspace})
	if err != nil {
		t.Fatal(err)
	}
	request := func(target string, session *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", target, nil)
		if session != nil {
			r.AddCookie(session)
		}
		w := httptest.NewRecorder()
		instance.Handler().ServeHTTP(w, r)
		return w
	}
	if w := request("/app/api/catalog", nil); w.Code != 401 {
		t.Fatalf("anonymous=%d %s", w.Code, w.Body)
	}
	session := request("/app/api/session", cookie)
	if session.Code != 200 || !strings.Contains(session.Body.String(), `"workspace_id":"`+compositionWorkspace+`"`) {
		t.Fatalf("session=%d %s", session.Code, session.Body)
	}
	type page struct {
		Items []webItem `json:"items"`
		Next  string    `json:"next_cursor"`
	}
	first := request("/app/api/catalog", cookie)
	var a page
	if first.Code != 200 || json.Unmarshal(first.Body.Bytes(), &a) != nil || len(a.Items) != 24 || a.Next == "" {
		t.Fatalf("first page=%d %s", first.Code, first.Body)
	}
	for _, secret := range []string{"foreign-secret", "do-not-expose", "private-storage-secret", "revoked", "draft"} {
		if strings.Contains(first.Body.String(), secret) {
			t.Fatalf("catalog exposed %s", secret)
		}
	}
	second := request("/app/api/catalog?cursor="+url.QueryEscape(a.Next), cookie)
	var b page
	if second.Code != 200 || json.Unmarshal(second.Body.Bytes(), &b) != nil || len(b.Items) != 6 || b.Next != "" || a.Items[23].ID >= b.Items[0].ID {
		t.Fatalf("second page=%d %s", second.Code, second.Body)
	}
	search := request("/app/api/catalog?q=skill-29&kind=skill", cookie)
	var hits page
	if search.Code != 200 || json.Unmarshal(search.Body.Bytes(), &hits) != nil || len(hits.Items) != 1 || hits.Items[0].ID != "skill-29" {
		t.Fatalf("search=%d %s", search.Code, search.Body)
	}
	for _, id := range []string{"foreign-secret", "revoked", "draft"} {
		if w := request("/app/api/artifact?kind=skill&id="+id+"&version=1.0.0", cookie); w.Code != 404 {
			t.Fatalf("unavailable %s=%d %s", id, w.Code, w.Body)
		}
	}
	item := request("/app/api/artifact?kind=skill&id=skill-01&version=1.0.0", cookie)
	if item.Code != 200 || strings.Contains(item.Body.String(), "do-not-expose") || !strings.Contains(item.Body.String(), `"value":"`+digest+`"`) {
		t.Fatalf("item=%d %s", item.Code, item.Body)
	}
	foreignCookie, err := instance.IssueOAuthSession(compositionSubject, []string{"foreign"})
	if err != nil {
		t.Fatal(err)
	}
	if w := request("/app/api/catalog", foreignCookie); w.Code != 401 {
		t.Fatalf("foreign cookie=%d", w.Code)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET active=false WHERE workspace_id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	if w := request("/app/api/catalog", cookie); w.Code != 401 {
		t.Fatalf("revoked membership=%d %s", w.Code, w.Body)
	}
	// The SQL read also rejects the previously authorized principal after revoke.
	stale := ports.Principal{Issuer: compositionIssuer, Audience: compositionIssuer, Subject: compositionSubject, WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read"}, PolicyGeneration: 1}
	if items, err := (postgresWebCatalog{pool: instance.pool}).list(ctx, stale, "", "", webPosition{}); err != nil || len(items) != 0 {
		t.Fatalf("read-time authority guard items=%d err=%v", len(items), err)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET active=true WHERE workspace_id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspaces SET policy_generation=2 WHERE id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	if w := request("/app/api/session", cookie); w.Code != 401 {
		t.Fatalf("outdated membership=%d", w.Code)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET policy_generation=2 WHERE workspace_id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	if w := request("/app/api/catalog?cursor="+url.QueryEscape(a.Next), cookie); w.Code != 400 {
		t.Fatalf("stale policy cursor=%d %s", w.Code, w.Body)
	}
	if w := request("/app/api/session", cookie); w.Code != 200 {
		t.Fatalf("current membership=%d %s", w.Code, w.Body)
	}
}
