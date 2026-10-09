package resolution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

var (
	ErrBudgetExceeded   = errors.New("resolution response budget exceeded")
	ErrScanLimit        = errors.New("binding catalog exceeds resolution scan limit")
	ErrArtifactRevoked  = errors.New("resolution artifact is revoked")
	ErrArtifactNotFound = errors.New("resolution artifact is unavailable")
	ErrAccessDenied     = errors.New("resolution artifact access denied")
)

type Catalog interface {
	Get(context.Context, ports.ArtifactRef) (ports.CatalogRecord, error)
	Search(context.Context, ports.SearchQuery) (ports.SearchPage, error)
}
type Authorizer interface {
	Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error)
}
type ResolutionStore interface {
	PutPinnedResolution(context.Context, ports.PinnedResolution) error
}
type Clock interface{ Now() time.Time }

const (
	StatusReady              = "ready"
	StatusRequiresConnection = "requires_connection"
	StatusRequiresSelection  = "requires_selection"
	StatusIncomplete         = "incomplete"
	StatusUnsupportedRuntime = "unsupported_runtime"
	StatusRequiresGateway    = "requires_gateway"
	maxBindingScan           = 100
)

type Request struct {
	Principal        ports.Principal
	Skill            ports.ArtifactRef
	RuntimeID        string
	OwnedConnections bool
	MaxBytes         int
	Lease            time.Duration
}
type Digest struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}
type ArtifactPin struct {
	Reference      string `json:"reference"`
	Kind           string `json:"kind"`
	Digest         Digest `json:"digest"`
	ManifestDigest Digest `json:"manifest_digest"`
}
type Finding struct {
	CapabilityID string        `json:"capability_id"`
	Required     bool          `json:"required"`
	Status       string        `json:"status"`
	Binding      *ArtifactPin  `json:"binding,omitempty"`
	Closure      []ArtifactPin `json:"closure,omitempty"`
	Provenance   string        `json:"provenance,omitempty"`
	Reason       string        `json:"reason,omitempty"`
}
type Result struct {
	ID       string      `json:"resolution_id"`
	Status   string      `json:"aggregate"`
	Skill    ArtifactPin `json:"skill"`
	Findings []Finding   `json:"findings"`
}

type RuntimeRequirements struct {
	LocalExecution bool     `json:"local_execution"`
	Filesystem     []string `json:"filesystem"`
}
type skillMetadata struct {
	ID                   string                  `json:"id"`
	Version              string                  `json:"version"`
	RequiredCapabilities []capabilityRequirement `json:"required_capabilities"`
	RuntimeRequirements  RuntimeRequirements     `json:"runtime_requirements"`
}
type capabilityRequirement struct {
	ID              string `json:"id"`
	ContractVersion string `json:"contract_version"`
	Optional        bool   `json:"optional,omitempty"`
}
type bindingMetadata struct {
	ID             string       `json:"id"`
	Version        string       `json:"version"`
	CapabilityRef  string       `json:"capability_ref"`
	ToolRef        string       `json:"tool_ref"`
	ProviderRef    string       `json:"provider_ref"`
	AdapterVersion string       `json:"adapter_version"`
	FixtureDigest  ports.Digest `json:"fixture_digest"`
	Conformance    string       `json:"conformance"`
	ExactVersions  bool         `json:"exact_versions"`
	// Legacy catalog records may carry runtime restrictions. Frozen v1 does not
	// define this field, so it is deliberately not used to grant readiness.
}
type capabilityMetadata struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}
type Resolver struct {
	catalog     Catalog
	auth        Authorizer
	store       ResolutionStore
	clock       Clock
	connections *ConnectionService
}

func NewResolver(c Catalog, a Authorizer, store ResolutionStore, clock Clock, connections *ConnectionService) (*Resolver, error) {
	if c == nil || a == nil || store == nil || clock == nil {
		return nil, fmt.Errorf("resolution requires catalog, authorizer, store, and clock")
	}
	return &Resolver{catalog: c, auth: a, store: store, clock: clock, connections: connections}, nil
}

func (r *Resolver) Resolve(ctx context.Context, req Request) (Result, error) {
	if req.Principal.WorkspaceID == "" || req.Principal.Subject == "" || req.Skill.ID == "" || req.Skill.Version == "" || req.MaxBytes <= 0 || strings.TrimSpace(req.RuntimeID) == "" {
		return Result{}, fmt.Errorf("invalid resolution request")
	}
	if req.Skill.WorkspaceID != req.Principal.WorkspaceID || req.Skill.Kind != ports.KindSkill {
		return Result{}, fmt.Errorf("skill reference is outside the authenticated workspace")
	}
	if err := r.authorize(ctx, req.Principal, req.Skill); err != nil {
		return Result{}, err
	}
	skill, err := r.getAdmitted(ctx, req.Skill)
	if err != nil {
		return Result{}, fmt.Errorf("load skill %s: %w", req.Skill.ID, err)
	}
	var sm skillMetadata
	if err := decodeJSONMetadata(skill.Metadata, &sm); err != nil {
		return Result{}, fmt.Errorf("decode skill requirements: %w", err)
	}
	if sm.ID != req.Skill.ID || sm.Version != req.Skill.Version {
		return Result{}, fmt.Errorf("skill manifest identity does not match its immutable reference")
	}
	skillPin, err := pin(skill)
	if err != nil {
		return Result{}, fmt.Errorf("skill closure: %w", err)
	}
	findings := make([]Finding, 0, len(sm.RequiredCapabilities))
	for _, capability := range sm.RequiredCapabilities {
		capabilityRef := ports.ArtifactRef{WorkspaceID: req.Principal.WorkspaceID, Kind: ports.KindCapability, ID: capability.ID, Version: capability.ContractVersion}
		finding, err := r.resolveCapability(ctx, req, capability, capabilityRef, sm.RuntimeRequirements)
		if err != nil {
			return Result{}, err
		}
		findings = append(findings, finding)
	}
	status := StatusReady
	for _, f := range findings {
		if f.Required && f.Status != StatusReady {
			status = StatusIncomplete
			break
		}
	}
	return r.persist(ctx, req, skillPin, findings, status)
}

func (r *Resolver) resolveCapability(ctx context.Context, req Request, requirement capabilityRequirement, capRef ports.ArtifactRef, runtimeRequirements RuntimeRequirements) (Finding, error) {
	capID := requirement.ID
	finding := Finding{CapabilityID: capID, Required: !requirement.Optional, Status: StatusIncomplete}
	if capID == "" || requirement.ContractVersion == "" {
		finding.Reason = "invalid capability reference"
		return finding, nil
	}
	if err := r.authorize(ctx, req.Principal, capRef); err != nil {
		return Finding{}, err
	}
	capability, err := r.getAdmitted(ctx, capRef)
	if err != nil {
		if errors.Is(err, ErrArtifactRevoked) {
			return Finding{}, err
		}
		if !errors.Is(err, ErrArtifactNotFound) {
			return Finding{}, err
		}
		finding.Reason = "capability unavailable"
		return finding, nil
	}
	var cm capabilityMetadata
	if err := decodeJSONMetadata(capability.Metadata, &cm); err != nil {
		return Finding{}, fmt.Errorf("decode capability %s: %w", capID, err)
	}
	if cm.ID != capRef.ID || cm.Version != capRef.Version {
		finding.Reason = "capability manifest identity mismatch"
		return finding, nil
	}
	capPin, err := pin(capability)
	if err != nil {
		finding.Reason = "capability closure incomplete"
		return finding, nil
	}

	page, err := r.catalog.Search(ctx, ports.SearchQuery{Principal: req.Principal, Kinds: []ports.ArtifactKind{ports.KindBinding}, Limit: maxBindingScan})
	if err != nil {
		return Finding{}, fmt.Errorf("find capability bindings: %w", err)
	}
	if len(page.Records) >= maxBindingScan || page.Next.ID != "" {
		finding.Reason = ErrScanLimit.Error()
		return finding, nil
	}
	var matches []ports.CatalogRecord
	for _, candidate := range page.Records {
		if candidate.Ref.WorkspaceID != req.Principal.WorkspaceID || candidate.Ref.Kind != ports.KindBinding {
			continue
		}
		var bm bindingMetadata
		if err := decodeBinding(candidate.Metadata, &bm); err != nil {
			// Any malformed candidate could be an omitted conflicting binding.
			finding.Reason = "binding metadata is invalid"
			return finding, nil
		}
		if bm.CapabilityRef == capabilityRef(capRef) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		finding.Reason = "no admitted binding"
		return finding, nil
	}
	if len(matches) > 1 {
		finding.Status = StatusRequiresSelection
		finding.Reason = "multiple admitted bindings"
		return finding, nil
	}
	bindingRecord := matches[0]
	if err := r.authorize(ctx, req.Principal, bindingRecord.Ref); err != nil {
		return Finding{}, err
	}
	bindingRecord, err = r.getAdmitted(ctx, bindingRecord.Ref)
	if err != nil {
		if errors.Is(err, ErrArtifactRevoked) {
			return Finding{}, err
		}
		if !errors.Is(err, ErrArtifactNotFound) {
			return Finding{}, err
		}
		finding.Reason = "binding unavailable"
		return finding, nil
	}
	var bm bindingMetadata
	if err := decodeBinding(bindingRecord.Metadata, &bm); err != nil {
		finding.Reason = "binding metadata is invalid"
		return finding, nil
	}
	if bm.ID != bindingRecord.Ref.ID || bm.Version != bindingRecord.Ref.Version || bm.CapabilityRef != capabilityRef(capRef) || bm.Conformance != "passed" || !bm.ExactVersions || bm.AdapterVersion == "" || !validDigest(bm.FixtureDigest) {
		finding.Reason = "binding is incompatible or incomplete"
		return finding, nil
	}
	if bm.ID == "" || bm.Version == "" {
		finding.Reason = "binding version is incomplete"
		return finding, nil
	}
	bindingPin, err := pin(bindingRecord)
	if err != nil {
		finding.Reason = "binding closure incomplete"
		return finding, nil
	}
	toolRef, err := parseRef(req.Principal.WorkspaceID, ports.KindTool, bm.ToolRef)
	if err != nil {
		finding.Reason = "binding tool reference is invalid"
		return finding, nil
	}
	providerRef, err := parseRef(req.Principal.WorkspaceID, ports.KindProvider, bm.ProviderRef)
	if err != nil {
		finding.Reason = "binding provider reference is invalid"
		return finding, nil
	}
	if err := r.authorize(ctx, req.Principal, toolRef); err != nil {
		return Finding{}, err
	}
	if err := r.authorize(ctx, req.Principal, providerRef); err != nil {
		return Finding{}, err
	}
	tool, err := r.getAdmitted(ctx, toolRef)
	if err != nil {
		if errors.Is(err, ErrArtifactRevoked) {
			return Finding{}, err
		}
		if !errors.Is(err, ErrArtifactNotFound) {
			return Finding{}, err
		}
		finding.Reason = "binding tool unavailable"
		return finding, nil
	}
	provider, err := r.getAdmitted(ctx, providerRef)
	if err != nil {
		if errors.Is(err, ErrArtifactRevoked) {
			return Finding{}, err
		}
		if !errors.Is(err, ErrArtifactNotFound) {
			return Finding{}, err
		}
		finding.Reason = "binding provider unavailable"
		return finding, nil
	}
	toolPin, err := pin(tool)
	if err != nil {
		finding.Reason = "tool closure incomplete"
		return finding, nil
	}
	providerPin, err := pin(provider)
	if err != nil {
		finding.Reason = "provider closure incomplete"
		return finding, nil
	}
	var toolContract struct {
		ID                string `json:"id"`
		Version           string `json:"version"`
		Lifecycle         string `json:"lifecycle"`
		ExecutionLocation string `json:"execution_location"`
		ProviderAction    struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"provider_action"`
		Credentials struct {
			Type   string   `json:"type"`
			Scopes []string `json:"scopes"`
		} `json:"credentials"`
	}
	if err := decodeJSONMetadata(tool.Metadata, &toolContract); err != nil || (toolContract.ExecutionLocation != "client" && toolContract.ExecutionLocation != "runtime" && toolContract.ExecutionLocation != "gist_gateway") || toolContract.ID != toolRef.ID || toolContract.Version != toolRef.Version || (toolContract.Lifecycle != "published" && toolContract.Lifecycle != "deprecated") || toolContract.ProviderAction.ID != providerRef.ID || toolContract.ProviderAction.Version != providerRef.Version {
		finding.Reason = "execution schema is incompatible or incomplete"
		return finding, nil
	}
	var providerContract struct {
		SupportState string `json:"support_state"`
	}
	if err := decodeJSONMetadata(provider.Metadata, &providerContract); err != nil || (providerContract.SupportState != "resolvable" && providerContract.SupportState != "executable") {
		finding.Reason = "provider is not admitted for resolution"
		return finding, nil
	}
	closure := []ArtifactPin{capPin, bindingPin, toolPin, providerPin}
	finding.Binding = &bindingPin
	finding.Closure = closure
	if toolContract.ExecutionLocation == "gist_gateway" {
		finding.Status = StatusRequiresGateway
		finding.Reason = "binding requires an execution gateway"
		return finding, nil
	}
	if runtimeRequirements.LocalExecution && toolContract.ExecutionLocation != "client" {
		finding.Status = StatusUnsupportedRuntime
		finding.Reason = "skill requires client-local execution"
		return finding, nil
	}
	if len(runtimeRequirements.Filesystem) > 0 {
		finding.Status = StatusUnsupportedRuntime
		finding.Reason = "runtime filesystem permissions are not present in the v1 request"
		return finding, nil
	}
	requiresConnection := toolContract.Credentials.Type != "" && toolContract.Credentials.Type != "none"
	if requiresConnection && !req.OwnedConnections {
		finding.Status = StatusRequiresConnection
		finding.Reason = "runtime connection required"
		return finding, nil
	}
	if requiresConnection {
		finding.Provenance = "client_asserted"
	}
	finding.Status = StatusReady
	return finding, nil
}

func decodeBinding(raw []byte, out *bindingMetadata) error {
	return decodeMetadata(raw, out)
}

func decodeJSONMetadata(raw []byte, out any) error {
	if err := rejectDuplicateJSONFields(raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func decodeMetadata(raw []byte, out any) error {
	if err := rejectDuplicateJSONFields(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

// rejectDuplicateJSONFields rejects duplicate object keys case-insensitively,
// matching encoding/json's case-insensitive struct-field matching behavior.
func rejectDuplicateJSONFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON data")
		}
		return err
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make([]string, 0)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("invalid JSON object key")
			}
			for _, previous := range seen {
				if strings.EqualFold(previous, key) {
					return fmt.Errorf("duplicate JSON field %q", key)
				}
			}
			seen = append(seen, key)
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
}

func (r *Resolver) getAdmitted(ctx context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	record, err := r.catalog.Get(ctx, ref)
	if err != nil {
		return ports.CatalogRecord{}, err
	}
	if record.State == "revoked" {
		return ports.CatalogRecord{}, ErrArtifactRevoked
	}
	if record.Ref != ref || record.State != "published" && record.State != "deprecated" || !validDigest(record.Digest) {
		return ports.CatalogRecord{}, fmt.Errorf("catalog record is not a complete immutable pin")
	}
	if !validDigest(record.ManifestDigest) && ref.Kind != ports.KindSkill {
		alias, ok := documentMetadataAlias(record)
		if ok {
			record.ManifestDigest = alias
		}
	}
	if !validDigest(record.ManifestDigest) {
		return ports.CatalogRecord{}, fmt.Errorf("catalog record is not a complete immutable pin")
	}
	return record, nil
}

func (r *Resolver) authorize(ctx context.Context, p ports.Principal, ref ports.ArtifactRef) error {
	decision, err := r.auth.Decide(ctx, p, ports.ActionRead, &ref)
	if err != nil {
		return fmt.Errorf("authorize %s: %w", ref.ID, err)
	}
	if !decision.Allowed {
		return fmt.Errorf("authorize %s: %w", ref.ID, ErrAccessDenied)
	}
	return nil
}

func (r *Resolver) persist(ctx context.Context, req Request, skill ArtifactPin, findings []Finding, status string) (Result, error) {
	id, err := newID()
	if err != nil {
		return Result{}, err
	}
	result := Result{ID: id, Status: status, Skill: skill, Findings: findings}
	encoded, err := json.Marshal(result)
	if err != nil {
		return Result{}, fmt.Errorf("encode resolution: %w", err)
	}
	if len(encoded) > req.MaxBytes {
		return Result{}, fmt.Errorf("resolution requires %d bytes, budget is %d: %w", len(encoded), req.MaxBytes, ErrBudgetExceeded)
	}
	expires := leaseExpiry(r.clock.Now(), req.Lease).Unix()
	pfindings := make([]ports.PinnedFinding, len(findings))
	for i, finding := range findings {
		pfindings[i] = ports.PinnedFinding{CapabilityID: finding.CapabilityID, Status: finding.Status, Required: finding.Required, Provenance: finding.Provenance}
		if finding.Binding != nil {
			pfindings[i].BindingRef = portRef(req.Principal.WorkspaceID, *finding.Binding)
		}
		pfindings[i].Closure = make([]ports.ArtifactPin, 0, len(finding.Closure))
		for _, item := range finding.Closure {
			pfindings[i].Closure = append(pfindings[i].Closure, ports.ArtifactPin{Ref: portRef(req.Principal.WorkspaceID, item), Digest: ports.Digest{Algorithm: item.Digest.Algorithm, Value: item.Digest.Value}, ManifestDigest: ports.Digest{Algorithm: item.ManifestDigest.Algorithm, Value: item.ManifestDigest.Value}})
		}
	}
	rootPin := ports.ArtifactPin{Ref: portRef(req.Principal.WorkspaceID, skill), Digest: ports.Digest{Algorithm: skill.Digest.Algorithm, Value: skill.Digest.Value}, ManifestDigest: ports.Digest{Algorithm: skill.ManifestDigest.Algorithm, Value: skill.ManifestDigest.Value}}
	if err := r.store.PutPinnedResolution(ctx, ports.PinnedResolution{ID: id, Principal: req.Principal, Skill: req.Skill, SkillPin: rootPin, ExpiresAt: expires, Findings: pfindings}); err != nil {
		return Result{}, fmt.Errorf("store resolution %s: %w", id, err)
	}
	return result, nil
}

func pin(record ports.CatalogRecord) (ArtifactPin, error) {
	if !validDigest(record.ManifestDigest) && record.Ref.Kind != ports.KindSkill {
		if alias, ok := documentMetadataAlias(record); ok {
			record.ManifestDigest = alias
		}
	}
	if !validDigest(record.Digest) || !validDigest(record.ManifestDigest) {
		return ArtifactPin{}, fmt.Errorf("invalid digest")
	}
	return ArtifactPin{Reference: capabilityRef(record.Ref), Kind: string(record.Ref.Kind), Digest: Digest{Algorithm: record.Digest.Algorithm, Value: record.Digest.Value}, ManifestDigest: Digest{Algorithm: record.ManifestDigest.Algorithm, Value: record.ManifestDigest.Value}}, nil
}

func documentMetadataAlias(record ports.CatalogRecord) (ports.Digest, bool) {
	if !validDigest(record.DocumentDigest) || record.DocumentDigest.Algorithm != "sha256" {
		return ports.Digest{}, false
	}
	sum := sha256.Sum256(record.Metadata)
	if hex.EncodeToString(sum[:]) != record.DocumentDigest.Value {
		return ports.Digest{}, false
	}
	// The frozen resolver field name is retained as a compatibility alias for
	// typed v2 document bytes. This does not create a v2 manifest digest.
	return record.DocumentDigest, true
}
func capabilityRef(ref ports.ArtifactRef) string { return ref.ID + "@" + ref.Version }
func portRef(workspace string, pin ArtifactPin) ports.ArtifactRef {
	id, version, _ := strings.Cut(pin.Reference, "@")
	return ports.ArtifactRef{WorkspaceID: workspace, Kind: ports.ArtifactKind(pin.Kind), ID: id, Version: version}
}
func parseRef(workspace string, kind ports.ArtifactKind, value string) (ports.ArtifactRef, error) {
	id, version, ok := strings.Cut(value, "@")
	if !ok || id == "" || version == "" || strings.Contains(version, "@") {
		return ports.ArtifactRef{}, fmt.Errorf("invalid exact reference")
	}
	return ports.ArtifactRef{WorkspaceID: workspace, Kind: kind, ID: id, Version: version}, nil
}
func validDigest(d ports.Digest) bool {
	if d.Algorithm != "sha256" || len(d.Value) != sha256.Size*2 || strings.ToLower(d.Value) != d.Value {
		return false
	}
	decoded, err := hex.DecodeString(d.Value)
	return err == nil && len(decoded) == sha256.Size
}
