//go:build integration

package wiring

import (
	"encoding/json"
	"io"
	"testing"
)

func TestM2aWiringAgainstConfiguredService(t *testing.T) {
	f := requireFixture(t)
	read := f.token(t, "q3-tenant-a", f.workA, "catalog:read")
	publish := f.token(t, "q3-tenant-a", f.workA, "catalog:publish")
	for _, tc := range []struct {
		path   string
		status int
	}{{"/healthz", 200}, {"/readyz", 200}} {
		resp := f.do(t, "GET", tc.path, "", "")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.status {
			t.Fatalf("%s status=%d body=%s", tc.path, resp.StatusCode, body)
		}
	}
	// Publish scope alone is not a read grant: policy is checked against the
	// requested action before catalog lookup/ranking.
	resp := f.do(t, "GET", "/v1/skills/q3-fixture-skill/versions/1.0.0", publish, "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("publish-only read status=%d body=%s", resp.StatusCode, body)
	}
	resp = f.do(t, "POST", "/v1/discover", read, `{"query":"fixture","max_bytes":4096}`)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("discover status=%d body=%s", resp.StatusCode, body)
	}
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &page); err != nil || len(page.Items) == 0 {
		t.Fatalf("discover body=%s", body)
	}
	resp = f.do(t, "POST", "/v1/resolve", read, `{"skill_ref":"q3-fixture-skill@1.0.0","runtime":{"id":"go","owned_connections":false},"max_bytes":4096}`)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("resolve status=%d body=%s", resp.StatusCode, body)
	}
	// The fixture skill declares no required capabilities, so every
	// requirement resolves and the stored resolution reports ready.
	var resolved struct {
		ID       string            `json:"resolution_id"`
		Status   string            `json:"status"`
		Findings []json.RawMessage `json:"findings"`
	}
	if json.Unmarshal(body, &resolved) != nil || resolved.ID == "" || resolved.Status != "ready" || len(resolved.Findings) != 0 {
		t.Fatalf("resolve body=%s", body)
	}
	// The configured loopback broker is opaque to the registry and returns no
	// credentials; both initiation and polling are real HTTP calls.
	resp = f.do(t, "POST", "/v1/connections", read, `{"capability":{"kind":"capability","id":"cap","version":"1.0.0"}}`)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("connection status=%d body=%s", resp.StatusCode, body)
	}
	var connection struct {
		ID string `json:"ID"`
	}
	if json.Unmarshal(body, &connection) != nil || connection.ID == "" {
		t.Fatalf("connection body=%s", body)
	}
}
