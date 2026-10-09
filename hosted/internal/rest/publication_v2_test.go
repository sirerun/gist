package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type v2PublisherStub struct {
	result ports.PublicationResult
	calls  int
}

func (p *v2PublisherStub) PublishV2(context.Context, ports.Principal, ports.ArtifactKind, []byte) (ports.PublicationResult, error) {
	p.calls++
	return p.result, nil
}

type v2ReaderStub struct {
	result      ports.PublicationRead
	ref         ports.ArtifactRef
	limit       int64
	packageBody bool
}

func (r *v2ReaderStub) ReadV2(_ context.Context, _ ports.Principal, ref ports.ArtifactRef, limit int64, pkg bool) (ports.PublicationRead, error) {
	r.ref, r.limit, r.packageBody = ref, limit, pkg
	return r.result, nil
}

func TestV2PublicationStatusAndDigestHeaders(t *testing.T) {
	pub := &v2PublisherStub{result: ports.PublicationResult{Body: []byte(`{"ok":true}`), Created: true, ArtifactDigest: ports.Digest{Algorithm: "sha256", Value: strings.Repeat("a", 64)}}}
	h, err := New(Services{Identity: testIdentity{}, Authorizer: testPolicy{}, V2Publisher: pub, Limits: Limits{MaxResponseBytes: 1024}})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/v2/publish/tool", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 || w.Body.String() != string(pub.result.Body) || w.Header().Get("X-Gist-Artifact-Digest") != "sha256:"+strings.Repeat("a", 64) {
		t.Fatalf("status=%d headers=%v body=%q", w.Code, w.Header(), w.Body.String())
	}
	pub.result.Created = false
	r = httptest.NewRequest(http.MethodPost, "/v2/publish/tool", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("replay status=%d", w.Code)
	}
}

func TestV2ReadRejectsAmbiguousQueryAndClampsBudget(t *testing.T) {
	reader := &v2ReaderStub{result: ports.PublicationRead{Body: []byte(`{}`), ContentType: "application/json", ArtifactDigest: ports.Digest{Algorithm: "sha256", Value: strings.Repeat("a", 64)}, BodyDigest: ports.Digest{Algorithm: "sha256", Value: "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"}}}
	h, err := New(Services{Identity: testIdentity{}, Authorizer: testPolicy{}, V2Reader: reader, Limits: Limits{MaxResponseBytes: 10}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v2/artifacts/tool?id=x&id=y&version=1&max_bytes=4", "/v2/artifacts/tool?id=x&version=1&unexpected=z&max_bytes=4"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 422 {
			t.Fatalf("%s status=%d", path, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/v2/artifacts/tool?id=tool.x&version=1.0.0&max_bytes=40", nil)
	r.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || reader.limit != 10 || reader.ref.WorkspaceID != "workspace-a" || reader.ref.Kind != ports.KindTool {
		t.Fatalf("status=%d limit=%d ref=%+v", w.Code, reader.limit, reader.ref)
	}
}
