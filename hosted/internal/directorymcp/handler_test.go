package directorymcp

import (
	"encoding/json"
	"github.com/sirerun/gist/catalog/directory"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *Handler {
	t.Helper()
	idx, err := directory.New(directory.Snapshot{Format: directory.SnapshotFormat, Revision: "test", GeneratedAt: time.Now(), Sources: []directory.Source{{ID: "fixture", URL: "https://example.com", Revision: "1", SHA256: strings.Repeat("a", 64), RetrievedAt: time.Now(), Attribution: "Fixture"}}, Entries: []directory.Record{{ID: "fixture/example", Kind: "mcp", ServiceID: "example", Title: "Example", Description: "Remote MCP endpoint listed by Make as a verified server. The operator is not verified by Gist. This reachability check is not an authenticated session or workflow qualification.", Tags: []string{"files"}, Source: directory.RecordSource{ID: "fixture", EntryID: "example", URL: "https://example.com"}, Remotes: []directory.Remote{{URL: "https://example.com/mcp", Transport: "streamable-http", Check: &directory.EndpointCheck{CheckedAt: time.Now(), HTTPStatus: 401, Result: "authentication-required"}}}, Auth: directory.Auth{Type: "none"}, Official: true, OfficialEvidenceURL: "https://example.com/vendor", Verification: []directory.Qualification{{Client: "Zati", Workflow: "Read files", EvidenceURL: "https://example.com/private-evidence", CheckedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Result: "passed"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return New(idx, "https://gist.example", 2)
}
func invoke(h *Handler, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/directory/mcp", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDiscoveryMCPProtocol(t *testing.T) {
	h := fixture(t)
	for _, tt := range []struct {
		body     string
		code     int
		contains string
	}{
		{`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"fixture","version":"1"}}}`, 200, `"protocolVersion":"2025-06-18"`},
		{`{"jsonrpc":"2.0","method":"notifications/initialized"}`, 202, ""},
		{`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, 200, "gist_directory_export"},
		{`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"gist_directory_search","arguments":{"query":"Example","limit":1}}}`, 200, "fixture/example"},
		{`{"jsonrpc":"2.0","id":"4","method":"tools/call","params":{"name":"gist_directory_export","arguments":{"id":"fixture/example"}}}`, 200, directory.ExportFormat},
		{`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"gmail_send","arguments":{}}}`, 200, `"code":-32602`},
		{`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"gist_directory_search","arguments":{"workspace":"private"}}}`, 200, `"code":-32602`},
		{`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"gist_directory_get","arguments":{"id":"private-secret"}}}`, 200, `"isError":true`},
		{`[{}]`, 200, `"code":-32600`}, {`{`, 200, `"code":-32700`},
	} {
		w := invoke(h, tt.body)
		if w.Code != tt.code || !strings.Contains(w.Body.String(), tt.contains) {
			t.Fatalf("body=%s status=%d result=%s", tt.body, w.Code, w.Body)
		}
	}
	w := invoke(h, `{"jsonrpc":"2.0","id":8,"method":"tools/list"}`)
	var out map[string]any
	if json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatal(w.Body.String())
	}
}

func TestDiscoveryMCPHidesInternalDirectoryEvidence(t *testing.T) {
	h := fixture(t)
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"gist_directory_search","arguments":{"query":"Example"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_directory_get","arguments":{"id":"fixture/example"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"gist_directory_export","arguments":{"id":"fixture/example"}}}`,
	}
	for _, request := range requests {
		w := invoke(h, request)
		if w.Code != 200 {
			t.Fatalf("status=%d %s", w.Code, w.Body)
		}
		body := strings.ToLower(w.Body.String())
		for _, secret := range []string{"make", "operator is not verified", "endpoint responded", "authentication-required", "workflow qualification", "private-evidence", "official_evidence_url", "verification", "featured"} {
			if strings.Contains(body, secret) {
				t.Fatalf("public MCP response exposed %q: %s", secret, body)
			}
		}
		for _, useful := range []string{"fixture/example", "example", "https://example.com/mcp", "streamable-http"} {
			if !strings.Contains(body, useful) {
				t.Fatalf("public MCP response omitted %q: %s", useful, body)
			}
		}
	}
}
func TestDiscoveryMCPTransportGuards(t *testing.T) {
	h := fixture(t)
	for _, tt := range []struct {
		method, origin, accept, media, target string
		status                                int
	}{
		{"GET", "", "application/json, text/event-stream", "application/json", "/directory/mcp", 405},
		{"POST", "https://evil.example", "application/json, text/event-stream", "application/json", "/directory/mcp", 403},
		{"POST", "", "application/json", "application/json", "/directory/mcp", 406},
		{"POST", "", "application/json, text/event-stream", "text/plain", "/directory/mcp", 415},
		{"POST", "", "application/json, text/event-stream", "application/json", "/directory/mcp?workspace=x", 400},
	} {
		r := httptest.NewRequest(tt.method, tt.target, strings.NewReader(`{}`))
		r.Header.Set("Origin", tt.origin)
		r.Header.Set("Accept", tt.accept)
		r.Header.Set("Content-Type", tt.media)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.status || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("%+v: %d %s", tt, w.Code, w.Body)
		}
	}
}

func TestProviderAuthorizationTool(t *testing.T) {
	h := fixture(t)
	w := invoke(h, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if !strings.Contains(w.Body.String(), "gist_directory_authorization") {
		t.Fatal("authorization tool not discoverable")
	}
	w = invoke(h, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_directory_authorization","arguments":{"id":"fixture/example"}}}`)
	var response struct {
		Result struct {
			Structured directory.AuthorizationGuide `json:"structuredContent"`
			IsError    bool                         `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	g := response.Result.Structured
	if response.Result.IsError || g.ID != "fixture/example" || g.Status != "needs_research" || g.ConnectionTested || len(g.Methods) != 0 {
		t.Fatalf("unexpected authorization response %s", w.Body)
	}
	w = invoke(h, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"gist_directory_authorization","arguments":{"id":"fixture/example","token":"secret"}}}`)
	if !strings.Contains(w.Body.String(), `"code":-32602`) {
		t.Fatal("credential input accepted")
	}
}

func TestProviderAuthorizationAppSelector(t *testing.T) {
	h := fixture(t)
	w := invoke(h, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"gist_directory_authorization","arguments":{"app":"google-sheets"}}}`)
	if !strings.Contains(w.Body.String(), "gist.directory.app-authorization/v1") {
		t.Fatal(w.Body)
	}
	w = invoke(h, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_directory_authorization","arguments":{"app":"google-sheets","id":"fixture/example"}}}`)
	if !strings.Contains(w.Body.String(), `"code":-32602`) {
		t.Fatal("ambiguous selector accepted")
	}
}

func TestAppAuthorizationV2MCP(t *testing.T) {
	h := fixture(t)
	w := invoke(h, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"gist_directory_authorization","arguments":{"app":"shopify","version":2}}}`)
	if !strings.Contains(w.Body.String(), "gist.directory.app-authorization/v2") || !strings.Contains(w.Body.String(), "api_url_template") {
		t.Fatal(w.Body)
	}
	w = invoke(h, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_directory_authorization","arguments":{"app":"shopify","version":3}}}`)
	if !strings.Contains(w.Body.String(), `"code":-32602`) {
		t.Fatal("unsupported version accepted")
	}
}

func TestAppAuthorizationMCPRejectsExplicitZeroAndNullVersion(t *testing.T) {
	h := fixture(t)
	for _, version := range []string{"0", "null"} {
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"gist_directory_authorization","arguments":{"app":"shopify","version":` + version + `}}}`
		w := invoke(h, body)
		if !strings.Contains(w.Body.String(), `"code":-32602`) {
			t.Fatalf("invalid version %s accepted: %s", version, w.Body)
		}
	}
}
