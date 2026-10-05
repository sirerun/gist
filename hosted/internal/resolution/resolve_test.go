package resolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type catalogDouble struct {
	records  map[string]ports.CatalogRecord
	bindings []ports.CatalogRecord
}

func (c catalogDouble) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	r, ok := c.records[key(ref)]
	if !ok {
		return ports.CatalogRecord{}, ErrArtifactNotFound
	}
	if r.State == "revoked" {
		return ports.CatalogRecord{}, ErrArtifactRevoked
	}
	return r, nil
}
func (c catalogDouble) Search(_ context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	if q.Principal.WorkspaceID == "" {
		return ports.SearchPage{}, errors.New("missing principal")
	}
	return ports.SearchPage{Records: append([]ports.CatalogRecord(nil), c.bindings...)}, nil
}
func key(ref ports.ArtifactRef) string { return string(ref.Kind) + ":" + ref.ID + "@" + ref.Version }

type policyDouble struct {
	allowed  bool
	deniedID string
}

func (p policyDouble) Decide(_ context.Context, _ ports.Principal, _ ports.Action, ref *ports.ArtifactRef) (ports.Decision, error) {
	if ref != nil && ref.ID == p.deniedID {
		return ports.Decision{}, nil
	}
	return ports.Decision{Allowed: p.allowed}, nil
}

type resolutionDouble struct{ value ports.Resolution }

func (s *resolutionDouble) Put(_ context.Context, r ports.Resolution) error { s.value = r; return nil }

type clockDouble struct{ now time.Time }

func (c clockDouble) Now() time.Time { return c.now }

var principal = ports.Principal{Subject: "p", WorkspaceID: "w", Scopes: []string{"catalog:read"}}

func ref(kind ports.ArtifactKind, id, version string) ports.ArtifactRef {
	return ports.ArtifactRef{WorkspaceID: "w", Kind: kind, ID: id, Version: version}
}
func record(kind ports.ArtifactKind, id, version, state string, metadata any) ports.CatalogRecord {
	raw, _ := json.Marshal(metadata)
	sum := sha256.Sum256(raw)
	return ports.CatalogRecord{Ref: ref(kind, id, version), State: state, Digest: ports.Digest{Algorithm: "sha256", Value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, ManifestDigest: ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}, Metadata: raw}
}
func fixture(bindingVersions ...string) catalogDouble {
	skill := record(ports.KindSkill, "skill/demo", "1.2.0", "published", map[string]any{"id": "skill/demo", "version": "1.2.0", "required_capabilities": []any{map[string]any{"id": "cap/demo", "contract_version": "2.0.0"}}, "runtime_requirements": map[string]any{"local_execution": false}})
	cap := record(ports.KindCapability, "cap/demo", "2.0.0", "published", map[string]any{"id": "cap/demo", "version": "2.0.0"})
	tool := record(ports.KindTool, "tool/demo", "3.1.0", "published", map[string]any{"id": "tool/demo", "version": "3.1.0", "lifecycle": "published", "execution_location": "client", "provider_action": map[string]any{"id": "provider/demo", "version": "4.0.0"}, "credentials": map[string]any{"type": "none", "scopes": []string{}}})
	provider := record(ports.KindProvider, "provider/demo", "4.0.0", "published", map[string]any{"id": "provider/demo", "version": "4.0.0", "support_state": "executable"})
	records := map[string]ports.CatalogRecord{key(skill.Ref): skill, key(cap.Ref): cap, key(tool.Ref): tool, key(provider.Ref): provider}
	var bindings []ports.CatalogRecord
	for _, version := range bindingVersions {
		binding := record(ports.KindBinding, "bind/demo", version, "published", map[string]any{"id": "bind/demo", "version": version, "capability_ref": "cap/demo@2.0.0", "tool_ref": "tool/demo@3.1.0", "provider_ref": "provider/demo@4.0.0", "adapter_version": "adapter.8", "fixture_digest": map[string]any{"algorithm": "sha256", "value": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}, "conformance": "passed", "exact_versions": true})
		bindings = append(bindings, binding)
		records[key(binding.Ref)] = binding
	}
	return catalogDouble{records: records, bindings: bindings}
}
func newResolver(t *testing.T, c catalogDouble, policy policyDouble, store *resolutionDouble) *Resolver {
	t.Helper()
	r, err := NewResolver(c, policy, store, clockDouble{time.Unix(100, 0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func request() Request {
	return Request{Principal: principal, Skill: ref(ports.KindSkill, "skill/demo", "1.2.0"), RuntimeID: "com.example.runtime", MaxBytes: 8192}
}

func TestResolveUsesActualImmutableBindingAndCompleteClosure(t *testing.T) {
	store := &resolutionDouble{}
	c := fixture("2.3.4")
	got, err := newResolver(t, c, policyDouble{allowed: true}, store).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusReady || len(got.Findings) != 1 || got.Findings[0].Binding == nil {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Findings[0].Binding.Reference != "bind/demo@2.3.4" {
		t.Fatalf("binding version was not preserved: %+v", got.Findings[0].Binding)
	}
	if len(got.Findings[0].Closure) != 4 {
		t.Fatalf("expected capability, binding, tool and provider pins, got %+v", got.Findings[0].Closure)
	}
	if got.Skill.Digest.Algorithm != "sha256" || got.Findings[0].Closure[0].ManifestDigest.Value == "" {
		t.Fatalf("missing digest closure: %+v", got)
	}
	root := c.records[key(ref(ports.KindSkill, "skill/demo", "1.2.0"))]
	if store.value.Skill.Version != "1.2.0" || store.value.SkillPin.Ref != root.Ref || store.value.SkillPin.Digest != root.Digest || store.value.SkillPin.ManifestDigest != root.ManifestDigest || store.value.Findings[0].Status != StatusReady || store.value.Findings[0].BindingRef.Version != "2.3.4" || len(store.value.Findings[0].Closure) != 4 {
		t.Fatalf("resolution persistence missing: %+v", store.value)
	}
}

func TestResolveRejectsCaseVariantDuplicateBindingFields(t *testing.T) {
	c := fixture("2.3.4")
	binding := c.bindings[0]
	binding.Metadata = []byte(`{"id":"bind/demo","version":"2.3.4","capability_ref":"cap/demo@2.0.0","CAPABILITY_REF":"attacker@9.9.9","tool_ref":"tool/demo@3.1.0","provider_ref":"provider/demo@4.0.0","adapter_version":"adapter.8","fixture_digest":{"algorithm":"sha256","value":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},"conformance":"passed","exact_versions":true}`)
	c.bindings[0] = binding
	c.records[key(binding.Ref)] = binding
	got, err := newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady || got.Findings[0].Reason != "binding metadata is invalid" {
		t.Fatalf("case-variant duplicate binding field was accepted: %+v", got)
	}
}
func TestResolveFailsClosedForAmbiguousBinding(t *testing.T) {
	got, err := newResolver(t, fixture("2.3.4", "2.4.0"), policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady || got.Findings[0].Status != StatusRequiresSelection {
		t.Fatalf("ambiguous binding became ready: %+v", got)
	}
}
func TestResolveFailsClosedForPrivateOrRevokedClosure(t *testing.T) {
	_, err := newResolver(t, fixture("2.3.4"), policyDouble{allowed: true, deniedID: "skill/demo"}, &resolutionDouble{}).Resolve(context.Background(), request())
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("private skill did not deny resolution: %v", err)
	}
	c := fixture("2.3.4")
	b := c.bindings[0]
	b.State = "revoked"
	c.records[key(b.Ref)] = b
	c.bindings = []ports.CatalogRecord{b}
	_, err = newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if !errors.Is(err, ErrArtifactRevoked) {
		t.Fatalf("revoked binding did not fail as revoked: %v", err)
	}
}
func TestResolveFailsClosedForIncompleteClosureAndGatewayOnly(t *testing.T) {
	c := fixture("2.3.4")
	delete(c.records, key(ref(ports.KindProvider, "provider/demo", "4.0.0")))
	got, err := newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady {
		t.Fatalf("incomplete provider closure became ready: %+v", got)
	}
	c = fixture("2.3.4")
	tool := c.records[key(ref(ports.KindTool, "tool/demo", "3.1.0"))]
	tool.Metadata, _ = json.Marshal(map[string]any{"id": "tool/demo", "version": "3.1.0", "lifecycle": "published", "execution_location": "gist_gateway", "provider_action": map[string]any{"id": "provider/demo", "version": "4.0.0"}, "credentials": map[string]any{"type": "none", "scopes": []string{}}})
	sum := sha256.Sum256(tool.Metadata)
	tool.ManifestDigest.Value = hex.EncodeToString(sum[:])
	c.records[key(tool.Ref)] = tool
	got, err = newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady || got.Findings[0].Status != StatusRequiresGateway {
		t.Fatalf("gateway binding became ready: %+v", got)
	}
}
func TestResolvePreservesClientAssertionAndEnforcesFinalWireBudget(t *testing.T) {
	req := request()
	req.OwnedConnections = true
	c := fixture("2.3.4")
	toolRef := ref(ports.KindTool, "tool/demo", "3.1.0")
	tool := c.records[key(toolRef)]
	var toolMeta map[string]any
	_ = json.Unmarshal(tool.Metadata, &toolMeta)
	toolMeta["credentials"] = map[string]any{"type": "oauth", "scopes": []string{"read"}}
	tool.Metadata, _ = json.Marshal(toolMeta)
	sum := sha256.Sum256(tool.Metadata)
	tool.ManifestDigest.Value = hex.EncodeToString(sum[:])
	c.records[key(toolRef)] = tool
	got, err := newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Findings[0].Provenance != "client_asserted" {
		t.Fatalf("client assertion provenance missing: %+v", got.Findings[0])
	}
	req.MaxBytes = 1
	_, err = newResolver(t, fixture("2.3.4"), policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), req)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected final response budget error, got %v", err)
	}
}
func TestResolveFailsClosedAtBindingSearchLimit(t *testing.T) {
	c := fixture("2.3.4")
	for i := 0; i < maxBindingScan-1; i++ {
		b := record(ports.KindBinding, fmt.Sprintf("other/%03d", i), "1.0.0", "published", map[string]any{"capability_ref": "other@1.0.0"})
		c.bindings = append(c.bindings, b)
	}
	got, err := newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady || got.Findings[0].Reason != ErrScanLimit.Error() {
		t.Fatalf("scan saturation became ready: %+v", got)
	}
}
func TestResolveRejectsMissingRuntimeID(t *testing.T) {
	req := request()
	req.RuntimeID = ""
	_, err := newResolver(t, fixture("2.3.4"), policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), req)
	if err == nil {
		t.Fatal("empty runtime identifier was accepted")
	}
}

func TestResolveHonorsFrozenRuntimeAndProviderContracts(t *testing.T) {
	c := fixture("2.3.4")
	skillRef := ref(ports.KindSkill, "skill/demo", "1.2.0")
	skill := c.records[key(skillRef)]
	var skillMeta map[string]any
	_ = json.Unmarshal(skill.Metadata, &skillMeta)
	skillMeta["runtime_requirements"] = map[string]any{"local_execution": true}
	skill.Metadata, _ = json.Marshal(skillMeta)
	sum := sha256.Sum256(skill.Metadata)
	skill.ManifestDigest.Value = hex.EncodeToString(sum[:])
	c.records[key(skillRef)] = skill
	toolRef := ref(ports.KindTool, "tool/demo", "3.1.0")
	tool := c.records[key(toolRef)]
	var toolMeta map[string]any
	_ = json.Unmarshal(tool.Metadata, &toolMeta)
	toolMeta["execution_location"] = "runtime"
	tool.Metadata, _ = json.Marshal(toolMeta)
	sum = sha256.Sum256(tool.Metadata)
	tool.ManifestDigest.Value = hex.EncodeToString(sum[:])
	c.records[key(toolRef)] = tool
	got, err := newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady || got.Findings[0].Status != StatusUnsupportedRuntime {
		t.Fatalf("runtime-incompatible binding became ready: %+v", got)
	}
}

func TestResolveUsesTrustedCatalogPinsForJSONBReencodedMetadata(t *testing.T) {
	c := fixture("2.3.4")
	ref := ref(ports.KindSkill, "skill/demo", "1.2.0")
	record := c.records[key(ref)]
	// PostgreSQL JSONB may normalize object ordering/whitespace after the
	// publication digest was calculated over the original manifest bytes.
	record.Metadata = []byte(`{ "runtime_requirements" : { "local_execution" : false }, "required_capabilities" : [{"contract_version":"2.0.0","id":"cap/demo"}], "version":"1.2.0", "id":"skill/demo" }`)
	c.records[key(ref)] = record
	got, err := newResolver(t, c, policyDouble{allowed: true}, &resolutionDouble{}).Resolve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusReady || got.Skill.ManifestDigest.Value != record.ManifestDigest.Value {
		t.Fatalf("catalog pins were not preserved across JSONB normalization: %+v", got)
	}
}
