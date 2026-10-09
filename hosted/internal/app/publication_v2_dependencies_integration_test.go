//go:build integration

package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/contract"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/publicationv2"
	"github.com/sirerun/gist/hosted/internal/storage"
)

func TestPublicationV2BindingDenials(t *testing.T) {
	t.Run("revoked current dependency", func(t *testing.T) {
		h := startMatrixHarness(t, nil)
		for _, kind := range []string{"capability", "tool", "provider"} {
			h.publish(t, kind)
		}
		ref := h.artifacts["capability"].prepared.Ref
		catalog, err := storage.NewPostgres(h.instance.pool)
		if err != nil {
			t.Fatal(err)
		}
		ref.WorkspaceID = compositionWorkspace
		principal := ports.Principal{Issuer: compositionIssuer, Subject: compositionSubject, Audience: compositionIssuer, WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read", "catalog:publish"}, PolicyGeneration: 1, SubjectType: "workload"}
		if _, err := catalog.RevokeVersionForPrincipal(h.ctx, principal, ref); err != nil {
			t.Fatalf("revoke current dependency through storage revoker: %v", err)
		}
		current, err := catalog.Get(h.ctx, ref)
		if !errors.Is(err, storage.ErrRevoked) && (err != nil || current.State != "revoked") {
			t.Fatalf("dependency did not become revoked: state=%q err=%v", current.State, err)
		}
		status, _, body := h.request(t, http.MethodPost, "/v2/publish/binding", h.artifacts["binding"].envelope)
		if status < 400 || status >= 500 {
			t.Fatalf("revoked binding dependency status=%d body=%s", status, body)
		}
		assertPublicationRows(t, h.fixture, h.artifacts["binding"].prepared.Ref, h.artifacts["binding"].prepared.ArtifactDigest.Value, false)
		assertNoPublicationAttempt(t, h, h.artifacts["binding"].prepared.Ref)
	})
	t.Run("missing verifier", func(t *testing.T) {
		h := startMatrixHarness(t, func(c *Config) { c.PublicationV2.BindingVerifier = nil })
		for _, kind := range []string{"capability", "tool", "provider"} {
			h.publish(t, kind)
		}
		status, _, body := h.request(t, http.MethodPost, "/v2/publish/binding", h.artifacts["binding"].envelope)
		if status != http.StatusForbidden {
			t.Fatalf("missing binding verifier status=%d body=%s", status, body)
		}
		assertPublicationRows(t, h.fixture, h.artifacts["binding"].prepared.Ref, h.artifacts["binding"].prepared.ArtifactDigest.Value, false)
		assertNoPublicationAttempt(t, h, h.artifacts["binding"].prepared.Ref)
	})
	for _, field := range []string{"expected_capability_output", "result_digest"} {
		t.Run("tampered "+field, func(t *testing.T) {
			h := startMatrixHarness(t, nil)
			for _, kind := range []string{"capability", "tool", "provider"} {
				h.publish(t, kind)
			}
			e := h.artifacts["binding"].evidence
			if field == "result_digest" {
				e.Conformance.ResultDigest = "sha256:" + strings.Repeat("0", 64)
			} else {
				golden, err := base64.StdEncoding.DecodeString(e.Conformance.RetainedFixtureBase64)
				if err != nil {
					t.Fatal(err)
				}
				var corpus map[string]any
				if err = json.Unmarshal(golden, &corpus); err != nil {
					t.Fatal(err)
				}
				cases := corpus["cases"].([]any)
				cases[0].(map[string]any)[field] = map[string]int{"n": 999}
				golden, err = json.Marshal(corpus)
				if err != nil {
					t.Fatal(err)
				}
				e.Conformance.RetainedFixtureBase64 = base64.StdEncoding.EncodeToString(golden)
				e.Conformance.FixtureDigest = matrixDigest(golden)
			}
			raw := matrixJSON(t, e)
			ref := h.artifacts["binding"].prepared.Ref
			ref.WorkspaceID = compositionWorkspace
			updated, err := h.fixture.adminPool.Exec(h.ctx, `UPDATE review_records SET evidence=$5 WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version, raw)
			if err != nil {
				t.Fatal(err)
			}
			if updated.RowsAffected() != 1 {
				t.Fatalf("expected one current binding review row updated, got %d", updated.RowsAffected())
			}
			status, _, body := h.request(t, http.MethodPost, "/v2/publish/binding", h.artifacts["binding"].envelope)
			if status < 400 || status >= 500 {
				t.Fatalf("tampered binding evidence status=%d body=%s", status, body)
			}
			wantCases := 0
			if field == "result_digest" {
				// This pin is checked against the computed result after the
				// executor runs all five cases; rejection is still mandatory.
				wantCases = 5
			}
			if h.verifier.cases != wantCases {
				t.Fatalf("verifier executed cases=%d want=%d", h.verifier.cases, wantCases)
			}
			assertPublicationRows(t, h.fixture, ref, h.artifacts["binding"].prepared.ArtifactDigest.Value, false)
			assertNoPublicationAttempt(t, h, ref)
		})
	}
}

func assertNoPublicationAttempt(t *testing.T, h *matrixHarness, ref ports.ArtifactRef) {
	t.Helper()
	var count int
	if err := h.fixture.adminPool.QueryRow(h.ctx, `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, compositionWorkspace, ref.Kind, ref.ID, ref.Version).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("denied publication left %d durable publication attempts for %s/%s@%s", count, ref.Kind, ref.ID, ref.Version)
	}
}

func TestPublicationV2RequiredCapabilityDenials(t *testing.T) {
	// The full-manifest parser and catalog lookup are exercised using the schema's
	// actual {id, contract_version} reference shape. A missing exact ref denies.
	h := startMatrixHarness(t, nil)
	h.publish(t, "capability")
	var env map[string]any
	if err := json.Unmarshal(h.artifacts["skill"].envelope, &env); err != nil {
		t.Fatal(err)
	}
	packageObj := env["artifact"].(map[string]any)["package"].(map[string]any)
	archive, err := base64.StdEncoding.DecodeString(packageObj["data"].(string))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		files[f.Name] = b
	}
	var manifest map[string]any
	if err = json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["required_capabilities"] = []any{map[string]any{"id": "capability/absent", "contract_version": "1.0.0"}}
	files["manifest.json"], err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var inventory []contract.InventoryEntry
	if err = json.Unmarshal(matrixJSON(t, manifest["inventory"]), &inventory); err != nil {
		t.Fatal(err)
	}
	for i := range inventory {
		if inventory[i].Path == "manifest.json" {
			sum := sha256.Sum256(files["manifest.json"])
			inventory[i].Size = int64(len(files["manifest.json"]))
			inventory[i].SHA256 = hex.EncodeToString(sum[:])
		}
	}
	closure, _, err := contract.PackageDigest(inventory)
	if err != nil {
		t.Fatal(err)
	}
	manifest["inventory"] = inventory
	manifest["package_digest"] = map[string]string{"algorithm": "sha256", "value": closure}
	files["manifest.json"], err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, name := range []string{"manifest.json", "SKILL.md"} {
		w, e := zw.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(files[name]); e != nil {
			t.Fatal(e)
		}
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	packageObj["data"] = base64.StdEncoding.EncodeToString(out.Bytes())
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	// Decode computes/validates the complete package closure. This acceptance
	// intentionally fails precisely if the present manifest format needs a
	// different package rebuild contract.
	newPrepared, err := publicationv2.Decode(ports.KindSkill, b, publicationv2.DefaultLimits())
	if err != nil {
		t.Fatalf("rebuilt skill fixture invalid: %v", err)
	}
	evidence := h.artifacts["skill"].evidence
	evidence.Artifact.ArtifactDigest = matrixDigest(newPrepared.Artifact)
	evidence.Source.CaptureDigest = matrixDigest(newPrepared.Artifact)
	evidence.Source.RetainedBytesBase64 = base64.StdEncoding.EncodeToString(newPrepared.Artifact)
	evidence.Source.MediaType = "application/zip"
	if _, err = publicationv2.ValidateEvidenceJSON(matrixJSON(t, evidence)); err != nil {
		t.Fatalf("rebuilt evidence invalid: %v", err)
	}
	updated, err := h.fixture.adminPool.Exec(h.ctx, `UPDATE review_records SET evidence=$5 WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4`, compositionWorkspace, ports.KindSkill, newPrepared.Ref.ID, newPrepared.Ref.Version, matrixJSON(t, evidence))
	if err != nil {
		t.Fatal(err)
	}
	if updated.RowsAffected() != 1 {
		t.Fatalf("expected one skill review row updated, got %d", updated.RowsAffected())
	}
	status, _, body := h.request(t, http.MethodPost, "/v2/publish/skill", b)
	if status < 400 || status >= 500 {
		t.Fatalf("missing required capability status=%d body=%s", status, body)
	}
	if status == http.StatusInternalServerError {
		t.Fatalf("unexpected server failure for missing required capability: %s", body)
	}
}

func TestPublicationV2DocumentPinCompatibility(t *testing.T) {
	h := startMatrixHarness(t, nil)
	for _, kind := range []string{"capability", "tool", "provider"} {
		h.publish(t, kind)
	}
	// Binding publication proves exact dependency resolution against the current
	// catalog rows; skill publication retains a true manifest digest separately.
	h.publish(t, "binding")
	catalog, err := storage.NewPostgres(h.instance.pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"capability", "tool", "provider", "binding"} {
		ref := h.artifacts[kind].prepared.Ref
		ref.WorkspaceID = compositionWorkspace
		rec, err := catalog.Get(h.ctx, ref)
		if err != nil {
			t.Fatalf("get %s: %v", kind, err)
		}
		if rec.State != "published" || rec.DocumentDigest.Algorithm != "sha256" || rec.DocumentDigest.Value != strings.TrimPrefix(matrixDigest(rec.Metadata), "sha256:") {
			t.Fatalf("%s generic document pin mismatch: %+v", kind, rec)
		}
		if kind != "binding" && (rec.Digest.Algorithm != rec.DocumentDigest.Algorithm || rec.Digest.Value != rec.DocumentDigest.Value) {
			t.Fatalf("%s generic digest alias mismatch: digest=%+v document=%+v", kind, rec.Digest, rec.DocumentDigest)
		}
	}
	// Check stored skill-specific digest semantics independently; the generic
	// pin alias must never be mistaken for the manifest's package closure.
	h.publish(t, "skill")
	skillRef := h.artifacts["skill"].prepared.Ref
	skillRef.WorkspaceID = compositionWorkspace
	skill, err := catalog.Get(h.ctx, skillRef)
	if err != nil {
		t.Fatal(err)
	}
	if skill.ManifestDigest.Algorithm != "sha256" || skill.ManifestDigest.Value != strings.TrimPrefix(matrixDigest(skill.Metadata), "sha256:") {
		t.Fatalf("skill true manifest digest mismatch: %+v", skill.ManifestDigest)
	}
	if skill.Digest.Value == skill.ManifestDigest.Value {
		t.Fatalf("skill generic pin unexpectedly aliases manifest digest: %+v", skill)
	}
}
