package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFirstEnrollmentRequiresExactFounderAndV1(t *testing.T) {
	cfg := validConfig()
	cfg.FirstEnrollment = true
	if cfg.Validate() == nil {
		t.Fatal("accepted arbitrary maintenance actor")
	}
	cfg.EventMaintenanceTargets[0].Subject = cfg.GitHubEnrollment.FounderAccount
	cfg.GitHubEnrollment.BootstrapScopes = []string{"catalog:read", "catalog:publish"}
	cfg.GitHubEnrollment.AllowedScopes = append([]string(nil), cfg.GitHubEnrollment.BootstrapScopes...)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.PublicationV2 = &PublicationV2Config{}
	if cfg.Validate() == nil {
		t.Fatal("accepted publication v2 during bootstrap")
	}
}

func TestFirstEnrollmentAdmissionDeniesAllNonEnrollmentAPIs(t *testing.T) {
	reached := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++; w.WriteHeader(http.StatusNoContent) })
	h := requestContext{admission: func() error { return errors.New("not ready") }, rest: handler, mcp: handler, oauth: handler, enrollment: handler}
	for _, path := range []string{"/mcp", "/v1/skills", "/v2/skills", "/oauth/token", "/oauth/authorize", "/operator/invitations"} {
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest(http.MethodPost, path, nil))
		if out.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: %d", path, out.Code)
		}
	}
	if reached != 0 {
		t.Fatal("unready API reached a handler")
	}
	for _, path := range []string{"/auth/github/start", "/auth/github/callback"} {
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest(http.MethodGet, path, nil))
		if out.Code != http.StatusNoContent {
			t.Fatalf("%s: %d", path, out.Code)
		}
	}
}
