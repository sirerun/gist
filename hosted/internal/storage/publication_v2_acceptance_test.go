//go:build integration

package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// publicationV2RestrictedPool applies migrations in a fixture-owned database,
// then executes every publication operation after SET ROLE to a deliberately
// unprivileged role. The helper asserts the role attributes before use.
func publicationV2RestrictedPool(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	base := os.Getenv("GIST_DATABASE_URL")
	if base == "" {
		t.Fatal("GIST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	adminCfg, err := pgx.ParseConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	adminCfg.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("connect to owned PostgreSQL fixture: %v", err)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	tag := hex.EncodeToString(suffix)
	dbName, role := "gist_pub_v2_"+tag, "gist_pub_v2_role_"+tag
	identDB, identRole := pgx.Identifier{dbName}.Sanitize(), pgx.Identifier{role}.Sanitize()
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+identDB); err != nil {
		t.Fatalf("create owned disposable database: %v", err)
	}
	cleanup := func() {
		_, _ = admin.Exec(context.Background(), `DROP DATABASE IF EXISTS `+identDB+` WITH (FORCE)`)
		_, _ = admin.Exec(context.Background(), `DROP ROLE IF EXISTS `+identRole)
	}
	if _, err := admin.Exec(ctx, `CREATE ROLE `+identRole+` NOLOGIN NOSUPERUSER NOBYPASSRLS`); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `GRANT `+identRole+` TO CURRENT_USER`); err != nil {
		cleanup()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(base)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = dbName
	adminDB, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		adminDB.Close()
		cleanup()
		t.Fatalf("find migrations: %v (%d files)", err, len(files))
	}
	sort.Strings(files)
	for _, file := range files {
		raw, e := os.ReadFile(file)
		if e != nil {
			adminDB.Close()
			cleanup()
			t.Fatal(e)
		}
		if _, e = adminDB.Exec(ctx, string(raw)); e != nil {
			adminDB.Close()
			cleanup()
			t.Fatalf("apply migration %s: %v", filepath.Base(file), e)
		}
	}
	for _, q := range []string{`GRANT USAGE ON SCHEMA public TO ` + identRole, `GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO ` + identRole, `GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO ` + identRole, `GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO ` + identRole} {
		if _, err := adminDB.Exec(ctx, q); err != nil {
			adminDB.Close()
			cleanup()
			t.Fatal(err)
		}
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE `+identRole)
		return err
	}
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		adminDB.Close()
		cleanup()
		t.Fatal(err)
	}
	t.Cleanup(func() { runtime.Close(); adminDB.Close(); cleanup(); _ = admin.Close(context.Background()) })
	var current string
	var super, bypass bool
	if err := runtime.QueryRow(ctx, `SELECT current_user,rolsuper,rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&current, &super, &bypass); err != nil || current != role || super || bypass {
		t.Fatalf("restricted role assertion current=%q super=%v bypass=%v err=%v", current, super, bypass, err)
	}
	var attemptsForced, identitiesForced bool
	if err := adminDB.QueryRow(ctx, `SELECT (SELECT relforcerowsecurity FROM pg_class WHERE oid='publication_attempts'::regclass),(SELECT relforcerowsecurity FROM pg_class WHERE oid='publication_idempotency'::regclass)`).Scan(&attemptsForced, &identitiesForced); err != nil || !attemptsForced || !identitiesForced {
		t.Fatalf("forced RLS attempts=%v identities=%v err=%v", attemptsForced, identitiesForced, err)
	}
	var selectPrivilege, insertPrivilege bool
	if err := adminDB.QueryRow(ctx, `SELECT has_table_privilege($1,'publication_attempts','SELECT'),has_table_privilege($1,'publication_attempts','INSERT')`, role).Scan(&selectPrivilege, &insertPrivilege); err != nil || !selectPrivilege || !insertPrivilege {
		t.Fatalf("restricted runtime table rights select=%v insert=%v err=%v", selectPrivilege, insertPrivilege, err)
	}
	return adminDB, runtime
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
	root := t.TempDir()
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
	var attempts, events, identities int
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
	obj, err := NewObjectStore(t.TempDir())
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
	root := t.TempDir()
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
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM publication_idempotency WHERE workspace_id=$1`, p.WorkspaceID).Scan(&identities); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1`, p.WorkspaceID).Scan(&events)
	}); err != nil {
		t.Fatal(err)
	}
	// A contender may see the committed version before reserving an attempt.
	// Both schedules retain two permanent identities and exactly one event.
	if attempts < 1 || attempts > 2 || identities != 2 || events != 1 {
		t.Fatalf("race attempts=%d identities=%d events=%d want 1..2/2/1", attempts, identities, events)
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

// These tests drive failed admissions through Publish so every cleanup row has
// a real durable identity and an object staged by the production path.
type publicationV2DeleteBackend struct {
	blobBackend
	mu       sync.Mutex
	fail     map[string]bool
	blockKey string
	entered  chan struct{}
	resume   chan struct{}
}

func (b *publicationV2DeleteBackend) delete(ctx context.Context, key string) error {
	b.mu.Lock()
	fail := b.fail[key]
	block := key == b.blockKey
	b.mu.Unlock()
	if block {
		close(b.entered)
		select {
		case <-b.resume:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if fail {
		return errors.New("injected exact-key delete failure")
	}
	d, ok := b.blobBackend.(interface {
		delete(context.Context, string) error
	})
	if !ok {
		return errors.New("delegate has no exact delete")
	}
	return d.delete(ctx, key)
}

func publicationV2OutboxFailure(t *testing.T, admin *pgxpool.Pool) {
	t.Helper()
	if _, err := admin.Exec(context.Background(), `CREATE FUNCTION fail_publication_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced outbox failure'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), `CREATE TRIGGER fail_publication_event BEFORE INSERT ON event_outbox FOR EACH ROW EXECUTE FUNCTION fail_publication_event()`); err != nil {
		t.Fatal(err)
	}
}
func publicationV2RemoveOutboxFailure(t *testing.T, admin *pgxpool.Pool) {
	t.Helper()
	if _, err := admin.Exec(context.Background(), `DROP TRIGGER fail_publication_event ON event_outbox`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), `DROP FUNCTION fail_publication_event()`); err != nil {
		t.Fatal(err)
	}
}
func publicationV2FailPublish(t *testing.T, store *PublicationStore, p ports.Principal, id, key string, fence PublicationFence) (ports.PreparedPublication, string) {
	t.Helper()
	raw := []byte(`{"logical_name":"` + id + `","summary":"cleanup acceptance"}`)
	prep, receipt := publicationV2Prepared(raw, key, 1<<20)
	prep.Ref.ID = id
	var r ports.PublicationReceipt
	if err := json.Unmarshal(receipt, &r); err != nil {
		t.Fatal(err)
	}
	r.ID = id
	receipt, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Publish(context.Background(), p, prep, receipt, fence); err == nil {
		t.Fatal("forced outbox failure unexpectedly published")
	}
	return prep, ""
}
func publicationV2Expired(t *testing.T, admin *pgxpool.Pool, workspace, key string) string {
	t.Helper()
	var object string
	err := admin.QueryRow(context.Background(), `UPDATE publication_attempts SET lease_until=clock_timestamp()-interval '1 second' WHERE workspace_id=$1 AND idempotency_key=$2 RETURNING object_key`, workspace, key).Scan(&object)
	if err != nil {
		t.Fatal(err)
	}
	return object
}
func publicationV2SetDue(t *testing.T, admin *pgxpool.Pool, workspace, key string) {
	t.Helper()
	if _, err := admin.Exec(context.Background(), `UPDATE publication_attempts SET lease_until=clock_timestamp()-interval '1 second',claim_until=NULL WHERE workspace_id=$1 AND idempotency_key=$2`, workspace, key); err != nil {
		t.Fatal(err)
	}
}

func TestPublicationV2RetiredRetryAndLatePut(t *testing.T) {
	admin, pool := publicationV2RestrictedPool(t)
	root := t.TempDir()
	base, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := NewPublicationStore(pool, base)
	p := publicationV2Principal()
	f := func(context.Context, pgx.Tx) error { return nil }
	publicationV2OutboxFailure(t, admin)
	prep, _ := publicationV2FailPublish(t, store, p, "gist/test/retired", "retired-key", f)
	oldKey := publicationV2Expired(t, admin, p.WorkspaceID, "retired-key")
	publicationV2RemoveOutboxFailure(t, admin)
	// Keep both namespaces as deletion canaries while the retired key is
	// revisited: one committed owned key and one legacy digest-only object.
	committedPrep, committedReceipt := publicationV2Prepared([]byte(`{"logical_name":"committed canary"}`), "committed-canary", 1<<20)
	committedPrep.Ref.ID = "gist/test/committed-canary"
	var committed ports.PublicationReceipt
	_ = json.Unmarshal(committedReceipt, &committed)
	committed.ID = committedPrep.Ref.ID
	committedReceipt, _ = json.Marshal(committed)
	if _, err := store.Publish(context.Background(), p, committedPrep, committedReceipt, f); err != nil {
		t.Fatalf("publish committed canary: %v", err)
	}
	legacy := []byte("legacy global canary")
	legacyDigest := sha256.Sum256(legacy)
	legacyHex := hex.EncodeToString(legacyDigest[:])
	if err := base.Put(context.Background(), ports.Digest{Algorithm: "sha256", Value: legacyHex}, bytes.NewReader(legacy), int64(len(legacy))); err != nil {
		t.Fatalf("stage legacy global canary: %v", err)
	}
	if err := store.Cleanup(context.Background(), p, 20, f); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := admin.QueryRow(context.Background(), `SELECT state FROM publication_attempts WHERE workspace_id=$1 AND idempotency_key='retired-key'`, p.WorkspaceID).Scan(&state); err != nil || state != "retired" {
		t.Fatalf("retired tombstone state=%q err=%v", state, err)
	}
	if err := base.StageOwned(context.Background(), oldKey, prep.Artifact, prep.ArtifactDigest); err != nil {
		t.Fatalf("simulate late backend completion: %v", err)
	}
	publicationV2SetDue(t, admin, p.WorkspaceID, "retired-key")
	if err := store.Cleanup(context.Background(), p, 20, f); err != nil {
		t.Fatal(err)
	}
	if _, err := base.blobs.get(context.Background(), oldKey); !errors.Is(err, ErrNotFound) {
		t.Fatalf("late old key remains: %v", err)
	}
	committedKey := ""
	if err := admin.QueryRow(context.Background(), `SELECT object_key FROM catalog_versions WHERE workspace_id=$1 AND artifact_id=$2`, p.WorkspaceID, committedPrep.Ref.ID).Scan(&committedKey); err != nil {
		t.Fatal(err)
	}
	if _, err := base.blobs.get(context.Background(), committedKey); err != nil {
		t.Fatalf("committed object was removed: %v", err)
	}
	if _, err := base.blobs.get(context.Background(), legacyHex); err != nil {
		t.Fatalf("legacy global object was removed: %v", err)
	}
	changed, _ := publicationV2Prepared([]byte(`{"logical_name":"changed"}`), "retired-key", 1<<20)
	changed.Ref.ID = prep.Ref.ID
	r := ports.PublicationReceipt{Kind: changed.Ref.Kind, ID: changed.Ref.ID, Version: changed.Ref.Version, ArtifactDigest: "sha256:" + changed.ArtifactDigest.Value}
	body, _ := json.Marshal(r)
	if _, err := store.Publish(context.Background(), p, changed, body, f); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed identity after retirement=%v", err)
	}
	// Fill the active-attempt quota and prove that replacing the retired
	// attempt cannot bypass the same admission bound. These rows are test-owned
	// and removed before the succeeding retry below.
	quotaIDs := make([]string, 64)
	for i := range quotaIDs {
		id := fmt.Sprintf("%032x", i+1)
		quotaIDs[i] = id
		key := "v2/" + id + "/" + strings.Repeat("c", 64)
		if _, err := admin.Exec(context.Background(), `INSERT INTO publication_attempts(workspace_id,attempt_id,kind,artifact_id,version,idempotency_key,artifact_digest,object_key,state,lease_until,artifact_size) VALUES($1,$2,'capability',$3,'1.0.0',$2,$4,$5,'staged',clock_timestamp()+interval '1 day',1)`, p.WorkspaceID, id, "quota-"+id, strings.Repeat("c", 64), key); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Publish(context.Background(), p, prep, func() []byte {
		_, b := publicationV2Prepared(prep.Artifact, "retired-key", 1<<20)
		var x ports.PublicationReceipt
		_ = json.Unmarshal(b, &x)
		x.ID = prep.Ref.ID
		b, _ = json.Marshal(x)
		return b
	}(), f); !errors.Is(err, ErrPublicationBudget) {
		t.Fatalf("retry bypassed active-attempt quota: %v", err)
	}
	if _, err := admin.Exec(context.Background(), `DELETE FROM publication_attempts WHERE workspace_id=$1 AND attempt_id=ANY($2)`, p.WorkspaceID, quotaIDs); err != nil {
		t.Fatal(err)
	}
	result, err := store.Publish(context.Background(), p, prep, func() []byte {
		_, b := publicationV2Prepared(prep.Artifact, "retired-key", 1<<20)
		var x ports.PublicationReceipt
		_ = json.Unmarshal(b, &x)
		x.ID = prep.Ref.ID
		b, _ = json.Marshal(x)
		return b
	}(), f)
	if err != nil || !result.Created {
		t.Fatalf("retired retry=%+v err=%v", result, err)
	}
	var newKey string
	if err := admin.QueryRow(context.Background(), `SELECT a.object_key FROM publication_idempotency i JOIN publication_attempts a USING(workspace_id,attempt_id) WHERE i.workspace_id=$1 AND i.kind=$2 AND i.idempotency_key='retired-key'`, p.WorkspaceID, prep.Ref.Kind).Scan(&newKey); err != nil || newKey == oldKey {
		t.Fatalf("retry key old=%q new=%q err=%v", oldKey, newKey, err)
	}
}

func TestPublicationV2CleanupFaultFairness(t *testing.T) {
	admin, pool := publicationV2RestrictedPool(t)
	root := t.TempDir()
	base, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := &publicationV2DeleteBackend{blobBackend: base.blobs, fail: map[string]bool{}}
	obj := newObjectStore(wrapper)
	store, _ := NewPublicationStore(pool, obj)
	p := publicationV2Principal()
	f := func(context.Context, pgx.Tx) error { return nil }
	publicationV2OutboxFailure(t, admin)
	keys := make([]string, 24)
	for i := range keys {
		key := fmt.Sprintf("fair-%02d", i)
		_, _ = publicationV2FailPublish(t, store, p, fmt.Sprintf("gist/test/fair-%02d", i), key, f)
		keys[i] = publicationV2Expired(t, admin, p.WorkspaceID, key)
		if i < 20 {
			wrapper.fail[key] = true
		}
	}
	publicationV2RemoveOutboxFailure(t, admin)
	for i := 0; i < 20; i++ {
		wrapper.fail[keys[i]] = true
	}
	if err := store.Cleanup(context.Background(), p, 20, f); err != nil {
		t.Fatal(err)
	}
	var retired, deleting int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FILTER(WHERE state='retired'),count(*) FILTER(WHERE state='deleting') FROM publication_attempts WHERE workspace_id=$1`, p.WorkspaceID).Scan(&retired, &deleting); err != nil {
		t.Fatal(err)
	}
	if retired != 20 || deleting != 0 {
		t.Fatalf("first bounded pass states retired=%d deleting=%d", retired, deleting)
	}
	var pending int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1 AND state='staged'`, p.WorkspaceID).Scan(&pending); err != nil || pending != 4 {
		t.Fatalf("bounded first pass left staged=%d err=%v", pending, err)
	}
	for i := 0; i < 20; i++ {
		wrapper.fail[keys[i]] = false
	}
	for i := 20; i < 24; i++ {
		publicationV2SetDue(t, admin, p.WorkspaceID, fmt.Sprintf("fair-%02d", i))
	}
	if err := store.Cleanup(context.Background(), p, 20, f); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		publicationV2SetDue(t, admin, p.WorkspaceID, fmt.Sprintf("fair-%02d", i))
	}
	if err := store.Cleanup(context.Background(), p, 20, f); err != nil {
		t.Fatal(err)
	}
	var identities, tombstones int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM publication_idempotency WHERE workspace_id=$1`, p.WorkspaceID).Scan(&identities); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1 AND state='retired'`, p.WorkspaceID).Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if identities != 24 || tombstones != 24 {
		t.Fatalf("permanent records identities=%d tombstones=%d", identities, tombstones)
	}
}

func TestPublicationV2CleanupClaimAndTenantDenials(t *testing.T) {
	admin, pool := publicationV2RestrictedPool(t)
	root := t.TempDir()
	base, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := &publicationV2DeleteBackend{blobBackend: base.blobs, fail: map[string]bool{}, entered: make(chan struct{}), resume: make(chan struct{})}
	obj := newObjectStore(wrapper)
	store, _ := NewPublicationStore(pool, obj)
	p := publicationV2Principal()
	f := func(context.Context, pgx.Tx) error { return nil }
	publicationV2OutboxFailure(t, admin)
	prep, _ := publicationV2FailPublish(t, store, p, "gist/test/claim", "claim-key", f)
	oldKey := publicationV2Expired(t, admin, p.WorkspaceID, "claim-key")
	publicationV2RemoveOutboxFailure(t, admin)
	wrapper.blockKey = oldKey
	cleanDone := make(chan error, 1)
	go func() { cleanDone <- store.Cleanup(context.Background(), p, 20, f) }()
	<-wrapper.entered
	var claimedGeneration int64
	if err := admin.QueryRow(context.Background(), `UPDATE publication_attempts SET generation=generation+1 WHERE workspace_id=$1 AND idempotency_key='claim-key' RETURNING generation`, p.WorkspaceID).Scan(&claimedGeneration); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(context.Background(), p, prep, func() []byte {
		_, b := publicationV2Prepared(prep.Artifact, "claim-key", 1<<20)
		var x ports.PublicationReceipt
		_ = json.Unmarshal(b, &x)
		x.ID = prep.Ref.ID
		b, _ = json.Marshal(x)
		return b
	}(), f); !errors.Is(err, ErrConflict) {
		t.Fatalf("publish during deleting claim=%v", err)
	}
	close(wrapper.resume)
	if err := <-cleanDone; err != nil {
		t.Fatal(err)
	}
	wrapper.blockKey = ""
	var state string
	var generation int64
	if err := admin.QueryRow(context.Background(), `SELECT state,generation FROM publication_attempts WHERE workspace_id=$1 AND idempotency_key='claim-key'`, p.WorkspaceID).Scan(&state, &generation); err != nil {
		t.Fatal(err)
	}
	if state != "deleting" || generation != claimedGeneration {
		t.Fatalf("stale cleanup changed current claim state=%q generation=%d want deleting/%d", state, generation, claimedGeneration)
	}
	// A crashed claimed deletion is resumed after the five-minute lease.
	if _, err := admin.Exec(context.Background(), `UPDATE publication_attempts SET claim_until=clock_timestamp()-interval '1 second' WHERE workspace_id=$1 AND idempotency_key='claim-key'`, p.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if err := store.Cleanup(context.Background(), p, 20, f); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(context.Background(), `SELECT state FROM publication_attempts WHERE workspace_id=$1 AND idempotency_key='claim-key'`, p.WorkspaceID).Scan(&state); err != nil || state != "retired" {
		t.Fatalf("expired claim resume state=%q err=%v", state, err)
	}
	// A currently rejecting maintenance fence must stop before object IO.
	publicationV2OutboxFailure(t, admin)
	_, _ = publicationV2FailPublish(t, store, p, "gist/test/fence", "fence-key", f)
	fenceKey := publicationV2Expired(t, admin, p.WorkspaceID, "fence-key")
	publicationV2RemoveOutboxFailure(t, admin)
	reject := func(context.Context, pgx.Tx) error { return errors.New("maintenance authority rejected") }
	if err := store.Cleanup(context.Background(), p, 20, reject); err == nil {
		t.Fatal("rejecting fence allowed cleanup")
	}
	if _, err := base.blobs.get(context.Background(), fenceKey); err != nil {
		t.Fatalf("rejecting fence performed object deletion: %v", err)
	}
	if err := store.Cleanup(context.Background(), p, 20, f); err != nil {
		t.Fatal(err)
	}
	// Foreign workspace INSERT is denied by forced RLS under the runtime role;
	// no unscoped attempt rows are visible.
	foreign := "foreign-tenant"
	err = WithTenantPrincipal(context.Background(), pool, tenantFromPrincipal(p), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO publication_attempts(workspace_id,attempt_id,kind,artifact_id,version,idempotency_key,artifact_digest,object_key,state,lease_until,artifact_size) VALUES($1,$2,'capability','x','1.0.0','x',$3,$4,'staged',clock_timestamp(),1)`, foreign, strings.Repeat("a", 32), strings.Repeat("b", 64), "v2/"+strings.Repeat("a", 32)+"/"+strings.Repeat("b", 64))
		return e
	})
	if err == nil {
		t.Fatal("foreign tenant attempt insert passed forced RLS")
	}
	var visible int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM publication_attempts`).Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("unscoped attempts=%d err=%v", visible, err)
	}
}
