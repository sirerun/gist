//go:build integration

package app

import (
	"context"
	"github.com/sirerun/gist/hosted/internal/storage"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type enrollmentRoundTrip func(*http.Request) (*http.Response, error)

func (f enrollmentRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFirstEnrollmentVerifiedFounderUnlocksOnlyAfterMaintenance(t *testing.T) {
	fixture := newCompositionPostgres(t)
	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	cfg.FirstEnrollment = true
	cfg.EventMaintenanceTargets[0].Subject = cfg.GitHubEnrollment.FounderAccount
	cfg.GitHubEnrollment.BootstrapScopes = []string{"catalog:read", "catalog:publish", "catalog:resolve"}
	cfg.GitHubEnrollment.AllowedScopes = append([]string(nil), cfg.GitHubEnrollment.BootstrapScopes...)
	githubCalls := 0
	cfg.GitHubEnrollmentHTTPClient = &http.Client{Transport: enrollmentRoundTrip(func(r *http.Request) (*http.Response, error) {
		githubCalls++
		var body string
		switch r.URL.String() {
		case "https://github.com/login/oauth/access_token":
			body = `{"access_token":"fixture-only","token_type":"bearer"}`
		case "https://api.github.com/user":
			body = `{"id":123456}`
		default:
			t.Fatalf("unexpected external URL %s", r.URL.String())
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	instance, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := instance.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	check := func(path string, want int) {
		t.Helper()
		out := httptest.NewRecorder()
		instance.Handler().ServeHTTP(out, httptest.NewRequest(http.MethodGet, path, nil))
		if out.Code != want {
			t.Fatalf("%s: %d %s", path, out.Code, out.Body.String())
		}
	}
	check("/healthz", 200)
	check("/readyz", 503)
	check("/mcp", 503)
	check("/oauth/token", 503)
	start := httptest.NewRecorder()
	instance.Handler().ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/auth/github/start", nil))
	if start.Code != 302 {
		t.Fatalf("start: %d %s", start.Code, start.Body.String())
	}
	location, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	callback := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=fixture-code&state="+url.QueryEscape(location.Query().Get("state")), nil)
	for _, cookie := range start.Result().Cookies() {
		callback.AddCookie(cookie)
	}
	out := httptest.NewRecorder()
	instance.Handler().ServeHTTP(out, callback)
	if out.Code != 200 || githubCalls != 2 {
		t.Fatalf("verified callback: %d %s calls=%d", out.Code, out.Body.String(), githubCalls)
	}
	check("/readyz", 503)
	check("/oauth/token", 503)
	// Use the normal janitor, with a deterministic tick rather than waiting a minute.
	tick := make(chan time.Time, 1)
	tick <- time.Now()
	close(tick)
	instance.runEventJanitor(ctx, instance.identities.(*storage.Postgres), tick)
	check("/readyz", 200)
	check("/v1/skills/example/versions", 401)
	if _, err := fixture.adminPool.Exec(ctx, `UPDATE workspace_memberships SET active=false WHERE workspace_id=$1`, cfg.GitHubEnrollment.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	tick = make(chan time.Time, 1)
	tick <- time.Now()
	close(tick)
	instance.runEventJanitor(ctx, instance.identities.(*storage.Postgres), tick)
	check("/readyz", 503)
	check("/mcp", 503)
	check("/oauth/token", 503)
	if restarted, err := New(ctx, cfg); err == nil {
		_ = restarted.Shutdown(context.Background())
		t.Fatal("revoked established workspace re-entered bootstrap")
	}
}

func TestFirstEnrollmentDefaultStillRejectsEmptyDatabase(t *testing.T) {
	fixture := newCompositionPostgres(t)
	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	if instance, err := New(context.Background(), cfg); err == nil {
		_ = instance.Shutdown(context.Background())
		t.Fatal("default startup accepted missing maintainer")
	}
	cfg.FirstEnrollment = true
	cfg.EventMaintenanceTargets[0].Subject = cfg.GitHubEnrollment.FounderAccount
	cfg.GitHubEnrollment.BootstrapScopes = []string{"catalog:publish"}
	cfg.GitHubEnrollment.AllowedScopes = []string{"catalog:publish"}
	fixture.seedTenant(t)
	if instance, err := New(context.Background(), cfg); err == nil {
		_ = instance.Shutdown(context.Background())
		t.Fatal("existing workspace with missing founder accepted bootstrap")
	}
}
