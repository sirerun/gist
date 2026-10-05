//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/identity"
)

func TestCoreHTTPRejectsForeignCursorIssuerAndStalePolicy(t *testing.T) {
	f := newCompositionPostgres(t)
	f.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, statement := range []string{
		`INSERT INTO workspaces(id,name,policy_generation) VALUES('other-workspace','other-workspace',1)`,
		`INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,active,policy_generation)
		 SELECT workspace_id,issuer,'other-subject',role,scopes,active,policy_generation FROM workspace_memberships`,
		`INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,active,policy_generation)
		 SELECT 'other-workspace',issuer,subject,role,scopes,active,policy_generation FROM workspace_memberships`,
	} {
		if _, err := f.adminPool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	cfg := compositionConfig(f.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	a, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown app: %v", err)
		}
	})
	mint := func(instance *App, subject, workspace string) string {
		t.Helper()
		token, err := instance.MintWorkloadToken(ctx, identity.WorkloadRequest{Subject: subject, WorkspaceID: workspace, Scopes: []string{"catalog:read"}})
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	token := mint(a, compositionSubject, compositionWorkspace)
	opened := compositionHTTP(t, a.Handler(), http.MethodGet, "/v1/events", token, nil)
	var page struct {
		NextCursor string `json:"next_cursor"`
	}
	if opened.Code != http.StatusOK || json.Unmarshal(opened.Body.Bytes(), &page) != nil || page.NextCursor == "" {
		t.Fatalf("valid event cursor was not opened: %d", opened.Code)
	}
	path := "/v1/events?cursor=" + url.QueryEscape(page.NextCursor)
	assertDenied := func(name, target, bearer string) {
		t.Helper()
		response := compositionHTTP(t, a.Handler(), http.MethodGet, target, bearer, nil)
		if response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden && response.Code != http.StatusNotFound {
			t.Fatalf("%s accepted unauthorized request: %d", name, response.Code)
		}
	}
	assertDenied("foreign subject cursor", path, mint(a, "other-subject", compositionWorkspace))
	assertDenied("foreign workspace cursor", path, mint(a, compositionSubject, "other-workspace"))

	// A second real app uses the same key material but a different configured
	// issuer. Its stored identity and membership exist; the first app must still
	// reject that signed token. These are test-owned grants, not enrollment.
	foreignIssuer := "https://other.gist.example"
	if _, err := f.adminPool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,active,policy_generation)
	 SELECT workspace_id,$1,subject,role,scopes,active,policy_generation FROM workspace_memberships WHERE issuer=$2`, foreignIssuer, compositionIssuer); err != nil {
		t.Fatal(err)
	}
	foreignCfg := cfg
	foreignCfg.PublicOrigin, foreignCfg.ResourceAudience = foreignIssuer, foreignIssuer
	foreign, err := New(ctx, foreignCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := foreign.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown foreign issuer app: %v", err)
		}
	})
	assertDenied("foreign issuer", "/v1/events", mint(foreign, compositionSubject, compositionWorkspace))
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspaces SET policy_generation=2 WHERE id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET policy_generation=2 WHERE workspace_id=$1`, compositionWorkspace); err != nil {
		t.Fatal(err)
	}
	assertDenied("stale token generation", path, token)
	currentToken := mint(a, compositionSubject, compositionWorkspace)
	assertDenied("stale cursor generation", path, currentToken)
	if _, err := f.adminPool.Exec(ctx, `UPDATE workspace_memberships SET active=false WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, compositionWorkspace, compositionIssuer, compositionSubject); err != nil {
		t.Fatal(err)
	}
	assertDenied("inactive current membership", "/v1/events", currentToken)
}
