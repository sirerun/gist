//go:build integration

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationV2BudgetsAndGrammar(t *testing.T) {
	h := startMatrixHarness(t, func(c *Config) { c.MaxRequestBytes = 16 << 20 })
	a := h.artifacts["skill"]
	type dbCounts struct{ versions, attempts, ledger, events int }
	count := func() dbCounts {
		t.Helper()
		var versions, attempts, ledger, events int
		for query, dst := range map[string]*int{
			"SELECT count(*) FROM catalog_versions":        &versions,
			"SELECT count(*) FROM publication_attempts":    &attempts,
			"SELECT count(*) FROM publication_idempotency": &ledger,
			"SELECT count(*) FROM event_outbox":            &events,
		} {
			if err := h.fixture.adminPool.QueryRow(h.ctx, query).Scan(dst); err != nil {
				t.Fatal(err)
			}
		}
		return dbCounts{versions, attempts, ledger, events}
	}
	before := count()
	mutate := func(raw []byte, field, value string) []byte {
		t.Helper()
		var root map[string]json.RawMessage
		if err := json.Unmarshal(raw, &root); err != nil {
			t.Fatal(err)
		}
		root[field] = json.RawMessage(value)
		b, err := json.Marshal(root)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, tc := range []struct {
		name, value string
		want        int
	}{
		{"one", "1", 413}, {"zero", "0", 422}, {"overflow", "9223372036854775808", 422}, {"fraction", "1.5", 422},
	} {
		t.Run("budget_"+tc.name, func(t *testing.T) {
			body := mutate(a.envelope, "max_bytes", tc.value)
			status, _, got := h.request(t, http.MethodPost, "/v2/publish/skill", body)
			if status != tc.want {
				t.Fatalf("status=%d want=%d body=%s", status, tc.want, got)
			}
		})
	}
	if after := count(); after != before {
		t.Fatalf("budget rejection wrote publication state: before=%v after=%v", before, after)
	}
	bad := map[string][]byte{
		"duplicate_key":     []byte(strings.Replace(string(a.envelope), `"max_bytes":1048576`, `"max_bytes":1048576,"max_bytes":1048576`, 1)),
		"escaped_duplicate": []byte(strings.Replace(string(a.envelope), `"max_bytes":1048576`, `"max_bytes":1048576,"max_\u0062ytes":1048576`, 1)),
		"nested_duplicate":  []byte(strings.Replace(string(a.envelope), `"artifact":`, `"artifact":{"id":"x","id":"y"},"discarded_artifact":`, 1)),
		"invalid_utf8":      append(append([]byte(nil), a.envelope...), 0xff),
		"invalid_surrogate": []byte(strings.Replace(string(a.envelope), `"idempotency_key":`, `"idempotency_key":"bad\uD800","discarded":`, 1)),
		"trailing_json":     append(append([]byte(nil), a.envelope...), []byte(` {}`)...),
		"unknown_field":     mutate(a.envelope, "unexpected", `true`),
	}
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			beforeCase := count()
			// Isolate each parser case under its own key so an incorrectly
			// accepted duplicate cannot turn a later request into a replay.
			body = bytes.Replace(body, []byte("matrix-skill"), []byte("matrix-skill-"+name), 1)
			status, _, _ := h.request(t, http.MethodPost, "/v2/publish/skill", body)
			if status < 400 || status >= 500 {
				t.Fatalf("malformed envelope status=%d", status)
			}
			if after := count(); after != beforeCase {
				t.Fatalf("malformed request wrote state: before=%v after=%v", beforeCase, after)
			}
		})
	}
	// A requested one-byte error response must remain independently bounded.
	body := mutate(a.envelope, "max_bytes", "1")
	status, _, got := h.request(t, http.MethodPost, "/v2/publish/skill", body)
	if status != 413 || len(got) > 16<<10 {
		t.Fatalf("tiny-budget status=%d body-bytes=%d", status, len(got))
	}

	// Query parsing must reject malformed and repeated read parameters.
	for _, suffix := range []string{"?id=x&version=1&max_bytes=bad", "?id=x&id=y&version=1&max_bytes=1048576", "?id=x&version=1&version=2&max_bytes=1048576", "?id=x&version=1&max_bytes=1048576&max_bytes=1048576"} {
		status, _, _ := h.request(t, http.MethodGet, "/v2/artifacts/skill"+suffix, nil)
		if status < 400 || status >= 500 {
			t.Errorf("query %s status=%d", suffix, status)
		}
	}
}

func TestPublicationV2ReplicaReplayAndRead(t *testing.T) {
	h := startMatrixHarness(t, nil)
	a := h.artifacts["skill"]
	receipt := h.publish(t, "skill")
	// Capture the durable object key through the restricted runtime catalog.
	var key string
	if err := h.fixture.adminPool.QueryRow(h.ctx, "SELECT object_key FROM catalog_versions WHERE workspace_id=$1 AND kind='skill' AND artifact_id=$2 AND version=$3", compositionWorkspace, a.prepared.Ref.ID, a.prepared.Ref.Version).Scan(&key); err != nil {
		t.Fatal(err)
	}
	root := h.instance.cfg.ObjectStoreRoot
	object := filepath.Join(root, filepath.FromSlash(key))
	rel, err := filepath.Rel(root, object)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("catalog object key escapes owned object root: %q", key)
	}
	got, err := os.ReadFile(object)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, a.prepared.Artifact) {
		t.Fatalf("stored object differs from original publication bytes")
	}
	// A separately constructed App uses the same restricted DB and object root.
	cfg := h.instance.cfg
	second, err := New(h.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Shutdown(h.ctx)
	assertCompositionRuntimeRole(t, second, h.fixture.role)
	server := httptest.NewTLSServer(second.Handler())
	t.Cleanup(server.Close)
	req := func(method, path string, body []byte) (int, []byte) {
		t.Helper()
		r, e := http.NewRequestWithContext(h.ctx, method, server.URL+path, bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		r.Header.Set("Authorization", "Bearer "+h.token)
		if method == http.MethodPost {
			r.Header.Set("Content-Type", "application/json")
		}
		resp, e := server.Client().Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		b, e := io.ReadAll(resp.Body)
		if e != nil {
			t.Fatal(e)
		}
		return resp.StatusCode, b
	}
	status, replay := req(http.MethodPost, "/v2/publish/skill", a.envelope)
	if status != 200 || !bytes.Equal(replay, receipt) {
		t.Fatalf("replica replay status=%d exact=%v", status, bytes.Equal(replay, receipt))
	}
	q := "?id=" + url.QueryEscape(a.prepared.Ref.ID) + "&version=" + url.QueryEscape(a.prepared.Ref.Version) + "&max_bytes=1048576"
	status, raw := req(http.MethodGet, "/v2/artifacts/skill"+q, nil)
	if status != 200 || !bytes.Equal(raw, a.prepared.Metadata) {
		t.Fatalf("replica metadata status=%d exact=%v", status, bytes.Equal(raw, a.prepared.Metadata))
	}
	status, zip := req(http.MethodGet, "/v2/artifacts/skill/package"+q, nil)
	if status != 200 || !bytes.Equal(zip, a.prepared.Artifact) {
		t.Fatalf("replica package status=%d exact=%v", status, bytes.Equal(zip, a.prepared.Artifact))
	}
	// Read budgets are exact boundaries: full length succeeds, one byte less
	// is denied without returning a truncated success body.
	for _, tc := range []struct {
		name, path string
		body       []byte
	}{
		{"metadata", "/v2/artifacts/skill", a.prepared.Metadata},
		{"package", "/v2/artifacts/skill/package", a.prepared.Artifact},
	} {
		t.Run("read_budget_"+tc.name, func(t *testing.T) {
			full := q[:strings.LastIndex(q, "max_bytes=")+len("max_bytes=")] + fmt.Sprint(len(tc.body))
			status, b := req(http.MethodGet, tc.path+full, nil)
			if status != 200 || !bytes.Equal(b, tc.body) {
				t.Fatalf("exact length status=%d exact=%v", status, bytes.Equal(b, tc.body))
			}
			short := q[:strings.LastIndex(q, "max_bytes=")+len("max_bytes=")] + fmt.Sprint(len(tc.body)-1)
			status, b = req(http.MethodGet, tc.path+short, nil)
			if status < 400 || status >= 500 || bytes.Equal(b, tc.body) {
				t.Fatalf("short length status=%d returnedFullBody=%v", status, bytes.Equal(b, tc.body))
			}
		})
	}

	// Replaying with a different client budget retains the request identity.
	largeBudget := []byte(strings.Replace(string(a.envelope), `"max_bytes":1048576`, `"max_bytes":9223372036854775807`, 1))
	status, again := req(http.MethodPost, "/v2/publish/skill", largeBudget)
	if status != 200 || !bytes.Equal(again, receipt) {
		t.Fatalf("clamped-budget replay status=%d exact=%v", status, bytes.Equal(again, receipt))
	}
	// Use a document artifact with an explicit outer version field so this
	// changes request identity without reserializing or altering raw document bytes.
	versionArtifact := h.artifacts["capability"]
	versionReceipt := h.publish(t, "capability")
	_ = versionReceipt
	versionAt := bytes.Index(versionArtifact.envelope, []byte(versionArtifact.prepared.Ref.Version))
	if versionAt < 0 {
		t.Fatal("outer artifact version missing from capability envelope")
	}
	changed := append(append(append([]byte(nil), versionArtifact.envelope[:versionAt]...), []byte("9.9.9")...), versionArtifact.envelope[versionAt+len(versionArtifact.prepared.Ref.Version):]...)
	status, _ = req(http.MethodPost, "/v2/publish/capability", changed)
	if status != http.StatusConflict {
		t.Fatalf("changed-version replay status=%d want=409", status)
	}

	// Deny physical-object loss and corruption without reporting a truncated
	// success. The key came from the isolated fixture's owned catalog row.
	if err := os.Remove(object); err != nil {
		t.Fatal(err)
	}
	status, b := req(http.MethodGet, "/v2/artifacts/skill/package"+q, nil)
	if status < 400 || status >= 500 || bytes.Equal(b, a.prepared.Artifact) {
		t.Fatalf("missing object status=%d returnedOriginal=%v", status, bytes.Equal(b, a.prepared.Artifact))
	}
	if err := os.WriteFile(object, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	status, b = req(http.MethodGet, "/v2/artifacts/skill/package"+q, nil)
	if status < 400 || status >= 500 || bytes.Equal(b, []byte("corrupt")) {
		t.Fatalf("corrupt object status=%d body=%q", status, b)
	}

	// Revocation through the owned administrator is visible to the fresh app.
	if _, err := h.fixture.adminPool.Exec(h.ctx, "UPDATE catalog_versions SET state='revoked' WHERE workspace_id=$1 AND kind='skill' AND artifact_id=$2 AND version=$3", compositionWorkspace, a.prepared.Ref.ID, a.prepared.Ref.Version); err != nil {
		t.Fatal(err)
	}
	status, _ = req(http.MethodPost, "/v2/publish/skill", a.envelope)
	if status < 400 || status >= 500 {
		t.Fatalf("revoked replay status=%d", status)
	}
	status, _ = req(http.MethodGet, "/v2/artifacts/skill"+q, nil)
	if status < 400 || status >= 500 {
		t.Fatalf("revoked read status=%d", status)
	}
}

func TestPublicationV2ConfiguredPackageCaps(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*Config)
	}{
		{"request", func(c *Config) { c.MaxRequestBytes = 128 }},
		{"package", func(c *Config) { c.MaxPackageBytes = 1 }},
		{"expanded", func(c *Config) { c.MaxExpandedBytes = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := startMatrixHarness(t, tc.configure)
			a := h.artifacts["skill"]
			status, _, body := h.request(t, http.MethodPost, "/v2/publish/skill", a.envelope)
			if status < 400 || status >= 500 {
				t.Fatalf("over-cap package status=%d body=%s", status, body)
			}
			for _, query := range []string{"SELECT count(*) FROM catalog_versions", "SELECT count(*) FROM publication_attempts", "SELECT count(*) FROM publication_idempotency", "SELECT count(*) FROM event_outbox"} {
				var n int
				if err := h.fixture.adminPool.QueryRow(h.ctx, query).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n != 0 {
					t.Fatalf("over-cap rejection wrote state in %s: %d", query, n)
				}
			}
		})
	}
}
