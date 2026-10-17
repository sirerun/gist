// Package preparation assembles complete, currently authorized catalog context.
// It owns neither inference runtime nor credentials, admission or execution.
package preparation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/sirerun/gist/catalog/discovery"
	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/resolution"
	"github.com/sirerun/gist/catalog/skills/inboxbriefing"
)

type Mode string

const (
	Lexical  Mode = "lexical"
	Semantic Mode = "semantic"
	Hybrid   Mode = "hybrid"
)

type Fallback string

const (
	Deny            Fallback = "deny"
	LexicalFallback Fallback = "lexical"
)
const SourceVersion = "preparation-v1"
const RecipeVersion = "inbox-v1"

type Request struct {
	Principal      ports.Principal
	Intent         string
	RuntimeID      string
	Mode           Mode
	Guide          bool
	Fallback       Fallback
	MaxBytes       int
	TopK           int
	CatalogVersion string
	RecipeVersion  string
}
type EmbeddingRequest struct {
	Principal    ports.Principal
	ModelVersion string
	Purpose      string
	Texts        []string
}
type EmbeddingResult struct {
	ModelVersion string
	Vectors      [][]float64
}
type Embedder interface {
	Embed(context.Context, EmbeddingRequest) (EmbeddingResult, error)
}
type Resolver interface {
	Resolve(context.Context, resolution.Request) (resolution.Result, error)
}
type Service struct {
	Catalog      ports.CatalogStore
	Authorizer   ports.Authorizer
	Retriever    packages.Retriever
	Resolver     Resolver
	Embedder     Embedder
	ModelVersion string
}
type Candidate struct {
	Ref   ports.ArtifactRef
	Score float64
}
type Contract struct {
	Ref    ports.ArtifactRef
	Digest ports.Digest
	Bytes  []byte
}
type Bundle struct {
	Skill      packages.Package
	Resolution resolution.Result
	Contracts  []Contract
	Guide      string
}
type Provenance struct {
	SourceVersion, CatalogVersion, ModelVersion, RecipeVersion, WorkspaceID string
	PolicyGeneration                                                        uint64
	SkillPin                                                                ports.ArtifactPin
	PackageDigest                                                           string
	Contracts                                                               []ContractPin
}
type ContractPin struct {
	Ref    ports.ArtifactRef
	Digest ports.Digest
}
type Result struct {
	Status     string
	Candidates []Candidate
	Selected   *Bundle
	Findings   []string
	Mode       Mode
	Provenance Provenance
}

// Prepare never treats ranking as permission or semantic capability proof. All
// returned schemas/assets are complete, bounded and freshly authorized. It has
// no persistent cache; model/catalog/recipe/scope changes cannot reuse old state.
func (s Service) Prepare(ctx context.Context, req Request) (Result, error) {
	out := Result{Status: "no_match", Mode: req.Mode, Provenance: Provenance{SourceVersion: SourceVersion, CatalogVersion: req.CatalogVersion, ModelVersion: s.ModelVersion, RecipeVersion: req.RecipeVersion, WorkspaceID: req.Principal.WorkspaceID, PolicyGeneration: req.Principal.PolicyGeneration}}
	if s.Catalog == nil || s.Authorizer == nil || s.Resolver == nil || req.Principal.WorkspaceID == "" || req.Principal.Subject == "" || req.Principal.PolicyGeneration == 0 || strings.TrimSpace(req.Intent) == "" || len(req.Intent) > 8192 || req.MaxBytes <= 0 || req.MaxBytes > 65536 || req.CatalogVersion == "" || req.RuntimeID == "" || req.RecipeVersion != RecipeVersion || req.TopK <= 0 || req.TopK > 16 || (req.Mode != Lexical && req.Mode != Semantic && req.Mode != Hybrid) || (req.Fallback != Deny && req.Fallback != LexicalFallback) {
		return Result{}, errors.New("preparation: invalid or unversioned request")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	page, err := s.Catalog.Search(ctx, ports.SearchQuery{Principal: req.Principal, Kinds: []ports.ArtifactKind{ports.KindSkill}, Limit: 100, MaxBytes: 65536})
	if err != nil {
		return Result{}, err
	}
	if page.Next.ID != "" || len(page.Records) > 100 {
		return Result{}, errors.New("preparation: complete candidate scan exceeds budget")
	}
	records := map[ports.ArtifactRef]ports.CatalogRecord{}
	for _, record := range page.Records {
		if record.Ref.WorkspaceID != req.Principal.WorkspaceID {
			return Result{}, errors.New("preparation: foreign workspace in index")
		}
		if record.Ref.Kind != ports.KindSkill || (record.State != "published" && record.State != "deprecated") {
			continue
		}
		decision, e := s.Authorizer.Decide(ctx, req.Principal, ports.ActionRead, &record.Ref)
		if e != nil {
			return Result{}, e
		}
		if !decision.Allowed {
			continue
		}
		records[record.Ref] = record
	}
	if len(records) == 0 {
		return out, nil
	}
	lex, err := discovery.New(s.Catalog, s.Authorizer)
	if err != nil {
		return Result{}, err
	}
	lexical, e := lex.Search(ctx, discovery.Request{Principal: req.Principal, Query: req.Intent, Kinds: []ports.ArtifactKind{ports.KindSkill}, Limit: 100, MaxBytes: 65536})
	if e != nil {
		return Result{}, e
	}
	lexRanks := map[ports.ArtifactRef]int{}
	for i, candidate := range lexical.Candidates {
		if _, ok := records[candidate.Ref]; ok {
			lexRanks[candidate.Ref] = i + 1
		}
	}
	ranked := []Candidate{}
	mode := req.Mode
	if mode != Lexical {
		ranked, err = s.semantic(ctx, req, records)
		if err != nil {
			if req.Fallback != LexicalFallback {
				out.Status = "unsupported"
				out.Findings = []string{"embedding unavailable: " + err.Error()}
				return out, nil
			}
			mode = Lexical
			out.Mode = Lexical
			out.Findings = append(out.Findings, "explicit lexical fallback; embedding unavailable")
		}
	}
	if mode == Lexical {
		for ref, rank := range lexRanks {
			ranked = append(ranked, Candidate{ref, 1 / float64(rank)})
		}
	}
	if mode == Hybrid {
		for i := range ranked {
			ranked[i].Score = 1 / float64(60+i+1)
			if rank, ok := lexRanks[ranked[i].Ref]; ok {
				ranked[i].Score += 1 / float64(60+rank)
			}
		}
	}
	sortCandidates(ranked)
	if len(ranked) > req.TopK {
		ranked = ranked[:req.TopK]
	}
	for _, candidate := range ranked {
		if err := s.recheck(ctx, req.Principal, records[candidate.Ref]); err != nil {
			return Result{}, err
		}
	}
	out.Candidates = ranked
	if len(ranked) == 0 {
		return out, nil
	}
	if len(ranked) > 1 && math.Abs(ranked[0].Score-ranked[1].Score) < 1e-12 {
		out.Status = "requires_selection"
		return out, nil
	}
	chosen := ranked[0].Ref
	// Only maintained recipes establish covered workflows. Similar-looking
	// unsupported requests are never promoted to a general-purpose planner.
	if chosen.ID != inboxbriefing.SkillID || chosen.Version != inboxbriefing.Version || unsupportedIntent(req.Intent) {
		out.Status = "unsupported"
		out.Findings = append(out.Findings, "no maintained recipe covers this intent")
		return out, nil
	}
	record := records[chosen]
	resolved, err := s.Resolver.Resolve(ctx, resolution.Request{Principal: req.Principal, Skill: chosen, RuntimeID: req.RuntimeID, OwnedConnections: true, MaxBytes: req.MaxBytes, Lease: time.Minute})
	if err != nil {
		return Result{}, err
	}
	if resolved.Status != resolution.StatusReady {
		out.Status = "requires_connection"
		for _, finding := range resolved.Findings {
			if finding.Required && finding.Status == resolution.StatusUnsupportedRuntime {
				out.Status = "unsupported"
			}
		}
		out.Findings = append(out.Findings, "exact current capability closure is incomplete")
		return out, nil
	}
	pin := ports.ArtifactPin{Ref: chosen, Digest: record.Digest, ManifestDigest: record.ManifestDigest}
	pkg, err := s.Retriever.RetrieveSkill(ctx, req.Principal, pin, packages.Limits{MaxPackageBytes: 65536, MaxFileBytes: 65536, MaxFiles: 64})
	if err != nil {
		return Result{}, err
	}
	contracts := []Contract{}
	manifests := map[ports.ArtifactRef]ports.Digest{}
	seen := map[ports.ArtifactRef]bool{}
	for _, finding := range resolved.Findings {
		if finding.Required && finding.Status != resolution.StatusReady {
			return Result{}, errors.New("preparation: incomplete required closure")
		}
		for _, item := range finding.Closure {
			ref, e := closureRef(req.Principal.WorkspaceID, item)
			if e != nil {
				return Result{}, e
			}
			if seen[ref] {
				continue
			}
			seen[ref] = true
			if e = s.authorized(ctx, req.Principal, ref); e != nil {
				return Result{}, e
			}
			env, e := s.Retriever.Retrieve(ctx, req.Principal, ref, int64(req.MaxBytes))
			if e != nil {
				return Result{}, e
			}
			want := ports.Digest{Algorithm: item.Digest.Algorithm, Value: item.Digest.Value}
			manifest := ports.Digest{Algorithm: item.ManifestDigest.Algorithm, Value: item.ManifestDigest.Value}
			if env.Record.Digest != want || env.Record.ManifestDigest != manifest || !matches(env.Bytes, want) {
				return Result{}, errors.New("preparation: contract bytes differ from pinned resolution")
			}
			contracts = append(contracts, Contract{ref, want, append([]byte(nil), env.Bytes...)})
			manifests[ref] = manifest
		}
	}
	sort.Slice(contracts, func(i, j int) bool { return refKey(contracts[i].Ref) < refKey(contracts[j].Ref) })
	bundle := Bundle{Skill: pkg, Resolution: resolved, Contracts: contracts}
	if req.Guide {
		bundle.Guide = "Recipe inbox-v1: list unread messages, then read at most 20 selected messages; prepare the maintained briefing template. Never send, draft, label, mark read, delete, follow links or fetch attachments. Tool calls remain subject to caller approval.\nSources: " + chosen.ID + "@" + chosen.Version + "; package sha256:" + pkg.PackageDigest
	}
	// Validate current state/pins and policy again after all I/O. This detects
	// observed changes, not atomic revocation; callers recheck at actual use.
	for _, candidate := range ranked {
		if e = s.recheck(ctx, req.Principal, records[candidate.Ref]); e != nil {
			return Result{}, e
		}
	}
	for _, contract := range contracts {
		current, e := s.Catalog.Get(ctx, contract.Ref)
		if e != nil {
			return Result{}, e
		}
		if current.Ref != contract.Ref || current.ManifestDigest != manifests[contract.Ref] || current.Digest != contract.Digest || (current.State != "published" && current.State != "deprecated") {
			return Result{}, errors.New("preparation: closure changed during assembly")
		}
		if e = s.authorized(ctx, req.Principal, contract.Ref); e != nil {
			return Result{}, e
		}
	}
	refreshed, err := s.Resolver.Resolve(ctx, resolution.Request{Principal: req.Principal, Skill: chosen, RuntimeID: req.RuntimeID, OwnedConnections: true, MaxBytes: req.MaxBytes, Lease: time.Minute})
	if err != nil {
		return Result{}, err
	}
	if !sameResolution(resolved, refreshed) {
		return Result{}, errors.New("preparation: current bindings or connection changed")
	}
	out.Status = "ready"
	out.Selected = &bundle
	out.Provenance.SkillPin = pin
	out.Provenance.PackageDigest = pkg.PackageDigest
	for _, c := range contracts {
		out.Provenance.Contracts = append(out.Provenance.Contracts, ContractPin{c.Ref, c.Digest})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return Result{}, err
	}
	if len(encoded) > req.MaxBytes {
		return Result{Status: "budget_exceeded", Mode: out.Mode, Findings: []string{"complete assets and schemas exceed context budget"}, Provenance: out.Provenance}, nil
	}
	return out, nil
}
func (s Service) authorized(ctx context.Context, p ports.Principal, ref ports.ArtifactRef) error {
	d, e := s.Authorizer.Decide(ctx, p, ports.ActionRead, &ref)
	if e != nil {
		return e
	}
	if !d.Allowed {
		return errors.New("preparation: denied current artifact")
	}
	return nil
}
func (s Service) recheck(ctx context.Context, p ports.Principal, old ports.CatalogRecord) error {
	now, e := s.Catalog.Get(ctx, old.Ref)
	if e != nil {
		return e
	}
	if now.Ref != old.Ref || now.Digest != old.Digest || now.ManifestDigest != old.ManifestDigest || now.PackageDigest != old.PackageDigest || (now.State != "published" && now.State != "deprecated") {
		return errors.New("preparation: candidate changed during preparation")
	}
	return s.authorized(ctx, p, old.Ref)
}
func (s Service) semantic(ctx context.Context, req Request, records map[ports.ArtifactRef]ports.CatalogRecord) ([]Candidate, error) {
	if s.Embedder == nil || s.ModelVersion == "" {
		return nil, errors.New("no explicit embedding adapter/model")
	}
	refs := make([]ports.ArtifactRef, 0, len(records))
	for ref := range records {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refKey(refs[i]) < refKey(refs[j]) })
	texts := make([]string, len(refs))
	for i, ref := range refs {
		var meta struct {
			LogicalName string `json:"logical_name"`
			Summary     string `json:"summary"`
		}
		if e := json.Unmarshal(records[ref].Metadata, &meta); e != nil {
			return nil, e
		}
		texts[i] = meta.LogicalName + " " + meta.Summary
		if len(texts[i]) > 8192 {
			return nil, errors.New("catalog description exceeds embedding input budget")
		}
	}
	docs, e := s.Embedder.Embed(ctx, EmbeddingRequest{req.Principal, s.ModelVersion, "document", texts})
	if e != nil {
		return nil, e
	}
	query, e := s.Embedder.Embed(ctx, EmbeddingRequest{req.Principal, s.ModelVersion, "query", []string{req.Intent}})
	if e != nil {
		return nil, e
	}
	if docs.ModelVersion != s.ModelVersion || query.ModelVersion != s.ModelVersion || len(docs.Vectors) != len(refs) || len(query.Vectors) != 1 {
		return nil, errors.New("embedding model/index identity mismatch")
	}
	result := []Candidate{}
	for i, ref := range refs {
		score, e := cosine(query.Vectors[0], docs.Vectors[i])
		if e != nil {
			return nil, e
		}
		if score >= 0.30 {
			result = append(result, Candidate{ref, score})
		}
	}
	sortCandidates(result)
	return result, nil
}
func cosine(a, b []float64) (float64, error) {
	if len(a) != len(b) || (len(a) != 256 && len(a) != 768) {
		return 0, errors.New("embedding dimension mismatch")
	}
	var dot, aa, bb float64
	for i, x := range a {
		y := b[i]
		if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
			return 0, errors.New("invalid embedding value")
		}
		dot += x * y
		aa += x * x
		bb += y * y
	}
	if aa == 0 || bb == 0 || math.IsInf(aa, 0) || math.IsInf(bb, 0) {
		return 0, errors.New("zero embedding")
	}
	return dot / math.Sqrt(aa*bb), nil
}
func sortCandidates(items []Candidate) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return refKey(items[i].Ref) < refKey(items[j].Ref)
	})
}
func refKey(ref ports.ArtifactRef) string { return string(ref.Kind) + ":" + ref.ID + "@" + ref.Version }
func closureRef(workspace string, pin resolution.ArtifactPin) (ports.ArtifactRef, error) {
	id, version, ok := strings.Cut(pin.Reference, "@")
	if !ok || id == "" || version == "" || strings.Contains(version, "@") {
		return ports.ArtifactRef{}, errors.New("invalid closure reference")
	}
	return ports.ArtifactRef{WorkspaceID: workspace, Kind: ports.ArtifactKind(pin.Kind), ID: id, Version: version}, nil
}
func matches(raw []byte, digest ports.Digest) bool {
	sum := sha256.Sum256(raw)
	return digest.Algorithm == "sha256" && digest.Value == hex.EncodeToString(sum[:])
}
func sameResolution(a, b resolution.Result) bool {
	if a.Status != b.Status || a.Skill != b.Skill {
		return false
	}
	x, _ := json.Marshal(a.Findings)
	y, _ := json.Marshal(b.Findings)
	return string(x) == string(y)
}
func unsupportedIntent(intent string) bool {
	lower := strings.ToLower(intent)
	for _, word := range []string{"send ", "draft ", "delete ", "label ", "mark read", "attachment", "follow link", "click ", "execute ", "password", "secret"} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}
