package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicRoutes(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/", "/library", "/library/item", "/developers", "/docs", "/signin"} {
		t.Run(p, func(t *testing.T) {
			if !Handles(p) {
				t.Fatal("route not claimed")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
				t.Fatal("not HTML")
			}
			if !strings.Contains(w.Body.String(), `id="main"`) {
				t.Fatal("missing main landmark")
			}
			if strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "style=") {
				t.Fatal("inline execution violates CSP")
			}
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Fatal("missing CSP")
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing nosniff")
			}
		})
	}
}

func TestPublicRouteBoundary(t *testing.T) {
	for _, p := range []string{"/mcp", "/app/api/session", "/auth/github/start", "/oauth/authorize", "/v1/discover", "/library/other", "/readyz"} {
		if Handles(p) {
			t.Errorf("private or unowned route claimed: %s", p)
		}
	}
}

func TestStaticAssetsAndMethods(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/assets/site.css", "/assets/site.js", "/assets/brand/gist-logo-green.svg", "/assets/brand/gist-symbol-white.svg"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Errorf("asset %s status %d bytes %d", p, w.Code, w.Body.Len())
		}
	}
	for _, p := range []string{"/assets/", "/assets/brand/", "/assets/../templates/pages.html", "/assets/missing.js", "/not-owned"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != 404 {
			t.Errorf("invalid asset %s status %d", p, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
	if w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
		t.Fatal("mutation accepted")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/", nil))
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("HEAD returned a body")
	}
}
