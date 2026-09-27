//go:build integration

package wiring

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestC1HTTPAndRemoteMCPBoundaries(t *testing.T) {
	f := requireFixture(t)
	tok := f.token(t, "q3-tenant-a", f.workA, "catalog:read", "catalog:publish")
	cases := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"discover", "POST", "/v1/discover", `{"query":"fixture","max_bytes":4096}`, 200, ""},
		{"versions", "GET", "/v1/skills/q3-fixture-skill/versions", "", 200, ""},
		{"skill", "GET", "/v1/skills/q3-fixture-skill/versions/1.0.0", "", 200, ""},
		{"package", "GET", "/v1/skills/q3-fixture-skill/versions/1.0.0/package", "", 200, ""},
		{"tool", "GET", "/v1/tools/tool/versions/1.0.0", "", 200, ""},
		{"capability", "GET", "/v1/capabilities/cap/versions/1.0.0", "", 200, ""},
		{"resolve", "POST", "/v1/resolve", `{"skill":{"kind":"skill","id":"q3-fixture-skill","version":"1.0.0"},"runtime_id":"go","local_execution":true,"max_bytes":4096}`, 200, ""},
		{"connection", "POST", "/v1/connections", `{"capability":{"kind":"capability","id":"cap","version":"1.0.0"}}`, 201, ""},
		{"connection poll", "GET", "/v1/connections/broker-c1", "", 200, ""},
		{"taxonomies", "GET", "/v1/taxonomies", "", 200, ""},
		{"taxonomy nodes", "GET", "/v1/taxonomies/tax/nodes", "", 404, "not_found"},
		{"publish skills", "POST", "/v1/publish/skills", `{}`, 503, "service_unavailable"},
		{"publish capabilities", "POST", "/v1/publish/capabilities", `{}`, 503, "service_unavailable"},
		{"publish tools", "POST", "/v1/publish/tools", `{}`, 503, "service_unavailable"},
		{"publish providers", "POST", "/v1/publish/providers", `{}`, 503, "service_unavailable"},
		{"publish bindings", "POST", "/v1/publish/bindings", `{}`, 503, "service_unavailable"},
		{"publish taxonomies", "POST", "/v1/publish/taxonomies", `{}`, 503, "service_unavailable"},
		{"publish revocations", "POST", "/v1/publish/revocations", `{}`, 422, "validation_failed"},
		{"revoke identity without scope", "POST", "/v1/identities/revoke", `{"issuer":"https://issuer.test","subject":"q3-other"}`, 403, "forbidden"},
		{"events", "GET", "/v1/events", "", 200, ""},
		{"batch", "POST", "/v1/artifacts/batch-get", `{"references":[{"workspace_id":"q3-tenant-a","kind":"skill","id":"q3-fixture-skill","version":"1.0.0"}],"max_bytes":4096}`, 200, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.do(t, tc.method, tc.path, tok, tc.body)
			defer resp.Body.Close()
			raw, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tc.status {
				t.Fatalf("status=%d want %d body=%s", resp.StatusCode, tc.status, raw)
			}
			if tc.code != "" {
				var e struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(raw, &e); err != nil || e.Code != tc.code {
					t.Fatalf("body=%s want code %s", raw, tc.code)
				}
			} else if len(raw) == 0 {
				t.Fatal("endpoint returned an empty body")
			}
		})
	}
	first := f.do(t, "GET", "/v1/skills/q3-fixture-skill/versions/1.0.0/package", tok, "")
	etag := first.Header.Get("ETag")
	first.Body.Close()
	if etag == "" {
		t.Fatal("package response did not set ETag")
	}
	req, err := http.NewRequest("GET", f.server.URL+"/v1/skills/q3-fixture-skill/versions/1.0.0/package", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+f.token(t, "q3-tenant-b", f.workB, "catalog:read"))
	req.Header.Set("If-None-Match", etag)
	denied, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Body.Close()
	if denied.StatusCode != 404 {
		t.Fatalf("cross-tenant conditional request status=%d want 404", denied.StatusCode)
	}
	unauth, err := http.NewRequest("POST", f.server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.server.Client().Do(unauth)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 401 || r.Header.Get("WWW-Authenticate") == "" {
		t.Fatalf("MCP auth status=%d challenge=%q", r.StatusCode, r.Header.Get("WWW-Authenticate"))
	}
	unsupported := f.mcpRequest(t, tok, "", `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"unsupported"}}`)
	if unsupported.StatusCode != 200 || !strings.Contains(string(unsupported.body), "Unsupported protocol version") {
		t.Fatalf("MCP version response=%s", unsupported.body)
	}
	init := f.mcpRequest(t, tok, "", `{"jsonrpc":"2.0","id":3,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if init.StatusCode != 200 || init.session == "" {
		t.Fatalf("MCP initialize status=%d session=%q", init.StatusCode, init.session)
	}
	budget := f.mcpRequest(t, tok, init.session, strings.Repeat("x", (1<<20)+1))
	if budget.StatusCode != 413 {
		t.Fatalf("MCP request budget status=%d body=%s", budget.StatusCode, budget.body)
	}
}

type mcpResult struct {
	StatusCode int
	session    string
	body       []byte
}

func (f *fixture) mcpRequest(t *testing.T, token, session, body string) mcpResult {
	t.Helper()
	req, err := http.NewRequest("POST", f.server.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return mcpResult{StatusCode: resp.StatusCode, session: resp.Header.Get("Mcp-Session-Id"), body: raw}
}
