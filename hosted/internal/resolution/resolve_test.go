package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type catalogDouble struct {
	records map[string]ports.CatalogRecord
}

func (c catalogDouble) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	r, ok := c.records[string(ref.Kind)+":"+ref.ID+"@"+ref.Version]
	if !ok {
		return ports.CatalogRecord{}, errors.New("not found")
	}
	return r, nil
}

type policyDouble struct {
	allowed     bool
	generations []uint64
}

func (p *policyDouble) Decide(_ context.Context, principal ports.Principal, _ ports.Action, _ *ports.ArtifactRef) (ports.Decision, error) {
	p.generations = append(p.generations, principal.PolicyGeneration)
	return ports.Decision{Allowed: p.allowed}, nil
}

type resolutionDouble struct{ value ports.Resolution }

func (s *resolutionDouble) Put(_ context.Context, r ports.Resolution) error { s.value = r; return nil }

type clockDouble struct{ now time.Time }

func (c clockDouble) Now() time.Time { return c.now }

func ref(kind ports.ArtifactKind, id, version string) ports.ArtifactRef {
	return ports.ArtifactRef{WorkspaceID: "w", Kind: kind, ID: id, Version: version}
}
func metadata(v any) []byte { b, _ := json.Marshal(v); return b }
func fixture() catalogDouble {
	return catalogDouble{records: map[string]ports.CatalogRecord{
		"skill:s@1":                 {Ref: ref(ports.KindSkill, "s", "1"), Metadata: metadata(map[string]any{"required_capabilities": []string{"gist/cap@1.0.0"}})},
		"capability:gist/cap@1.0.0": {Ref: ref(ports.KindCapability, "gist/cap", "1.0.0"), Metadata: []byte(`{}`)},
		"binding:b@1.0.0":           {Ref: ref(ports.KindBinding, "b", "1.0.0"), Metadata: metadata(map[string]any{"capability_ref": "gist/cap@1.0.0", "requires_connection": true})},
	}}
}

func TestResolutionNeverReportsReadyForMissingConnection(t *testing.T) {
	policy := &policyDouble{allowed: true}
	store := &resolutionDouble{}
	r, err := NewResolver(fixture(), policy, store, clockDouble{now: time.Unix(100, 0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Resolve(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w", PolicyGeneration: 7}, Skill: ref(ports.KindSkill, "s", "1"), RuntimeID: "client", SelectedBindings: map[string]string{"gist/cap@1.0.0": "b"}, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady || got.Findings[0].Status != StatusRequiresConnection {
		t.Fatalf("unexpected resolution: %+v", got)
	}
	if store.value.ExpiresAt != 160 {
		t.Fatalf("expected bounded 60 second lease, got %d", store.value.ExpiresAt)
	}
}

func TestRuntimeConnectionAssertionBecomesClientAssertedReady(t *testing.T) {
	policy := &policyDouble{allowed: true}
	store := &resolutionDouble{}
	r, _ := NewResolver(fixture(), policy, store, clockDouble{now: time.Unix(100, 0)}, nil)
	got, err := r.Resolve(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, Skill: ref(ports.KindSkill, "s", "1"), RuntimeID: "client", ConnectionAsserted: map[string]ConnectionAssertion{"gist/cap@1.0.0": {Status: "connected", ScopeSummary: "users:write"}}, SelectedBindings: map[string]string{"gist/cap@1.0.0": "b"}, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusReady || got.Findings[0].Trust == "" {
		t.Fatalf("expected disclosed client assertion: %+v", got)
	}
}

func TestConnectionServiceFailsClosedWithoutBroker(t *testing.T) {
	_, err := NewConnectionService(nil).Begin(context.Background(), ConnectionRequest{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, Capability: ref(ports.KindCapability, "c", "1")})
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("expected service unavailable, got %v", err)
	}
}
