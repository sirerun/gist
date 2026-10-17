package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/sirerun/gist/catalog/composition"
	"github.com/sirerun/gist/catalog/ports"
	lexical "github.com/sirerun/gist/internal/cataloglexical"
)

const (
	MaxIndexQueryBytes    = 1024
	MaxIndexQueryTerms    = 32
	MaxIndexExamined      = 4096
	MaxIndexResults       = 64
	MaxIndexResponseBytes = 256 << 10
	MaxIndexMetadataBytes = 256 << 10
)

var ErrIndexGeneration = errors.New("catalog index generation changed")

// IndexScope binds a physically separate index to one complete policy snapshot.
// RuntimeID also scopes caller-owned connection and binding admissions.
type IndexScope struct {
	Principal ports.Principal
	RuntimeID string
}

// IndexPolicy must reject stale or revoked scope/connection admission state.
// It is checked before and after successful reads, including empty/incomplete
// outcomes. PolicyGeneration must advance on connection/admission authority
// changes as well as catalog grants; the gate must reject the old scope. A new
// authority scope requires a new index. Content updates never renew authority.
// It is not an authority lease; callers recheck at actual tool use.
type IndexPolicy interface {
	Check(context.Context, IndexScope) error
}

// IndexedBinding is trusted caller admission evidence, never derived from names
// or lexical matches. Composition verifies the exact retrieved closure again.
type IndexedBinding struct {
	Requirement composition.Requirement
	Candidate   composition.Candidate
}

type IndexEntry struct {
	Record   ports.CatalogRecord
	Bindings []IndexedBinding
}

type IndexSearch struct {
	Scope                        IndexScope
	Generation                   uint64 // zero accepts the current coherent generation
	Query                        string
	Kinds                        []ports.ArtifactKind
	Tags                         []string
	Limit, MaxExamined, MaxBytes int
}

type IndexResult struct {
	Status     string      `json:"status"`
	Generation uint64      `json:"generation"`
	Candidates []Candidate `json:"candidates"`
}

type capabilityKey struct{ ID, Version, Digest string }
type indexedEntry struct {
	candidate Candidate
	terms     []string
	lexical   lexical.Index
	bindings  []IndexedBinding
}
type indexSnapshot struct {
	generation uint64
	entries    map[ports.ArtifactRef]*indexedEntry
	postings   map[string]map[ports.ArtifactRef]struct{}
	bindings   map[capabilityKey][]composition.Candidate
}

// ScopedIndex is derived summary data. It never retains raw catalog documents
// or tool schemas, and does not implement/replace the artifact store. No result
// cache is used. Current and previous immutable snapshots support one rollback.
type ScopedIndex struct {
	mu                sync.RWMutex
	scope             IndexScope
	auth              Authorizer
	policy            IndexPolicy
	current, previous *indexSnapshot
}

func NewScopedIndex(ctx context.Context, scope IndexScope, auth Authorizer, policy IndexPolicy, entries []IndexEntry) (*ScopedIndex, error) {
	if auth == nil || policy == nil || scope.Principal.WorkspaceID == "" || scope.Principal.Subject == "" || scope.RuntimeID == "" {
		return nil, errors.New("index requires scope, runtime, authorizer and current policy")
	}
	scope = cloneIndexScope(scope)
	x := &ScopedIndex{scope: scope, auth: auth, policy: policy}
	s, err := x.build(ctx, entries)
	if err != nil {
		return nil, err
	}
	s.generation = 1
	x.current = s
	return x, nil
}

func cloneIndexScope(s IndexScope) IndexScope {
	s.Principal.Scopes = append([]string(nil), s.Principal.Scopes...)
	return s
}
func scopeKey(s IndexScope) string {
	s = cloneIndexScope(s)
	sort.Strings(s.Principal.Scopes)
	b, _ := json.Marshal(s)
	return string(b)
}
func (x *ScopedIndex) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return x.policy.Check(ctx, cloneIndexScope(x.scope))
}
func (x *ScopedIndex) begin(ctx context.Context, scope IndexScope, generation uint64) (*indexSnapshot, error) {
	if scopeKey(scope) != scopeKey(x.scope) {
		return nil, errors.New("index scope mismatch")
	}
	if err := x.check(ctx); err != nil {
		return nil, err
	}
	x.mu.RLock()
	s := x.current
	x.mu.RUnlock()
	if generation != 0 && s.generation != generation {
		return nil, ErrIndexGeneration
	}
	return s, nil
}
func (x *ScopedIndex) finish(ctx context.Context, s *indexSnapshot) error {
	if err := x.check(ctx); err != nil {
		return err
	}
	x.mu.RLock()
	same := x.current == s
	x.mu.RUnlock()
	if !same {
		return ErrIndexGeneration
	}
	return nil
}
func (x *ScopedIndex) authorize(ctx context.Context, ref ports.ArtifactRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	decision, err := x.auth.Decide(ctx, cloneIndexScope(x.scope).Principal, ports.ActionRead, &ref)
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return errors.New("index authorization revoked or denied")
	}
	return nil
}
func keyFor(r composition.Requirement) capabilityKey {
	return capabilityKey{r.CapabilityID, r.ContractVersion, r.ContractDigest}
}
func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (x *ScopedIndex) analyze(ctx context.Context, e IndexEntry) (*indexedEntry, error) {
	r := e.Record
	if r.Ref.WorkspaceID != x.scope.Principal.WorkspaceID || r.Ref.ID == "" || r.Ref.Version == "" || r.Ref.Kind == "" {
		return nil, errors.New("invalid indexed reference")
	}
	if r.State != "published" && r.State != "deprecated" {
		return nil, errors.New("index requires published or deprecated records")
	}
	if len(r.Metadata) > MaxIndexMetadataBytes {
		return nil, errors.New("indexed metadata exceeds limit")
	}
	if err := x.authorize(ctx, r.Ref); err != nil {
		return nil, err
	}
	c, err := candidateFromRecord(r)
	if err != nil {
		return nil, err
	}
	d, err := lexical.DocumentFromMetadata(string(r.Ref.Kind), r.Ref.ID, r.Metadata)
	if err != nil {
		return nil, err
	}
	texts := []string{d.ID, d.LogicalName, d.Kind, d.Description}
	texts = append(texts, d.Capabilities...)
	texts = append(texts, d.Taxonomy...)
	terms := lexical.Query(strings.Join(texts, " "))
	if len(e.Bindings) > MaxIndexResults {
		return nil, errors.New("too many connection admissions per binding record")
	}
	for _, b := range e.Bindings {
		v := b.Candidate
		if r.Ref.Kind != ports.KindBinding || v.Binding.Ref != r.Ref || v.Binding.Digest != r.Digest || !v.Admitted || v.ContractDigest != b.Requirement.ContractDigest || !validDigest(v.ContractDigest) || b.Requirement.CapabilityID == "" || b.Requirement.ContractVersion == "" || v.Tool.Ref.WorkspaceID != r.Ref.WorkspaceID || v.Provider.Ref.WorkspaceID != r.Ref.WorkspaceID {
			return nil, errors.New("invalid caller binding evidence")
		}
		if v.Tool.Ref.Kind != ports.KindTool || v.Provider.Ref.Kind != ports.KindProvider || v.Tool.Ref.ID == "" || v.Tool.Ref.Version == "" || v.Provider.Ref.ID == "" || v.Provider.Ref.Version == "" {
			return nil, errors.New("invalid binding closure reference")
		}
		for _, ref := range []ports.ArtifactRef{v.Tool.Ref, v.Provider.Ref} {
			if err := x.authorize(ctx, ref); err != nil {
				return nil, err
			}
		}
	}
	return &indexedEntry{candidate: c, terms: terms, lexical: lexical.NewIndex(d), bindings: append([]IndexedBinding(nil), e.Bindings...)}, nil
}
func emptySnapshot() *indexSnapshot {
	return &indexSnapshot{entries: map[ports.ArtifactRef]*indexedEntry{}, postings: map[string]map[ports.ArtifactRef]struct{}{}, bindings: map[capabilityKey][]composition.Candidate{}}
}
func (x *ScopedIndex) build(ctx context.Context, entries []IndexEntry) (*indexSnapshot, error) {
	if err := x.check(ctx); err != nil {
		return nil, err
	}
	s := emptySnapshot()
	for _, e := range entries {
		if _, ok := s.entries[e.Record.Ref]; ok {
			return nil, errors.New("duplicate indexed reference")
		}
		v, err := x.analyze(ctx, e)
		if err != nil {
			return nil, err
		}
		s.entries[e.Record.Ref] = v
		for _, term := range v.terms {
			if s.postings[term] == nil {
				s.postings[term] = map[ports.ArtifactRef]struct{}{}
			}
			s.postings[term][e.Record.Ref] = struct{}{}
		}
		for _, b := range v.bindings {
			k := keyFor(b.Requirement)
			s.bindings[k] = append(s.bindings[k], b.Candidate)
		}
	}
	if err := x.check(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Exact finds a selected artifact summary without lexical ranking or its budget.
func (x *ScopedIndex) Exact(ctx context.Context, scope IndexScope, ref ports.ArtifactRef) (Candidate, bool, error) {
	s, err := x.begin(ctx, scope, 0)
	if err != nil {
		return Candidate{}, false, err
	}
	if ref.WorkspaceID != scope.Principal.WorkspaceID {
		return Candidate{}, false, errors.New("reference scope mismatch")
	}
	e, ok := s.entries[ref]
	if ok {
		if err = x.authorize(ctx, ref); err != nil {
			return Candidate{}, false, err
		}
	}
	if err = x.finish(ctx, s); err != nil {
		return Candidate{}, false, err
	}
	if !ok {
		return Candidate{}, false, nil
	}
	return cloneCandidate(e.candidate), true, nil
}
func cloneCandidate(c Candidate) Candidate { c.Tags = append([]string(nil), c.Tags...); return c }

// Search uses inverted postings and the existing lexical ranking model. It
// refuses incomplete ranking instead of returning an arbitrary bounded prefix.
func (x *ScopedIndex) Search(ctx context.Context, q IndexSearch) (IndexResult, error) {
	s, err := x.begin(ctx, q.Scope, q.Generation)
	if err != nil {
		return IndexResult{}, err
	}
	if len(q.Query) > MaxIndexQueryBytes || q.Limit <= 0 || q.Limit > MaxIndexResults || q.MaxExamined <= 0 || q.MaxExamined > MaxIndexExamined || q.MaxBytes <= 0 || q.MaxBytes > MaxIndexResponseBytes || len(q.Kinds) > 64 || len(q.Tags) > 64 {
		return IndexResult{}, errors.New("invalid index search budget")
	}
	terms := lexical.Query(q.Query)
	if len(terms) > MaxIndexQueryTerms {
		return IndexResult{}, errors.New("too many query terms")
	}
	out := IndexResult{Status: "complete", Generation: s.generation, Candidates: []Candidate{}}
	complete := func() (IndexResult, error) {
		if err := x.finish(ctx, s); err != nil {
			return IndexResult{}, err
		}
		b, err := json.Marshal(out)
		if err != nil {
			return IndexResult{}, err
		}
		if len(b) > q.MaxBytes {
			return IndexResult{}, errors.New("index response budget too small")
		}
		return out, nil
	}
	seen := map[ports.ArtifactRef]bool{}
	examined := 0
	for _, term := range terms {
		for ref := range s.postings[term] {
			if err := ctx.Err(); err != nil {
				return IndexResult{}, err
			}
			if examined == q.MaxExamined {
				out.Status = "incomplete_budget"
				return complete()
			}
			examined++
			seen[ref] = true
		}
	}
	ranked := make([]rankedCandidate, 0, len(seen))
	for ref := range seen {
		if err := x.authorize(ctx, ref); err != nil {
			return IndexResult{}, err
		}
		e := s.entries[ref]
		if matchesFilters(e.candidate, Request{Kinds: q.Kinds, Tags: q.Tags}) {
			ranked = append(ranked, rankedCandidate{Candidate: e.candidate, match: e.lexical.Score(terms)})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if c := ranked[i].match.Better(ranked[j].match); c != 0 {
			return c > 0
		}
		a, b := ranked[i].Ref, ranked[j].Ref
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Version < b.Version
	})
	for _, r := range ranked {
		if len(out.Candidates) == q.Limit {
			break
		}
		out.Candidates = append(out.Candidates, cloneCandidate(r.Candidate))
	}
	b, err := json.Marshal(out)
	if err != nil {
		return IndexResult{}, err
	}
	if len(b) > q.MaxBytes {
		out.Status = "incomplete_budget"
		out.Candidates = []Candidate{}
	}
	return complete()
}

// Lookup implements composition.BindingIndex. Over-budget results contain no
// candidates; composition cannot interpret a truncated set as a unique choice.
func (x *ScopedIndex) Lookup(ctx context.Context, q composition.Query) (composition.Candidates, error) {
	s, err := x.begin(ctx, IndexScope{Principal: q.Principal, RuntimeID: q.RuntimeID}, 0)
	if err != nil {
		return composition.Candidates{}, err
	}
	if q.Limit <= 0 || q.Limit > MaxIndexResults {
		return composition.Candidates{}, errors.New("invalid binding limit")
	}
	rows := s.bindings[keyFor(q.Requirement)]
	out := composition.Candidates{Complete: len(rows) <= q.Limit}
	if out.Complete {
		for _, v := range rows {
			for _, ref := range []ports.ArtifactRef{v.Binding.Ref, v.Tool.Ref, v.Provider.Ref} {
				if err = x.authorize(ctx, ref); err != nil {
					return composition.Candidates{}, err
				}
			}
			out.Items = append(out.Items, v)
		}
		sort.Slice(out.Items, func(i, j int) bool {
			a, _ := json.Marshal(out.Items[i])
			b, _ := json.Marshal(out.Items[j])
			return string(a) < string(b)
		})
	}
	if err = x.finish(ctx, s); err != nil {
		return composition.Candidates{}, err
	}
	return out, nil
}

// Rebuild analyzes a full replacement and atomically publishes it only if the
// caller's expected generation is still current. It never merges concurrent edits.
func (x *ScopedIndex) Rebuild(ctx context.Context, expected uint64, entries []IndexEntry) (uint64, error) {
	s, err := x.build(ctx, entries)
	if err != nil {
		return 0, err
	}
	return x.publish(ctx, expected, s)
}
func (x *ScopedIndex) publish(ctx context.Context, expected uint64, s *indexSnapshot) (uint64, error) {
	if err := x.check(ctx); err != nil {
		return 0, err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.current.generation != expected || expected == ^uint64(0) {
		return 0, ErrIndexGeneration
	}
	s.generation = expected + 1
	x.previous = x.current
	x.current = s
	return s.generation, nil
}

// Update reanalyzes only changed entries. Top-level maps are O(N) shallow copies;
// only affected posting sets and binding buckets are copied. It is not O(changes).
func (x *ScopedIndex) Update(ctx context.Context, expected uint64, upserts []IndexEntry, remove []ports.ArtifactRef) (uint64, error) {
	old, err := x.begin(ctx, x.scope, expected)
	if err != nil {
		return 0, err
	}
	changed := map[ports.ArtifactRef]*indexedEntry{}
	for _, ref := range remove {
		if ref.WorkspaceID != x.scope.Principal.WorkspaceID {
			return 0, errors.New("remove scope mismatch")
		}
		changed[ref] = nil
	}
	for _, e := range upserts {
		if _, ok := changed[e.Record.Ref]; ok {
			return 0, errors.New("duplicate update reference")
		}
		v, err := x.analyze(ctx, e)
		if err != nil {
			return 0, err
		}
		changed[e.Record.Ref] = v
	}
	s := emptySnapshot()
	for k, v := range old.entries {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		s.entries[k] = v
	}
	for k, v := range old.postings {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		s.postings[k] = v
	}
	for k, v := range old.bindings {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		s.bindings[k] = v
	}
	touchedTerms := map[string]bool{}
	touchedBindings := map[capabilityKey]bool{}
	for ref, v := range changed {
		if e := old.entries[ref]; e != nil {
			for _, t := range e.terms {
				touchedTerms[t] = true
			}
			for _, b := range e.bindings {
				touchedBindings[keyFor(b.Requirement)] = true
			}
		}
		if v == nil {
			delete(s.entries, ref)
		} else {
			s.entries[ref] = v
			for _, t := range v.terms {
				touchedTerms[t] = true
			}
			for _, b := range v.bindings {
				touchedBindings[keyFor(b.Requirement)] = true
			}
		}
	}
	for t := range touchedTerms {
		set := map[ports.ArtifactRef]struct{}{}
		for ref := range old.postings[t] {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			if _, ok := changed[ref]; !ok {
				set[ref] = struct{}{}
			}
		}
		for ref, v := range changed {
			if v != nil {
				for _, term := range v.terms {
					if term == t {
						set[ref] = struct{}{}
						break
					}
				}
			}
		}
		if len(set) == 0 {
			delete(s.postings, t)
		} else {
			s.postings[t] = set
		}
	}
	for k := range touchedBindings {
		rows := []composition.Candidate{}
		for _, v := range old.bindings[k] {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			if _, ok := changed[v.Binding.Ref]; !ok {
				rows = append(rows, v)
			}
		}
		for _, v := range changed {
			if v != nil {
				for _, b := range v.bindings {
					if keyFor(b.Requirement) == k {
						rows = append(rows, b.Candidate)
					}
				}
			}
		}
		if len(rows) == 0 {
			delete(s.bindings, k)
		} else {
			s.bindings[k] = rows
		}
	}
	return x.publish(ctx, expected, s)
}

// Rollback restores prior content under a NEW generation after current policy
// validation. It never restores old authority; read policy checks remain required.
func (x *ScopedIndex) Rollback(ctx context.Context, expected uint64) (uint64, error) {
	if err := x.check(ctx); err != nil {
		return 0, err
	}
	x.mu.RLock()
	if x.current.generation != expected {
		x.mu.RUnlock()
		return 0, ErrIndexGeneration
	}
	p := x.previous
	x.mu.RUnlock()
	if p == nil {
		return 0, fmt.Errorf("no previous index generation")
	}
	s := &indexSnapshot{entries: p.entries, postings: p.postings, bindings: p.bindings}
	return x.publish(ctx, expected, s)
}

var _ composition.BindingIndex = (*ScopedIndex)(nil)
