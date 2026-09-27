package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/lexical"
	"github.com/sirerun/gist/hosted/internal/ports"
)

// Authorizer is deliberately consumed here rather than embedded in a search
// implementation. Indexes are derived data and can contain revoked records.
type Authorizer interface {
	Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error)
}

type Searcher interface {
	Search(context.Context, ports.SearchQuery) (ports.SearchPage, error)
}

type Candidate struct {
	Ref                     ports.ArtifactRef  `json:"ref"`
	Kind                    ports.ArtifactKind `json:"kind"`
	ID                      string             `json:"id"`
	Version                 string             `json:"version"`
	LogicalName             string             `json:"logical_name,omitempty"`
	Summary                 string             `json:"summary,omitempty"`
	Trust                   string             `json:"trust,omitempty"`
	Tags                    []string           `json:"tags,omitempty"`
	RequiredCapabilityCount int                `json:"required_capability_count"`
	ResolutionRequired      bool               `json:"resolution_required"`
	EstimatedBytes          int                `json:"estimated_bytes,omitempty"`
}

type Request struct {
	Principal ports.Principal
	Query     string
	Kinds     []ports.ArtifactKind
	Tags      []string
	Cursor    ports.Cursor
	Limit     int
	MaxBytes  int
}

type Response struct {
	Candidates    []Candidate   `json:"candidates"`
	NextCursor    *ports.Cursor `json:"next_cursor"`
	TokenEstimate int           `json:"token_estimate"`
}

type metadata struct {
	LogicalName          string   `json:"logical_name"`
	Summary              string   `json:"summary"`
	Trust                string   `json:"trust"`
	Tags                 []string `json:"tags"`
	RequiredCapabilities []string `json:"required_capabilities"`
	EstimatedBytes       int      `json:"estimated_bytes"`
}

type Service struct {
	search  Searcher
	auth    Authorizer
	cache   *cache
	version string
}

func New(search Searcher, auth Authorizer) (*Service, error) {
	if search == nil || auth == nil {
		return nil, fmt.Errorf("discovery requires searcher and authorizer")
	}
	return &Service{search: search, auth: auth, cache: newCache(), version: "discovery-v1"}, nil
}

func (s *Service) Search(ctx context.Context, req Request) (Response, error) {
	if req.Principal.WorkspaceID == "" || req.Principal.Subject == "" || req.MaxBytes <= 0 {
		return Response{}, fmt.Errorf("invalid discovery request")
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	key := cacheKey(req, s.version)
	records, next, ok := s.cache.get(key)
	if req.Cursor.ID != "" {
		if req.Cursor.WorkspaceID != req.Principal.WorkspaceID || req.Cursor.ExpiresAt <= time.Now().Unix() {
			return Response{}, fmt.Errorf("cursor expired or bound to another workspace")
		}
		if req.Cursor.PrincipalHash != "" && req.Cursor.PrincipalHash != principalHash(req.Principal) {
			return Response{}, fmt.Errorf("cursor bound to another principal")
		}
	}
	if !ok {
		page, err := s.search.Search(ctx, ports.SearchQuery{Principal: req.Principal, Text: req.Query, Kinds: req.Kinds, Tags: req.Tags, Cursor: req.Cursor, Limit: req.Limit, MaxBytes: req.MaxBytes})
		if err != nil {
			return Response{}, fmt.Errorf("search catalog: %w", err)
		}
		records, next = page.Records, page.Next
		s.cache.put(key, records, next)
	}

	// Authorization is intentionally before ranking and limiting. A hidden
	// record must not influence order, counts, snippets, or budget decisions.
	terms := lexical.Query(req.Query)
	textQuery := strings.TrimSpace(req.Query) != ""
	visible := make([]rankedCandidate, 0, len(records))
	for _, record := range records {
		ref := record.Ref
		decision, err := s.auth.Decide(ctx, req.Principal, ports.ActionRead, &ref)
		if err != nil {
			return Response{}, fmt.Errorf("authorize discovery record %s: %w", ref.ID, err)
		}
		if !decision.Allowed {
			continue
		}
		candidate, err := candidateFromRecord(record)
		if err != nil {
			return Response{}, fmt.Errorf("decode discovery metadata %s: %w", ref.ID, err)
		}
		if !matchesFilters(candidate, req) {
			continue
		}
		var match lexical.Match
		if textQuery {
			doc, err := lexical.DocumentFromMetadata(string(ref.Kind), ref.ID, record.Metadata)
			if err != nil {
				return Response{}, fmt.Errorf("decode discovery metadata %s: %w", ref.ID, err)
			}
			// Empty or no-match is a valid result; never force similarity.
			if match = lexical.NewIndex(doc).Score(terms); !match.Matched() {
				continue
			}
		}
		visible = append(visible, rankedCandidate{Candidate: candidate, match: match})
	}
	sort.SliceStable(visible, func(i, j int) bool {
		if c := visible[i].match.Better(visible[j].match); c != 0 {
			return c > 0
		}
		a, b := visible[i].Candidate, visible[j].Candidate
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Version < b.Version
	})

	result := Response{Candidates: make([]Candidate, 0, min(req.Limit, len(visible)))}
	if next.ID != "" {
		result.NextCursor = &next
	}
	for _, ranked := range visible {
		candidate := ranked.Candidate
		if len(result.Candidates) == req.Limit {
			break
		}
		trial := append(append([]Candidate(nil), result.Candidates...), candidate)
		responseBytes, err := json.Marshal(Response{Candidates: trial})
		if err != nil {
			return Response{}, fmt.Errorf("encode discovery response: %w", err)
		}
		if len(responseBytes) > req.MaxBytes {
			break // discovery may reduce optional candidates; it never truncates one.
		}
		result.Candidates = trial
	}
	for {
		b, err := json.Marshal(result)
		if err != nil {
			return Response{}, fmt.Errorf("encode discovery result: %w", err)
		}
		result.TokenEstimate = (len(b) + 3) / 4
		b, err = json.Marshal(result)
		if err != nil {
			return Response{}, fmt.Errorf("encode discovery result with estimate: %w", err)
		}
		if len(b) <= req.MaxBytes {
			break
		}
		if len(result.Candidates) == 0 {
			return Response{}, fmt.Errorf("discovery response requires %d bytes, budget is %d: budget exceeded", len(b), req.MaxBytes)
		}
		result.Candidates = result.Candidates[:len(result.Candidates)-1]
	}
	return result, nil
}

func candidateFromRecord(r ports.CatalogRecord) (Candidate, error) {
	var m metadata
	if len(r.Metadata) > 0 {
		if err := json.Unmarshal(r.Metadata, &m); err != nil {
			return Candidate{}, err
		}
	}
	return Candidate{Ref: r.Ref, Kind: r.Ref.Kind, ID: r.Ref.ID, Version: r.Ref.Version, LogicalName: m.LogicalName, Summary: m.Summary, Trust: m.Trust, Tags: m.Tags, RequiredCapabilityCount: len(m.RequiredCapabilities), ResolutionRequired: len(m.RequiredCapabilities) > 0, EstimatedBytes: m.EstimatedBytes}, nil
}

// rankedCandidate carries a visible candidate's lexical match for ranking.
type rankedCandidate struct {
	Candidate
	match lexical.Match
}

// matchesFilters applies the metadata filters (kind and tags). Query text is
// matched separately by the shared lexical model.
func matchesFilters(c Candidate, req Request) bool {
	if len(req.Kinds) > 0 {
		found := false
		for _, k := range req.Kinds {
			if c.Kind == k {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	for _, wanted := range req.Tags {
		found := false
		for _, tag := range c.Tags {
			if strings.EqualFold(tag, wanted) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func principalHash(p ports.Principal) string {
	scopes := append([]string(nil), p.Scopes...)
	sort.Strings(scopes)
	b, _ := json.Marshal(struct {
		I, S, A, W string
		G          uint64
		Scopes     []string
	}{p.Issuer, p.Subject, p.Audience, p.WorkspaceID, p.PolicyGeneration, scopes})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func cacheKey(r Request, version string) string {
	b, _ := json.Marshal(struct {
		I, S, A, W string
		G          uint64
		Sc         []string
		Q          string
		K          []ports.ArtifactKind
		T          []string
		C          ports.Cursor
		V          string
	}{r.Principal.Issuer, r.Principal.Subject, r.Principal.Audience, r.Principal.WorkspaceID, r.Principal.PolicyGeneration, append([]string(nil), r.Principal.Scopes...), r.Query, r.Kinds, r.Tags, r.Cursor, version})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
