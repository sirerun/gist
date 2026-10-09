//go:build integration

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/contract"
	"github.com/sirerun/gist/hosted/internal/identity"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/publicationv2"
)

func matrixDigest(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
func matrixJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// This executor actually validates and converts five retained identity-adapter
// cases. It qualifies synthetic source behavior only, never provider execution.
type matrixIdentityVerifier struct{ cases int }

func (v *matrixIdentityVerifier) VerifyPublicationBinding(ctx context.Context, records []ports.CatalogRecord, e ports.PublicationAdmissionEvidence, golden []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(records) != 3 || e.Conformance == nil {
		return errors.New("missing fixture pins")
	}
	c := e.Conformance
	if c.ExecutorID != "fixture-identity" || c.ExecutorRevision != "test-v1" || c.AdapterVersion != "1" || matrixDigest(golden) != c.FixtureDigest {
		return errors.New("wrong fixture executor")
	}
	var corpus struct {
		FixtureVersion  string `json:"fixture_version"`
		CapabilityID    string `json:"capability_id"`
		ContractVersion string `json:"contract_version"`
		ProviderID      string `json:"provider_id"`
		ToolID          string `json:"tool_id"`
		ToolVersion     string `json:"tool_version"`
		BindingID       string `json:"binding_id"`
		BindingVersion  string `json:"binding_version"`
		Cases           []struct {
			Name           string          `json:"name"`
			Input          json.RawMessage `json:"capability_input"`
			ProviderInput  json.RawMessage `json:"expected_provider_input"`
			ProviderOutput json.RawMessage `json:"provider_output"`
			Output         json.RawMessage `json:"expected_capability_output"`
			Classes        []string        `json:"expected_effect_classes"`
		} `json:"cases"`
	}
	if json.Unmarshal(golden, &corpus) != nil || len(corpus.Cases) < 5 || len(corpus.Cases) != c.ExecutedCaseCount || corpus.FixtureVersion != "1" {
		return errors.New("invalid fixture corpus")
	}
	if corpus.CapabilityID != records[0].Ref.ID || corpus.ContractVersion != records[0].Ref.Version || corpus.ToolID != records[1].Ref.ID || corpus.ToolVersion != records[1].Ref.Version || corpus.ProviderID != records[2].Ref.ID || corpus.BindingID != e.Artifact.ID || corpus.BindingVersion != e.Artifact.Version {
		return errors.New("corpus artifact mismatch")
	}
	schemas := make([]map[string]json.RawMessage, 2)
	for i := range schemas {
		if records[i].Ref.WorkspaceID != e.WorkspaceID || records[i].State != "published" || matrixDigest(records[i].Metadata) != "sha256:"+records[i].DocumentDigest.Value {
			return errors.New("record digest mismatch")
		}
		if json.Unmarshal(records[i].Metadata, &schemas[i]) != nil {
			return errors.New("record schema mismatch")
		}
	}
	var actual []json.RawMessage
	seen := map[string]bool{}
	for _, tc := range corpus.Cases {
		if tc.Name == "" || seen[tc.Name] || len(tc.Classes) != 1 || tc.Classes[0] != "read_only" {
			return errors.New("invalid case identity/effect")
		}
		seen[tc.Name] = true
		if err := contract.ValidateJSONSchema(schemas[0]["input_schema"], tc.Input); err != nil {
			return err
		}
		convertedInput := append([]byte(nil), tc.Input...)
		if !bytes.Equal(convertedInput, tc.ProviderInput) {
			return errors.New("provider conversion differs")
		}
		if err := contract.ValidateJSONSchema(schemas[1]["input_schema"], convertedInput); err != nil {
			return err
		}
		if err := contract.ValidateJSONSchema(schemas[1]["output_schema"], tc.ProviderOutput); err != nil {
			return err
		}
		convertedOutput := append([]byte(nil), tc.ProviderOutput...)
		if err := contract.ValidateJSONSchema(schemas[0]["output_schema"], convertedOutput); err != nil {
			return err
		}
		if !bytes.Equal(convertedOutput, tc.Output) {
			return errors.New("capability conversion differs")
		}
		actual = append(actual, convertedOutput)
		v.cases++
	}
	out, err := json.Marshal(actual)
	if err != nil {
		return err
	}
	if matrixDigest(out) != c.ResultDigest {
		return errors.New("computed result digest differs")
	}
	return nil
}

type matrixPublication struct {
	prepared ports.PreparedPublication
	envelope []byte
	evidence ports.PublicationAdmissionEvidence
}

func matrixFixtures(t *testing.T) (map[string]matrixPublication, *matrixIdentityVerifier) {
	t.Helper()
	raw, err := os.ReadFile("../../../contracts/registry/v2/fixtures/valid.json")
	if err != nil {
		t.Fatal(err)
	}
	var envelopes map[string]json.RawMessage
	if err = json.Unmarshal(raw, &envelopes); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second).Format(time.RFC3339)
	reviewerAt := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Second).Format(time.RFC3339)
	grant := []byte("Synthetic local redistribution permission; no external provider or public license claim.")
	source := []byte("Synthetic captured action schema version1; fixture identity conversion.")
	sourceURI := "https://fixture.invalid/source/action-v1"
	scalarSchema := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"n"}, "properties": map[string]any{"n": map[string]any{"type": "integer", "minimum": 0}}}
	cases := make([]map[string]any, 5)
	outputs := make([]json.RawMessage, 5)
	for i := range cases {
		in := map[string]int{"n": i}
		out := map[string]int{"n": i * 2}
		cases[i] = map[string]any{"name": fmt.Sprintf("case-%d", i), "capability_input": in, "expected_provider_input": in, "provider_output": out, "expected_capability_output": out, "expected_effect_classes": []string{"read_only"}}
		outputs[i] = matrixJSON(t, out)
	}
	golden := matrixJSON(t, map[string]any{"fixture_version": "1", "capability_id": "capability/demo", "contract_version": "1.0.0", "provider_id": "provider/demo", "tool_id": "tool/demo", "tool_version": "1.0.0", "binding_id": "binding/demo", "binding_version": "1.0.0", "cases": cases})
	resultBytes := matrixJSON(t, outputs)
	result := map[string]matrixPublication{}
	for _, kind := range []string{"skill", "capability", "tool", "provider", "binding", "taxonomy"} {
		body := append([]byte(nil), envelopes[kind]...)
		retained := append([]byte(nil), source...)
		if kind != "skill" {
			var env map[string]any
			if err = json.Unmarshal(body, &env); err != nil {
				t.Fatal(err)
			}
			artifact := env["artifact"].(map[string]any)
			doc := artifact["document"].(map[string]any)
			switch kind {
			case "capability":
				doc["input_schema"] = scalarSchema
				doc["output_schema"] = scalarSchema
				doc["required_scopes"] = []string{"fixture:read"}
			case "tool":
				doc["input_schema"] = scalarSchema
				doc["output_schema"] = scalarSchema
				doc["lifecycle"] = "published"
				doc["provenance"] = sourceURI
				doc["capture_time"] = at
				doc["digest"] = map[string]any{"algorithm": "sha256", "value": matrixDigest(source)[7:]}
				doc["credentials"] = map[string]any{"type": "fixture", "scopes": []string{"fixture:read"}}
			case "provider":
				doc["support_state"] = "resolvable"
				doc["last_verified_at"] = at
				doc["capture"] = map[string]any{"source": sourceURI, "digest": map[string]any{"algorithm": "sha256", "value": matrixDigest(source)[7:]}}
			case "binding":
				doc["fixture_digest"] = map[string]any{"algorithm": "sha256", "value": matrixDigest(golden)[7:]}
			}
			metadata, er := json.MarshalIndent(doc, "", "  ")
			if er != nil {
				t.Fatal(er)
			}
			body = []byte(fmt.Sprintf("{\"artifact\":{\"id\":%s,\"version\":%s,\"document\":%s},\"max_bytes\":1048576,\"idempotency_key\":%s}", matrixJSON(t, artifact["id"]), matrixJSON(t, artifact["version"]), metadata, matrixJSON(t, "matrix-"+kind)))
			if kind != "tool" && kind != "provider" {
				retained = metadata
			}
		}
		prepared, er := publicationv2.Decode(ports.ArtifactKind(kind), body, publicationv2.DefaultLimits())
		if er != nil {
			t.Fatalf("%s decode: %v", kind, er)
		}
		uri := sourceURI
		if kind == "skill" {
			retained = prepared.Artifact
			uri = "https://fixture.invalid/source/skill"
		}
		e := ports.PublicationAdmissionEvidence{
			EvidenceVersion: "1", WorkspaceID: compositionWorkspace, Synthetic: true,
			Artifact: ports.PublicationEvidenceArtifact{Kind: prepared.Ref.Kind, ID: prepared.Ref.ID, Version: prepared.Ref.Version, ArtifactDigest: matrixDigest(prepared.Artifact)},
			Source:   ports.PublicationEvidenceSource{URI: uri, CapturedAt: at, Collector: ports.PublicationEvidenceCollector{ID: "fixture-collector", Version: "1"}, CaptureDigest: matrixDigest(retained), RetainedBytesBase64: base64.StdEncoding.EncodeToString(retained), MediaType: "application/json"},
			Rights:   ports.PublicationEvidenceRights{License: "LicenseRef-Synthetic-Local", Redistribution: true, Scope: "workspace_only", GrantDigest: matrixDigest(grant), AllowedUse: []string{"private_distribution", "catalog_metadata"}, RetainedGrantBase64: base64.StdEncoding.EncodeToString(grant)},
			Reviewer: ports.PublicationEvidenceReviewer{Issuer: "fixture-issuer", Subject: "fixture-maintainer", ReviewedAt: reviewerAt},
		}
		if kind == "skill" {
			e.Source.MediaType = "application/zip"
		}
		if kind == "provider" {
			e.ApprovedSupportState = "resolvable"
			e.ProviderQualification = &ports.PublicationProviderQualification{ReceiptRef: "https://fixture.invalid/qualification", ReceiptDigest: matrixDigest(source), ValidUntil: time.Now().UTC().Add(time.Hour).Format(time.RFC3339)}
		}
		if kind == "binding" {
			e.Conformance = &ports.PublicationConformance{AdapterVersion: "1", ExecutorID: "fixture-identity", ExecutorRevision: "test-v1", FixtureDigest: matrixDigest(golden), RetainedFixtureBase64: base64.StdEncoding.EncodeToString(golden), ExecutedCaseCount: 5, ResultDigest: matrixDigest(resultBytes), ExactRefs: []string{"capability/demo@1.0.0", "tool/demo@1.0.0", "provider/demo@1.0.0"}, Passed: true}
		}
		if kind == "taxonomy" {
			e.TaxonomyGrant = &ports.PublicationTaxonomyGrant{TaxonomyID: prepared.Ref.ID, Edition: prepared.Ref.Version, AllowedFields: []string{"id", "edition", "attribution", "nodes"}}
		}
		if _, er = publicationv2.ValidateEvidenceJSON(matrixJSON(t, e)); er != nil {
			t.Fatalf("%s evidence schema: %v", kind, er)
		}
		if er = publicationv2.ValidateEvidence(prepared, ports.Principal{WorkspaceID: compositionWorkspace}, e, time.Now().UTC(), true); er != nil {
			t.Fatalf("%s evidence: %v", kind, er)
		}
		result[kind] = matrixPublication{prepared: prepared, envelope: body, evidence: e}
	}
	return result, &matrixIdentityVerifier{}
}

func TestPublicationV2AllSixKindsRealHTTP(t *testing.T) {
	fixture := newCompositionPostgres(t)
	fixture.seedTenant(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	artifacts, verifier := matrixFixtures(t)
	for kind, a := range artifacts {
		if _, err := fixture.adminPool.Exec(ctx, "INSERT INTO namespace_reservations(prefix,workspace_id,owner_id,status) VALUES($1,$2,$3,'active')", kind, compositionWorkspace, compositionSubject); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.adminPool.Exec(ctx, "INSERT INTO review_records(workspace_id,kind,artifact_id,version,decision,reviewer_id,evidence) VALUES($1,$2,$3,$4,'approved',$5,$6)", compositionWorkspace, a.prepared.Ref.Kind, a.prepared.Ref.ID, a.prepared.Ref.Version, "fixture-maintainer", matrixJSON(t, a.evidence)); err != nil {
			t.Fatal(err)
		}
	}
	cfg := compositionConfig(fixture.runtimeURL, t.TempDir(), compositionSigningConfig(t))
	cfg.MaxRequestBytes = 16 << 20
	cfg.PublicationV2 = &PublicationV2Config{AllowSynthetic: true, TrustedReviewers: []TrustedPublicationReviewer{{Issuer: "fixture-issuer", Subject: "fixture-maintainer"}}, BindingVerifier: verifier, MaintenanceTargets: []MaintenanceTarget{{WorkspaceID: compositionWorkspace, Subject: compositionSubject}}}
	instance, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := instance.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	assertCompositionRuntimeRole(t, instance, fixture.role)
	token, err := instance.MintWorkloadToken(ctx, identity.WorkloadRequest{Subject: compositionSubject, WorkspaceID: compositionWorkspace, Scopes: []string{"catalog:read", "catalog:publish"}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(instance.Handler())
	defer server.Close()
	request := func(method, path string, body []byte) (int, http.Header, []byte) {
		req, er := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(body))
		if er != nil {
			t.Fatal(er)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, er := server.Client().Do(req)
		if er != nil {
			t.Fatal(er)
		}
		got, er := io.ReadAll(resp.Body)
		resp.Body.Close()
		if er != nil {
			t.Fatal(er)
		}
		return resp.StatusCode, resp.Header, got
	}
	for _, kind := range []string{"skill", "capability", "tool", "provider", "binding", "taxonomy"} {
		t.Run(kind, func(t *testing.T) {
			a := artifacts[kind]
			status, headers, receipt := request(http.MethodPost, "/v2/publish/"+kind, a.envelope)
			if status != 201 {
				t.Fatalf("publish %s status=%d body=%s", kind, status, receipt)
			}
			if headers.Get("X-Gist-Artifact-Digest") != matrixDigest(a.prepared.Artifact) {
				t.Fatalf("artifact header %s", headers.Get("X-Gist-Artifact-Digest"))
			}
			status, _, replay := request(http.MethodPost, "/v2/publish/"+kind, a.envelope)
			if status != 200 || !bytes.Equal(receipt, replay) {
				t.Fatalf("replay status=%d body=%s", status, replay)
			}
			q := url.Values{"id": {a.prepared.Ref.ID}, "version": {a.prepared.Ref.Version}, "max_bytes": {"1048576"}}
			status, h, body := request(http.MethodGet, "/v2/artifacts/"+kind+"?"+q.Encode(), nil)
			if status != 200 || !bytes.Equal(body, a.prepared.Metadata) || h.Get("X-Gist-Body-Digest") != matrixDigest(a.prepared.Metadata) || h.Get("X-Gist-Artifact-Digest") != matrixDigest(a.prepared.Artifact) {
				t.Fatalf("metadata status=%d exact=%v headers=%v body=%s", status, bytes.Equal(body, a.prepared.Metadata), h, body)
			}
			if kind == "skill" {
				if h.Get("X-Gist-Manifest-Digest") != matrixDigest(a.prepared.Metadata) || h.Get("X-Gist-Package-Digest") != "sha256:"+a.prepared.PackageDigest.Value {
					t.Fatal("skill digest domains differ")
				}
				status, hh, blob := request(http.MethodGet, "/v2/artifacts/skill/package?"+q.Encode(), nil)
				if status != 200 || !bytes.Equal(blob, a.prepared.Artifact) || hh.Get("X-Gist-Body-Digest") != matrixDigest(a.prepared.Artifact) {
					t.Fatalf("zip status=%d exact=%v", status, bytes.Equal(blob, a.prepared.Artifact))
				}
			} else if h.Get("X-Gist-Manifest-Digest") != "" || h.Get("X-Gist-Package-Digest") != "" {
				t.Fatal("non-skill invented skill digest")
			}
			var events int
			if er := fixture.adminPool.QueryRow(ctx, "SELECT count(*) FROM event_outbox WHERE workspace_id=$1 AND artifact_kind=$2 AND artifact_id=$3 AND artifact_version=$4", compositionWorkspace, a.prepared.Ref.Kind, a.prepared.Ref.ID, a.prepared.Ref.Version).Scan(&events); er != nil || events != 1 {
				t.Fatalf("events=%d err=%v", events, er)
			}
		})
	}
	if verifier.cases < 5 {
		t.Fatalf("offline corpus executed only%d cases", verifier.cases)
	}
}
