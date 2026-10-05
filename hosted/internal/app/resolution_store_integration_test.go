//go:build integration

package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// resolutionTestPool creates a throwaway database with every migration
// applied and drops it when the test ends. A missing Postgres is a failure,
// not a skip: this test guards the production resolution store.
func resolutionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		t.Fatalf("parse postgres URL: %v", err)
	}
	adminCfg.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("connect to postgres (set GIST_DATABASE_URL): %v", err)
	}
	defer admin.Close(context.Background())
	dbName := "gist_registry_resolution_" + postgresTestSuffix(t)
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+dbName+`"`); err != nil {
		t.Fatalf("create database: %v", err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		drop, err := pgx.ConnectConfig(context.Background(), adminCfg)
		if err != nil {
			t.Errorf("connect to PostgreSQL for owned database cleanup: %v", err)
			return
		}
		defer drop.Close(context.Background())
		if _, err := drop.Exec(context.Background(), `DROP DATABASE "`+dbName+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop owned test database %q: %v", dbName, err)
		}
	})
	cfg, err := pgxpool.ParseConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = dbName
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find migrations: %v (%d files)", err, len(files))
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(file), err)
		}
	}
	return pool
}

func postgresTestSuffix(t *testing.T) string {
	t.Helper()
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("generate unique PostgreSQL test resource name: %v", err)
	}
	return hex.EncodeToString(suffix)
}

// TestPostgresResolutionStoreRoundTrip stores and reads back a resolution
// through the production store. Resolve persists every result before it
// answers, so a rejected Put surfaces to clients as a 503.
func TestPostgresResolutionStoreRoundTrip(t *testing.T) {
	pool := resolutionTestPool(t)
	ctx := context.Background()
	const workspace = "resolution-store-ws"
	if err := storage.WithTenant(ctx, pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1)`, workspace)
		return err
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	store := &postgresResolutionStore{pool: pool}
	want := ports.Resolution{
		ID:        "res-" + strings.Repeat("a", 8),
		Principal: ports.Principal{Issuer: "https://issuer.example.invalid", Subject: "agent-1", Audience: "https://registry.example.invalid", WorkspaceID: workspace, Scopes: []string{"catalog:read", "catalog:resolve"}, PolicyGeneration: 1},
		Skill:     ports.ArtifactRef{WorkspaceID: workspace, Kind: ports.KindSkill, ID: "gist/skill/asset-skill", Version: "1.0.0"},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		Findings:  []ports.Finding{{CapabilityID: "gist/document/parse@1.0.0", Status: "ready"}},
	}
	if err := store.Put(ctx, want); err != nil {
		t.Fatalf("put resolution: %v", err)
	}
	got, err := store.Get(ctx, ports.Cursor{ID: want.ID, WorkspaceID: workspace, PrincipalHash: ports.PrincipalHash(want.Principal)})
	if err != nil {
		t.Fatalf("get resolution: %v", err)
	}
	// Another principal in the same workspace, or a caller with no binding,
	// must not read the resolution even with its ID.
	other := want.Principal
	other.Subject = "agent-2"
	for name, hash := range map[string]string{"foreign principal": ports.PrincipalHash(other), "empty binding": ""} {
		if _, err := store.Get(ctx, ports.Cursor{ID: want.ID, WorkspaceID: workspace, PrincipalHash: hash}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("%s: get returned err=%v, want pgx.ErrNoRows", name, err)
		}
	}
	if got.ID != want.ID || got.Skill != want.Skill || len(got.Findings) != 1 || got.Findings[0] != want.Findings[0] {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}

	// The principal hash binds the stored resolution to the principal. It must
	// differ when subject and workspace split the same characters differently.
	var hash string
	if err := storage.WithTenant(ctx, pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT principal_hash FROM resolutions WHERE id=$1`, want.ID).Scan(&hash)
	}); err != nil {
		t.Fatalf("read principal hash: %v", err)
	}
	if hash != resolutionPrincipalHash(want.Principal) {
		t.Fatalf("stored principal hash %q is not the principal's hash", hash)
	}
	a := resolutionPrincipalHash(ports.Principal{Issuer: "i", Subject: "ab", WorkspaceID: "c"})
	b := resolutionPrincipalHash(ports.Principal{Issuer: "i", Subject: "a", WorkspaceID: "bc"})
	if a == b {
		t.Fatal("principal hash is ambiguous across the subject/workspace boundary")
	}
}

func TestPostgresPinnedResolutionRoundTripAndPrincipalBinding(t *testing.T) {
	pool := resolutionTestPool(t)
	ctx := context.Background()
	const workspace = "pinned-resolution-ws"
	if err := storage.WithTenant(ctx, pool, workspace, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1)`, workspace)
		return err
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	roleName := "gist_wire_rls_" + postgresTestSuffix(t)
	if _, err := pool.Exec(ctx, `CREATE ROLE "`+roleName+`" NOSUPERUSER NOBYPASSRLS NOLOGIN`); err != nil {
		t.Fatalf("create restricted role: %v", err)
	}
	var restrictedPool *pgxpool.Pool
	t.Cleanup(func() {
		if restrictedPool != nil {
			restrictedPool.Close()
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `DROP OWNED BY "`+roleName+`"`); err != nil {
			t.Errorf("drop owned grants for test role %q: %v", roleName, err)
			return
		}
		if _, err := pool.Exec(cleanupCtx, `REVOKE "`+roleName+`" FROM CURRENT_USER`); err != nil {
			t.Errorf("revoke owned test role %q: %v", roleName, err)
			return
		}
		if _, err := pool.Exec(cleanupCtx, `DROP ROLE "`+roleName+`"`); err != nil {
			t.Errorf("drop owned test role %q: %v", roleName, err)
		}
	})
	if _, err := pool.Exec(ctx, `GRANT "`+roleName+`" TO CURRENT_USER`); err != nil {
		t.Fatalf("grant restricted role: %v", err)
	}
	if _, err := pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO "`+roleName+`"`); err != nil {
		t.Fatalf("grant schema usage: %v", err)
	}
	if _, err := pool.Exec(ctx, `GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "`+roleName+`"`); err != nil {
		t.Fatalf("grant table privileges: %v", err)
	}
	if _, err := pool.Exec(ctx, `GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO "`+roleName+`"`); err != nil {
		t.Fatalf("grant sequence privileges: %v", err)
	}
	restrictedCfg := pool.Config().Copy()
	restrictedCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE "`+roleName+`"`)
		return err
	}
	restrictedPool, err := pgxpool.NewWithConfig(ctx, restrictedCfg)
	if err != nil {
		t.Fatalf("connect with restricted role: %v", err)
	}
	var isSuper, canBypass bool
	if err := restrictedPool.QueryRow(ctx, `SELECT rolsuper,rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&isSuper, &canBypass); err != nil {
		t.Fatal(err)
	}
	if isSuper || canBypass {
		t.Fatalf("integration role bypasses RLS: superuser=%t bypassrls=%t", isSuper, canBypass)
	}
	principal := ports.Principal{Issuer: "https://issuer.example.invalid", Subject: "agent-1", Audience: "https://registry.example.invalid", WorkspaceID: workspace, Scopes: []string{"catalog:read", "catalog:resolve"}, PolicyGeneration: 1}
	skill := ports.ArtifactRef{WorkspaceID: workspace, Kind: ports.KindSkill, ID: "gist/skill/root", Version: "1.2.0"}
	pin := func(kind ports.ArtifactKind, id, version, digest, manifest string) ports.ArtifactPin {
		return ports.ArtifactPin{Ref: ports.ArtifactRef{WorkspaceID: workspace, Kind: kind, ID: id, Version: version}, Digest: ports.Digest{Algorithm: "sha256", Value: digest}, ManifestDigest: ports.Digest{Algorithm: "sha256", Value: manifest}}
	}
	root := pin(ports.KindSkill, skill.ID, skill.Version, strings.Repeat("a", 64), strings.Repeat("b", 64))
	want := ports.PinnedResolution{ID: "pin-" + strings.Repeat("c", 8), Principal: principal, Skill: skill, SkillPin: root, ExpiresAt: time.Now().Add(time.Hour).Unix(), Findings: []ports.PinnedFinding{{CapabilityID: "gist/document/parse@1.0.0", Status: "ready", BindingRef: ports.ArtifactRef{WorkspaceID: workspace, Kind: ports.KindCapability, ID: "gist/document/parse", Version: "1.0.0"}, Required: true, Provenance: "declared", Closure: []ports.ArtifactPin{root, pin(ports.KindCapability, "gist/document/parse", "1.0.0", strings.Repeat("d", 64), strings.Repeat("e", 64)), pin(ports.KindTool, "gist/tool/parse", "2.0.0", strings.Repeat("f", 64), strings.Repeat("1", 64))}}}}
	store := &postgresResolutionStore{pool: restrictedPool}
	if err := store.PutPinnedResolution(ctx, want); err != nil {
		t.Fatalf("put pinned resolution: %v", err)
	}
	cursor := ports.Cursor{ID: want.ID, WorkspaceID: workspace, PrincipalHash: ports.PrincipalHash(principal)}
	got, err := store.GetPinnedResolution(ctx, cursor)
	if err != nil {
		t.Fatalf("get pinned resolution: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pinned round trip mismatch: got %+v want %+v", got, want)
	}
	other := principal
	other.Subject = "agent-2"
	if _, err := store.GetPinnedResolution(ctx, ports.Cursor{ID: want.ID, WorkspaceID: workspace, PrincipalHash: ports.PrincipalHash(other)}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("wrong principal returned %v, want not found", err)
	}
	if _, err := store.GetPinnedResolution(ctx, ports.Cursor{ID: want.ID, WorkspaceID: "other-workspace", PrincipalHash: ports.PrincipalHash(principal)}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("wrong workspace returned %v, want not found", err)
	}
	expired := want
	expired.ID = "expired-" + strings.Repeat("d", 8)
	expired.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	if err := store.PutPinnedResolution(ctx, expired); err != nil {
		t.Fatalf("put expired resolution: %v", err)
	}
	if _, err := store.GetPinnedResolution(ctx, ports.Cursor{ID: expired.ID, WorkspaceID: workspace, PrincipalHash: ports.PrincipalHash(principal)}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expired row returned %v, want not found", err)
	}
}
