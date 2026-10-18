package webui

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/net/html"
)

func TestMarkdownNegotiation(t *testing.T) {
	for _, tc := range []struct {
		accept string
		md     bool
	}{{"", false}, {"*/*", false}, {"text/plain", false}, {"text/markdown", true}, {"text/markdown, text/html;q=0.8", true}, {"text/html, text/markdown;q=0.5", false}, {"text/markdown;q=0, */*;q=1", false}, {"text/markdown;q=1, text/*;q=0.2", true}, {"text/markdown;q=0.2, text/html;q=0, */*;q=1", true}, {`text/markdown; note="a,b", text/html;q=0.8`, true}, {"text/markdown;q=2,text/html", false}} {
		t.Run(tc.accept, func(t *testing.T) {
			h := Handler()
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Accept", tc.accept)
			h.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatal(w.Code)
			}
			if strings.HasPrefix(w.Header().Get("Content-Type"), "text/markdown") != tc.md {
				t.Fatal(w.Header(), w.Body.String())
			}
			if w.Header().Get("Vary") != "Accept" {
				t.Fatal("missing Vary")
			}
			if tc.md && (!strings.HasPrefix(w.Body.String(), "# Gist") || w.Body.Len() < 500) {
				t.Fatal("insufficient Markdown")
			}
		})
	}
}
func TestAgentPublicDocuments(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/index.md", "/developers.md", "/docs.md", "/library.md", "/about", "/contact", "/privacy", "/pricing", "/cli"} {
		for _, accept := range []string{"text/html", "text/markdown"} {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", p, nil)
			r.Header.Set("Accept", accept)
			h.ServeHTTP(w, r)
			if w.Code != 200 || w.Body.Len() < 500 {
				t.Fatalf("%s %s %d %d", p, accept, w.Code, w.Body.Len())
			}
			if accept == "text/markdown" || strings.HasSuffix(p, ".md") {
				if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/markdown") {
					t.Fatal(p, w.Header())
				}
			}
		}
	}
	for p := range machineFiles {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 200 {
			t.Fatal(p, w.Code)
		}
		if !strings.HasSuffix(p, ".txt") && !strings.HasSuffix(p, ".md") && !json.Valid(w.Body.Bytes()) {
			t.Fatal("invalid JSON", p)
		}
		head := httptest.NewRecorder()
		h.ServeHTTP(head, httptest.NewRequest("HEAD", p, nil))
		if head.Code != 200 || head.Body.Len() != 0 {
			t.Fatal("HEAD", p)
		}
	}
}
func TestAgentNotFoundAndProtectedBoundary(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/does-not-exist", "/api/no-such-route", "/assets/missing.js", "/apps/no-such-app", "/connectors/missing"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", p, nil)
		r.Header.Set("Accept", "text/markdown")
		h.ServeHTTP(w, r)
		if w.Code != 404 || w.Header().Get("Content-Type") != "text/markdown; charset=utf-8" || w.Body.Len() < 100 || !strings.Contains(w.Body.String(), "/llms.txt") {
			t.Fatal(p, w.Code, w.Header(), w.Body.String())
		}
	}
	for _, p := range []string{"/v1/discover", "/v2/publish/skill", "/mcp", "/directory/mcp", "/oauth/token", "/auth/github/start", "/app/api/bookmarks", "/.well-known/oauth-protected-resource", "/healthz", "/api/directory"} {
		if PublicFallback(p) {
			t.Fatal("protected route intercepted", p)
		}
	}
	if !PublicFallback("/does-not-exist") || !PublicFallback("/api/does-not-exist") {
		t.Fatal("public unknown not caught")
	}
}
func TestHomepageIdentityAndHeadingHierarchy(t *testing.T) {
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	previous, count := 0, 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' {
			level := int(n.Data[1] - '0')
			if level > previous+1 {
				t.Errorf("heading jump%d to%d", previous, level)
			}
			previous = level
			count++
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if count < 4 {
		t.Fatal("insufficient meaningful headings")
	}
	for _, want := range []string{`rel="canonical" href="https://gist.sire.run/"`, `property="og:image"`, `property="og:type"`, `lang="en"`, `type="application/ld+json"`, `name="is-agentic-site-type" content="app"`} {
		if !strings.Contains(body, want) {
			t.Fatal("missing", want)
		}
	}
	start := strings.Index(body, `<script type="application/ld+json">`) + len(`<script type="application/ld+json">`)
	end := strings.Index(body[start:], "</script>") + start
	raw := body[start:end]
	if !json.Valid([]byte(raw)) {
		t.Fatal("invalid JSON-LD")
	}
	hash := sha256.Sum256([]byte(raw))
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "sha256-"+base64.StdEncoding.EncodeToString(hash[:])) {
		t.Fatal("JSON-LD CSP hash mismatch")
	}
}
func TestPublicOpenAPIIsSelfDescribing(t *testing.T) {
	raw, err := files.ReadFile("agentdocs/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openapi3.NewLoader().LoadFromData(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for p, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" || seen[op.OperationID] || op.Description == "" {
				t.Fatal("operation identity", p, method)
			}
			seen[op.OperationID] = true
			for _, p := range op.Parameters {
				if p.Value.Schema == nil || p.Value.Description == "" {
					t.Fatal("untyped parameter")
				}
			}
			if op.Responses.Status(200).Value.Content.Get("application/json").Schema == nil {
				t.Fatal("missing response schema", p)
			}
		}
	}
	if len(seen) != 6 {
		t.Fatal("unexpected public operation count", len(seen))
	}
}
