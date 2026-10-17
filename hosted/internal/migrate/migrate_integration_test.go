//go:build integration

package migrate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sirerun/gist/hosted/migrations"
)

func migrationDatabase(t *testing.T) (*pgx.Conn, func() *pgx.Conn) {
	t.Helper()
	raw := os.Getenv("GIST_DATABASE_URL")
	if raw == "" {
		t.Fatal("GIST_DATABASE_URL required")
	}
	cfg, err := pgx.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid fixture database URL")
	}
	cfg.Database = "postgres"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("fixture administration unavailable")
	}
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	name := "gist_migration_" + hex.EncodeToString(id)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal("cannot create owned fixture database")
	}
	cfg = cfg.Copy()
	cfg.Database = name
	var opened []*pgx.Conn
	var mu sync.Mutex
	connect := func() *pgx.Conn {
		t.Helper()
		conn, err := pgx.ConnectConfig(context.Background(), cfg.Copy())
		if err != nil {
			t.Fatal("cannot connect owned fixture")
		}
		mu.Lock()
		opened = append(opened, conn)
		mu.Unlock()
		return conn
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, conn := range opened {
			_ = conn.Close(ctx)
		}
		if _, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Error("cannot remove owned fixture database")
		}
		_ = admin.Close(ctx)
	})
	return connect(), connect
}
func TestMigrationFreshApplyAndConcurrentReplay(t *testing.T) {
	conn, connect := migrationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r, err := Load(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	before, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if before.LedgerExists || len(before.Applied) != 0 || len(before.Pending) != 10 {
		t.Fatal("status mutated empty database")
	}
	// Two release processes serialize; the second verifies and replays no SQL.
	other := connect()
	errs := make(chan error, 2)
	go func() { errs <- r.Apply(ctx, conn) }()
	go func() { errs <- r.Apply(ctx, other) }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	status, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Applied) != 10 || len(status.Pending) != 0 {
		t.Fatal("incomplete migration ledger")
	}
	if status.SchemaSHA256 == before.SchemaSHA256 {
		t.Fatal("schema unchanged after real migrations")
	}
	if _, err := conn.Exec(ctx, `UPDATE public.registry_schema_migrations SET sha256=repeat('0',64) WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if r.Apply(ctx, conn) == nil {
		t.Fatal("drifted ledger accepted")
	}
}
func TestMigrationRollbackRetainsNoPartialDDL(t *testing.T) {
	conn, _ := migrationDatabase(t)
	ctx := context.Background()
	r, err := Load(fstest.MapFS{"001_failure.sql": {Data: []byte("CREATE TABLE public.must_rollback(id integer); SELECT 1/0;")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(ctx, conn); err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error=%v", err)
	}
	status, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if status.LedgerExists || len(status.Relations) != 0 {
		t.Fatal("failed SQL or ledger escaped transaction")
	}
}
func TestMigrationExplicitAdoptionAndSchemaFingerprint(t *testing.T) {
	conn, _ := migrationDatabase(t)
	ctx := context.Background()
	r, err := Load(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range r.Migrations[:2] {
		if _, err := conn.Exec(ctx, migration.SQL); err != nil {
			t.Fatal("cannot construct reviewed legacy prefix")
		}
	}
	if r.Apply(ctx, conn) == nil {
		t.Fatal("auto-baselined legacy schema")
	}
	status, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if r.Adopt(ctx, conn, 2, status.ManifestSHA256, strings.Repeat("0", 64)) == nil {
		t.Fatal("schema mismatch adopted")
	}
	if r.Adopt(ctx, conn, 2, strings.Repeat("0", 64), status.SchemaSHA256) == nil {
		t.Fatal("manifest mismatch adopted")
	}
	if _, err := conn.Exec(ctx, `INSERT INTO public.workspaces(id,name) VALUES('fixture','private row value')`); err != nil {
		t.Fatal(err)
	}
	same, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if same.SchemaSHA256 != status.SchemaSHA256 {
		t.Fatal("row data affected schema fingerprint")
	}
	if err := r.Adopt(ctx, conn, 2, status.ManifestSHA256, status.SchemaSHA256); err != nil {
		t.Fatal(err)
	}
	adopted, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(adopted.Applied) != 2 || !adopted.Applied[0].Adopted || adopted.SchemaSHA256 != status.SchemaSHA256 {
		t.Fatal("incorrect adopted prefix or ledger contaminated fingerprint")
	}
	if r.Adopt(ctx, conn, 2, status.ManifestSHA256, status.SchemaSHA256) == nil {
		t.Fatal("adopted existing ledger again")
	}
	if err := r.Apply(ctx, conn); err != nil {
		t.Fatal(err)
	}
	final, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(final.Applied) != 10 {
		t.Fatal("did not migrate suffix")
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE public.workspaces ADD COLUMN fixture_drift boolean`); err != nil {
		t.Fatal(err)
	}
	changed, err := r.Status(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if changed.SchemaSHA256 == final.SchemaSHA256 {
		t.Fatal("DDL drift not fingerprinted")
	}
}
