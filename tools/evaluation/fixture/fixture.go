// Package fixture supplies a frozen shared synthetic catalog for source smoke
// tests. Descriptors and recipes are fixed independently of task text and labels.
// It never executes a tool or connects to a provider.
package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"time"

	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/catalog/resolution"
	"github.com/sirerun/gist/catalog/skills/inboxbriefing"
)

const CatalogVersion = "source-smoke-shared-catalog-v1"

var Principal = ports.Principal{Subject: "synthetic-operator", WorkspaceID: "synthetic-workspace", PolicyGeneration: 1}

type Catalog struct {
	Records    map[ports.ArtifactRef]ports.CatalogRecord
	Bytes      map[ports.ArtifactRef][]byte
	Deny       bool
	BeforeGet  func(*Catalog, ports.ArtifactRef)
	BeforeOpen func(*Catalog, ports.ArtifactRef)
}

func Ref(kind ports.ArtifactKind, id string) ports.ArtifactRef {
	return ports.ArtifactRef{WorkspaceID: Principal.WorkspaceID, Kind: kind, ID: id, Version: "1.0.0"}
}
func Hash(b []byte) ports.Digest {
	h := sha256.Sum256(b)
	return ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(h[:])}
}
func (c *Catalog) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	if c.BeforeGet != nil {
		c.BeforeGet(c, ref)
	}
	r, ok := c.Records[ref]
	if !ok {
		return r, errors.New("fixture artifact unavailable")
	}
	return r, nil
}
func (c *Catalog) Search(_ context.Context, q ports.SearchQuery) (ports.SearchPage, error) {
	page := ports.SearchPage{}
	for _, r := range c.Records {
		if r.Ref.WorkspaceID != q.Principal.WorkspaceID {
			continue
		}
		for _, kind := range q.Kinds {
			if r.Ref.Kind == kind {
				page.Records = append(page.Records, r)
			}
		}
	}
	sort.Slice(page.Records, func(i, j int) bool { return page.Records[i].Ref.ID < page.Records[j].Ref.ID })
	return page, nil
}
func (c *Catalog) Decide(_ context.Context, p ports.Principal, _ ports.Action, ref *ports.ArtifactRef) (ports.Decision, error) {
	return ports.Decision{Allowed: !c.Deny && p.Subject == Principal.Subject && p.WorkspaceID == Principal.WorkspaceID && ref.WorkspaceID == p.WorkspaceID}, nil
}
func (c *Catalog) PutPinnedResolution(context.Context, ports.PinnedResolution) error { return nil }
func (c *Catalog) Put(context.Context, ports.Digest, io.Reader, int64) error {
	return errors.New("read-only fixture")
}

type reader struct{ *bytes.Reader }

func (reader) Close() error { return nil }
func (c *Catalog) Open(_ context.Context, ref ports.ArtifactRef) (ports.ArtifactReader, error) {
	if c.BeforeOpen != nil {
		c.BeforeOpen(c, ref)
	}
	b, ok := c.Bytes[ref]
	if !ok {
		return nil, errors.New("fixture bytes unavailable")
	}
	return reader{bytes.NewReader(b)}, nil
}
func (c *Catalog) add(kind ports.ArtifactKind, id string, metadata any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	ref := Ref(kind, id)
	c.Records[ref] = ports.CatalogRecord{Ref: ref, State: "published", Digest: Hash(raw), ManifestDigest: Hash(raw), Metadata: raw}
	c.Bytes[ref] = raw
	return nil
}

// New uses scenario only for current-state overlays. It never reads desired
// skill IDs, descriptions, gold labels or queries to construct the candidate set.
func New(scenario string) (*Catalog, error) {
	c := &Catalog{Records: map[ports.ArtifactRef]ports.CatalogRecord{}, Bytes: map[ports.ArtifactRef][]byte{}}
	files, err := inboxbriefing.Files()
	if err != nil {
		return nil, err
	}
	archive, err := inboxbriefing.Archive()
	if err != nil {
		return nil, err
	}
	skill := Ref(ports.KindSkill, inboxbriefing.SkillID)
	c.Records[skill] = ports.CatalogRecord{Ref: skill, State: "published", Digest: Hash(archive), ManifestDigest: Hash(files["manifest.json"]), Metadata: files["manifest.json"]}
	c.Bytes[skill] = archive
	entries := []struct {
		kind     ports.ArtifactKind
		id       string
		metadata any
	}{
		{ports.KindCapability, inboxbriefing.CapabilityID, map[string]any{"id": inboxbriefing.CapabilityID, "version": "1.0.0"}},
		{ports.KindProvider, "synthetic/provider", map[string]any{"id": "synthetic/provider", "version": "1.0.0", "support_state": "resolvable"}},
		{ports.KindTool, "synthetic/mail-read", map[string]any{"id": "synthetic/mail-read", "version": "1.0.0", "lifecycle": "published", "execution_location": "client", "provider_action": map[string]string{"id": "synthetic/provider", "version": "1.0.0"}, "credentials": map[string]string{"type": "none"}, "input_schema": map[string]any{"type": "object", "properties": map[string]any{"operation": map[string]any{"type": "string", "enum": []string{"list_unread", "read_message"}}}, "required": []string{"operation"}, "additionalProperties": false}, "output_schema": map[string]string{"type": "object"}}},
		{ports.KindBinding, "synthetic/mail-binding", map[string]any{"id": "synthetic/mail-binding", "version": "1.0.0", "capability_ref": inboxbriefing.CapabilityID + "@1.0.0", "tool_ref": "synthetic/mail-read@1.0.0", "provider_ref": "synthetic/provider@1.0.0", "adapter_version": "controlled-composite-read-v1", "fixture_digest": Hash([]byte("source-only controlled read fixture")), "conformance": "passed", "exact_versions": true}},
	}
	for _, entry := range entries {
		if err := c.add(entry.kind, entry.id, entry.metadata); err != nil {
			return nil, err
		}
	}
	// Static distractors intentionally lack maintained recipes/verified packages;
	// selecting them must abstain. They are not shipped broader capabilities.
	for _, d := range []struct{ id, name, summary string }{
		{"synthetic/calendar", "Calendar appointment lookup", "Find scheduled appointments and availability."},
		{"synthetic/warehouse", "Warehouse inventory levels", "Read warehouse stock quantities and reorder levels."},
		{"synthetic/send-mail", "Send an email message", "Compose and send outbound email to a recipient."},
		{"synthetic/files", "Search local files", "Locate filesystem documents and inspect their content."},
	} {
		archive, manifest, err := syntheticSkill(d.id, d.name, d.summary)
		if err != nil {
			return nil, err
		}
		ref := Ref(ports.KindSkill, d.id)
		c.Records[ref] = ports.CatalogRecord{Ref: ref, State: "published", Digest: Hash(archive), ManifestDigest: Hash(manifest), Metadata: manifest}
		c.Bytes[ref] = archive
	}
	if err := Overlay(c, scenario); err != nil {
		return nil, err
	}
	return c, nil
}
func (c *Catalog) Service() (preparation.Service, error) {
	r, err := resolution.NewResolver(c, c, c, ports.SystemClock{}, nil)
	if err != nil {
		return preparation.Service{}, err
	}
	return preparation.Service{Catalog: c, Authorizer: c, Retriever: packages.Retriever{Authorizer: c, Catalog: c, Artifacts: c}, Resolver: r, TaskAdmitter: staticTaskAdmission{}, Clock: ports.SystemClock{}, MaxTaskAdmissionAge: time.Minute}, nil
}
func Request(intent string) preparation.Request {
	return preparation.Request{Principal: Principal, Intent: intent, RuntimeID: "controlled-inbox-read-fixture-v1", Mode: preparation.Lexical, Fallback: preparation.Deny, Guide: true, MaxBytes: 65536, TopK: 4, CatalogVersion: CatalogVersion, RecipeVersion: preparation.RecipeVersion}
}

// Explicit source-only caller admission supports two independently authored
// fixture requests. It is not an NL classifier, gold-label lookup or recipe
// inference. Other intents require a real caller-owned admission implementation.
type staticTaskAdmission struct{}

func (staticTaskAdmission) AdmitTask(_ context.Context, q preparation.TaskAdmissionRequest) (preparation.TaskAdmissionDecision, error) {
	allowed := q.Intent == "Summarize recent unread inbox email" || q.Intent == "Summarize unread inbox email"
	allowed = allowed && q.Principal.WorkspaceID == Principal.WorkspaceID && q.Principal.Subject == Principal.Subject && q.Skill.Ref == Ref(ports.KindSkill, inboxbriefing.SkillID) && q.RuntimeID == "controlled-inbox-read-fixture-v1" && q.RecipeVersion == preparation.RecipeVersion
	now := time.Now().UTC()
	return preparation.TaskAdmissionDecision{ScopeDigest: q.ScopeDigest, Admitted: allowed, Generation: "explicit-source-task-admission-v1", CheckedAt: now, ValidUntil: now.Add(time.Minute)}, nil
}
