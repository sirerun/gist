package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	c "github.com/sirerun/gist/catalog/composition"
	d "github.com/sirerun/gist/catalog/discovery"
	"github.com/sirerun/gist/catalog/ports"
)

type scaleAuthority struct {
	scope            d.IndexScope
	revoked          atomic.Bool
	mu               sync.Mutex
	denied           map[ports.ArtifactRef]bool
	blockRef         ports.ArtifactRef
	blocked, release chan struct{}
	blockOnce        atomic.Bool
}

func (a *scaleAuthority) Check(ctx context.Context, s d.IndexScope) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if a.revoked.Load() || string(encode(s)) != string(encode(a.scope)) {
		return errors.New("current policy revoked")
	}
	return nil
}
func (a *scaleAuthority) Decide(ctx context.Context, p ports.Principal, _ ports.Action, r *ports.ArtifactRef) (ports.Decision, error) {
	if e := ctx.Err(); e != nil {
		return ports.Decision{}, e
	}
	a.mu.Lock()
	denied := r == nil || a.denied[*r]
	block := r != nil && *r == a.blockRef
	blocked, release := a.blocked, a.release
	a.mu.Unlock()
	if block && a.blockOnce.CompareAndSwap(true, false) {
		close(blocked)
		select {
		case <-release:
		case <-ctx.Done():
			return ports.Decision{}, ctx.Err()
		}
	}
	return ports.Decision{Allowed: !denied && r.WorkspaceID == p.WorkspaceID && p.Subject == a.scope.Principal.Subject}, nil
}

type scalePartition struct {
	scope      d.IndexScope
	auth       *scaleAuthority
	entries    []d.IndexEntry
	index      *d.ScopedIndex
	generation uint64
}

func scaleDataset(cfg configuration, n, partitions int) ([]scalePartition, error) {
	if n < 10 || partitions < 1 || n%(10*partitions) != 0 {
		return nil, errors.New("dataset must preserve exact partition mixture")
	}
	out := make([]scalePartition, partitions)
	for i := range out {
		scope := d.IndexScope{Principal: ports.Principal{Subject: fmt.Sprintf("operator-%02d", i), WorkspaceID: fmt.Sprintf("scale-%02d", i), Scopes: []string{"catalog:read"}, PolicyGeneration: 1}, RuntimeID: "scale-runtime"}
		out[i] = scalePartition{scope: scope, auth: &scaleAuthority{scope: scope, denied: map[ports.ArtifactRef]bool{}}, generation: 1}
	}
	if cfg.Kinds["tool"] != 40 || cfg.Kinds["skill"] != 30 || cfg.Kinds["capability"] != 10 || cfg.Kinds["binding"] != 20 || cfg.CommonPercent != 80 {
		return nil, errors.New("unsupported frozen distribution")
	}
	for i := 0; i < n; i++ {
		partition := i % partitions
		local := i / partitions
		kind := ports.KindTool
		switch slot := local % 10; {
		case slot >= 8:
			kind = ports.KindBinding
		case slot >= 7:
			kind = ports.KindCapability
		case slot >= 4:
			kind = ports.KindSkill
		}
		unique := fmt.Sprintf("unique%09d", i)
		summary := unique + fmt.Sprintf(" topic%03d", local%100)
		if local%5 != 4 {
			summary += " common"
		}
		ref := ports.ArtifactRef{WorkspaceID: out[partition].scope.Principal.WorkspaceID, Kind: kind, ID: fmt.Sprintf("scale/record%09d", i), Version: "1.0.0"}
		metadata := encode(map[string]any{"logical_name": unique, "summary": summary, "tags": []string{fmt.Sprintf("bucket%03d", local%100)}})
		out[partition].entries = append(out[partition].entries, d.IndexEntry{Record: ports.CatalogRecord{Ref: ref, State: "published", Metadata: metadata, Digest: digest(metadata)}})
	}
	return out, nil
}

type queryObservation struct {
	ElapsedNS   int64  `json:"elapsed_ns"`
	ResultBytes int    `json:"result_bytes"`
	Status      string `json:"status"`
	Error       string `json:"error"`
	Candidates  int    `json:"candidates"`
	Generation  uint64 `json:"generation"`
	Partition   int    `json:"partition"`
}

func scaleQuery(ctx context.Context, p *scalePartition, class string, partition int) queryObservation {
	ob := queryObservation{Partition: partition}
	start := time.Now()
	var result any
	switch class {
	case "exact_hit", "exact_miss":
		ref := p.entries[0].Record.Ref
		if class == "exact_miss" {
			ref.ID = "absent/never-indexed"
		}
		v, ok, e := p.index.Exact(ctx, p.scope, ref)
		ob.ElapsedNS = time.Since(start).Nanoseconds()
		result = struct {
			Candidate d.Candidate
			Found     bool
		}{v, ok}
		ob.Status = "miss"
		if ok {
			ob.Status = "hit"
			ob.Candidates = 1
		}
		if e != nil {
			ob.Error = e.Error()
		}
		ob.Generation = p.generation
	default:
		query := "zzznomatchzzz"
		if class == "common_lexical" {
			query = "common"
		}
		if class == "unique_lexical_hit" {
			var m struct {
				Name string `json:"logical_name"`
			}
			_ = json.Unmarshal(p.entries[0].Record.Metadata, &m)
			query = m.Name
		}
		v, e := p.index.Search(ctx, d.IndexSearch{Scope: p.scope, Generation: p.generation, Query: query, Limit: 64, MaxExamined: 4096, MaxBytes: 256 << 10})
		ob.ElapsedNS = time.Since(start).Nanoseconds()
		result = v
		ob.Status = v.Status
		ob.Candidates = len(v.Candidates)
		ob.Generation = v.Generation
		if e != nil {
			ob.Error = e.Error()
		}
	}
	ob.ResultBytes = len(encode(result))
	return ob
}

type scaleObservation struct {
	Schema                  string                        `json:"schema_version"`
	Source                  string                        `json:"source"`
	LogicalRecords          int                           `json:"logical_records"`
	PrivatePartitions       int                           `json:"private_partitions"`
	Host                    hostEvidence                  `json:"host"`
	DatasetNS               int64                         `json:"dataset_ns"`
	BuildNS                 int64                         `json:"build_ns"`
	UpdateNS                int64                         `json:"update_ns"`
	RebuildNS               int64                         `json:"rebuild_ns"`
	RollbackNS              int64                         `json:"rollback_ns"`
	PeakRSSBytes            int64                         `json:"peak_rss_bytes"`
	SnapshotProjectionBytes int64                         `json:"snapshot_projection_bytes"`
	ProjectionCandidates    int                           `json:"snapshot_projection_candidates"`
	QueryClasses            map[string][]queryObservation `json:"query_classes"`
	Cold                    map[string][]queryObservation `json:"cold"`
	Checks                  map[string]bool               `json:"checks"`
	CheckDetails            map[string]any                `json:"check_details"`
	Generations             [][]uint64                    `json:"generations"`
	Error                   string                        `json:"error"`
	ProjectionBoundary      string                        `json:"snapshot_projection_boundary"`
}

func measureProjection(ctx context.Context, partitions []scalePartition) (int64, int, error) {
	// Count a JSON array of full public snapshot projections without retaining a
	// second copy of all candidates. Every known reference is looked up exactly.
	bytes := int64(2)
	count := 0
	for i, p := range partitions {
		if i > 0 {
			bytes++
		}
		prefix := encode(struct {
			Scope      d.IndexScope `json:"scope"`
			Generation uint64       `json:"generation"`
		}{p.scope, p.generation})
		bytes += int64(len(prefix) - 1 + len(`,"candidates":[]}`))
		for j, entry := range p.entries {
			v, ok, e := p.index.Exact(ctx, p.scope, entry.Record.Ref)
			if e != nil {
				return bytes, count, e
			}
			if !ok {
				return bytes, count, errors.New("projection missing exact candidate")
			}
			if j > 0 {
				bytes++
			}
			bytes += int64(len(encode(v)))
			count++
		}
	}
	return bytes, count, nil
}
func runScale(ctx context.Context, cfg configuration, n, partitionCount int, source string) (ob scaleObservation) {
	ob = scaleObservation{Schema: "gist.composition-scale-observations/v1", Source: source, LogicalRecords: n, PrivatePartitions: partitionCount, Host: host(cfg), QueryClasses: map[string][]queryObservation{}, Cold: map[string][]queryObservation{}, Checks: map[string]bool{}, CheckDetails: map[string]any{}, ProjectionBoundary: "all exact public candidates plus scope and generation; excludes internal postings; RSS includes dataset and retained generations"}
	defer func() {
		rss, e := peakRSS()
		ob.PeakRSSBytes = rss
		if e != nil {
			ob.Error += e.Error()
		}
	}()
	allowed := false
	for _, run := range cfg.Runs {
		if run.Records == n && run.Partitions == partitionCount {
			allowed = true
		}
	}
	if !allowed {
		ob.Error = "size/partition combination not frozen"
		return
	}
	start := time.Now()
	partitions, e := scaleDataset(cfg, n, partitionCount)
	ob.DatasetNS = time.Since(start).Nanoseconds()
	if e != nil {
		ob.Error = e.Error()
		return
	}
	start = time.Now()
	for i := range partitions {
		p := &partitions[i]
		p.index, e = d.NewScopedIndex(ctx, p.scope, p.auth, p.auth, p.entries)
		if e != nil {
			ob.Error = e.Error()
			return
		}
		ob.Generations = append(ob.Generations, []uint64{1})
	}
	ob.BuildNS = time.Since(start).Nanoseconds()
	for _, class := range cfg.QueryClasses {
		for i := range partitions {
			ob.Cold[class] = append(ob.Cold[class], scaleQuery(ctx, &partitions[i], class, i))
		}
		for sample := 0; sample < cfg.Samples; sample++ {
			i := sample % len(partitions)
			ob.QueryClasses[class] = append(ob.QueryClasses[class], scaleQuery(ctx, &partitions[i], class, i))
		}
	}
	ob.SnapshotProjectionBytes, ob.ProjectionCandidates, e = measureProjection(ctx, partitions)
	if e != nil {
		ob.Error = e.Error()
		return
	}
	p := &partitions[0]
	p.auth.revoked.Store(true)
	revoked := scaleQuery(ctx, p, "common_lexical", 0)
	p.auth.revoked.Store(false)
	ob.Checks["revocation"] = revoked.Error != "" && revoked.Candidates == 0
	ob.CheckDetails["revocation"] = revoked
	ref := p.entries[0].Record.Ref
	p.auth.mu.Lock()
	p.auth.denied[ref] = true
	p.auth.mu.Unlock()
	denied := scaleQuery(ctx, p, "exact_hit", 0)
	p.auth.mu.Lock()
	delete(p.auth.denied, ref)
	p.auth.mu.Unlock()
	ob.Checks["denial"] = denied.Error != "" && denied.Candidates == 0
	ob.CheckDetails["denial"] = denied
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cancelResult := scaleQuery(cancelled, p, "exact_hit", 0)
	ob.Checks["cancellation"] = cancelResult.Error != "" && cancelResult.Candidates == 0
	ob.CheckDetails["cancellation"] = cancelResult
	broad := scaleQuery(ctx, p, "common_lexical", 0)
	exact := scaleQuery(ctx, p, "exact_hit", 0)
	ob.Checks["exact_after_broad_exhaustion"] = exact.Error == "" && exact.Status == "hit"
	ob.CheckDetails["exact_after_broad_exhaustion"] = map[string]any{"broad": broad, "exact": exact}
	// Hold one actual old-snapshot reader across update AND rebuild so peak RSS
	// includes a generation older than the immediately previous snapshot.
	p.auth.mu.Lock()
	p.auth.blockRef = ref
	p.auth.blocked = make(chan struct{})
	p.auth.release = make(chan struct{})
	p.auth.blockOnce.Store(true)
	p.auth.mu.Unlock()
	type readerResult struct {
		Error     string
		Found     bool
		Candidate d.Candidate
	}
	readerDone := make(chan readerResult, 1)
	go func() {
		candidate, found, err := p.index.Exact(ctx, p.scope, ref)
		r := readerResult{Found: found, Candidate: candidate}
		if err != nil {
			r.Error = err.Error()
		}
		readerDone <- r
	}()
	select {
	case <-p.auth.blocked:
	case <-ctx.Done():
		ob.Error = ctx.Err().Error()
		return
	}
	release := func() { close(p.auth.release) }
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	upserts := make([][]d.IndexEntry, len(partitions))
	for i := 0; i < cfg.UpdateRecords; i++ {
		pi := i % len(partitions)
		local := i / len(partitions)
		entry := partitions[pi].entries[local]
		entry.Record.Metadata = encode(map[string]any{"logical_name": fmt.Sprintf("updated%09d", i), "summary": "updated common"})
		entry.Record.Digest = digest(entry.Record.Metadata)
		partitions[pi].entries[local] = entry
		upserts[pi] = append(upserts[pi], entry)
	}
	start = time.Now()
	for i := range partitions {
		part := &partitions[i]
		part.generation, e = part.index.Update(ctx, part.generation, upserts[i], nil)
		if e != nil {
			ob.Error = e.Error()
			return
		}
		ob.Generations[i] = append(ob.Generations[i], part.generation)
	}
	ob.UpdateNS = time.Since(start).Nanoseconds()
	start = time.Now()
	for i := range partitions {
		part := &partitions[i]
		part.generation, e = part.index.Rebuild(ctx, part.generation, part.entries)
		if e != nil {
			ob.Error = e.Error()
			return
		}
		ob.Generations[i] = append(ob.Generations[i], part.generation)
	}
	ob.RebuildNS = time.Since(start).Nanoseconds()
	ob.Checks["full_rebuild"] = true
	release()
	released = true
	var old readerResult
	select {
	case old = <-readerDone:
	case <-ctx.Done():
		ob.Error = ctx.Err().Error()
		return
	}
	ob.Checks["inflight_generation_refusal"] = old.Error == d.ErrIndexGeneration.Error() && !old.Found && old.Candidate.ID == ""
	ob.CheckDetails["inflight_generation_refusal"] = old
	start = time.Now()
	for i := range partitions {
		part := &partitions[i]
		prior := part.generation
		part.generation, e = part.index.Rollback(ctx, prior)
		if e != nil {
			ob.Error = e.Error()
			return
		}
		ob.Generations[i] = append(ob.Generations[i], part.generation)
		if part.generation <= prior {
			ob.Error = "nonmonotonic rollback"
			return
		}
	}
	ob.RollbackNS = time.Since(start).Nanoseconds()
	ob.Checks["rollback"] = true
	detail, ok, e := measureAlternatives(ctx, cfg)
	if e != nil {
		ob.Error = e.Error()
		return
	}
	ob.Checks["binding_alternatives_1_4_64_65"] = ok
	ob.CheckDetails["binding_alternatives_1_4_64_65"] = detail
	return
}
func measureAlternatives(ctx context.Context, cfg configuration) (map[string]any, bool, error) {
	f, e := makeFixture(cfg, scenario{ID: "scale-alternatives", Scenario: "sixty_five_alternatives"})
	if e != nil {
		return nil, false, e
	}
	all := []d.IndexEntry{}
	for _, entry := range f.entries {
		if entry.Bindings[0].Requirement.Key == "list" {
			all = append(all, entry)
		}
	}
	observations := map[string]any{}
	valid := true
	for _, n := range []int{1, 4, 64, 65} {
		index, e := d.NewScopedIndex(ctx, f.policy.scope, f.store, f.policy, all[:n])
		if e != nil {
			return observations, false, e
		}
		q := c.Query{Principal: f.request.Principal, RuntimeID: f.request.RuntimeID, Requirement: all[0].Bindings[0].Requirement, Limit: 64}
		start := time.Now()
		result, e := index.Lookup(ctx, q)
		elapsed := time.Since(start).Nanoseconds()
		errorText := ""
		if e != nil {
			errorText = e.Error()
		}
		observations[fmt.Sprint(n)] = map[string]any{"elapsed_ns": elapsed, "result_bytes": len(encode(result)), "complete": result.Complete, "count": len(result.Items), "error": errorText}
		if e != nil || (n <= 64 && (!result.Complete || len(result.Items) != n)) || (n == 65 && (result.Complete || len(result.Items) != 0)) {
			valid = false
		}
	}
	return observations, valid, nil
}
