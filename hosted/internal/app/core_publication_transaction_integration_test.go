//go:build integration

package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/contract"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
	"github.com/sirerun/gist/hosted/internal/storage"
)

func TestPublisherCatalogAndOutboxShareTransaction(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, fixture.runtimeURL)
	if err != nil {
		t.Fatalf("open restricted application pool: %v", err)
	}
	defer pool.Close()
	catalog, err := storage.NewPostgres(pool)
	if err != nil {
		t.Fatalf("create catalog adapter: %v", err)
	}
	objects, err := storage.NewObjectStore(t.TempDir())
	if err != nil {
		t.Fatalf("create object store: %v", err)
	}
	objects.UseCatalog(catalog)
	principal := ports.Principal{
		Issuer: compositionIssuer, Audience: compositionIssuer, Subject: compositionSubject,
		WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read", "catalog:publish"}, PolicyGeneration: 1,
	}
	pub := publisher{pool: pool, objects: objects, limits: Config{MaxPackageBytes: 1 << 20, MaxExpandedBytes: 2 << 20, MaxResponseBytes: 1 << 20}}

	goodID := "core-publish-transaction-success"
	goodArchive := validCoreSkillArchive(t, goodID, "1.0.0")
	goodDigest := sha256.Sum256(goodArchive)
	goodRef := ports.ArtifactRef{WorkspaceID: compositionWorkspace, Kind: ports.KindSkill, ID: goodID, Version: "1.0.0"}
	response, err := pub.Publish(ctx, principal, ports.KindSkill, goodArchive)
	if err != nil {
		t.Fatalf("publish valid package: %v", err)
	}
	var result struct {
		Ref      ports.ArtifactRef `json:"ref"`
		Reviewer string            `json:"reviewer"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		t.Fatalf("decode publisher response: %v", err)
	}
	if result.Ref != goodRef || result.Reviewer != compositionSubject {
		t.Fatalf("unexpected publisher response: %+v", result)
	}
	assertPublicationRows(t, fixture, goodRef, hex.EncodeToString(goodDigest[:]), true)
	reader, err := objects.Open(ctx, goodRef)
	if err != nil {
		t.Fatalf("open published package: %v", err)
	}
	gotArchive, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(gotArchive, goodArchive) {
		t.Fatalf("published object does not round trip: read=%v close=%v bytes_equal=%t", readErr, closeErr, bytes.Equal(gotArchive, goodArchive))
	}

	failedID := "core-publish-transaction-rollback"
	failedArchive := validCoreSkillArchive(t, failedID, "1.0.0")
	failedDigest := sha256.Sum256(failedArchive)
	failedRef := ports.ArtifactRef{WorkspaceID: compositionWorkspace, Kind: ports.KindSkill, ID: failedID, Version: "1.0.0"}
	_, err = fixture.adminPool.Exec(ctx, `CREATE FUNCTION fail_core_publication_outbox() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN
		IF NEW.artifact_id = TG_ARGV[0] THEN
			RAISE EXCEPTION 'forced outbox insert failure';
		END IF;
		RETURN NEW;
	END $$`)
	if err != nil {
		t.Fatalf("create test outbox trigger function: %v", err)
	}
	_, err = fixture.adminPool.Exec(ctx, `CREATE TRIGGER fail_core_publication_outbox BEFORE INSERT ON event_outbox
		FOR EACH ROW EXECUTE FUNCTION fail_core_publication_outbox('`+failedID+`')`)
	if err != nil {
		t.Fatalf("create test outbox trigger: %v", err)
	}
	if _, err := pub.Publish(ctx, principal, ports.KindSkill, failedArchive); err == nil {
		t.Fatal("publisher succeeded despite forced outbox insert failure")
	}
	assertPublicationRows(t, fixture, failedRef, hex.EncodeToString(failedDigest[:]), false)
	if _, err := objects.Open(ctx, failedRef); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("failed transaction exposed an unbound object through the catalog: %v", err)
	}

	for _, tc := range []struct {
		name  string
		bytes int
	}{{name: "zero", bytes: 0}, {name: "too-small", bytes: 1}} {
		budgetID := "core-publish-budget-" + tc.name
		budgetRef := ports.ArtifactRef{WorkspaceID: compositionWorkspace, Kind: ports.KindSkill, ID: budgetID, Version: "1.0.0"}
		objectRoot := t.TempDir()
		budgetObjects, err := storage.NewObjectStore(objectRoot)
		if err != nil {
			t.Fatalf("create budget object store: %v", err)
		}
		budgetObjects.UseCatalog(catalog)
		budgetPublisher := publisher{pool: pool, objects: budgetObjects, limits: Config{MaxPackageBytes: 1 << 20, MaxExpandedBytes: 2 << 20, MaxResponseBytes: tc.bytes}}
		budgetArchive := validCoreSkillArchive(t, budgetID, "1.0.0")
		if _, err := budgetPublisher.Publish(ctx, principal, ports.KindSkill, budgetArchive); !errors.Is(err, rest.ErrBudgetExceeded) {
			t.Fatalf("response budget %d did not reject before publication: %v", tc.bytes, err)
		}
		assertPublicationRows(t, fixture, budgetRef, "", false)
		if files, err := os.ReadDir(objectRoot); err != nil || len(files) != 0 {
			t.Fatalf("budget rejection staged object bytes: files=%d err=%v", len(files), err)
		}
		if _, err := budgetObjects.Open(ctx, budgetRef); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("budget rejection exposed an object binding: %v", err)
		}
	}
}

func assertPublicationRows(t *testing.T, fixture compositionPostgres, ref ports.ArtifactRef, digest string, wantPresent bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var versionCount, eventCount int
	var gotDigest, state string
	err := fixture.adminPool.QueryRow(ctx, `SELECT count(*), COALESCE(max(digest_value), ''), COALESCE(max(state), '')
		FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`,
		ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&versionCount, &gotDigest, &state)
	if err != nil {
		t.Fatalf("query catalog publication row: %v", err)
	}
	err = fixture.adminPool.QueryRow(ctx, `SELECT count(*) FROM event_outbox
		WHERE workspace_id=$1 AND event_type='version_published' AND artifact_kind=$2 AND artifact_id=$3 AND artifact_version=$4 AND policy_generation=1`,
		ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&eventCount)
	if err != nil {
		t.Fatalf("query publication outbox row: %v", err)
	}
	if wantPresent {
		if versionCount != 1 || eventCount != 1 || gotDigest != digest || state != "published" {
			t.Fatalf("publication not committed atomically: versions=%d events=%d state=%q digest=%q want=%q", versionCount, eventCount, state, gotDigest, digest)
		}
		return
	}
	if versionCount != 0 || eventCount != 0 {
		t.Fatalf("failed publication left database state: versions=%d events=%d", versionCount, eventCount)
	}
}

func validCoreSkillArchive(t *testing.T, id, version string) []byte {
	t.Helper()
	skill := []byte("---\nname: transaction-fixture\ndescription: Verify publication transaction boundaries.\n---\nUse the controlled local operation.\n")
	sum := sha256.Sum256(skill)
	inventory := []contract.InventoryEntry{{Path: "SKILL.md", Size: int64(len(skill)), SHA256: hex.EncodeToString(sum[:]), MediaType: "text/markdown"}}
	packageDigest, _, err := contract.PackageDigest(inventory)
	if err != nil {
		t.Fatalf("compute fixture inventory digest: %v", err)
	}
	manifest, err := json.Marshal(map[string]any{
		"id": id, "version": version,
		"package_digest": map[string]string{"algorithm": "sha256", "value": packageDigest},
		"inventory":      inventory,
		"publication":    map[string]string{"status": "approved", "provenance": "fixture_asserted"},
		"trust":          "operator_asserted",
	})
	if err != nil {
		t.Fatalf("marshal fixture manifest: %v", err)
	}
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, content := range map[string][]byte{"manifest.json": manifest, "SKILL.md": skill} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create archive entry %s: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write archive entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close fixture archive: %v", err)
	}
	return b.Bytes()
}
