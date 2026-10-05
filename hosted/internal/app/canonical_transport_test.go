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
	"github.com/sirerun/gist/hosted/internal/resolution"
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
	badRuntime := `{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"\u0000","owned_connections":true},"max_bytes":2048}`
	badRuntimeREST := httptest.NewRecorder()
	brr := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(badRuntime))
	brr.Header.Set("Authorization", "Bearer caller")
	rh.ServeHTTP(badRuntimeREST, brr)
	if badRuntimeREST.Code != 422 {
		t.Fatalf("malformed runtime REST status=%d body=%s", badRuntimeREST.Code, badRuntimeREST.Body.String())
	}
	badRuntimeMCP := mcpCall(4, "tools/call", map[string]any{"name": "gist_resolve", "arguments": json.RawMessage(badRuntime)}, sid)
	var badRuntimeRPC rpcEnvelope
	if err := json.Unmarshal(badRuntimeMCP.Body.Bytes(), &badRuntimeRPC); err != nil {
		t.Fatal(err)
	}
	if !badRuntimeRPC.Result.IsError {
		t.Fatalf("malformed runtime accepted by MCP: %s", badRuntimeMCP.Body.String())
	}
}

type transportFixtureCatalog struct {
	records  map[string]ports.CatalogRecord
	bindings []ports.CatalogRecord
}

func transportRefKey(ref ports.ArtifactRef) string {
	return string(ref.Kind) + "/" + ref.WorkspaceID + "/" + ref.ID + "@" + ref.Version
}
func (c transportFixtureCatalog) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	if r, ok := c.records[transportRefKey(ref)]; ok {
		return r, nil
	}
	return ports.CatalogRecord{}, resolution.ErrArtifactNotFound
}
func (c transportFixtureCatalog) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{Records: c.bindings}, nil
}
func transportRecord(ref ports.ArtifactRef, state string, meta any) ports.CatalogRecord {
	raw, _ := json.Marshal(meta)
	return ports.CatalogRecord{Ref: ref, State: state, Metadata: raw, Digest: ports.Digest{Algorithm: "sha256", Value: strings.Repeat("a", 64)}, ManifestDigest: ports.Digest{Algorithm: "sha256", Value: strings.Repeat("b", 64)}}
}
func transportScenarioAdapter(t *testing.T, scenario string) rest.Resolver {
	t.Helper()
	const ws = "workspace"
	skill := ports.ArtifactRef{WorkspaceID: ws, Kind: ports.KindSkill, ID: "skill/demo", Version: "1.0.0"}
	cap := ports.ArtifactRef{WorkspaceID: ws, Kind: ports.KindCapability, ID: "cap/demo", Version: "2.0.0"}
	binding := ports.ArtifactRef{WorkspaceID: ws, Kind: ports.KindBinding, ID: "binding/demo", Version: "1.0.0"}
	tool := ports.ArtifactRef{WorkspaceID: ws, Kind: ports.KindTool, ID: "tool/demo", Version: "3.1.0"}
	provider := ports.ArtifactRef{WorkspaceID: ws, Kind: ports.KindProvider, ID: "provider/demo", Version: "4.0.0"}
	toolLocation := "client"
	bindingState := "published"
	if scenario == "gateway" {
		toolLocation = "gist_gateway"
	}
	if scenario == "revoked" {
		bindingState = "revoked"
	}
	records := map[string]ports.CatalogRecord{}
	records[transportRefKey(skill)] = transportRecord(skill, "published", map[string]any{"id": skill.ID, "version": skill.Version, "required_capabilities": []any{map[string]any{"id": "cap/demo", "contract_version": "2.0.0"}}, "runtime_requirements": map[string]any{"local_execution": false}})
	records[transportRefKey(cap)] = transportRecord(cap, "published", map[string]any{"id": cap.ID, "version": cap.Version})
	records[transportRefKey(binding)] = transportRecord(binding, bindingState, map[string]any{"id": binding.ID, "version": binding.Version, "capability_ref": "cap/demo@2.0.0", "tool_ref": "tool/demo@3.1.0", "provider_ref": "provider/demo@4.0.0", "adapter_version": "adapter.actual", "conformance": "passed", "exact_versions": true, "fixture_digest": map[string]any{"algorithm": "sha256", "value": strings.Repeat("c", 64)}})
	records[transportRefKey(tool)] = transportRecord(tool, "published", map[string]any{"id": tool.ID, "version": tool.Version, "lifecycle": "published", "execution_location": toolLocation, "provider_action": map[string]any{"id": provider.ID, "version": provider.Version}, "credentials": map[string]any{"type": "none", "scopes": []any{}}})
	records[transportRefKey(provider)] = transportRecord(provider, "published", map[string]any{"support_state": "resolvable"})
	bindings := []ports.CatalogRecord{records[transportRefKey(binding)]}
	if scenario == "ambiguous" {
		b2 := binding
		b2.ID = "binding/second"
		b2.Version = "1.0.0"
		records[transportRefKey(b2)] = transportRecord(b2, "published", map[string]any{"id": b2.ID, "version": b2.Version, "capability_ref": "cap/demo@2.0.0", "tool_ref": "tool/demo@3.1.0", "provider_ref": "provider/demo@4.0.0", "adapter_version": "adapter.actual", "conformance": "passed", "exact_versions": true, "fixture_digest": map[string]any{"algorithm": "sha256", "value": strings.Repeat("c", 64)}})
		bindings = append(bindings, records[transportRefKey(b2)])
	}
	if scenario == "wrong_workspace" {
		foreign := binding
		foreign.WorkspaceID = "other-workspace"
		records[transportRefKey(foreign)] = transportRecord(foreign, "published", map[string]any{"id": foreign.ID, "version": foreign.Version, "capability_ref": "cap/demo@2.0.0"})
		bindings = []ports.CatalogRecord{records[transportRefKey(foreign)]}
	}
	resolver, err := resolution.NewResolver(transportFixtureCatalog{records: records, bindings: bindings}, canonicalAuth{}, canonicalStore{}, canonicalClock{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewCanonicalResolver(resolver, 4096)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func TestCanonicalResolveTransportFailClosedParity(t *testing.T) {
	for _, scenario := range []struct{ name, want string }{{"ambiguous", "requires_selection"}, {"wrong_workspace", "incomplete"}, {"gateway", "requires_gateway"}} {
		t.Run(scenario.name, func(t *testing.T) {
			services := rest.Services{Identity: transportIdentity{}, Authorizer: canonicalAuth{}, Resolver: transportScenarioAdapter(t, scenario.name)}
			rh, err := rest.New(services)
			if err != nil {
				t.Fatal(err)
			}
			mh, err := remotemcp.New(remotemcp.Config{Services: services, AllowedOrigins: map[string]bool{"https://client.example": true}})
			if err != nil {
				t.Fatal(err)
			}
			body := `{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}`
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer caller")
			rh.ServeHTTP(rr, req)
			if rr.Code != 200 {
				t.Fatalf("REST status=%d body=%s", rr.Code, rr.Body.String())
			}
			var restResult struct {
				Aggregate string `json:"aggregate"`
				Findings  []struct {
					Status string `json:"status"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &restResult); err != nil {
				t.Fatal(err)
			}
			if restResult.Aggregate != "incomplete" || len(restResult.Findings) != 1 || restResult.Findings[0].Status != scenario.want {
				t.Fatalf("REST result=%s", rr.Body.String())
			}
			initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
			initReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(initBody))
			initReq.Header.Set("Authorization", "Bearer caller")
			initReq.Header.Set("Origin", "https://client.example")
			initW := httptest.NewRecorder()
			mh.ServeHTTP(initW, initReq)
			sid := initW.Header().Get("Mcp-Session-Id")
			if initW.Code != 200 || sid == "" {
				t.Fatalf("MCP initialize %d %s", initW.Code, initW.Body.String())
			}
			rpcBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_resolve","arguments":{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}}}`
			mreq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(rpcBody))
			mreq.Header.Set("Authorization", "Bearer caller")
			mreq.Header.Set("Origin", "https://client.example")
			mreq.Header.Set("Mcp-Session-Id", sid)
			mw := httptest.NewRecorder()
			mh.ServeHTTP(mw, mreq)
			var rpc rpcEnvelope
			if err := json.Unmarshal(mw.Body.Bytes(), &rpc); err != nil {
				t.Fatal(err)
			}
			var mcpResult struct {
				Aggregate string `json:"aggregate"`
				Findings  []struct {
					Status string `json:"status"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(rpc.Result.Structured, &mcpResult); err != nil {
				t.Fatalf("MCP body: %v (%s)", err, mw.Body.String())
			}
			if rpc.Result.IsError || mcpResult.Aggregate != "incomplete" || len(mcpResult.Findings) != 1 || mcpResult.Findings[0].Status != scenario.want {
				t.Fatalf("MCP result=%s", mw.Body.String())
			}
		})
	}
	// Revoked pins are transport errors in both surfaces, never a successful result.
	services := rest.Services{Identity: transportIdentity{}, Authorizer: canonicalAuth{}, Resolver: transportScenarioAdapter(t, "revoked")}
	rh, _ := rest.New(services)
	mh, _ := remotemcp.New(remotemcp.Config{Services: services, AllowedOrigins: map[string]bool{"https://client.example": true}})
	body := `{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer caller")
	rh.ServeHTTP(rr, req)
	if rr.Code == 200 {
		t.Fatalf("revoked binding returned REST success: %s", rr.Body.String())
	}
	init := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`))
	init.Header.Set("Authorization", "Bearer caller")
	init.Header.Set("Origin", "https://client.example")
	iw := httptest.NewRecorder()
	mh.ServeHTTP(iw, init)
	sid := iw.Header().Get("Mcp-Session-Id")
	m := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_resolve","arguments":{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}}}`))
	m.Header.Set("Authorization", "Bearer caller")
	m.Header.Set("Origin", "https://client.example")
	m.Header.Set("Mcp-Session-Id", sid)
	mw := httptest.NewRecorder()
	mh.ServeHTTP(mw, m)
	var revoked rpcEnvelope
	if err := json.Unmarshal(mw.Body.Bytes(), &revoked); err != nil {
		t.Fatal(err)
	}
	if !revoked.Result.IsError {
		t.Fatalf("revoked binding returned MCP success: %s", mw.Body.String())
	}
}

// Duplicate canonical fields must survive both transport layers so the strict
// resolver can reject them before allocating a pinned resolution.
func TestCanonicalResolveTransportRejectsDuplicateKeys(t *testing.T) {
	services := rest.Services{Identity: transportIdentity{}, Authorizer: canonicalAuth{}, Resolver: testCanonicalAdapter(t, 4096)}
	rh, err := rest.New(services)
	if err != nil {
		t.Fatal(err)
	}
	mh, err := remotemcp.New(remotemcp.Config{Services: services, AllowedOrigins: map[string]bool{"https://client.example": true}})
	if err != nil {
		t.Fatal(err)
	}
	init := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`))
	init.Header.Set("Authorization", "Bearer caller")
	init.Header.Set("Origin", "https://client.example")
	iw := httptest.NewRecorder()
	mh.ServeHTTP(iw, init)
	sid := iw.Header().Get("Mcp-Session-Id")
	if iw.Code != 200 || sid == "" {
		t.Fatalf("initialize: %d %s", iw.Code, iw.Body.String())
	}
	cases := map[string]string{
		"skill_ref":  `{"skill_ref":"skill/demo@1.0.0","skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}`,
		"runtime_id": `{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","id":"runtime.example","owned_connections":true},"max_bytes":2048}`,
		"max_bytes":  `{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048,"max_bytes":2048}`,
	}
	for name, raw := range cases {
		t.Run(name+"/REST", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(raw))
			req.Header.Set("Authorization", "Bearer caller")
			w := httptest.NewRecorder()
			rh.ServeHTTP(w, req)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("duplicate accepted: status=%d body=%s", w.Code, w.Body.String())
			}
		})
		t.Run(name+"/MCP", func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_resolve","arguments":` + raw + `}}`
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer caller")
			req.Header.Set("Origin", "https://client.example")
			req.Header.Set("Mcp-Session-Id", sid)
			w := httptest.NewRecorder()
			mh.ServeHTTP(w, req)
			var envelope rpcEnvelope
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusOK || !envelope.Result.IsError || len(envelope.Result.Content) != 1 || !strings.Contains(envelope.Result.Content[0].Text, "validation_failed") {
				t.Fatalf("duplicate accepted: status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCanonicalResolveRESTRejectsTrailingJSON(t *testing.T) {
	h, err := rest.New(rest.Services{Identity: transportIdentity{}, Authorizer: canonicalAuth{}, Resolver: testCanonicalAdapter(t, 4096)})
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{` {}`, ` null`, ` trailing`} {
		raw := `{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example"},"max_bytes":2048}` + suffix
		req := httptest.NewRequest(http.MethodPost, "/v1/resolve", strings.NewReader(raw))
		req.Header.Set("Authorization", "Bearer caller")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("trailing JSON accepted: status=%d body=%s", w.Code, w.Body.String())
		}
	}
}
