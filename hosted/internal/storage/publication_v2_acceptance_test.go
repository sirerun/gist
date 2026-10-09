//go:build integration

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// publicationV2RestrictedPool applies migrations in a fixture-owned database,
// then executes every publication operation after SET ROLE to a deliberately
// unprivileged role. The helper asserts the role attributes before use.
func publicationV2RestrictedPool(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	admin := searchTestPool(t)
	ctx := context.Background()
	role := "gist_pub_v2_" + time.Now().UTC().Format("150405000000")
	if _, err := admin.Exec(ctx, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` NOLOGIN NOSUPERUSER NOBYPASSRLS`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `GRANT `+pgx.Identifier{role}.Sanitize()+` TO CURRENT_USER`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`GRANT USAGE ON SCHEMA public TO ` + pgx.Identifier{role}.Sanitize(),
		`GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO ` + pgx.Identifier{role}.Sanitize(),
		`GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO ` + pgx.Identifier{role}.Sanitize(),
		`GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO ` + pgx.Identifier{role}.Sanitize(),
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	cfg := admin.Config()
	cfg.ConnConfig = admin.Config().ConnConfig.Copy()
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE `+pgx.Identifier{role}.Sanitize())
		return err
	}
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	var current string
	var super, bypass bool
	if err := runtime.QueryRow(ctx, `SELECT current_user,rolsuper,rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&current, &super, &bypass); err != nil {
		t.Fatal(err)
	}
	if current != role || super || bypass {
		t.Fatalf("restricted role assertion current=%q super=%v bypass=%v", current, super, bypass)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP OWNED BY `+pgx.Identifier{role}.Sanitize())
		_, _ = admin.Exec(context.Background(), `DROP ROLE IF EXISTS `+pgx.Identifier{role}.Sanitize())
	})
	return admin, runtime
}

func publicationV2Principal() ports.Principal {
	return ports.Principal{Issuer: "fixture", Subject: "publisher", Audience: "registry", WorkspaceID: "pub-v2-workspace", Scopes: []string{"catalog:publish"}, PolicyGeneration: 9}
}

func publicationV2Prepared(raw []byte, key string, max int64) (ports.PreparedPublication, []byte) {
	sum := sha256.Sum256(raw)
	d := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	ref := ports.ArtifactRef{Kind: ports.KindCapability, ID: "gist/test/capability", Version: "1.0.0"}
	p := ports.PreparedPublication{Ref: ref, IdempotencyKey: key, MaxBytes: max, Artifact: raw, Metadata: raw, ArtifactDigest: d, DocumentDigest: d}
	b, _ := json.Marshal(ports.PublicationReceipt{Kind: ref.Kind, ID: ref.ID, Version: ref.Version, ArtifactDigest: "sha256:" + d.Value})
	return p, b
}

func TestPublicationV2PublishReplayAliasRestartAndConflict(t *testing.T) {
	_, pool := publicationV2RestrictedPool(t)
	root := filepath.Join("..", "..", "..", "..", "..", "cache", "gist-publish-pg", "objects", t.Name())
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		t.Fatal(err)
	}
	obj, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewPublicationStore(pool, obj)
	if err != nil {
		t.Fatal(err)
	}
	p := publicationV2Principal()
	raw := []byte(`{"logical_name":"fixture capability","summary":"durable v2 acceptance"}`)
	prepared, receipt := publicationV2Prepared(raw, "first-key", 1<<20)
	fence := func(context.Context, pgx.Tx) error { return nil }
	first, err := store.Publish(context.Background(), p, prepared, receipt, fence)
	if err != nil || !first.Created {
		t.Fatalf("publish=%+v err=%v", first, err)
	}
	replay, err := store.Publish(context.Background(), p, prepared, receipt, fence)
	if err != nil || replay.Created || string(replay.Body) != string(first.Body) {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	second := prepared
	second.IdempotencyKey = "second-key"
	alias, err := store.Publish(context.Background(), p, second, receipt, fence)
	if err != nil || alias.Created || string(alias.Body) != string(first.Body) {
		t.Fatalf("alias=%+v err=%v", alias, err)
	}
	var attempts, events int
	if err := WithTenantPrincipal(context.Background(), pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1`, p.WorkspaceID).Scan(&attempts); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1`, p.WorkspaceID).Scan(&events)
	}); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || events != 1 {
		t.Fatalf("attempts=%d events=%d want 1/1", attempts, events)
	}
	changed, _ := publicationV2Prepared([]byte(`{"logical_name":"changed"}`), "first-key", 1<<20)
	if _, err := store.Publish(context.Background(), p, changed, func() []byte { _, b := publicationV2Prepared(changed.Artifact, "first-key", 1<<20); return b }(), fence); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed identity error=%v", err)
	}
	// A separately constructed store proves replay is backed by PostgreSQL and
	// the owned filesystem object, not process-local state.
	otherObj, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewPublicationStore(pool, otherObj)
	if err != nil {
		t.Fatal(err)
	}
	got, err := other.Publish(context.Background(), p, prepared, receipt, fence)
	if err != nil || got.Created {
		t.Fatalf("restart replay=%+v err=%v", got, err)
	}
	ref := prepared.Ref
	ref.WorkspaceID = p.WorkspaceID
	read, err := other.Read(context.Background(), p, ref, 1<<20, false, fence)
	if err != nil || string(read.Body) != string(raw) {
		t.Fatalf("retained metadata read=%q err=%v", read.Body, err)
	}
	catalog, err := NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := catalog.Get(context.Background(), ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: prepared.Ref.Kind, ID: prepared.Ref.ID, Version: prepared.Ref.Version})
	if err != nil || projected.ArtifactSize != int64(len(raw)) || projected.DocumentDigest != prepared.DocumentDigest || projected.ObjectKey == "" {
		t.Fatalf("catalog projection=%+v err=%v", projected, err)
	}
	if err := WithTenantPrincipal(context.Background(), pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE catalog_versions SET state='revoked',revoked_at=clock_timestamp() WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, p.WorkspaceID, prepared.Ref.Kind, prepared.Ref.ID, prepared.Ref.Version)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Publish(context.Background(), p, prepared, receipt, fence); !errors.Is(err, ErrConflict) {
		t.Fatalf("revoked replay error=%v", err)
	}
}

func TestPublicationV2BudgetAndForcedRLS(t *testing.T) {
	_, pool := publicationV2RestrictedPool(t)
	p := publicationV2Principal()
	var visible int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM publication_attempts`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("unscoped RLS exposed %d attempts", visible)
	}
	ctx := context.Background()
	if err := WithTenantPrincipal(ctx, pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO workspaces(id,name) VALUES($1,$1)`, p.WorkspaceID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	obj, err := NewObjectStore(filepath.Join("..", "..", "..", "..", "..", "cache", "gist-publish-pg", "objects", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewPublicationStore(pool, obj)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"name":"budget"}`)
	prepared, receipt := publicationV2Prepared(raw, "budget-key", int64(len(raw)))
	if _, err := store.Publish(ctx, p, prepared, receipt, func(context.Context, pgx.Tx) error { return nil }); !errors.Is(err, ErrPublicationBudget) {
		t.Fatalf("small response budget error=%v", err)
	}
	var attempts, identities int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1`, p.WorkspaceID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM publication_idempotency WHERE workspace_id=$1`, p.WorkspaceID).Scan(&identities); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || identities != 0 {
		t.Fatalf("budget failure mutated attempts=%d identities=%d", attempts, identities)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM catalog_versions WHERE workspace_id='foreign-workspace'`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("foreign workspace leaked %d rows", visible)
	}
}

func TestPublicationV2ConcurrentKeysAndOutboxRollbackRetry(t *testing.T) {
	admin, pool := publicationV2RestrictedPool(t)
	root := filepath.Join("..", "..", "..", "..", "cache", "gist-publish-pg", "objects", t.Name())
	obj, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewPublicationStore(pool, obj)
	if err != nil {
		t.Fatal(err)
	}
	p := publicationV2Principal()
	fence := func(context.Context, pgx.Tx) error { return nil }
	raw := []byte(`{"logical_name":"concurrent publication"}`)
	a, receipt := publicationV2Prepared(raw, "race-a", 1<<20)
	b, _ := publicationV2Prepared(raw, "race-b", 1<<20)
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, prepared := range []ports.PreparedPublication{a, b} {
		prepared := prepared
		go func() { <-start; _, e := store.Publish(context.Background(), p, prepared, receipt, fence); errs <- e }()
	}
	close(start)
	for range 2 {
		if e := <-errs; e != nil {
			t.Fatalf("concurrent alias publish: %v", e)
		}
	}
	var attempts, events int
	if err := WithTenantPrincipal(context.Background(), pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1`, p.WorkspaceID).Scan(&attempts); e != nil {
			return e
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1`, p.WorkspaceID).Scan(&events)
	}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || events != 1 {
		t.Fatalf("race attempts=%d events=%d want 2/1", attempts, events)
	}

	// A failing outbox insert must roll back the catalog/commit transition but
	// leave its owned staged bytes available for a later idempotent retry.
	_, err = admin.Exec(context.Background(), `CREATE FUNCTION fail_publication_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced outbox failure'; END $$`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(context.Background(), `CREATE TRIGGER fail_publication_event BEFORE INSERT ON event_outbox FOR EACH ROW EXECUTE FUNCTION fail_publication_event()`)
	if err != nil {
		t.Fatal(err)
	}
	c, cReceipt := publicationV2Prepared([]byte(`{"logical_name":"rollback fixture"}`), "rollback-key", 1<<20)
	c.Ref.ID = "gist/test/rollback"
	var rollbackReceipt ports.PublicationReceipt
	if err := json.Unmarshal(cReceipt, &rollbackReceipt); err != nil {
		t.Fatal(err)
	}
	rollbackReceipt.ID = c.Ref.ID
	cReceipt, err = json.Marshal(rollbackReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Publish(context.Background(), p, c, cReceipt, fence); err == nil {
		t.Fatal("expected forced outbox failure")
	}
	_, err = admin.Exec(context.Background(), `DROP TRIGGER fail_publication_event ON event_outbox`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(context.Background(), `DROP FUNCTION fail_publication_event()`)
	if err != nil {
		t.Fatal(err)
	}
	var staged, catalog int
	if err := WithTenantPrincipal(context.Background(), pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1 AND idempotency_key='rollback-key' AND state='staged'`, p.WorkspaceID).Scan(&staged); e != nil {
			return e
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM catalog_versions WHERE workspace_id=$1 AND artifact_id=$2`, p.WorkspaceID, c.Ref.ID).Scan(&catalog)
	}); err != nil {
		t.Fatal(err)
	}
	if staged != 1 || catalog != 0 {
		t.Fatalf("rollback staged=%d catalog=%d", staged, catalog)
	}
	result, err := store.Publish(context.Background(), p, c, cReceipt, fence)
	if err != nil || !result.Created {
		t.Fatalf("retry result=%+v err=%v", result, err)
	}
}
