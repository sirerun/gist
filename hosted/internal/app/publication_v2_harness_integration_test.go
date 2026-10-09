//go:build integration

package app

import (
	"bytes"
	"context"
	"errors"
	"github.com/sirerun/gist/hosted/internal/identity"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// matrixHarness owns a real restricted-role application, PostgreSQL database,
// TLS HTTP server and filesystem. Only trusted synthetic reviews/namespaces
// are seeded; callers publish every positive artifact through HTTP themselves.
type matrixHarness struct {
	fixture          compositionPostgres
	artifacts        map[string]matrixPublication
	verifier         *matrixIdentityVerifier
	instance         *App
	server           *httptest.Server
	token            string
	ctx              context.Context
	shutdownExpected error
}

func startMatrixHarness(t *testing.T, configure func(*Config)) *matrixHarness {
	t.Helper()
	f := newCompositionPostgres(t)
	f.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	artifacts, verifier := matrixFixtures(t)
	for kind, a := range artifacts {
		if _, err := f.adminPool.Exec(ctx, "INSERT INTO namespace_reservations(prefix,workspace_id,owner_id,status) VALUES($1,$2,$3,'active')", kind, compositionWorkspace, compositionSubject); err != nil {
			t.Fatal(err)
		}
		if _, err := f.adminPool.Exec(ctx, "INSERT INTO review_records(workspace_id,kind,artifact_id,version,decision,reviewer_id,evidence) VALUES($1,$2,$3,$4,'approved',$5,$6)", compositionWorkspace, a.prepared.Ref.Kind, a.prepared.Ref.ID, a.prepared.Ref.Version, "fixture-maintainer", matrixJSON(t, a.evidence)); err != nil {
			t.Fatal(err)
		}
	}
	cfg := compositionConfig(f.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	cfg.MaxRequestBytes = 16 << 20
	cfg.PublicationV2 = &PublicationV2Config{AllowSynthetic: true, TrustedReviewers: []TrustedPublicationReviewer{{Issuer: "fixture-issuer", Subject: "fixture-maintainer"}}, BindingVerifier: verifier, MaintenanceTargets: []MaintenanceTarget{{WorkspaceID: compositionWorkspace, Subject: compositionSubject}}}
	if configure != nil {
		configure(&cfg)
	}
	a, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	h := &matrixHarness{fixture: f, artifacts: artifacts, verifier: verifier, instance: a, ctx: ctx}
	t.Cleanup(func() {
		if err := a.Shutdown(context.Background()); !errors.Is(err, h.shutdownExpected) {
			t.Errorf("shutdown = %v want %v", err, h.shutdownExpected)
		}
	})
	assertCompositionRuntimeRole(t, a, f.role)
	token, err := a.MintWorkloadToken(ctx, identity.WorkloadRequest{Subject: compositionSubject, WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read", "catalog:publish", "catalog:resolve"}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(a.Handler())
	t.Cleanup(server.Close)
	h.server, h.token = server, token
	return h
}
func (h *matrixHarness) request(t *testing.T, method, path string, body []byte) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(h.ctx, method, h.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, got
}
func (h *matrixHarness) publish(t *testing.T, kind string) []byte {
	t.Helper()
	status, _, body := h.request(t, http.MethodPost, "/v2/publish/"+kind, h.artifacts[kind].envelope)
	if status != 201 {
		t.Fatalf("%s publish status=%d body=%s", kind, status, body)
	}
	return body
}
