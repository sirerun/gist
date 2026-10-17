package preparation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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
	omit    bool
}

func (e *embedding) Embed(_ context.Context, r preparation.EmbeddingRequest) (preparation.EmbeddingResult, error) {
	e.calls++
	e.seen = append(e.seen, r.Texts...)
	result := preparation.EmbeddingResult{ModelVersion: e.model}
	for _, text := range r.Texts {
		vector := make([]float64, 256)
		if r.Purpose == "query" || (strings.Contains(text, "Read-only inbox briefing") && !e.omit) {
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

type currentConnection struct {
	calls          int
	ready          bool
	revokeOnSecond bool
	wrongScope     bool
	now            ports.Clock
}

func (c *currentConnection) CheckCurrentReadiness(_ context.Context, r preparation.ReadinessRequest) (preparation.ReadinessDecision, error) {
	c.calls++
	scope := r.Scope
	if c.wrongScope {
		scope.RuntimeID = "foreign-runtime"
	}
	ready := c.ready
	if c.revokeOnSecond && c.calls > 1 {
		ready = false
	}
	now := c.now.Now()
	return preparation.ReadinessDecision{Scope: scope, Ready: ready, Generation: "actual-fixture-connection-state-v1", CheckedAt: now, ValidUntil: now.Add(time.Minute)}, nil
}
func credentialed(t *testing.T) (*fixture.Catalog, preparation.Service, preparation.Request) {
	t.Helper()
	c, s, r := ready(t)
	ref := fixture.Ref(ports.KindTool, "synthetic/mail-read")
	record := c.Records[ref]
	var metadata map[string]any
	if e := json.Unmarshal(record.Metadata, &metadata); e != nil {
		t.Fatal(e)
	}
	metadata["credentials"] = map[string]any{"type": "oauth", "scopes": []string{"synthetic.mail.readonly"}}
	raw, e := json.Marshal(metadata)
	if e != nil {
		t.Fatal(e)
	}
	record.Metadata = raw
	record.Digest = fixture.Hash(raw)
	record.ManifestDigest = record.Digest
	c.Records[ref] = record
	c.Bytes[ref] = raw
	return c, s, r
}
func TestCredentialedReadinessRequiresFreshScopedCallerPort(t *testing.T) {
	_, s, r := credentialed(t)
	out, e := s.Prepare(context.Background(), r)
	if e != nil || out.Status != "requires_connection" || out.Selected != nil {
		t.Fatalf("invented connection: %+v %v", out, e)
	}
	for _, which := range []string{"false", "true", "revoked after IO", "wrong scope"} {
		t.Run(which, func(t *testing.T) {
			_, s, r := credentialed(t)
			port := &currentConnection{ready: which != "false", revokeOnSecond: which == "revoked after IO", wrongScope: which == "wrong scope", now: ports.SystemClock{}}
			s.CurrentReadiness = port
			s.Clock = port.now
			s.MaxReadinessAge = time.Minute
			out, e := s.Prepare(context.Background(), r)
			if which == "true" {
				if e != nil || out.Status != "ready" || port.calls != 2 {
					t.Fatalf("current admitted connection failed: %+v %v", out, e)
				}
				if out.Selected.Resolution.Findings[0].Provenance != "client_asserted" {
					t.Fatal("upgraded caller assertion provenance")
				}
			} else if out.Status == "ready" || out.Selected != nil {
				t.Fatalf("unavailable connection became ready: %+v %v", out, e)
			}
		})
	}
}

type staleSearch struct {
	*fixture.Catalog
	stale ports.CatalogRecord
}

func (c staleSearch) Search(ctx context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	page, e := c.Catalog.Search(ctx, q)
	for i, r := range page.Records {
		if r.Ref == c.stale.Ref {
			page.Records[i] = c.stale
		}
	}
	return page, e
}
func TestStaleDerivedIndexCannotExposeRevokedOrUnpinnedMetadataToEmbedding(t *testing.T) {
	for _, which := range []string{"revoked", "changed summary same pins"} {
		t.Run(which, func(t *testing.T) {
			c, s, r := ready(t)
			// Restrict nominations to one skill so distractor-only embeddings cannot
			// obscure whether the stale protected record reached the callback.
			for ref := range c.Records {
				if ref.Kind == ports.KindSkill && ref.ID != inboxbriefing.SkillID {
					delete(c.Records, ref)
				}
			}
			ref := fixture.Ref(ports.KindSkill, inboxbriefing.SkillID)
			old := c.Records[ref]
			current := old
			if which == "revoked" {
				current.State = "revoked"
			} else {
				current.Metadata = []byte(`{"logical_name":"changed authoritative metadata"}`)
			}
			c.Records[ref] = current
			s.Catalog = staleSearch{c, old}
			embed := &embedding{model: "unit-only-v1"}
			s.Embedder = embed
			s.ModelVersion = embed.model
			r.Mode = preparation.Semantic
			out, _ := s.Prepare(context.Background(), r)
			if embed.calls != 0 || len(out.Candidates) != 0 || out.Selected != nil {
				t.Fatal("stale/unauthenticated description reached embedding or exposure")
			}
		})
	}
}

func TestHybridUsesUnionAndRetainsLexicalHitsOmittedBySemanticThreshold(t *testing.T) {
	_, s, r := ready(t)
	embed := &embedding{model: "controlled-unit-v1", omit: true}
	s.Embedder = embed
	s.ModelVersion = embed.model
	r.Mode = preparation.Semantic
	out, e := s.Prepare(context.Background(), r)
	if e != nil || out.Status != "no_match" {
		t.Fatalf("unit semantic arm unexpectedly matched: %+v %v", out, e)
	}
	r.Mode = preparation.Hybrid
	out, e = s.Prepare(context.Background(), r)
	if e != nil || out.Status != "ready" || out.Selected == nil {
		t.Fatalf("hybrid dropped exact lexical coverage: %+v %v", out, e)
	}
}

func TestV2TypedDocumentAliasPreservesCompleteSchemas(t *testing.T) {
	for _, which := range []string{"valid", "wrong alias", "tampered bytes", "revoked"} {
		t.Run(which, func(t *testing.T) {
			c, s, r := ready(t)
			for ref, record := range c.Records {
				if ref.Kind != ports.KindSkill {
					record.DocumentDigest = fixture.Hash(record.Metadata)
					record.ManifestDigest = ports.Digest{}
					c.Records[ref] = record
				}
			}
			tool := fixture.Ref(ports.KindTool, "synthetic/mail-read")
			record := c.Records[tool]
			switch which {
			case "wrong alias":
				record.DocumentDigest.Value = strings.Repeat("b", 64)
				c.Records[tool] = record
			case "tampered bytes":
				c.Bytes[tool] = []byte("altered typed document")
			case "revoked":
				record.State = "revoked"
				c.Records[tool] = record
			}
			out, e := s.Prepare(context.Background(), r)
			if which == "valid" {
				if e != nil || out.Status != "ready" || out.Selected == nil {
					t.Fatalf("valid v2 aliases rejected: %+v %v", out, e)
				}
				for _, contract := range out.Selected.Contracts {
					if contract.Ref == tool && !bytes.Equal(contract.Bytes, c.Bytes[tool]) {
						t.Fatal("exact v2 tool schemas changed")
					}
				}
			} else if out.Status == "ready" || out.Selected != nil {
				t.Fatalf("invalid v2 alias/content accepted: %+v %v", out, e)
			}
		})
	}
}

func TestUnsupportedSensitiveIntentNeverReachesEmbedding(t *testing.T) {
	_, s, r := ready(t)
	embed := &embedding{model: "controlled-unit-v1"}
	s.Embedder = embed
	s.ModelVersion = embed.model
	r.Mode = preparation.Semantic
	r.Intent = "Find secret credentials in my mailbox"
	out, e := s.Prepare(context.Background(), r)
	if e != nil || out.Status != "unsupported" || embed.calls != 0 {
		t.Fatal("sensitive unsupported intent disclosed to adapter")
	}
}
func TestMissingToolSchemaNeverBecomesPartialContext(t *testing.T) {
	c, s, r := ready(t)
	tool := fixture.Ref(ports.KindTool, "synthetic/mail-read")
	record := c.Records[tool]
	var metadata map[string]any
	if e := json.Unmarshal(record.Metadata, &metadata); e != nil {
		t.Fatal(e)
	}
	delete(metadata, "output_schema")
	raw, e := json.Marshal(metadata)
	if e != nil {
		t.Fatal(e)
	}
	record.Metadata = raw
	record.Digest = fixture.Hash(raw)
	record.ManifestDigest = record.Digest
	c.Records[tool] = record
	c.Bytes[tool] = raw
	out, e := s.Prepare(context.Background(), r)
	if e == nil || out.Selected != nil {
		t.Fatal("missing required output schema exposed as ready context")
	}
}
