//go:build integration

package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/contract"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/rest"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// TestPublicationV2 exercises an authenticated TLS publish/replay/readback
// journey against the real App composition and restricted runtime role.
func TestPublicationV2(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cacheRoot := filepath.Join("..", "..", "..", "..", "cache")
	if err := os.MkdirAll(cacheRoot, 0700); err != nil {
		t.Fatalf("create task cache root: %v", err)
	}
	objectRoot, err := os.MkdirTemp(cacheRoot, "publication-v2-objects-")
	if err != nil {
		t.Fatalf("create owned object directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(objectRoot) })
	if _, err = fixture.adminPool.Exec(ctx, `INSERT INTO namespace_reservations(prefix,workspace_id,owner_id,status) VALUES('skill',$1,$2,'active')`, compositionWorkspace, compositionSubject); err != nil {
		t.Fatalf("seed owned namespace: %v", err)
	}
	if _, err = fixture.adminPool.Exec(ctx, `INSERT INTO namespace_reservations(prefix,workspace_id,owner_id,status) VALUES('skill_',$1,$2,'active')`, compositionWorkspace, compositionSubject); err != nil {
		t.Fatalf("seed literal-underscore namespace: %v", err)
	}
	cfg := compositionConfig(fixture.runtimeURL, objectRoot, compositionSigningConfig(t))
	cfg.MaxRequestBytes = 16 << 20
	cfg.PublicationV2 = &PublicationV2Config{AllowSynthetic: true, TrustedReviewers: []TrustedPublicationReviewer{{Issuer: "fixture-issuer", Subject: "fixture-maintainer"}}, MaintenanceTargets: []MaintenanceTarget{{WorkspaceID: compositionWorkspace, Subject: compositionSubject}}}
	instance, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("start v2 app with restricted PostgreSQL role: %v", err)
	}
	t.Cleanup(func() {
		if err := instance.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown v2 app: %v", err)
		}
	})
	assertCompositionRuntimeRole(t, instance, fixture.role)
	token, err := instance.MintWorkloadToken(ctx, identity.WorkloadRequest{Subject: compositionSubject, WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read", "catalog:publish"}})
	if err != nil {
		t.Fatalf("mint fixture workload token: %v", err)
	}
	var envelopes map[string]json.RawMessage
	valid, err := os.ReadFile("../../../contracts/registry/v2/fixtures/valid.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(valid, &envelopes); err != nil {
		t.Fatal(err)
	}
	envelope := envelopes["skill"]
	var packageEnvelope struct {
		Artifact struct {
			Package struct {
				Data string `json:"data"`
			} `json:"package"`
		} `json:"artifact"`
	}
	if err = json.Unmarshal(envelope, &packageEnvelope); err != nil {
		t.Fatal(err)
	}
	archive, err := base64.StdEncoding.Strict().DecodeString(packageEnvelope.Artifact.Package.Data)
	if err != nil {
		t.Fatal(err)
	}
	evidenceRaw, err := os.ReadFile("../../../contracts/registry/v2/fixtures/admission-evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	var evidence map[string]any
	if err = json.Unmarshal(evidenceRaw, &evidence); err != nil {
		t.Fatal(err)
	}
	evidence["workspace_id"] = compositionWorkspace
	evidence["synthetic"] = true
	evidence["reviewer"] = map[string]any{"issuer": "fixture-issuer", "subject": "fixture-maintainer", "reviewed_at": "2026-10-08T00:00:00Z"}
	evidence["source"].(map[string]any)["captured_at"] = "2026-10-08T00:00:00Z"
	evidenceRaw, err = json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.adminPool.Exec(ctx, `INSERT INTO review_records(workspace_id,kind,artifact_id,version,decision,reviewer_id,evidence) VALUES($1,'skill','skill/publication-fixture','1.0.0','approved','fixture-maintainer',$2::jsonb)`, compositionWorkspace, string(evidenceRaw)); err != nil {
		t.Fatalf("seed trusted local review record: %v", err)
	}
	server := httptest.NewTLSServer(instance.Handler())
	defer server.Close()
	client := server.Client()
	post := func(key string) *http.Response {
		var bodyMap map[string]any
		if err := json.Unmarshal(envelope, &bodyMap); err != nil {
			t.Fatal(err)
		}
		bodyMap["idempotency_key"] = key
		body, err := json.Marshal(bodyMap)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/v2/publish/skill", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatalf("TLS publication request: %v", err)
		}
		return response
	}
	first := post("v2-http-first")
	firstBody, err := io.ReadAll(first.Body)
	first.Body.Close()
	if err != nil || first.StatusCode != http.StatusCreated {
		t.Fatalf("first publish status=%d body=%s read_error=%v", first.StatusCode, firstBody, err)
	}
	var receipt map[string]string
	if err = json.Unmarshal(firstBody, &receipt); err != nil {
		t.Fatalf("decode canonical receipt: %v", err)
	}
	if first.Header.Get("X-Gist-Artifact-Digest") != receipt["artifact_digest"] || first.Header.Get("X-Gist-Manifest-Digest") != receipt["manifest_digest"] || first.Header.Get("X-Gist-Package-Digest") != receipt["package_digest"] {
		t.Fatalf("publish headers do not match receipt: headers=%v receipt=%v", first.Header, receipt)
	}
	replay := post("v2-http-first")
	replayBody, err := io.ReadAll(replay.Body)
	replay.Body.Close()
	if err != nil || replay.StatusCode != http.StatusOK || !bytes.Equal(replayBody, firstBody) {
		t.Fatalf("same-key replay status=%d exact_body=%t error=%v", replay.StatusCode, bytes.Equal(replayBody, firstBody), err)
	}
	alias := post("v2-http-alias")
	aliasBody, err := io.ReadAll(alias.Body)
	alias.Body.Close()
	if err != nil || alias.StatusCode != http.StatusOK || !bytes.Equal(aliasBody, firstBody) {
		t.Fatalf("new-key exact-version alias status=%d exact_body=%t error=%v", alias.StatusCode, bytes.Equal(aliasBody, firstBody), err)
	}
	readURL := server.URL + "/v2/artifacts/skill/package?id=skill%2Fpublication-fixture&version=1.0.0&max_bytes=1048576"
	readReq, err := http.NewRequestWithContext(ctx, http.MethodGet, readURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	readReq.Header.Set("Authorization", "Bearer "+token)
	read, err := client.Do(readReq)
	if err != nil {
		t.Fatalf("TLS readback request: %v", err)
	}
	got, readErr := io.ReadAll(read.Body)
	read.Body.Close()
	want := sha256.Sum256(archive)
	if readErr != nil || read.StatusCode != http.StatusOK || !bytes.Equal(got, archive) || read.Header.Get("X-Gist-Body-Digest") != "sha256:"+hex.EncodeToString(want[:]) {
		t.Fatalf("original-byte readback status=%d exact=%t body_digest=%q error=%v", read.StatusCode, bytes.Equal(got, archive), read.Header.Get("X-Gist-Body-Digest"), readErr)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open retained skill archive: %v", err)
	}
	var manifest []byte
	for _, file := range zipReader.File {
		if file.Name == "manifest.json" {
			rc, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			manifest, e = io.ReadAll(rc)
			_ = rc.Close()
			if e != nil {
				t.Fatal(e)
			}
			break
		}
	}
	if len(manifest) == 0 {
		t.Fatal("retained archive has no manifest")
	}
	manifestURL := server.URL + "/v2/artifacts/skill?id=skill%2Fpublication-fixture&version=1.0.0&max_bytes=1048576"
	manifestReq, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	manifestReq.Header.Set("Authorization", "Bearer "+token)
	manifestResponse, err := client.Do(manifestReq)
	if err != nil {
		t.Fatalf("TLS manifest readback request: %v", err)
	}
	manifestGot, manifestErr := io.ReadAll(manifestResponse.Body)
	manifestResponse.Body.Close()
	manifestSum := sha256.Sum256(manifest)
	if manifestErr != nil || manifestResponse.StatusCode != http.StatusOK || !bytes.Equal(manifestGot, manifest) || manifestResponse.Header.Get("X-Gist-Body-Digest") != "sha256:"+hex.EncodeToString(manifestSum[:]) {
		t.Fatalf("original manifest readback status=%d exact=%t body_digest=%q error=%v", manifestResponse.StatusCode, bytes.Equal(manifestGot, manifest), manifestResponse.Header.Get("X-Gist-Body-Digest"), manifestErr)
	}
	principal := ports.Principal{Issuer: compositionIssuer, Audience: compositionIssuer, Subject: compositionSubject, WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read", "catalog:publish"}, PolicyGeneration: 1}
	for _, tc := range []struct {
		id      string
		allowed bool
	}{{"skill_/literal", true}, {"skillX/not-owned", false}, {"skillish/nearforeign", false}} {
		err := storage.WithTenantPrincipal(ctx, instance.pool, tenantPrincipal(principal), func(ctx context.Context, tx pgx.Tx) error { return currentNamespace(ctx, tx, principal, tc.id) })
		if (err == nil) != tc.allowed {
			t.Errorf("namespace %q allowed=%t error=%v; want %t", tc.id, err == nil, err, tc.allowed)
		}
	}
}

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
	storeRoot := t.TempDir()
	objects, err := storage.NewObjectStore(storeRoot)
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
	// Catalog/outbox rollback does not compensate content-addressed staging.
	// Keep this limitation explicit: blindly deleting this shared digest could
	// destroy another committed version. Canonical publication must separately
	// qualify staging ownership, retention and reconciliation before production.
	staged, err := os.ReadFile(filepath.Join(storeRoot, hex.EncodeToString(failedDigest[:])))
	if err != nil || !bytes.Equal(staged, failedArchive) {
		t.Fatalf("private rollback staging differs from documented boundary: %v", err)
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
