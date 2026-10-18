package webui

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/sirerun/gist/catalog/directory"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var machineFiles = map[string]struct{ file, kind string }{
	"/openapi.json":            {"openapi.json", "application/vnd.oai.openapi+json"},
	"/server.json":             {"server.json", "application/json"},
	"/.well-known/mcp.json":    {"server.json", "application/json"},
	"/.well-known/api-catalog": {"api-catalog.json", `application/linkset+json; profile="https://www.rfc-editor.org/info/rfc9727"`},
	"/identity.jsonld":         {"identity.jsonld", "application/ld+json"},
	"/agent-instructions.md":   {"agent-instructions.md", "text/markdown"},
	"/llms.txt":                {"llms.txt", "text/markdown"},
}

var identity = sync.OnceValue(func() string {
	b, err := files.ReadFile("agentdocs/identity.jsonld")
	if err != nil {
		panic(err)
	}
	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		panic(err)
	}
	safe, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(safe)
})

func identityHash() string {
	sum := sha256.Sum256([]byte(identity()))
	return base64.StdEncoding.EncodeToString(sum[:])
}
func identityScript() template.HTML {
	return template.HTML(`<script type="application/ld+json">` + identity() + `</script>`)
}

func websiteHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self' 'sha256-"+identityHash()+"'; img-src 'self'; connect-src 'self'; font-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'; object-src 'none'")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Link", `</llms.txt>; rel="describedby"; type="text/markdown", </openapi.json>; rel="service-desc"; type="application/vnd.oai.openapi+json", </.well-known/api-catalog>; rel="api-catalog"`)
}

// Explicit Markdown preference is required; wildcards keep the HTML default.
// For each representation, a more specific range overrides a wildcard q-value.
var qualityValue = regexp.MustCompile(`^(0(\.[0-9]{0,3})?|1(\.0{0,3})?)$`)

func wantsMarkdown(accept string) bool {
	type preference struct {
		q                  float64
		specificity, order int
	}
	html, md := preference{-1, -1, 1 << 20}, preference{-1, -1, 1 << 20}
	explicit := false
	parts := []string{}
	start := 0
	quoted, escaped := false, false
	for i, c := range accept {
		if escaped {
			escaped = false
			continue
		}
		if quoted && c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			quoted = !quoted
		}
		if c == ',' && !quoted {
			parts = append(parts, accept[start:i])
			start = i + 1
		}
	}
	parts = append(parts, accept[start:])
	for order, p := range parts {
		media, params, err := mime.ParseMediaType(strings.TrimSpace(p))
		if err != nil {
			continue
		}
		q := 1.0
		if v, ok := params["q"]; ok {
			if !qualityValue.MatchString(v) {
				continue
			}
			q, err = strconv.ParseFloat(v, 64)
			if err != nil || q < 0 || q > 1 {
				continue
			}
		}
		if media == "text/markdown" {
			explicit = true
		}
		for name, dst := range map[string]*preference{"text/html": &html, "text/markdown": &md} {
			specificity := -1
			switch media {
			case name:
				specificity = 2
			case "text/*":
				specificity = 1
			case "*/*":
				specificity = 0
			}
			if specificity < 0 {
				continue
			}
			if specificity > dst.specificity || (specificity == dst.specificity && q > dst.q) {
				*dst = preference{q, specificity, order}
			}
		}
	}
	return explicit && md.q > 0 && (md.q > html.q || (md.q == html.q && (md.specificity > html.specificity || (md.specificity == html.specificity && md.order < html.order))))
}
func writeMarkdown(w http.ResponseWriter, r *http.Request, status int, body string) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Vary", "Accept")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(body))
	}
}

// PublicFallback keeps unknown website/public-API paths out of registry auth.
// Protected namespaces retain their authentication and protocol behavior.
func PublicFallback(p string) bool {
	if Handles(p) {
		return false
	}
	for _, prefix := range []string{"/app", "/auth", "/oauth", "/v1", "/v2"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return false
		}
	}
	if strings.HasPrefix(p, "/.well-known/oauth-") {
		return false
	}
	switch p {
	case "/mcp", "/directory/mcp", "/healthz", "/readyz", "/api/directory", "/api/directory/item", "/api/directory/export", "/api/directory/apps", "/api/directory/app":
		return false
	}
	return true
}
func NotFound(w http.ResponseWriter, r *http.Request) {
	websiteHeaders(w)
	w.Header().Set("Vary", "Accept")
	w.Header().Set("X-Robots-Tag", "noindex")
	body := "# Page not found\n\nThis URL does not identify a public Gist page or API endpoint. Use the developer guide or API description to choose a supported route.\n\n- [Gist developer guide](https://gist.sire.run/developers)\n- [OpenAPI](https://gist.sire.run/openapi.json)\n- [Agent instructions](https://gist.sire.run/llms.txt)\n- [Sitemap](https://gist.sire.run/sitemap.xml)\n"
	if wantsMarkdown(r.Header.Get("Accept")) {
		writeMarkdown(w, r, 404, body)
		return
	}

	renderPageStatus(w, r, "not-found", nil, siteData{Title: "Page not found · Gist", Page: "not-found", Description: "Find supported Gist pages, developer documentation and public API endpoints."}, 404)
}

func machineRoute(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/api" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodHead {
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "Gist public catalog API", "version": "1.0.0", "authentication": "none", "openapi_url": publicOrigin + "/openapi.json", "docs_url": publicOrigin + "/developers", "mcp_url": publicOrigin + "/directory/mcp", "endpoints": []string{"/api/directory", "/api/directory/item", "/api/directory/export", "/api/directory/apps", "/api/directory/app"}})
		}
		return true
	}
	if item, ok := machineFiles[r.URL.Path]; ok {
		raw, err := files.ReadFile("agentdocs/" + item.file)
		if err != nil {
			http.Error(w, "Document unavailable", 500)
			return true
		}
		w.Header().Set("Content-Type", item.kind+"; charset=utf-8")
		if r.Method != http.MethodHead {
			_, _ = w.Write(raw)
		}
		return true
	}
	if r.URL.Path == "/index.md" {
		body, _ := publicMarkdown("home", nil)
		writeMarkdown(w, r, 200, body)
		return true
	}
	if strings.HasSuffix(r.URL.Path, ".md") {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".md")
		if body, ok := publicMarkdown(name, nil); ok {
			writeMarkdown(w, r, 200, body)
			return true
		}
	}
	return false
}

func publicMarkdown(name string, view any) (string, bool) {
	if doc, ok := documentFor("/" + name); ok {
		return doc.Markdown(), true
	}
	switch name {
	case "public-document":
		if d, ok := view.(publicDocument); ok {
			return d.Markdown(), true
		}
	case "home":
		raw, _ := files.ReadFile("agentdocs/home.md")
		return string(raw), true
	case "developers", "docs":
		raw, _ := files.ReadFile("agentdocs/" + name + ".md")
		return string(raw), true
	case "library", "directory":
		return "# Gist AI integrations library\n\nBrowse public apps, MCP servers and OpenAPI integrations. No sign-in, API key or payment is required for public discovery. App listings describe software; connector IDs identify available setup descriptors. Your client authorizes provider accounts and executes the work.\n\n- [Browse apps](https://gist.sire.run/api/directory/apps?limit=24)\n- [Find MCP servers](https://gist.sire.run/api/directory?kind=mcp&limit=5)\n- [Find OpenAPI integrations](https://gist.sire.run/api/directory?kind=openapi&limit=5)\n- [API reference](https://gist.sire.run/openapi.json)\n- [Client setup](https://gist.sire.run/developers.md)\n", true
	case "app-pair":
		return "# OpenAI and Airtable workflow idea\n\nUse Airtable records as a selected brief and OpenAI to prepare content for review. In Zati, configure the required connections, scope and review steps. This is an idea to set up in your client, not an activated automation.\n\n## Steps\n\n1. Read the selected topics and source fields.\n2. Prepare a draft grounded in those sources.\n3. Review the proposed record changes before saving or publishing.\n\n[Explore OpenAI](https://gist.sire.run/apps/openai-gpt-3) · [Explore Airtable](https://gist.sire.run/apps/airtable) · [Explore Zati](https://zati.run)\n", true
	case "connector-preview":
		if c, ok := view.(directory.Record); ok {
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n\n%s\n\n## Connect in your client\n\n", c.Title, c.Description)
			for _, remote := range c.Remotes {
				fmt.Fprintf(&b, "- %s endpoint: %s\n", remote.Transport, remote.URL)
			}
			fmt.Fprintf(&b, "\nAuthentication hint: %s. Review provider setup in your client.\n\n[Public descriptor](%s/api/directory/item?id=%s)\n", c.Auth.Type, publicOrigin, url.QueryEscape(c.ID))
			return b.String(), true
		}
	case "app-detail":
		if v, ok := view.(appPage); ok {
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n\n%s\n\n## Workflow ideas\n\n", v.App.Name, v.App.Description)
			for _, idea := range v.Ideas {
				fmt.Fprintf(&b, "### %s\n\n%s\n\nTry asking: %s\n\n", idea.Title, idea.Description, idea.Prompt)
			}
			b.WriteString("## Connection options\n\n")
			for _, c := range v.Connections {
				fmt.Fprintf(&b, "- [%s](%s%s)\n", c.Name, publicOrigin, c.URL)
			}
			if len(v.Connections) == 0 {
				b.WriteString("MCP connection details are not available yet.\n")
			}
			b.WriteString("\nThese are workflow ideas to configure in Zati or another client. Your client controls account access and execution.\n\n[Explore Zati](https://zati.run)\n")
			return b.String(), true
		}
	}
	return "", false
}
