package rest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type recordingRevoker struct {
	calls []ports.ArtifactRef
	err   error
}

func (r *recordingRevoker) RevokeArtifact(_ context.Context, _ ports.Principal, ref ports.ArtifactRef) (RevocationNotice, error) {
	r.calls = append(r.calls, ref)
	if r.err != nil {
		return RevocationNotice{}, r.err
	}
	return RevocationNotice{Ref: ref, RevokedAt: 1700000000}, nil
}

type countingPublisher struct{ calls int }

func (p *countingPublisher) Publish(context.Context, ports.Principal, ports.ArtifactKind, []byte) ([]byte, error) {
	p.calls++
	return []byte(`{}`), nil
}

// Regression: POST /v1/publish/revocations went through the package publisher
// and inserted an ordinary catalog version. It must record a revocation for
// the named version instead.
func TestPublishRevocationRecordsRevocationNotCatalogVersion(t *testing.T) {
	rev := &recordingRevoker{}
	pub := &countingPublisher{}
	h := handlerWith(t, func(s *Services) { s.Revocations = rev; s.Publisher = pub })
	body := `{"artifact":{"kind":"skill","id":"s","version":"1.0.0","reason":"compromised"},"max_bytes":4096,"idempotency_key":"k1"}`
	w := do(t, h, "POST", "/v1/publish/revocations", body)
	if w.Code != 201 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if pub.calls != 0 {
		t.Fatalf("revocation reached the package publisher %d times", pub.calls)
	}
	if len(rev.calls) != 1 {
		t.Fatalf("revoker calls = %d", len(rev.calls))
	}
	want := ports.ArtifactRef{WorkspaceID: "workspace-a", Kind: ports.KindSkill, ID: "s", Version: "1.0.0"}
	if rev.calls[0] != want {
		t.Fatalf("revoked %+v, want %+v (workspace from the principal)", rev.calls[0], want)
	}
	var out struct {
		Notice struct {
			Kind, ID, Version, State string
			RevokedAt                int64 `json:"revoked_at"`
		} `json:"notice"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Notice.State != "revoked" || out.Notice.Version != "1.0.0" || out.Notice.RevokedAt != 1700000000 {
		t.Fatalf("notice = %+v", out.Notice)
	}
}

func TestPublishRevocationValidatesAndMapsMissingTarget(t *testing.T) {
	rev := &recordingRevoker{}
	h := handlerWith(t, func(s *Services) { s.Revocations = rev })
	for _, body := range []string{
		`{}`,
		`{"artifact":{"kind":"skill","id":"s","version":"1"},"max_bytes":4096}`,
		`{"artifact":{"kind":"connection","id":"s","version":"1"},"max_bytes":4096,"idempotency_key":"k"}`,
		`{"artifact":{"kind":"skill","id":"../x","version":"1"},"max_bytes":4096,"idempotency_key":"k"}`,
		`{"artifact":{"kind":"skill","id":"s","version":"1","workspace_id":"other"},"max_bytes":4096,"idempotency_key":"k"}`,
	} {
		if w := do(t, h, "POST", "/v1/publish/revocations", body); w.Code != 422 {
			t.Fatalf("%s: status=%d want 422", body, w.Code)
		}
	}
	if len(rev.calls) != 0 {
		t.Fatalf("invalid bodies reached the revoker: %v", rev.calls)
	}
	rev.err = ErrNotFound
	w := do(t, h, "POST", "/v1/publish/revocations", `{"artifact":{"kind":"skill","id":"s","version":"9"},"max_bytes":4096,"idempotency_key":"k"}`)
	if w.Code != 404 {
		t.Fatalf("missing target status=%d want 404", w.Code)
	}
}

type revokedCatalog struct{ testCatalog }

func (revokedCatalog) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	return ports.CatalogRecord{Ref: ref, State: "revoked", Metadata: []byte(`{"id":"ok"}`)}, nil
}

// A revoked pinned record is 409 artifact_revoked after authorization on
// every exact read, and a per-item artifact_revoked error in batch-get.
func TestRevokedPinnedRecordIsArtifactRevoked(t *testing.T) {
	h := handlerWith(t, func(s *Services) { s.Catalog = revokedCatalog{} })
	for _, path := range []string{"/v1/skills/s/versions/1", "/v1/skills/s/versions/1/package", "/v1/tools/t/versions/1", "/v1/capabilities/c/versions/1"} {
		w := do(t, h, "GET", path, "")
		var e Error
		_ = json.Unmarshal(w.Body.Bytes(), &e)
		if w.Code != 409 || e.Code != "artifact_revoked" {
			t.Fatalf("%s: status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
	w := do(t, h, "POST", "/v1/artifacts/batch-get", `{"references":[{"WorkspaceID":"workspace-a","Kind":"skill","ID":"s","Version":"1"}],"max_bytes":4096}`)
	if w.Code != 200 {
		t.Fatalf("batch status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			Artifact json.RawMessage `json:"artifact"`
			Error    *Error          `json:"error"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Error == nil || out.Items[0].Error.Code != "artifact_revoked" || out.Items[0].Artifact != nil {
		t.Fatalf("batch = %s", w.Body.String())
	}
}
