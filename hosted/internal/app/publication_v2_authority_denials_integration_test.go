//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// TestPublicationV2AuthorityDenials exercises current authority and trusted
// evidence over the real TLS HTTP boundary. Each negative case owns a fresh
// database, runtime role, server, and object root.
func TestPublicationV2AuthorityDenials(t *testing.T) {
	type denial struct {
		name   string
		status int
		mutate func(*testing.T, *matrixHarness)
		config func(*Config)
		body   func([]byte) []byte
	}
	update := func(query string, args ...any) func(*testing.T, *matrixHarness) {
		return func(t *testing.T, h *matrixHarness) {
			t.Helper()
			if _, err := h.fixture.adminPool.Exec(h.ctx, query, args...); err != nil {
				t.Fatalf("mutate controlled fixture row: %v", err)
			}
		}
	}
	cases := []denial{
		{name: "reader role", status: http.StatusForbidden, mutate: update(`UPDATE workspace_memberships SET role='reader' WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, compositionWorkspace, compositionIssuer, compositionSubject)},
		{name: "inactive membership", status: http.StatusUnauthorized, mutate: update(`UPDATE workspace_memberships SET active=false WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, compositionWorkspace, compositionIssuer, compositionSubject)},
		{name: "membership policy generation", status: http.StatusUnauthorized, mutate: update(`UPDATE workspace_memberships SET policy_generation=policy_generation+1 WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, compositionWorkspace, compositionIssuer, compositionSubject)},
		{name: "workspace policy generation", status: http.StatusUnauthorized, mutate: update(`UPDATE workspaces SET policy_generation=policy_generation+1 WHERE id=$1`, compositionWorkspace)},
		{name: "identity scopes empty", status: http.StatusForbidden, mutate: update(`UPDATE workload_identities SET scopes=ARRAY[]::text[] WHERE issuer=$1 AND subject=$2 AND workspace_id=$3`, compositionIssuer, compositionSubject, compositionWorkspace)},
		{name: "identity revoked", status: http.StatusUnauthorized, mutate: update(`UPDATE workload_identities SET revoked_at=clock_timestamp() WHERE issuer=$1 AND subject=$2 AND workspace_id=$3`, compositionIssuer, compositionSubject, compositionWorkspace)},
		{name: "identity expired", status: http.StatusForbidden, mutate: update(`UPDATE workload_identities SET expires_at=clock_timestamp()-interval '1 minute' WHERE issuer=$1 AND subject=$2 AND workspace_id=$3`, compositionIssuer, compositionSubject, compositionWorkspace)},
		{name: "namespace retired", status: http.StatusForbidden, mutate: update(`UPDATE namespace_reservations SET status='retired' WHERE prefix='capability' AND workspace_id=$1 AND owner_id=$2`, compositionWorkspace, compositionSubject)},
		{name: "namespace foreign owner", status: http.StatusForbidden, mutate: update(`UPDATE namespace_reservations SET owner_id='foreign-owner' WHERE prefix='capability' AND workspace_id=$1 AND owner_id=$2`, compositionWorkspace, compositionSubject)},
		{name: "review held", status: http.StatusUnprocessableEntity, mutate: update(`UPDATE review_records SET decision='held' WHERE workspace_id=$1 AND kind='capability' AND artifact_id=$2 AND version=$3`, compositionWorkspace, capabilityRefID, capabilityRefVersion)},
		{name: "review rejected", status: http.StatusUnprocessableEntity, mutate: update(`UPDATE review_records SET decision='rejected' WHERE workspace_id=$1 AND kind='capability' AND artifact_id=$2 AND version=$3`, compositionWorkspace, capabilityRefID, capabilityRefVersion)},
		{name: "reviewer unknown", status: http.StatusUnprocessableEntity, mutate: func(t *testing.T, h *matrixHarness) {
			a := h.artifacts["capability"]
			e := a.evidence
			e.Reviewer.Subject = "unknown-reviewer"
			_, err := h.fixture.adminPool.Exec(h.ctx, `UPDATE review_records SET reviewer_id=$1,evidence=$2::jsonb WHERE workspace_id=$3 AND kind=$4 AND artifact_id=$5 AND version=$6`, e.Reviewer.Subject, matrixJSON(t, e), compositionWorkspace, a.prepared.Ref.Kind, a.prepared.Ref.ID, a.prepared.Ref.Version)
			if err != nil {
				t.Fatalf("update controlled review record: %v", err)
			}
		}},
		{name: "artifact digest mismatch", status: http.StatusUnprocessableEntity, mutate: func(t *testing.T, h *matrixHarness) {
			mutateCapabilityEvidence(t, h, func(e *matrixEvidenceAlias) { e.Artifact.ArtifactDigest = "sha256:" + strings.Repeat("0", 64) })
		}},
		{name: "source digest mismatch", status: http.StatusUnprocessableEntity, mutate: func(t *testing.T, h *matrixHarness) {
			mutateCapabilityEvidence(t, h, func(e *matrixEvidenceAlias) { e.Source.CaptureDigest = "sha256:" + strings.Repeat("0", 64) })
		}},
		{name: "source retained bytes mismatch", status: http.StatusUnprocessableEntity, mutate: func(t *testing.T, h *matrixHarness) {
			mutateCapabilityEvidence(t, h, func(e *matrixEvidenceAlias) { e.Source.RetainedBytesBase64 = "Y2hhbmdlZA==" })
		}},
		{name: "grant digest mismatch", status: http.StatusUnprocessableEntity, mutate: func(t *testing.T, h *matrixHarness) {
			mutateCapabilityEvidence(t, h, func(e *matrixEvidenceAlias) { e.Rights.GrantDigest = "sha256:" + strings.Repeat("0", 64) })
		}},
		{name: "grant retained bytes mismatch", status: http.StatusUnprocessableEntity, mutate: func(t *testing.T, h *matrixHarness) {
			mutateCapabilityEvidence(t, h, func(e *matrixEvidenceAlias) { e.Rights.RetainedGrantBase64 = "Y2hhbmdlZA==" })
		}},
		{name: "synthetic evidence disabled", status: http.StatusUnprocessableEntity, config: func(c *Config) { c.PublicationV2.AllowSynthetic = false }},
		{name: "unknown outer authority fields", status: http.StatusUnprocessableEntity, body: func(b []byte) []byte {
			var envelope map[string]any
			if err := json.Unmarshal(b, &envelope); err != nil {
				return nil
			}
			envelope["tenant"] = "foreign-workspace"
			envelope["owner"] = "foreign-owner"
			envelope["publisher"] = "foreign-publisher"
			out, err := json.Marshal(envelope)
			if err != nil {
				return nil
			}
			return out
		}},
	}
	// The fixture tuple is intentionally constant and sourced from the harness
	// artifacts, while keeping SQL updates parameterized to one exact row.
	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			var objectRoot string
			h := startMatrixHarness(t, func(c *Config) {
				objectRoot = t.TempDir()
				c.ObjectStoreRoot = objectRoot
				if tc.config != nil {
					tc.config(c)
				}
			})
			if tc.mutate != nil {
				tc.mutate(t, h)
			}
			before := publicationDenialCounts(t, h)
			body := h.artifacts["capability"].envelope
			if tc.body != nil {
				body = tc.body(body)
			}
			status, _, response := h.request(t, http.MethodPost, "/v2/publish/capability", body)
			if status != tc.status {
				t.Fatalf("denial status=%d want=%d body=%s", status, tc.status, response)
			}
			assertPublicDenialBody(t, response)
			after := publicationDenialCounts(t, h)
			if after != before {
				t.Fatalf("denied POST changed durable publication state: before=%+v after=%+v", before, after)
			}
			assertNoRegularFiles(t, objectRoot)
			if tc.name == "unknown outer authority fields" {
				var workspaces int
				if err := h.fixture.adminPool.QueryRow(h.ctx, `SELECT count(*) FROM workspaces WHERE id='foreign-workspace'`).Scan(&workspaces); err != nil || workspaces != 0 {
					t.Fatalf("unknown request authority changed workspace state: count=%d err=%v", workspaces, err)
				}
			}
		})
	}

	t.Run("cached replay rechecks current membership", func(t *testing.T) {
		var objectRoot string
		h := startMatrixHarness(t, func(c *Config) { objectRoot = t.TempDir(); c.ObjectStoreRoot = objectRoot })
		first := h.publish(t, "capability")
		if len(first) == 0 {
			t.Fatal("initial HTTPS publication returned empty receipt")
		}
		if _, err := h.fixture.adminPool.Exec(h.ctx, `UPDATE workspace_memberships SET role='reader' WHERE workspace_id=$1 AND issuer=$2 AND subject=$3`, compositionWorkspace, compositionIssuer, compositionSubject); err != nil {
			t.Fatal(err)
		}
		before := publicationDenialCounts(t, h)
		status, _, response := h.request(t, http.MethodPost, "/v2/publish/capability", h.artifacts["capability"].envelope)
		if status != http.StatusForbidden {
			t.Fatalf("cached replay after rights revocation status=%d want=403 body=%s", status, response)
		}
		assertPublicDenialBody(t, response)
		after := publicationDenialCounts(t, h)
		if after != before {
			t.Fatalf("denied cached replay changed durable state: before=%+v after=%+v", before, after)
		}
	})

	t.Run("foreign workspace RLS denies namespace and idempotency", func(t *testing.T) {
		h := startMatrixHarness(t, nil)
		const foreign = "foreign-publication-workspace"
		principal := tenantPrincipal(portsPrincipalForWorkspace(compositionWorkspace))
		attempt := func(label, query string, args ...any) {
			t.Helper()
			err := storage.WithTenantPrincipal(h.ctx, h.instance.pool, principal, func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, query, args...)
				return err
			})
			if err == nil {
				t.Fatalf("foreign workspace %s INSERT unexpectedly succeeded", label)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("foreign workspace %s INSERT failed outside RLS policy (error=%v)", label, err)
			}
		}
		attempt("namespace", `INSERT INTO namespace_reservations(prefix,workspace_id,owner_id,status) VALUES('foreign',$1,'x','active')`, foreign)
		attempt("idempotency", `INSERT INTO publication_idempotency(workspace_id,kind,idempotency_key,artifact_id,version,artifact_digest,attempt_id,response_body) VALUES($1,'capability','foreign','foreign','1.0.0',repeat('a',64),repeat('b',32),'{}'::bytea)`, foreign)
		var selected int
		if err := storage.WithTenantPrincipal(h.ctx, h.instance.pool, principal, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM namespace_reservations WHERE workspace_id=$1`, foreign).Scan(&selected)
		}); err != nil || selected != 0 {
			t.Fatalf("tenant transaction sees foreign namespace rows=%d err=%v", selected, err)
		}
		if err := storage.WithTenantPrincipal(h.ctx, h.instance.pool, principal, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM publication_idempotency WHERE workspace_id=$1`, foreign).Scan(&selected)
		}); err != nil || selected != 0 {
			t.Fatalf("tenant transaction sees foreign idempotency rows=%d err=%v", selected, err)
		}
		var visible int
		if err := h.instance.pool.QueryRow(h.ctx, `SELECT count(*) FROM namespace_reservations WHERE workspace_id=$1`, foreign).Scan(&visible); err != nil || visible != 0 {
			t.Fatalf("runtime pool sees foreign namespace rows=%d err=%v", visible, err)
		}
		if err := h.instance.pool.QueryRow(h.ctx, `SELECT count(*) FROM publication_idempotency WHERE workspace_id=$1`, foreign).Scan(&visible); err != nil || visible != 0 {
			t.Fatalf("runtime pool sees foreign idempotency rows=%d err=%v", visible, err)
		}
	})
}

// Alias keeps mutation callbacks concise while serializing evidence through
// the same server-record JSON format that the harness seeded.
type matrixEvidenceAlias = struct {
	EvidenceVersion string `json:"evidence_version"`
	WorkspaceID     string `json:"workspace_id"`
	Synthetic       bool   `json:"synthetic"`
	Artifact        struct {
		Kind           string `json:"kind"`
		ID             string `json:"id"`
		Version        string `json:"version"`
		ArtifactDigest string `json:"artifact_digest"`
	} `json:"artifact"`
	Source struct {
		URI        string `json:"uri"`
		CapturedAt string `json:"captured_at"`
		Collector  struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"collector"`
		CaptureDigest       string `json:"capture_digest"`
		RetainedBytesBase64 string `json:"retained_bytes_base64"`
		MediaType           string `json:"media_type"`
	} `json:"source"`
	Rights struct {
		License             string   `json:"license"`
		Redistribution      bool     `json:"redistribution"`
		Scope               string   `json:"scope"`
		GrantDigest         string   `json:"grant_digest"`
		AllowedUse          []string `json:"allowed_use"`
		RetainedGrantBase64 string   `json:"retained_grant_base64"`
	} `json:"rights"`
	Reviewer struct {
		Issuer     string `json:"issuer"`
		Subject    string `json:"subject"`
		ReviewedAt string `json:"reviewed_at"`
	} `json:"reviewer"`
}

func mutateCapabilityEvidence(t *testing.T, h *matrixHarness, mutate func(*matrixEvidenceAlias)) {
	t.Helper()
	a := h.artifacts["capability"]
	b, err := json.Marshal(a.evidence)
	if err != nil {
		t.Fatal(err)
	}
	var e matrixEvidenceAlias
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	mutate(&e)
	if _, err := h.fixture.adminPool.Exec(h.ctx, `UPDATE review_records SET evidence=$1::jsonb WHERE workspace_id=$2 AND kind=$3 AND artifact_id=$4 AND version=$5`, matrixJSON(t, e), compositionWorkspace, a.prepared.Ref.Kind, a.prepared.Ref.ID, a.prepared.Ref.Version); err != nil {
		t.Fatalf("update controlled evidence row: %v", err)
	}
}

type publicationCountSnapshot struct{ catalog, outbox, idempotency, attempts int }

func publicationDenialCounts(t *testing.T, h *matrixHarness) publicationCountSnapshot {
	t.Helper()
	var got publicationCountSnapshot
	queries := []struct {
		dst   *int
		query string
	}{
		{&got.catalog, `SELECT count(*) FROM catalog_versions WHERE workspace_id=$1`},
		{&got.outbox, `SELECT count(*) FROM event_outbox WHERE workspace_id=$1`},
		{&got.idempotency, `SELECT count(*) FROM publication_idempotency WHERE workspace_id=$1`},
		{&got.attempts, `SELECT count(*) FROM publication_attempts WHERE workspace_id=$1`},
	}
	for _, q := range queries {
		if err := h.fixture.adminPool.QueryRow(h.ctx, q.query, compositionWorkspace).Scan(q.dst); err != nil {
			t.Fatalf("snapshot publication state: %v", err)
		}
	}
	return got
}

func assertPublicDenialBody(t *testing.T, body []byte) {
	t.Helper()
	text := string(body)
	for _, forbidden := range []string{"SQLSTATE", "select ", "insert into", "UPDATE ", "fixture-issuer", "fixture-maintainer", "retained_bytes_base64", "retained_grant_base64", "source URI"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("public denial body leaks internal/evidence detail %q: %s", forbidden, text)
		}
	}
	if len(body) == 0 {
		t.Fatal("denial returned empty public error body")
	}
}

func assertNoRegularFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			return errors.New("regular file exists")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("denied publication left an object file or root could not be inspected: %v", err)
	}
}

func portsPrincipalForWorkspace(workspace string) ports.Principal {
	return ports.Principal{Issuer: compositionIssuer, Subject: compositionSubject, WorkspaceID: workspace, Scopes: []string{"catalog:publish"}}
}

const (
	capabilityRefID      = "capability/demo"
	capabilityRefVersion = "1.0.0"
)
