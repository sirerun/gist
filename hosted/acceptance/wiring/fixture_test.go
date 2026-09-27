//go:build integration

package wiring

import (
	"archive/zip"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/app"
	"github.com/sirerun/gist/hosted/internal/contract"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

type fixture struct {
	app       *app.App
	server    *httptest.Server
	broker    *httptest.Server
	pool      *pgxpool.Pool
	adminDSN  string
	dbName    string
	baseURL   string
	audience  string
	workA     string
	workB     string
	artifact  string
	cleanupMu sync.Mutex
}

var (
	activeFixture *fixture
	fixtureErr    error
)

func TestMain(m *testing.M) {
	activeFixture, fixtureErr = startFixture()
	code := m.Run()
	if activeFixture != nil {
		activeFixture.Close()
	}
	os.Exit(code)
}

func requireFixture(t *testing.T) *fixture {
	t.Helper()
	if fixtureErr != nil {
		t.Fatalf("integration fixture is required and failed to start: %v", fixtureErr)
	}
	if activeFixture == nil {
		t.Fatal("integration fixture is required but was not started")
	}
	return activeFixture
}

func startFixture() (*fixture, error) {
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
		return nil, fmt.Errorf("parse postgres fixture URL: %w", err)
	}
	adminCfg.Database = "postgres"
	adminDSN := adminCfg.ConnString()
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return nil, fmt.Errorf("connect to reachable postgres fixture: %w", err)
	}
	defer admin.Close(context.Background())
	dbName := fmt.Sprintf("gist_registry_q3_%d", mathrand.New(mathrand.NewSource(time.Now().UnixNano())).Int63())
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+quoteIdent(dbName)); err != nil {
		return nil, fmt.Errorf("create fixture database: %w", err)
	}
	cleanupDB := true
	defer func() {
		if cleanupDB {
			_, _ = admin.Exec(context.Background(), `DROP DATABASE `+quoteIdent(dbName))
		}
	}()
	// pgxpool.Config.ConnString() returns the original parsed string and
	// ignores field mutations, so the database name must be rewritten in
	// the URL itself; otherwise migrations silently land in the admin DB.
	dsn, err := fixtureDSN(adminDSN, dbName)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open fixture pool: %w", err)
	}
	var connected string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&connected); err != nil || connected != dbName {
		pool.Close()
		return nil, fmt.Errorf("fixture pool connected to %q, want %q: %v", connected, dbName, err)
	}
	for _, name := range []string{"001_catalog.sql", "002_policy.sql", "003_identity.sql", "004_events.sql", "005_identity_workspace_key.sql", "006_catalog_version_order.sql"} {
		raw, readErr := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if readErr != nil {
			pool.Close()
			return nil, fmt.Errorf("read migration %s: %w", name, readErr)
		}
		sql := string(raw)
		if name == "004_events.sql" {
			sql = strings.Replace(sql, "CREATE POLICY event_tenant_isolation", "DROP POLICY IF EXISTS event_tenant_isolation ON event_outbox; CREATE POLICY event_tenant_isolation", 1)
			sql = strings.Replace(sql, "CREATE POLICY cursor_tenant_isolation", "DROP POLICY IF EXISTS cursor_tenant_isolation ON event_cursors; CREATE POLICY cursor_tenant_isolation", 1)
		}
		if _, execErr := pool.Exec(ctx, sql); execErr != nil {
			pool.Close()
			return nil, fmt.Errorf("apply migration %s: %w", name, execErr)
		}
	}
	artifact, err := os.MkdirTemp("", "gist-registry-q3-artifacts-")
	if err != nil {
		pool.Close()
		return nil, err
	}
	// Reserve the listener first so the exact HTTPS origin can be part of the
	// issuer and audience before app.New constructs its verifier.
	placeholder := httptest.NewUnstartedServer(http.NotFoundHandler())
	origin := "https://" + placeholder.Listener.Addr().String()
	broker := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/connections" && r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"id":"broker-c1","status":"pending","capability":{"kind":"capability","id":"cap","version":"1.0.0"}}`)
			return
		}
		if r.URL.Path == "/connections/broker-c1" {
			_, _ = io.WriteString(w, `{"id":"broker-c1","status":"connected","capability":{"kind":"capability","id":"cap","version":"1.0.0"}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	appCfg := app.Config{ListenAddress: placeholder.Listener.Addr().String(), PublicOrigin: origin, ResourceAudience: origin, DatabaseURL: dsn, ObjectStoreRoot: artifact, BrokerURL: broker.URL, BrokerClient: broker.Client(), RequestTimeout: 5 * time.Second, MaxPackageBytes: 10 << 20, MaxExpandedBytes: 50 << 20, MaxRequestBytes: 1 << 20, MaxResponseBytes: 2 << 20, MaxCatalogEntries: 10000, MaxConcurrentRequests: 20, MaxDiscoveryResults: 50, RetryAfter: 1}
	a, err := app.New(ctx, appCfg)
	if err != nil {
		broker.Close()
		placeholder.Close()
		pool.Close()
		return nil, fmt.Errorf("compose registry app: %w", err)
	}
	placeholder.Config.Handler = a.Handler()
	placeholder.StartTLS()
	f := &fixture{app: a, server: placeholder, broker: broker, pool: pool, adminDSN: adminDSN, dbName: dbName, baseURL: origin, audience: origin, workA: "q3-workload-a", workB: "q3-workload-b", artifact: artifact}
	for key, value := range map[string]string{"REGISTRY_BASE_URL": origin, "REGISTRY_AUDIENCE": origin, "REGISTRY_TEST_ACCOUNT": "q3-sandbox-account", "REGISTRY_TEST_ACCOUNT_SECRET": randomSecret(), "REGISTRY_ARTIFACT_DIR": artifact} {
		_ = os.Setenv(key, value)
	}
	if err := f.seed(ctx); err != nil {
		f.Close()
		return nil, fmt.Errorf("seed fixture: %w", err)
	}
	cleanupDB = false
	return f, nil
}

func (f *fixture) seed(ctx context.Context) error {
	for _, workspace := range []string{"q3-tenant-a", "q3-tenant-b"} {
		if err := storage.WithTenant(ctx, f.pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1) ON CONFLICT (id) DO NOTHING`, workspace); err != nil {
				return err
			}
			for _, subject := range []string{f.workA, f.workB} {
				if workspace == "q3-tenant-b" && subject == f.workA {
					continue
				}
				if _, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,'maintainer',$4,1) ON CONFLICT (workspace_id,issuer,subject) DO NOTHING`, workspace, f.baseURL, subject, []string{"catalog:read", "catalog:publish"}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("seed workspace %s: %w", workspace, err)
		}
	}
	// Seed non-package records so every catalog family has a real SQL-backed
	// response. The skill package itself is published through the HTTP handler.
	if err := storage.WithTenant(ctx, f.pool, "q3-tenant-a", func(ctx context.Context, tx pgx.Tx) error {
		rows := []struct{ kind, id, version, metadata string }{
			{"capability", "cap", "1.0.0", `{"id":"cap","version":"1.0.0"}`},
			{"tool", "tool", "1.0.0", `{"id":"tool","version":"1.0.0"}`},
			{"binding", "bind", "1.0.0", `{"capability_ref":"cap@1.0.0","execution_location":"local","supported_runtimes":["go"]}`},
			{"taxonomy", "tax", "1.0.0", `{"id":"tax","attribution":{"source":"q3"}}`},
		}
		for _, row := range rows {
			if _, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,owner_id) VALUES($1,$2,$3,$4,'published','sha256',$5,'sha256',$5,$6,$7) ON CONFLICT DO NOTHING`, "q3-tenant-a", row.kind, row.id, row.version, strings.Repeat("a", 64), row.metadata, f.workA); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	packageBytes, err := testPackage()
	if err != nil {
		return err
	}
	if err := f.app.SeedAcceptanceArtifact(ctx, ports.Principal{Issuer: f.baseURL, Subject: f.workA, Audience: f.audience, WorkspaceID: "q3-tenant-a", Scopes: []string{"catalog:read", "catalog:publish"}, PolicyGeneration: 1}, ports.ArtifactRef{WorkspaceID: "q3-tenant-a", Kind: ports.KindSkill, ID: "q3-fixture-skill", Version: "1.0.0"}, packageBytes, []byte(`{"id":"q3-fixture-skill","required_capabilities":[]}`)); err != nil {
		return fmt.Errorf("seed package into composed stores: %w", err)
	}
	return nil
}

func (f *fixture) token(t *testing.T, workspace, subject string, scopes ...string) string {
	t.Helper()
	tok, err := f.app.MintWorkloadToken(context.Background(), identity.WorkloadRequest{Subject: subject, WorkspaceID: workspace, Scopes: scopes, ParentSubject: subject, ParentScopes: []string{"catalog:read", "catalog:publish"}, TTL: 2 * time.Minute})
	if err != nil {
		t.Fatalf("mint workload token: %v", err)
	}
	return tok
}

func (f *fixture) do(t *testing.T, method, path, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (f *fixture) Close() {
	f.cleanupMu.Lock()
	defer f.cleanupMu.Unlock()
	if f.app != nil {
		_ = f.app.Shutdown(context.Background())
	}
	if f.server != nil {
		f.server.Close()
	}
	if f.broker != nil {
		f.broker.Close()
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
	_ = os.RemoveAll(f.artifact)
}

func quoteIdent(v string) string { return `"` + strings.ReplaceAll(v, `"`, `""`) + `"` }
func randomSecret() string {
	b := make([]byte, 24)
	_, _ = cryptorand.Read(b)
	return hex.EncodeToString(b)
}

func testPackage() ([]byte, error) {
	skill := []byte("---\nname: Q3 fixture skill\ndescription: A real acceptance package\n---\n\nUse the fixture.\n")
	sum := sha256.Sum256(skill)
	entries := []contract.InventoryEntry{{Path: "SKILL.md", Size: int64(len(skill)), SHA256: hex.EncodeToString(sum[:]), MediaType: "text/markdown"}}
	digest, _, err := contract.PackageDigest(entries)
	if err != nil {
		return nil, err
	}
	manifest := map[string]any{"id": "q3-fixture-skill", "version": "1.0.0", "entrypoint": "SKILL.md", "package_digest": map[string]string{"algorithm": "sha256", "value": digest}, "inventory": entries, "required_capabilities": []any{}, "runtime_requirements": map[string]any{"local_execution": true}, "effects": []string{}, "trust": "operator_asserted", "publication": map[string]string{"status": "approved", "provenance": "publisher_asserted"}}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for name, content := range map[string][]byte{"manifest.json": manifestBytes, "SKILL.md": skill} {
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

// fixtureDSN rewrites the database path of a postgres URL.
func fixtureDSN(adminDSN, dbName string) (string, error) {
	u, err := url.Parse(adminDSN)
	if err != nil {
		return "", fmt.Errorf("parse admin dsn: %w", err)
	}
	u.Path = "/" + dbName
	return u.String(), nil
}
