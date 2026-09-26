package remotemcp

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
	"net/http/httptest"
	"testing"
)

func TestMCPAndRESTShareBudgetErrorOutcome(t *testing.T) {
	h := mcpHandler(t)
	rh, _ := rest.New(rest.Services{Identity: identity{}, Authorizer: policy{}, Search: search{}})
	rr := httptest.NewRequest("POST", "/v1/discover", bytes.NewBufferString(`{"query":"x","max_bytes":1}`))
	rr.Header.Set("Authorization", "Bearer test")
	rw := httptest.NewRecorder()
	rh.ServeHTTP(rw, rr)
	if rw.Code != 413 {
		t.Fatalf("rest status=%d", rw.Code)
	}
	init := mcpRequest(t, h, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, nil)
	sid := init.Header().Get("Mcp-Session-Id")
	call := mcpRequest(t, h, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gist_discover","arguments":{"query":"x","max_bytes":1}}}`, map[string]string{"Mcp-Session-Id": sid})
	var v map[string]any
	if json.Unmarshal(call.Body.Bytes(), &v) != nil || v["error"] != nil {
		t.Fatalf("mcp=%s", call.Body.String())
	}
	if !bytes.Contains(call.Body.Bytes(), []byte("budget_exceeded")) {
		t.Fatalf("mcp body=%s", call.Body.String())
	}
}

var _ = context.Background
var _ ports.Principal
