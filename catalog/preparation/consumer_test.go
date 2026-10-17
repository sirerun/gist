package preparation_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/catalog/resolution"
	"github.com/sirerun/gist/catalog/skills/inboxbriefing"
	"github.com/sirerun/gist/tools/evaluation/fixture"
)

func ready(t *testing.T) (*fixture.Catalog, preparation.Service, preparation.Request) {
	t.Helper()
	c, e := fixture.New("ready")
	if e != nil {
		t.Fatal(e)
	}
	s, e := c.Service()
	if e != nil {
		t.Fatal(e)
	}
	return c, s, fixture.Request("Summarize recent unread inbox email")
}
func TestPublicPreparationPreservesCompleteInstructionsTemplateAndSchemas(t *testing.T) {
	_, s, r := ready(t)
	out, e := s.Prepare(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	if out.Status != "ready" || out.Selected == nil {
		t.Fatalf("not ready: %+v", out)
	}
	files, e := inboxbriefing.Files()
	if e != nil {
		t.Fatal(e)
	}
	for name, data := range files {
		if !bytes.Equal(data, out.Selected.Skill.Files[name]) {
			t.Fatalf("missing or altered asset %s", name)
		}
	}
	if len(out.Selected.Contracts) != 4 {
		t.Fatal("capability/binding/provider/tool closure incomplete")
	}
	found := false
	for _, contract := range out.Selected.Contracts {
		if contract.Ref.Kind == ports.KindTool {
			found = bytes.Contains(contract.Bytes, []byte("input_schema")) && bytes.Contains(contract.Bytes, []byte("output_schema"))
		}
	}
	if !found || !strings.Contains(out.Selected.Guide, "list unread") || out.Provenance.PackageDigest == "" || len(out.Provenance.Contracts) != 4 {
		t.Fatal("complete schemas/guide/provenance absent")
	}
	r.Guide = false
	out, e = s.Prepare(context.Background(), r)
	if e != nil || out.Selected == nil || out.Selected.Guide != "" {
		t.Fatal("guide ablation changed exact closure or failed")
	}
}
func TestMissingRevokedTamperedDeniedRuntimeAndBudgetFailClosed(t *testing.T) {
	for _, state := range []string{"missing_connection", "revoked", "unauthorized", "tampered", "unsupported_runtime"} {
		t.Run(state, func(t *testing.T) {
			c, e := fixture.New(state)
			if e != nil {
				t.Fatal(e)
			}
			s, e := c.Service()
			if e != nil {
				t.Fatal(e)
			}
			out, e := s.Prepare(context.Background(), fixture.Request("Summarize unread inbox email"))
			if out.Status == "ready" || out.Selected != nil {
				t.Fatalf("unsafe preparation %s: %+v %v", state, out, e)
			}
			if state == "unauthorized" && len(out.Candidates) != 0 {
				t.Fatal("disclosed denied candidates")
			}
		})
	}
	_, s, r := ready(t)
	r.MaxBytes = 4096
	out, e := s.Prepare(context.Background(), r)
	if e == nil && out.Status != "budget_exceeded" {
		t.Fatalf("truncated rather than refused: %+v %v", out, e)
	}
	if out.Selected != nil {
		t.Fatal("partial executable context escaped")
	}
}
func TestRevocationDuringIOAndCurrentConnectionRecheck(t *testing.T) {
	c, s, r := ready(t)
	c.BeforeOpen = func(c *fixture.Catalog, _ ports.ArtifactRef) {
		ref := fixture.Ref(ports.KindProvider, "synthetic/provider")
		record := c.Records[ref]
		record.State = "revoked"
		c.Records[ref] = record
	}
	out, e := s.Prepare(context.Background(), r)
	if e == nil || out.Selected != nil || len(out.Candidates) != 0 {
		t.Fatalf("revoked context exposed: %+v %v", out, e)
	}
	_, s, r = ready(t)
	s.Resolver = &changedResolver{inner: s.Resolver}
	out, e = s.Prepare(context.Background(), r)
	if e == nil || out.Selected != nil {
		t.Fatal("stale current connection reused")
	}
}

type changedResolver struct {
	inner preparation.Resolver
	calls int
}

func (c *changedResolver) Resolve(ctx context.Context, r resolution.Request) (resolution.Result, error) {
	c.calls++
	out, e := c.inner.Resolve(ctx, r)
	if c.calls == 2 {
		out.Status = resolution.StatusIncomplete
	}
	return out, e
}

type embedding struct {
	model   string
	calls   int
	seen    []string
	invalid bool
}

func (e *embedding) Embed(_ context.Context, r preparation.EmbeddingRequest) (preparation.EmbeddingResult, error) {
	e.calls++
	e.seen = append(e.seen, r.Texts...)
	result := preparation.EmbeddingResult{ModelVersion: e.model}
	for _, text := range r.Texts {
		vector := make([]float64, 256)
		if r.Purpose == "query" || strings.Contains(text, "Read-only inbox briefing") {
			vector[0] = 1
		} else {
			vector[1] = 1
		}
		if e.invalid {
			vector = vector[:255]
		}
		result.Vectors = append(result.Vectors, vector)
	}
	return result, nil
}
func TestOptionalEmbeddingVersionIsolationAndExplicitFallback(t *testing.T) {
	_, s, r := ready(t)
	r.Mode = preparation.Semantic
	out, e := s.Prepare(context.Background(), r)
	if e != nil || out.Status != "unsupported" {
		t.Fatalf("unavailable adapter invented readiness: %+v %v", out, e)
	}
	r.Fallback = preparation.LexicalFallback
	out, e = s.Prepare(context.Background(), r)
	if e != nil || out.Status != "ready" || out.Mode != preparation.Lexical {
		t.Fatalf("explicit lexical fallback failed: %+v %v", out, e)
	}
	r.Fallback = preparation.Deny
	embed := &embedding{model: "controlled-unit-model-v1"}
	s.Embedder = embed
	s.ModelVersion = embed.model
	for _, mode := range []preparation.Mode{preparation.Semantic, preparation.Hybrid} {
		r.Mode = mode
		out, e = s.Prepare(context.Background(), r)
		if e != nil || out.Status != "ready" {
			t.Fatalf("controlled vector adapter failed: %+v %v", out, e)
		}
	}
	if embed.calls != 4 {
		t.Fatal("unexpected embedding requests")
	}
	s.ModelVersion = "changed-model-v2"
	out, e = s.Prepare(context.Background(), r)
	if e != nil || out.Status != "unsupported" {
		t.Fatal("stale model identity accepted")
	}
	s.ModelVersion = embed.model
	embed.invalid = true
	out, e = s.Prepare(context.Background(), r)
	if e != nil || out.Status != "unsupported" {
		t.Fatal("dimension mismatch accepted")
	}
}
func TestAuthorizationPrecedesEmbeddingAndNoCachedPolicyBypass(t *testing.T) {
	c, s, r := ready(t)
	embed := &embedding{model: "controlled-unit-model-v1"}
	s.Embedder = embed
	s.ModelVersion = embed.model
	r.Mode = preparation.Semantic
	out, e := s.Prepare(context.Background(), r)
	if e != nil || out.Status != "ready" {
		t.Fatal(e)
	}
	c.Deny = true
	r.Principal.PolicyGeneration++
	out, e = s.Prepare(context.Background(), r)
	if e != nil || len(out.Candidates) != 0 || out.Selected != nil || embed.calls != 2 {
		t.Fatal("denied text reached model or cached context escaped")
	}
	c.Deny = false
	r.CatalogVersion = "catalog-v2"
	out, e = s.Prepare(context.Background(), r)
	if e != nil || out.Status != "ready" || out.Provenance.CatalogVersion != "catalog-v2" || out.Provenance.PolicyGeneration != 2 || embed.calls != 4 {
		t.Fatal("scope/catalog invalidation failed")
	}
	r.RecipeVersion = "unknown-v2"
	if _, e = s.Prepare(context.Background(), r); e == nil {
		t.Fatal("stale recipe accepted")
	}
	r.RecipeVersion = preparation.RecipeVersion
	r.Principal.WorkspaceID = "foreign"
	out, e = s.Prepare(context.Background(), r)
	if e != nil || out.Selected != nil || len(out.Candidates) != 0 {
		t.Fatal("foreign workspace content exposed")
	}
}
func TestUnknownIntentAndCancellation(t *testing.T) {
	_, s, r := ready(t)
	r.Intent = "Send an email message"
	out, e := s.Prepare(context.Background(), r)
	if e != nil || out.Status == "ready" {
		t.Fatalf("unsupported intent became ready: %+v %v", out, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = s.Prepare(ctx, r)
	if !errors.Is(e, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", e)
	}
}
