//go:build !live

package retrieval

// Local composition target: the real app.New composition (REST handler,
// discovery, policy, resolution, Postgres storage) served over TLS in
// process, backed by a throwaway PostgreSQL database seeded with the frozen
// corpus. A missing PostgreSQL is a failure, never a skip.

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/app"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// enforceThresholds is false locally: E2 reports E1 threshold verdicts and
// guards against regressions; E3 (live) enforces them.
const enforceThresholds = false

const seedOwner = "e2-smoke-seed"

func openTarget(s frozenSuite) (*target, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := os.Getenv("GIST_DATABASE_URL")
	if base == "" {
		base = os.Getenv("DATABASE_URL")
	}
	if base == "" {
		base = "postgres://dndungu@127.0.0.1:5432/postgres?sslmode=disable"
	}
	adminURL, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse postgres URL: %w", err)
	}
	adminURL.Path = "/postgres"
	adminDSN := adminURL.String()
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres (set GIST_DATABASE_URL): %w", err)
	}
	defer admin.Close(context.Background())
	suffix := make([]byte, 6)
	if _, err := cryptorand.Read(suffix); err != nil {
		return nil, err
	}
	dbName := "gist_registry_e2_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+quoteIdent(dbName)); err != nil {
		return nil, fmt.Errorf("create smoke database: %w", err)
	}
	var cleanups []func()
	closeAll := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	cleanups = append(cleanups, func() {
		c, err := pgx.Connect(context.Background(), adminDSN)
		if err != nil {
			return
		}
		_, _ = c.Exec(context.Background(), `DROP DATABASE IF EXISTS `+quoteIdent(dbName)+` WITH (FORCE)`)
		_ = c.Close(context.Background())
	})
	fail := func(err error) (*target, error) {
		closeAll()
		return nil, err
	}
	dbURL := *adminURL
	dbURL.Path = "/" + dbName
	dsn := dbURL.String()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fail(fmt.Errorf("open smoke pool: %w", err))
	}
	cleanups = append(cleanups, pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		return fail(err)
	}
	objects, err := os.MkdirTemp("", "gist-registry-e2-objects-")
	if err != nil {
		return fail(err)
	}
	cleanups = append(cleanups, func() { _ = os.RemoveAll(objects) })

	server := httptest.NewUnstartedServer(http.NotFoundHandler())
	cleanups = append(cleanups, server.Close)
	origin := "https://" + server.Listener.Addr().String()
	cfg := app.Config{ListenAddress: server.Listener.Addr().String(), PublicOrigin: origin, ResourceAudience: origin, DatabaseURL: dsn, ObjectStoreRoot: objects, RequestTimeout: 5 * time.Second, MaxPackageBytes: 10 << 20, MaxExpandedBytes: 50 << 20, MaxRequestBytes: 1 << 20, MaxResponseBytes: 2 << 20, MaxCatalogEntries: 10000, MaxConcurrentRequests: 20, MaxDiscoveryResults: 50, RetryAfter: 1, OAuthConsentSecret: []byte("acceptance-only-oauth-consent-secret-0123456789")}
	a, err := app.New(ctx, cfg)
	if err != nil {
		return fail(fmt.Errorf("compose registry app: %w", err))
	}
	cleanups = append(cleanups, func() { _ = a.Shutdown(context.Background()) })
	server.Config.Handler = a.Handler()
	server.StartTLS()

	rows, err := seedCatalog(s)
	if err != nil {
		return fail(err)
	}
	catalog, err := catalogHash(rows)
	if err != nil {
		return fail(err)
	}
	principals := smokePrincipals(s)
	if err := seedStore(ctx, pool, origin, principals, rows); err != nil {
		return fail(fmt.Errorf("seed smoke catalog: %w", err))
	}
	tokens := map[string]string{}
	for _, p := range principals {
		tok, err := a.MintWorkloadToken(ctx, identity.WorkloadRequest{Subject: p.Subject, WorkspaceID: p.WorkspaceID, Scopes: p.Scopes, ParentSubject: p.Subject, ParentScopes: p.Scopes, TTL: 10 * time.Minute})
		if err != nil {
			return fail(fmt.Errorf("mint token for %s: %w", p.Key, err))
		}
		tokens[p.Key] = tok
	}
	// The service configuration hash excludes the random listener, database
	// name and object directory so matched local runs compare equal.
	cfgView, err := canonicalJSON(map[string]any{"target": "local-composition", "request_timeout": cfg.RequestTimeout.String(), "max_package_bytes": cfg.MaxPackageBytes, "max_expanded_bytes": cfg.MaxExpandedBytes, "max_request_bytes": cfg.MaxRequestBytes, "max_response_bytes": cfg.MaxResponseBytes, "max_catalog_entries": cfg.MaxCatalogEntries, "max_concurrent_requests": cfg.MaxConcurrentRequests, "max_discovery_results": cfg.MaxDiscoveryResults})
	if err != nil {
		return fail(err)
	}
	client := server.Client()
	client.Timeout = 10 * time.Second
	return &target{name: "local-composition", baseURL: origin, client: client, tokens: tokens, catalogHash: catalog, configHash: sha256Hex(cfgView), close: closeAll}, nil
}

type smokePrincipal struct {
	Key, Subject, WorkspaceID string
	Scopes                    []string
}

func smokePrincipals(s frozenSuite) []smokePrincipal {
	ids := map[string]string{}
	for _, w := range s.corpus.Workspaces {
		ids[w.Key] = w.ID
	}
	out := make([]smokePrincipal, 0, len(s.corpus.Principals)+1)
	for _, p := range s.corpus.Principals {
		out = append(out, smokePrincipal{Key: p.Key, Subject: p.Subject, WorkspaceID: ids[p.WorkspaceKey], Scopes: p.Scopes})
	}
	return append(out, smokePrincipal{Key: adversaryOwnerKey, Subject: "fixture-user-adversary-owner", WorkspaceID: adversaryWorkspaceID, Scopes: []string{"catalog:read"}})
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no migrations found: %v", err)
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		sql := string(raw)
		// Same idempotency guard the wiring fixture applies to 004.
		if filepath.Base(file) == "004_events.sql" {
			sql = strings.Replace(sql, "CREATE POLICY event_tenant_isolation", "DROP POLICY IF EXISTS event_tenant_isolation ON event_outbox; CREATE POLICY event_tenant_isolation", 1)
			sql = strings.Replace(sql, "CREATE POLICY cursor_tenant_isolation", "DROP POLICY IF EXISTS cursor_tenant_isolation ON event_cursors; CREATE POLICY cursor_tenant_isolation", 1)
		}
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

func seedStore(ctx context.Context, pool *pgxpool.Pool, issuer string, principals []smokePrincipal, rows []seedRow) error {
	byWorkspace := map[string][]smokePrincipal{}
	for _, p := range principals {
		byWorkspace[p.WorkspaceID] = append(byWorkspace[p.WorkspaceID], p)
	}
	for _, r := range rows {
		if _, ok := byWorkspace[r.WorkspaceID]; !ok {
			byWorkspace[r.WorkspaceID] = nil
		}
	}
	for workspace, members := range byWorkspace {
		err := storage.WithTenant(ctx, pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1)`, workspace); err != nil {
				return err
			}
			for _, p := range members {
				if _, err := tx.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,issuer,subject,role,scopes,policy_generation) VALUES($1,$2,$3,'reader',$4,1)`, workspace, issuer, p.Subject, p.Scopes); err != nil {
					return err
				}
			}
			for _, r := range rows {
				if r.WorkspaceID != workspace {
					continue
				}
				digest := sha256Hex(r.Metadata)
				if _, err := tx.Exec(ctx, `INSERT INTO catalog_versions(workspace_id,kind,artifact_id,version,state,digest_algorithm,digest_value,manifest_digest_algorithm,manifest_digest_value,metadata,owner_id) VALUES($1,$2,$3,$4,$5,'sha256',$6,'sha256',$6,$7,$8)`, workspace, r.Kind, r.ID, r.Version, r.State, digest, string(r.Metadata), seedOwner); err != nil {
					return fmt.Errorf("insert %s %s: %w", r.Kind, r.ID, err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("workspace %s: %w", workspace, err)
		}
	}
	return nil
}

func quoteIdent(v string) string { return `"` + strings.ReplaceAll(v, `"`, `""`) + `"` }
