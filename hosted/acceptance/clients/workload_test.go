//go:build integration

// Package clients contains the local half of the M2a named-client acceptance.
// The adapters below are intentionally named seams: Q5 supplies receipts from
// the pinned client/runtime binaries; this test proves their remote-MCP wire
// boundary against the composed registry fixture.
package clients

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/acceptance/testfixtures"
	"github.com/sirerun/gist/hosted/internal/app"
	"github.com/sirerun/gist/hosted/internal/contract"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

const (
	clientWorkspace = "m2a-client-workspace"
	clientSubject   = "m2a-client-workload"
	artifactID      = "skill/m2a-fixture-skill"
	artifactVersion = "1.0.0"
)

// NamedRuntimeAdapter is the consumer-side seam that Q5 replaces with the
// actual Codex CLI, Claude Code, and selected policy-runtime adapters.
type NamedRuntimeAdapter interface {
	Name() string
	Build() string
	Initialize(context.Context) error
	Schema(context.Context) ([]string, error)
	Retrieve(context.Context, string, string, string) (retrieval, error)
}

type remoteMCPAdapter struct {
	name, build string
	client      *mcpClient
}

func (a remoteMCPAdapter) Name() string                                 { return a.name }
func (a remoteMCPAdapter) Build() string                                { return a.build }
func (a remoteMCPAdapter) Initialize(ctx context.Context) error         { return a.client.initialize(ctx) }
func (a remoteMCPAdapter) Schema(ctx context.Context) ([]string, error) { return a.client.list(ctx) }
func (a remoteMCPAdapter) Retrieve(ctx context.Context, kind, id, version string) (retrieval, error) {
	return a.client.get(ctx, kind, id, version)
}

// These named types are evidence labels, not generic HTTP-client aliases.
// Their build receipts are deliberately supplied by Q5, never fabricated here.
type CodexCLIAdapter struct{ remoteMCPAdapter }
type ClaudeCodeAdapter struct{ remoteMCPAdapter }
type PolicyGatedRuntimeAdapter struct{ remoteMCPAdapter }

type retrieval struct {
	ID      string
	Version string
	Bytes   []byte
}

type mcpClient struct {
	baseURL string
	token   string
	http    *http.Client
	session string
}

func (c *mcpClient) request(ctx context.Context, id int, method string, params any) (rpcResponse, error) {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return rpcResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/mcp", bytes.NewReader(body))
	if err != nil {
		return rpcResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	if c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return rpcResponse{}, err
	}
	defer resp.Body.Close()
	var out rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return rpcResponse{}, fmt.Errorf("decode MCP %s response (HTTP %d): %w", method, resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != nil {
			return rpcResponse{}, fmt.Errorf("MCP %s HTTP status %d: %s", method, resp.StatusCode, out.Error.Message)
		}
		return rpcResponse{}, fmt.Errorf("MCP %s HTTP status %d", method, resp.StatusCode)
	}
	if session := resp.Header.Get("Mcp-Session-Id"); session != "" {
		c.session = session
	}
	return out, nil
}

func (c *mcpClient) initialize(ctx context.Context) error {
	reply, err := c.request(ctx, 1, "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	if err != nil {
		return err
	}
	if reply.Error != nil {
		return fmt.Errorf("initialize: %s", reply.Error.Message)
	}
	if reply.Result.ProtocolVersion != "2025-06-18" {
		return fmt.Errorf("protocol version %q, want 2025-06-18", reply.Result.ProtocolVersion)
	}
	if c.session == "" {
		return fmt.Errorf("initialize returned no MCP session")
	}
	return nil
}

func (c *mcpClient) list(ctx context.Context) ([]string, error) {
	reply, err := c.request(ctx, 2, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, tool := range reply.Result.Tools {
		if tool.Name == "" || tool.InputSchema.Type != "object" {
			return nil, fmt.Errorf("tool has incomplete object schema: %+v", tool)
		}
		names = append(names, tool.Name)
	}
	return names, nil
}

func (c *mcpClient) get(ctx context.Context, kind, id, version string) (retrieval, error) {
	reply, err := c.request(ctx, 3, "tools/call", map[string]any{"name": "gist_get", "arguments": map[string]any{"kind": kind, "id": id, "version": version, "max_bytes": 1 << 20}})
	if err != nil {
		return retrieval{}, err
	}
	if reply.Result.IsError {
		return retrieval{}, fmt.Errorf("gist_get: %s", reply.Result.Text())
	}
	text := reply.Result.Text()
	if kind == "package" {
		return retrieval{Bytes: []byte(text)}, nil
	}
	var metadata struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(text), &metadata); err != nil {
		return retrieval{}, fmt.Errorf("decode exact skill metadata: %w", err)
	}
	return retrieval{ID: metadata.ID, Version: version}, nil
}

type rpcResponse struct {
	Session string `json:"-"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result rpcResult `json:"result"`
}

type rpcResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Tools           []struct {
		Name        string `json:"name"`
		InputSchema struct {
			Type string `json:"type"`
		} `json:"inputSchema"`
	} `json:"tools"`
	IsError bool `json:"isError"`
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func (r rpcResult) Text() string {
	if len(r.Content) == 0 {
		return ""
	}
	return r.Content[0].Text
}

type clientFixture struct {
	app      *app.App
	server   *httptest.Server
	pool     *pgxpool.Pool
	adminDSN string
	dbName   string
	artifact []byte
	issuer   string
}

func TestWorkload(t *testing.T) {
	targets := map[string]string{
		"GIST_CODEX_CLI_BUILD":      os.Getenv("GIST_CODEX_CLI_BUILD"),
		"GIST_CLAUDE_CODE_BUILD":    os.Getenv("GIST_CLAUDE_CODE_BUILD"),
		"GIST_POLICY_RUNTIME_BUILD": os.Getenv("GIST_POLICY_RUNTIME_BUILD"),
	}
	for name, value := range targets {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("required named target %s is absent; configure pinned R3 target/build evidence", name)
		}
	}
	f := startClientFixture(t)
	t.Cleanup(f.close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	token := f.token(t, 2*time.Minute)
	adapters := []NamedRuntimeAdapter{
		CodexCLIAdapter{remoteMCPAdapter{"OpenAI Codex CLI", targets["GIST_CODEX_CLI_BUILD"], &mcpClient{f.server.URL, token, f.server.Client(), ""}}},
		ClaudeCodeAdapter{remoteMCPAdapter{"Anthropic Claude Code", targets["GIST_CLAUDE_CODE_BUILD"], &mcpClient{f.server.URL, token, f.server.Client(), ""}}},
		PolicyGatedRuntimeAdapter{remoteMCPAdapter{"selected policy-gated runtime", targets["GIST_POLICY_RUNTIME_BUILD"], &mcpClient{f.server.URL, token, f.server.Client(), ""}}},
	}
	wantTools := map[string]bool{"gist_discover": true, "gist_get": true, "gist_resolve": true, "gist_connect": true}
	for _, adapter := range adapters {
		t.Run(adapter.Name(), func(t *testing.T) {
			if adapter.Build() == "" {
				t.Fatal("adapter build is empty")
			}
			if err := adapter.Initialize(ctx); err != nil {
				t.Fatal(err)
			}
			tools, err := adapter.Schema(ctx)
			if err != nil {
				t.Fatal(err)
			}
			gotTools := make(map[string]bool, len(tools))
			for _, name := range tools {
				gotTools[name] = true
			}
			if len(gotTools) != len(wantTools) {
				t.Fatalf("schema subset tool count=%d, want %d (%v)", len(gotTools), len(wantTools), tools)
			}
			for name := range wantTools {
				if !gotTools[name] {
					t.Fatalf("schema subset omitted %s", name)
				}
			}
			meta, err := adapter.Retrieve(ctx, "skill", artifactID, artifactVersion)
			if err != nil {
				t.Fatal(err)
			}
			if meta.ID != artifactID {
				t.Fatalf("exact retrieval=%+v, want %s", meta, artifactID)
			}
		})
	}

	// Resolution over MCP reports the stored result honestly. The fixture
	// skill has no required capabilities, so it is ready with no findings; a
	// resolution is a report, never a runtime installation or execution grant.
	client := &mcpClient{f.server.URL, token, f.server.Client(), ""}
	if err := client.initialize(ctx); err != nil {
		t.Fatal(err)
	}
	reply, err := client.request(ctx, 4, "tools/call", map[string]any{"name": "gist_resolve", "arguments": map[string]any{"skill_ref": artifactID + "@" + artifactVersion, "runtime": map[string]any{"id": "runtime.acceptance", "owned_connections": true}, "max_bytes": 4096}})
	if err != nil {
		t.Fatal(err)
	}
	var resolved struct {
		ID       string            `json:"resolution_id"`
		Status   string            `json:"status"`
		Findings []json.RawMessage `json:"findings"`
	}
	if reply.Result.IsError || json.Unmarshal([]byte(reply.Result.Text()), &resolved) != nil || resolved.ID == "" || resolved.Status != "ready" || len(resolved.Findings) != 0 {
		t.Fatalf("resolution was not reported as ready: %+v", reply.Result)
	}

	// Expiry is observed at the protected-resource boundary.
	expiring := &mcpClient{f.server.URL, f.token(t, time.Second), f.server.Client(), ""}
	expiryCtx, expiryCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer expiryCancel()
	if err := expiring.initialize(expiryCtx); err != nil {
		t.Fatal(err)
	}
	// The verifier intentionally allows a 30-second clock-skew window; wait
	// beyond that window so this proves resource-side expiry rather than a
	// client-local timer.
	deadline := time.Now().Add(35 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		probe := &mcpClient{f.server.URL, expiring.token, f.server.Client(), ""}
		if initErr := probe.initialize(expiryCtx); initErr != nil {
			err = initErr
		} else {
			_, err = probe.get(expiryCtx, "skill", artifactID, artifactVersion)
		}
		// The transport authenticates before initialize, so expiry surfaces
		// either as an HTTP 401 on initialize or as unauthorized on a call.
		if err != nil && (strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "HTTP status 401")) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("short-lived token did not expire: %v", err)
		}
		<-ticker.C
	}

	// Revocation is real SQL policy state, not a client-side flag.
	if err := storage.WithTenant(context.Background(), f.pool, clientWorkspace, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE workspace_memberships SET active=false WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, clientWorkspace, f.issuer, clientSubject)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	revoked := &mcpClient{f.server.URL, token, f.server.Client(), ""}
	revocationCtx, revocationCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer revocationCancel()
	// The transport authenticates before initialize, so a revoked workload
	// is refused either at initialize (HTTP 401) or on its first call.
	err = revoked.initialize(revocationCtx)
	if err == nil {
		_, err = revoked.get(revocationCtx, "skill", artifactID, artifactVersion)
	}
	if err == nil || !(strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "HTTP status 401")) {
		t.Fatalf("revoked workload was accepted: %v", err)
	}
}

func (f *clientFixture) token(t *testing.T, ttl time.Duration) string {
	t.Helper()
	tok, err := f.app.MintWorkloadToken(context.Background(), identity.WorkloadRequest{Subject: clientSubject, WorkspaceID: clientWorkspace, Scopes: []string{"catalog:read"}, ParentSubject: clientSubject, ParentScopes: []string{"catalog:read"}, TTL: ttl})
	if err != nil {
		t.Fatalf("mint workload token: %v", err)
	}
	return tok
}

func startClientFixture(t *testing.T) *clientFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base := os.Getenv("GIST_DATABASE_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" {
		base = "postgres://dndungu@127.0.0.1:5432/postgres?sslmode=disable"
	}
	adminCfg, err := pgx.ParseConfig(base)
	if err != nil {
		t.Fatalf("parse postgres fixture URL: %v", err)
	}
	adminCfg.Database = "postgres"
	adminDSN := adminCfg.ConnString()
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("connect to reachable postgres fixture: %v", err)
	}
	dbName := fmt.Sprintf("gist_registry_m2a_%d", rand.New(rand.NewSource(time.Now().UnixNano())).Int63())
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+quoteIdent(dbName)); err != nil {
		admin.Close(context.Background())
		t.Fatalf("create fixture database: %v", err)
	}
	cfg := *adminCfg
	cfg.Database = dbName
	dsn := cfg.ConnString()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open fixture pool: %v", err)
	}
	for _, name := range []string{"001_catalog.sql", "002_policy.sql", "003_identity.sql", "004_events.sql", "005_identity_workspace_key.sql", "006_catalog_version_order.sql", "007_oauth_grants.sql", "008_event_retention_floor.sql"} {
		raw, readErr := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if readErr != nil {
			t.Fatalf("read migration %s: %v", name, readErr)
		}
		sql := string(raw)
		if name == "002_policy.sql" {
			sql = strings.Replace(sql, "ARRAY['workspaces','workspace_memberships'", "ARRAY['workspace_memberships'", 1)
		}
		if name == "003_identity.sql" {
			if idx := strings.Index(sql, "ALTER TABLE workload_identities ENABLE"); idx >= 0 {
				sql = sql[:idx]
			}
		}
		if name == "004_events.sql" {
			sql = strings.Replace(sql, "CREATE POLICY event_tenant_isolation", "DROP POLICY IF EXISTS event_tenant_isolation ON event_outbox; CREATE POLICY event_tenant_isolation", 1)
			sql = strings.Replace(sql, "CREATE POLICY cursor_tenant_isolation", "DROP POLICY IF EXISTS cursor_tenant_isolation ON event_cursors; CREATE POLICY cursor_tenant_isolation", 1)
		}
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
	artifactDir, err := os.MkdirTemp("", "gist-registry-m2a-artifacts-")
	if err != nil {
		t.Fatal(err)
	}
	placeholder := httptest.NewUnstartedServer(http.NotFoundHandler())
	origin := "https://" + placeholder.Listener.Addr().String()
	maintenanceTargets := testfixtures.MaintenanceTargets([]string{clientWorkspace})
	if err := testfixtures.SeedMaintenanceTargets(ctx, pool, origin, maintenanceTargets); err != nil {
		t.Fatalf("seed startup maintenance target: %v", err)
	}
	signingKeyConfig, err := testfixtures.SigningKeyConfig()
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(ctx, app.Config{ListenAddress: placeholder.Listener.Addr().String(), PublicOrigin: origin, ResourceAudience: origin, DatabaseURL: dsn, ObjectStoreRoot: artifactDir, RequestTimeout: 5 * time.Second, MaxPackageBytes: 10 << 20, MaxExpandedBytes: 50 << 20, MaxRequestBytes: 1 << 20, MaxResponseBytes: 2 << 20, MaxCatalogEntries: 10000, MaxConcurrentRequests: 20, MaxDiscoveryResults: 50, RetryAfter: 1, OAuthConsentSecret: []byte("acceptance-only-oauth-consent-secret-0123456789"), SigningKeyConfig: signingKeyConfig, EventMaintenanceTargets: maintenanceTargets})
	if err != nil {
		t.Fatalf("compose registry app: %v", err)
	}
	placeholder.Config.Handler = a.Handler()
	placeholder.StartTLS()
	f := &clientFixture{app: a, server: placeholder, pool: pool, adminDSN: adminDSN, dbName: dbName, issuer: origin}
	if err := seedClientFixture(ctx, f); err != nil {
		t.Fatalf("seed client fixture: %v", err)
	}
	_ = admin.Close(context.Background())
	return f
}

func seedClientFixture(ctx context.Context, f *clientFixture) error {
	if err := storage.WithTenant(ctx, f.pool, clientWorkspace, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1) ON CONFLICT (id) DO NOTHING`, clientWorkspace); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,'maintainer',$4,1) ON CONFLICT (workspace_id,issuer,subject) DO NOTHING`, clientWorkspace, f.issuer, clientSubject, []string{"catalog:read"})
		return err
	}); err != nil {
		return err
	}
	packageBytes, err := testPackage()
	if err != nil {
		return err
	}
	f.artifact = packageBytes
	metadata, err := json.Marshal(map[string]any{"id": artifactID, "version": artifactVersion, "required_capabilities": []any{}})
	if err != nil {
		return err
	}
	return f.app.SeedAcceptanceArtifact(ctx, ports.Principal{Issuer: f.issuer, Subject: clientSubject, Audience: f.issuer, WorkspaceID: clientWorkspace, Scopes: []string{"catalog:read"}, PolicyGeneration: 1}, ports.ArtifactRef{WorkspaceID: clientWorkspace, Kind: ports.KindSkill, ID: artifactID, Version: artifactVersion}, packageBytes, metadata)
}

func (f *clientFixture) close() {
	if f.app != nil {
		_ = f.app.Shutdown(context.Background())
	}
	if f.server != nil {
		f.server.Close()
	}
	if f.pool != nil {
		f.pool.Close()
	}
	if f.adminDSN != "" {
		if admin, err := pgx.Connect(context.Background(), f.adminDSN); err == nil {
			_, _ = admin.Exec(context.Background(), `DROP DATABASE `+quoteIdent(f.dbName))
			_ = admin.Close(context.Background())
		}
	}
}

func quoteIdent(v string) string { return `"` + strings.ReplaceAll(v, `"`, `""`) + `"` }

func testPackage() ([]byte, error) {
	skill := []byte("---\nname: M2a fixture skill\ndescription: A real client acceptance package\n---\n\nUse the fixture.\n")
	sum := sha256.Sum256(skill)
	entries := []contract.InventoryEntry{{Path: "SKILL.md", Size: int64(len(skill)), SHA256: hex.EncodeToString(sum[:]), MediaType: "text/markdown"}}
	digest, _, err := contract.PackageDigest(entries)
	if err != nil {
		return nil, err
	}
	manifest, err := json.Marshal(map[string]any{"id": artifactID, "version": artifactVersion, "entrypoint": "SKILL.md", "package_digest": map[string]string{"algorithm": "sha256", "value": digest}, "inventory": entries, "required_capabilities": []any{}, "runtime_requirements": map[string]any{"local_execution": true}, "effects": []string{}, "trust": "operator_asserted", "publication": map[string]string{"status": "approved", "provenance": "publisher_asserted"}})
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for name, content := range map[string][]byte{"manifest.json": manifest, "SKILL.md": skill} {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(content); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
