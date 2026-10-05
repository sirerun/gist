package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/remotemcp"
	"github.com/sirerun/gist/hosted/internal/rest"
)

type transportIdentity struct{}

func (transportIdentity) Lookup(_ context.Context, token, audience string) (ports.IdentityRecord, error) {
	return ports.IdentityRecord{Issuer: "issuer", Subject: token, WorkspaceID: "workspace", Scopes: []string{"catalog:read"}}, nil
}
func (transportIdentity) Revoke(context.Context, string, string) error { return nil }

type rpcEnvelope struct {
	Result struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Structured json.RawMessage `json:"structuredContent"`
	} `json:"result"`
}

// Exercise both public transports with the exact same canonical resolver and
// authenticated principal fixture. MCP routes gist_resolve through REST, so
// this catches request/response translation drift at the actual handlers.
func TestCanonicalResolveRESTMCPTransportParity(t *testing.T) {
	adapter := testCanonicalAdapter(t, 4096)
	services := rest.Services{Identity: transportIdentity{}, Authorizer: canonicalAuth{}, Resolver: adapter}
	rh, err := rest.New(services)
	if err != nil {
		t.Fatal(err)
	}
	mh, err := remotemcp.New(remotemcp.Config{Services: services, AllowedOrigins: map[string]bool{"https://client.example": true}})
	if err != nil {
		t.Fatal(err)
	}
	request := `{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}`
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(request))
	r.Header.Set("Authorization", "Bearer caller")
	rh.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("REST status=%d body=%s", rr.Code, rr.Body.String())
	}
	var restBody map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &restBody); err != nil {
		t.Fatal(err)
	}
	if restBody["aggregate"] != "ready" || restBody["resolution_id"] == "" {
		t.Fatalf("REST canonical body=%s", rr.Body.String())
	}

	mcpCall := func(id int, method string, params any, session string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer caller")
		req.Header.Set("Origin", "https://client.example")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		w := httptest.NewRecorder()
		mh.ServeHTTP(w, req)
		return w
	}
	init := mcpCall(1, "initialize", map[string]any{"protocolVersion": "2025-06-18"}, "")
	if init.Code != 200 {
		t.Fatalf("MCP initialize=%d %s", init.Code, init.Body.String())
	}
	sid := init.Header().Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("MCP session missing")
	}
	result := mcpCall(2, "tools/call", map[string]any{"name": "gist_resolve", "arguments": json.RawMessage(request)}, sid)
	if result.Code != 200 {
		t.Fatalf("MCP transport=%d %s", result.Code, result.Body.String())
	}
	var rpc rpcEnvelope
	if err := json.Unmarshal(result.Body.Bytes(), &rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Result.IsError || len(rpc.Result.Content) != 1 {
		t.Fatalf("MCP result=%s", result.Body.String())
	}
	var mcpBody map[string]any
	if err := json.Unmarshal(rpc.Result.Structured, &mcpBody); err != nil {
		t.Fatalf("MCP structured content: %v (%s)", err, result.Body.String())
	}
	for _, key := range []string{"aggregate", "skill", "findings"} {
		if _, ok := mcpBody[key]; !ok {
			t.Fatalf("MCP response missing %q: %s", key, rpc.Result.Structured)
		}
	}
	// IDs are intentionally distinct per transport call; all frozen response fields agree.
	if mcpBody["aggregate"] != restBody["aggregate"] || mcpBody["skill"].(map[string]any)["reference"] != restBody["skill"].(map[string]any)["reference"] {
		t.Fatalf("REST/MCP response differs: REST=%s MCP=%s", rr.Body.String(), rpc.Result.Structured)
	}

	// Both routes map malformed canonical request fields to their transport's
	// validation error shape without ever invoking a provider.
	bad := `{"SKILL_REF":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}`
	badREST := httptest.NewRecorder()
	br := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(bad))
	br.Header.Set("Authorization", "Bearer caller")
	rh.ServeHTTP(badREST, br)
	if badREST.Code != 422 {
		t.Fatalf("REST malformed status=%d body=%s", badREST.Code, badREST.Body.String())
	}
	badMCP := mcpCall(3, "tools/call", map[string]any{"name": "gist_resolve", "arguments": json.RawMessage(bad)}, sid)
	var badRPC rpcEnvelope
	if err := json.Unmarshal(badMCP.Body.Bytes(), &badRPC); err != nil {
		t.Fatal(err)
	}
	if !badRPC.Result.IsError || len(badRPC.Result.Content) != 1 || !strings.Contains(badRPC.Result.Content[0].Text, "validation_failed") {
		t.Fatalf("MCP malformed response=%s", badMCP.Body.String())
	}
}
