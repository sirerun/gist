package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/hosted/internal/webui"
)

func TestPublicMissingPathDoesNotFallThroughToRegistryAuthentication(t *testing.T) {
	unauthorized := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) })
	for _, admission := range []func() error{nil, func() error { return errors.New("bootstrap pending") }} {
		h := requestContext{ui: webui.Handler(), rest: unauthorized, mcp: unauthorized, admission: admission}
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/some-path-that-does-not-exist", nil)
		r.Header.Set("Accept", "text/markdown")
		h.ServeHTTP(w, r)
		if w.Code != 404 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/markdown") {
			t.Fatal(w.Code, w.Body.String())
		}
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/discover", nil))
		expected := 401
		if admission != nil {
			expected = 503
		}
		if w.Code != expected {
			t.Fatal("registry auth changed", w.Code)
		}
	}
}
func TestPublicDirectoryResponsesMatchPublishedSchema(t *testing.T) {
	ui := webui.Handler()
	schema := httptest.NewRecorder()
	ui.ServeHTTP(schema, httptest.NewRequest("GET", "/openapi.json", nil))
	doc, err := openapi3.NewLoader().LoadFromData(schema.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := directory.DefaultApps(idx)
	if err != nil {
		t.Fatal(err)
	}
	h := &directoryAPI{index: idx, apps: apps, maxBytes: 256 << 10}
	for _, p := range []string{"/api", "/api/directory?limit=2", "/api/directory/item?id=mcp:google-gmail", "/api/directory/export?id=mcp:google-gmail", "/api/directory/apps?limit=2", "/api/directory/app?slug=google-email"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", p, nil)
		if r.URL.Path == "/api" {
			ui.ServeHTTP(w, r)
		} else {
			h.ServeHTTP(w, r)
		}
		if w.Code != 200 {
			t.Fatalf("%s %d %s", p, w.Code, w.Body)
		}
		var body any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		s := doc.Paths.Value(r.URL.Path).Get.Responses.Status(200).Value.Content.Get("application/json").Schema.Value
		if err := s.VisitJSON(body); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
}
