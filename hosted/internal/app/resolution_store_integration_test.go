//go:build integration

package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
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
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	dbName := "gist_registry_resolution_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+dbName+`"`); err != nil {
		t.Fatalf("create database: %v", err)
	}
	cfg, err := pgxpool.ParseConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		drop, err := pgx.ConnectConfig(context.Background(), adminCfg)
		if err != nil {
			return
		}
		defer drop.Close(context.Background())
		_, _ = drop.Exec(context.Background(), `DROP DATABASE IF EXISTS "`+dbName+`" WITH (FORCE)`)
	})
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
