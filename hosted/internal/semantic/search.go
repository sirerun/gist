// Package semantic adds bounded server-side retrieval over public directory
// metadata. It never receives workspace records, credentials or tool payloads.
package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/catalog/preparation"
)

const floor = 0.30
const maxDocuments = 10000
const queryTimeout = 8 * time.Second

type Embedder interface {
	preparation.Embedder
	ModelVersion() string
}
type document struct{ id, text string }
type snapshot struct {
	docs    []document
	vectors [][]float64
	version string
}
type cached struct {
	vector  []float64
	expires time.Time
}
type Service struct {
	index    *directory.Index
	apps     *directory.AppIndex
	embedder Embedder
	healthy  func() bool
	ready    atomic.Pointer[snapshot]
	slot     chan struct{}
	mu       sync.Mutex
	cache    map[[32]byte]cached
}

func New(index *directory.Index, apps *directory.AppIndex, embedder Embedder, healthy func() bool) *Service {
	return &Service{index: index, apps: apps, embedder: embedder, healthy: healthy, slot: make(chan struct{}, 1), cache: map[[32]byte]cached{}}
}

// Warm builds one immutable model/corpus generation. Until it completes,
// callers retain lexical retrieval. The caller owns cancellation and lifecycle.
func (s *Service) Warm(ctx context.Context) error {
	if s.embedder == nil {
		return errors.New("semantic: no embedder")
	}
	docs, err := s.documents()
	if err != nil {
		return err
	}
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	case <-ctx.Done():
		return ctx.Err()
	}
	snap := &snapshot{docs: docs, version: s.embedder.ModelVersion() + "\n" + s.index.Revision()}
	for start := 0; start < len(docs); start += 8 {
		end := min(start+8, len(docs))
		texts := make([]string, end-start)
		for j, d := range docs[start:end] {
			texts[j] = d.text
		}
		result, err := s.embedder.Embed(ctx, preparation.EmbeddingRequest{Purpose: "document", Texts: texts, ModelVersion: s.embedder.ModelVersion()})
		if err != nil {
			return fmt.Errorf("semantic: document batch %d failed: %w", start/8, err)
		}
		if result.ModelVersion != s.embedder.ModelVersion() || len(result.Vectors) != len(texts) {
			return errors.New("semantic: invalid document identity/count")
		}
		for _, v := range result.Vectors {
			if !valid(v) {
				return errors.New("semantic: invalid document vector")
			}
			snap.vectors = append(snap.vectors, append([]float64(nil), v...))
		}
	}
	snap.version = snapshotGeneration(s.embedder.ModelVersion(), s.index.Revision(), docs)
	s.ready.Store(snap)
	return nil
}
func (s *Service) documents() ([]document, error) {
	docs := []document{}
	for cursor := ""; ; {
		page, err := s.index.Search(directory.Query{Limit: 100, Cursor: cursor})
		if err != nil {
			return nil, err
		}
		for _, r := range page.Items {
			// Reproject to public fields even if a caller accidentally passes a raw index.
			p := directory.PublicRecordOf(r)
			text := p.Title + ". " + p.Description + ". " + strings.Join(p.Tags, " ")
			for _, skill := range p.Skills {
				text += ". " + skill.Title
			}
			docs = append(docs, document{"entry:" + p.ID, text})
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	for offset := 0; ; {
		page, err := s.apps.Search(directory.AppQuery{Limit: 100, Offset: offset})
		if err != nil {
			return nil, err
		}
		for _, a := range page.Items {
			text := a.Name + ". " + a.Description
			for _, c := range a.Categories {
				text += ". " + c.Name
			}
			docs = append(docs, document{"app:" + a.Slug, text})
		}
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	if len(docs) > maxDocuments {
		return nil, errors.New("semantic: corpus limit exceeded")
	}
	return docs, nil
}
func valid(v []float64) bool {
	if len(v) != 768 {
		return false
	}
	norm := 0.0
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
		norm += x * x
	}
	return math.Abs(norm-1) < 1e-4
}
func (s *Service) vector(ctx context.Context, text string) ([]float64, *snapshot, bool) {
	snap := s.ready.Load()
	if snap == nil || s.healthy != nil && !s.healthy() {
		return nil, nil, false
	}
	key := sha256.Sum256([]byte(text))
	s.mu.Lock()
	item, ok := s.cache[key]
	s.mu.Unlock()
	if ok && time.Now().Before(item.expires) {
		return item.vector, snap, true
	}
	// Do not queue anonymous traffic behind model inference. Busy requests fall
	// back immediately; each admitted inference has a strict wall-clock bound.
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	default:
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result, err := s.embedder.Embed(ctx, preparation.EmbeddingRequest{Purpose: "query", Texts: []string{text}, ModelVersion: s.embedder.ModelVersion()})
	if err != nil || result.ModelVersion != s.embedder.ModelVersion() || len(result.Vectors) != 1 || !valid(result.Vectors[0]) {
		return nil, nil, false
	}
	v := append([]float64(nil), result.Vectors[0]...)
	s.mu.Lock()
	if len(s.cache) >= 256 {
		s.cache = map[[32]byte]cached{}
	}
	s.cache[key] = cached{v, time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	return v, snap, true
}
func rank(snap *snapshot, v []float64, prefix string, lexical []string, allowed func(string) bool) []string {
	type hit struct {
		id    string
		score float64
	}
	semantic := []hit{}
	for n, d := range snap.docs {
		if !strings.HasPrefix(d.id, prefix) || !allowed(strings.TrimPrefix(d.id, prefix)) {
			continue
		}
		dot := 0.0
		for j, x := range v {
			dot += x * snap.vectors[n][j]
		}
		if dot >= floor {
			semantic = append(semantic, hit{strings.TrimPrefix(d.id, prefix), dot})
		}
	}
	sort.Slice(semantic, func(i, j int) bool {
		if semantic[i].score != semantic[j].score {
			return semantic[i].score > semantic[j].score
		}
		return semantic[i].id < semantic[j].id
	})
	if len(semantic) > 32 {
		semantic = semantic[:32]
	}
	scores := map[string]float64{}
	for n, id := range lexical {
		scores[id] += 1 / float64(60+n+1)
	}
	for n, h := range semantic {
		scores[h.id] += 1 / float64(60+n+1)
	}
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		return ids[i] < ids[j]
	})
	return ids
}
func (s *Service) Search(ctx context.Context, q directory.Query) (directory.Result, error) {
	// Validate all caller-controlled filters/cursors before invoking inference.
	if q.Limit < 0 || q.Limit > 100 || len(q.Cursor) > 1024 {
		return directory.Result{}, errors.New("semantic: invalid query bounds")
	}
	check := q
	check.Cursor = ""
	check.Limit = 100
	lexical, err := s.index.Search(check)
	if err != nil {
		return directory.Result{}, err
	}
	if strings.TrimSpace(q.Text) == "" {
		return s.index.Search(q)
	}
	v, snap, ok := s.vector(ctx, q.Text)
	if !ok {
		r, e := s.index.Search(q)
		r.SearchMode = "lexical_fallback"
		return r, e
	}
	ids := make([]string, 0, len(lexical.Items))
	for _, r := range lexical.Items {
		ids = append(ids, r.ID)
	}
	allowed := func(id string) bool {
		r, ok := s.index.Get(id)
		if !ok || q.Kind != "" && r.Kind != q.Kind {
			return false
		}
		if strings.TrimSpace(q.Tag) == "" {
			return true
		}
		for _, tag := range r.Tags {
			if strings.EqualFold(tag, strings.TrimSpace(q.Tag)) {
				return true
			}
		}
		return false
	}
	r, e := s.index.SearchRanked(q, rank(snap, v, "entry:", ids, allowed), snap.version)
	r.SearchMode = "hybrid"
	return r, e
}
func (s *Service) SearchApps(ctx context.Context, q directory.AppQuery) (directory.AppResult, error) {
	if q.Limit < 0 || q.Limit > 100 || q.Offset < 0 || q.Offset > directory.MaxEntries {
		return directory.AppResult{}, errors.New("semantic: invalid app bounds")
	}
	check := q
	check.Offset = 0
	check.Limit = 100
	lexical, err := s.apps.Search(check)
	if err != nil {
		return directory.AppResult{}, err
	}
	if strings.TrimSpace(q.Text) == "" {
		return s.apps.Search(q)
	}
	v, snap, ok := s.vector(ctx, q.Text)
	if !ok {
		r, e := s.apps.Search(q)
		r.SearchMode = "lexical_fallback"
		return r, e
	}
	ids := make([]string, 0, len(lexical.Items))
	for _, a := range lexical.Items {
		ids = append(ids, a.Slug)
	}
	allowed := func(slug string) bool {
		a, ok := s.apps.Get(slug)
		if !ok {
			return false
		}
		if q.ConnectableOnly && !a.Connectable() {
			return false
		}
		if q.Category == "" {
			return true
		}
		for _, c := range a.Categories {
			if c.Slug == q.Category {
				return true
			}
		}
		return false
	}
	r, e := s.apps.SearchRanked(q, rank(snap, v, "app:", ids, allowed))
	r.SearchMode = "hybrid"
	return r, e
}
