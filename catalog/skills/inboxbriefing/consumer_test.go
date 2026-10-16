package inboxbriefing_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/sirerun/gist/catalog/discovery"
	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/resolution"
	"github.com/sirerun/gist/catalog/skills/inboxbriefing"
)

type consumerCatalog struct {
	records map[ports.ArtifactRef]ports.CatalogRecord
	archive []byte
	stored  ports.PinnedResolution
}

func (c *consumerCatalog) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	r, ok := c.records[ref]
	if !ok {
		return r, resolution.ErrArtifactNotFound
	}
	return r, nil
}
func (c *consumerCatalog) Search(_ context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	p := ports.SearchPage{}
	for _, r := range c.records {
		if r.Ref.WorkspaceID != q.Principal.WorkspaceID {
			continue
		}
		for _, kind := range q.Kinds {
			if r.Ref.Kind == kind {
				p.Records = append(p.Records, r)
			}
		}
	}
	return p, nil
}
func (c *consumerCatalog) PutPinnedResolution(_ context.Context, r ports.PinnedResolution) error {
	c.stored = r
	return nil
}
func (c *consumerCatalog) Put(context.Context, ports.Digest, io.Reader, int64) error {
	return fmt.Errorf("read-only consumer")
}

type reader struct{ *bytes.Reader }

func (reader) Close() error { return nil }
func (c *consumerCatalog) Open(context.Context, ports.ArtifactRef) (ports.ArtifactReader, error) {
	return reader{bytes.NewReader(c.archive)}, nil
}

type consumerPolicy struct{}

func (consumerPolicy) Decide(_ context.Context, p ports.Principal, _ ports.Action, r *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{Allowed: p.Subject == "operator" && p.WorkspaceID == "inbox-workspace" && r.WorkspaceID == p.WorkspaceID}, nil
}
func digest(b []byte) ports.Digest {
	s := sha256.Sum256(b)
	return ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(s[:])}
}
func ref(kind ports.ArtifactKind, id string) ports.ArtifactRef {
	return ports.ArtifactRef{WorkspaceID: "inbox-workspace", Kind: kind, ID: id, Version: "1.0.0"}
}
func fixture(t *testing.T) (*consumerCatalog, ports.ArtifactPin) {
	t.Helper()
	archive, err := inboxbriefing.Archive()
	if err != nil {
		t.Fatal(err)
	}
	files, err := inboxbriefing.Files()
	if err != nil {
		t.Fatal(err)
	}
	skill := ref(ports.KindSkill, inboxbriefing.SkillID)
	pin := ports.ArtifactPin{Ref: skill, Digest: digest(archive), ManifestDigest: digest(files["manifest.json"])}
	c := &consumerCatalog{archive: archive, records: map[ports.ArtifactRef]ports.CatalogRecord{skill: {Ref: skill, State: "published", Digest: pin.Digest, ManifestDigest: pin.ManifestDigest, Metadata: files["manifest.json"]}}}
	return c, pin
}

// This is a public-library consumer test. Fixture bindings below are not Gmail
// admission evidence and are never included in the maintained skill package.
func TestPublicConsumerDiscoversResolvesAndRetrievesMaintainedSkill(t *testing.T) {
	ctx := context.Background()
	principal := ports.Principal{Subject: "operator", WorkspaceID: "inbox-workspace"}
	c, pin := fixture(t)
	search, err := discovery.New(c, consumerPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	found, err := search.Search(ctx, discovery.Request{Principal: principal, Query: "inbox", Kinds: []ports.ArtifactKind{ports.KindSkill}, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Candidates) != 1 || found.Candidates[0].Ref != pin.Ref || found.Candidates[0].RequiredCapabilityCount != 1 {
		t.Fatalf("unexpected discovery: %+v", found)
	}
	resolver, err := resolution.NewResolver(c, consumerPolicy{}, c, ports.SystemClock{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := resolution.Request{Principal: principal, Skill: found.Candidates[0].Ref, RuntimeID: "test.inbox.consumer", MaxBytes: 16384, Lease: time.Minute}
	incomplete, err := resolver.Resolve(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if incomplete.Status != resolution.StatusIncomplete {
		t.Fatalf("unconfigured provider appeared ready: %+v", incomplete)
	}
	add := func(kind ports.ArtifactKind, id string, metadata any) {
		raw, err := json.Marshal(metadata)
		if err != nil {
			t.Fatal(err)
		}
		r := ref(kind, id)
		c.records[r] = ports.CatalogRecord{Ref: r, State: "published", Digest: digest(raw), ManifestDigest: digest(raw), Metadata: raw}
	}
	add(ports.KindCapability, inboxbriefing.CapabilityID, map[string]any{"id": inboxbriefing.CapabilityID, "version": "1.0.0"})
	add(ports.KindProvider, "test-only/provider", map[string]any{"id": "test-only/provider", "version": "1.0.0", "support_state": "resolvable"})
	add(ports.KindTool, "test-only/read", map[string]any{"id": "test-only/read", "version": "1.0.0", "lifecycle": "published", "execution_location": "client", "provider_action": map[string]string{"id": "test-only/provider", "version": "1.0.0"}, "credentials": map[string]string{"type": "none"}})
	add(ports.KindBinding, "test-only/binding", map[string]any{"id": "test-only/binding", "version": "1.0.0", "capability_ref": inboxbriefing.CapabilityID + "@1.0.0", "tool_ref": "test-only/read@1.0.0", "provider_ref": "test-only/provider@1.0.0", "adapter_version": "fixture-v1", "fixture_digest": digest([]byte("controlled fixture; no mailbox")), "conformance": "passed", "exact_versions": true})
	resolved, err := resolver.Resolve(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != resolution.StatusReady || len(resolved.Findings) != 1 || len(resolved.Findings[0].Closure) != 4 {
		t.Fatalf("fixture closure failed: %+v", resolved)
	}
	if c.stored.SkillPin != pin {
		t.Fatalf("trusted skill pins changed: %+v", c.stored.SkillPin)
	}
	retriever := packages.Retriever{Authorizer: consumerPolicy{}, Catalog: c, Artifacts: c}
	pkg, err := retriever.RetrieveSkill(ctx, principal, c.stored.SkillPin, packages.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	files, err := inboxbriefing.Files()
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Files) != len(files) {
		t.Fatal("package members lost")
	}
	for name, want := range files {
		if !bytes.Equal(pkg.Files[name], want) {
			t.Fatalf("member changed: %s", name)
		}
	}
	if len(pkg.Files[inboxbriefing.TemplatePath]) == 0 {
		t.Fatal("template missing")
	}
	foreignRequest := request
	foreignRequest.Principal.WorkspaceID = "foreign"
	if _, err := resolver.Resolve(ctx, foreignRequest); err == nil {
		t.Fatal("foreign principal resolved skill")
	}
	providerRef := ref(ports.KindProvider, "test-only/provider")
	revokedProvider := c.records[providerRef]
	revokedProvider.State = "revoked"
	c.records[providerRef] = revokedProvider
	if _, err := resolver.Resolve(ctx, request); err == nil {
		t.Fatal("revoked closure resolved")
	}
	delete(c.records, providerRef)
	missing, err := resolver.Resolve(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if missing.Status != resolution.StatusIncomplete {
		t.Fatal("missing provider resolved")
	}
	for _, scenario := range []string{"wrong tenant", "tampered ZIP", "revoked", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			copy, p := fixture(t)
			who := principal
			switch scenario {
			case "wrong tenant":
				who.WorkspaceID = "foreign"
			case "tampered ZIP":
				copy.archive = append(copy.archive, 0)
			case "revoked":
				r := copy.records[p.Ref]
				r.State = "revoked"
				copy.records[p.Ref] = r
			case "missing":
				delete(copy.records, p.Ref)
			}
			r := packages.Retriever{Authorizer: consumerPolicy{}, Catalog: copy, Artifacts: copy}
			if _, err := r.RetrieveSkill(ctx, who, p, packages.DefaultLimits()); err == nil {
				t.Fatal("invalid artifact accepted")
			}
		})
	}
}
