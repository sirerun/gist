package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/resolution"
)

type canonicalCatalog struct{ skill ports.CatalogRecord }

func (c canonicalCatalog) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	if ref == c.skill.Ref {
		return c.skill, nil
	}
	return ports.CatalogRecord{}, context.Canceled
}
func (c canonicalCatalog) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

type canonicalAuth struct{}

func (canonicalAuth) Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{Allowed: true}, nil
}

type canonicalStore struct{}

func (canonicalStore) Put(context.Context, ports.Resolution) error { return nil }

type canonicalClock struct{}

func (canonicalClock) Now() time.Time { return time.Unix(1, 0) }
func testCanonicalAdapter(t *testing.T, max int) *canonicalResolverAdapter {
	t.Helper()
	skill := ports.ArtifactRef{WorkspaceID: "workspace", Kind: ports.KindSkill, ID: "skill/demo", Version: "1.0.0"}
	metadata, _ := json.Marshal(map[string]any{"id": "skill/demo", "version": "1.0.0", "required_capabilities": []any{}, "runtime_requirements": map[string]any{"local_execution": false}})
	record := ports.CatalogRecord{Ref: skill, State: "published", Metadata: metadata, Digest: ports.Digest{Algorithm: "sha256", Value: strings.Repeat("a", 64)}, ManifestDigest: ports.Digest{Algorithm: "sha256", Value: manifestDigest(metadata)}}
	resolver, err := resolution.NewResolver(canonicalCatalog{skill: record}, canonicalAuth{}, canonicalStore{}, canonicalClock{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &canonicalResolverAdapter{resolver: resolver, maxBytes: max}
}
func TestCanonicalAdapterRejectsLegacyUnknownAndDuplicateFields(t *testing.T) {
	adapter := testCanonicalAdapter(t, 4096)
	principal := ports.Principal{Subject: "caller", WorkspaceID: "workspace"}
	for _, body := range []string{
		`{"skill":{"id":"skill/demo"},"runtime_id":"client","max_bytes":1024}`,
		`{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"client"},"max_bytes":1024,"workspace_id":"foreign"}`,
		`{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"client","owned_connections":null},"max_bytes":1024}`,
		`{"skill_ref":"skill/demo@1.0.0","skill_ref":"skill/other@1.0.0","runtime":{"id":"client"},"max_bytes":1024}`,
	} {
		if _, err := adapter.Resolve(context.Background(), principal, []byte(body)); err == nil {
			t.Fatalf("accepted invalid request %s", body)
		}
	}
}
func TestCanonicalAdapterUsesFrozenFieldsAndAuthenticatedWorkspace(t *testing.T) {
	adapter := testCanonicalAdapter(t, 4096)
	principal := ports.Principal{Subject: "caller", WorkspaceID: "workspace"}
	got, err := adapter.Resolve(context.Background(), principal, []byte(`{"skill_ref":"skill/demo@1.0.0","runtime":{"id":"runtime.example","owned_connections":true},"max_bytes":2048}`))
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		ID        string `json:"resolution_id"`
		Aggregate string `json:"aggregate"`
		Skill     struct {
			Reference string `json:"reference"`
			Digest    struct {
				Algorithm string `json:"algorithm"`
			} `json:"digest"`
		} `json:"skill"`
	}
	if err := json.Unmarshal(got, &body); err != nil {
		t.Fatal(err)
	}
	if body.ID == "" || body.Aggregate != "ready" || body.Skill.Reference != "skill/demo@1.0.0" || body.Skill.Digest.Algorithm != "sha256" {
		t.Fatalf("unexpected canonical body: %s", got)
	}
}

func manifestDigest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
