package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type Catalog interface {
	Get(context.Context, ports.ArtifactRef) (ports.CatalogRecord, error)
}
type Authorizer interface {
	Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error)
}
type ResolutionStore interface {
	Put(context.Context, ports.Resolution) error
}
type Clock interface{ Now() time.Time }

const (
	StatusReady              = "ready"
	StatusRequiresConnection = "requires_connection"
	StatusRequiresSelection  = "requires_selection"
	StatusIncomplete         = "incomplete"
	StatusUnsupportedRuntime = "unsupported_runtime"
	StatusRequiresGateway    = "requires_gateway"
)

type Request struct {
	Principal          ports.Principal
	Skill              ports.ArtifactRef
	RuntimeID          string
	LocalExecution     bool
	OwnedConnections   bool
	ConnectionAsserted map[string]ConnectionAssertion
	SelectedBindings   map[string]string
	MaxBytes           int
	Lease              time.Duration
}
type ConnectionAssertion struct{ Status, Audience, ScopeSummary string }
type Finding struct {
	CapabilityID string             `json:"capability_id"`
	Status       string             `json:"status"`
	Binding      *ports.ArtifactRef `json:"binding,omitempty"`
	ConnectURL   string             `json:"connect_url,omitempty"`
	Trust        string             `json:"trust,omitempty"`
	Reason       string             `json:"reason,omitempty"`
}
type Result struct {
	ID            string            `json:"resolution_id"`
	Status        string            `json:"status"`
	Skill         ports.ArtifactRef `json:"skill"`
	ExpiresAt     int64             `json:"expires_at"`
	Findings      []Finding         `json:"findings"`
	TokenEstimate int               `json:"token_estimate"`
}

type skillMetadata struct {
	RequiredCapabilities []string `json:"required_capabilities"`
}
type bindingMetadata struct {
	CapabilityRef      string   `json:"capability_ref"`
	ExecutionLocation  string   `json:"execution_location"`
	RequiresConnection bool     `json:"requires_connection"`
	RequiresGateway    bool     `json:"requires_gateway"`
	SupportedRuntimes  []string `json:"supported_runtimes"`
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
	if req.Principal.WorkspaceID == "" || req.Principal.Subject == "" || req.Skill.ID == "" || req.Skill.Version == "" || req.MaxBytes <= 0 {
		return Result{}, fmt.Errorf("invalid resolution request")
	}
	if req.RuntimeID == "" {
		req.RuntimeID = "unknown"
	}
	if err := r.authorize(ctx, req.Principal, &req.Skill); err != nil {
		return Result{}, err
	}
	skill, err := r.catalog.Get(ctx, req.Skill)
	if err != nil {
		return Result{}, fmt.Errorf("load skill %s: %w", req.Skill.ID, err)
	}
	var sm skillMetadata
	if err := json.Unmarshal(skill.Metadata, &sm); err != nil {
		return Result{}, fmt.Errorf("decode skill requirements: %w", err)
	}
	findings := make([]Finding, 0, len(sm.RequiredCapabilities))
	for _, capability := range sm.RequiredCapabilities {
		finding, err := r.resolveCapability(ctx, req, capability)
		if err != nil {
			return Result{}, err
		}
		findings = append(findings, finding)
	}
	status := StatusReady
	for _, f := range findings {
		if f.Status != StatusReady {
			status = StatusIncomplete
			break
		}
	}
	return r.persist(ctx, req, findings, status)
}

func (r *Resolver) resolveCapability(ctx context.Context, req Request, capability string) (Finding, error) {
	capID, capVersion, ok := strings.Cut(capability, "@")
	if !ok || capID == "" || capVersion == "" {
		return Finding{CapabilityID: capability, Status: StatusIncomplete, Reason: "invalid capability reference"}, nil
	}
	capRef := ports.ArtifactRef{WorkspaceID: req.Skill.WorkspaceID, Kind: ports.KindCapability, ID: capID, Version: capVersion}
	if err := r.authorize(ctx, req.Principal, &capRef); err != nil {
		return Finding{}, err
	}
	if _, err := r.catalog.Get(ctx, capRef); err != nil {
		return Finding{CapabilityID: capID, Status: StatusIncomplete, Reason: "capability unavailable"}, nil
	}
	bindingID := req.SelectedBindings[capability]
	if bindingID == "" {
		return Finding{CapabilityID: capID, Status: StatusRequiresSelection, Reason: "binding selection required"}, nil
	}
	bindingRef := ports.ArtifactRef{WorkspaceID: req.Skill.WorkspaceID, Kind: ports.KindBinding, ID: bindingID, Version: "1.0.0"}
	if err := r.authorize(ctx, req.Principal, &bindingRef); err != nil {
		return Finding{}, err
	}
	binding, err := r.catalog.Get(ctx, bindingRef)
	if err != nil {
		return Finding{CapabilityID: capID, Status: StatusIncomplete, Reason: "binding unavailable"}, nil
	}
	var bm bindingMetadata
	if err := json.Unmarshal(binding.Metadata, &bm); err != nil {
		return Finding{}, fmt.Errorf("decode binding %s: %w", bindingID, err)
	}
	if bm.CapabilityRef != "" && bm.CapabilityRef != capability {
		return Finding{CapabilityID: capID, Status: StatusIncomplete, Reason: "binding capability mismatch"}, nil
	}
	if len(bm.SupportedRuntimes) > 0 {
		supported := false
		for _, runtime := range bm.SupportedRuntimes {
			if runtime == req.RuntimeID {
				supported = true
			}
		}
		if !supported {
			return Finding{CapabilityID: capID, Status: StatusUnsupportedRuntime, Reason: "runtime is not supported"}, nil
		}
	}
	if bm.RequiresGateway || bm.ExecutionLocation == "gist_gateway" {
		return Finding{CapabilityID: capID, Status: StatusRequiresGateway, Binding: &bindingRef, Trust: "catalog metadata only"}, nil
	}
	if bm.RequiresConnection {
		if assertion, ok := req.ConnectionAsserted[capability]; ok && assertion.Status == "connected" {
			return Finding{CapabilityID: capID, Status: StatusReady, Binding: &bindingRef, Trust: "client-asserted connection: " + assertion.ScopeSummary}, nil
		}
		return Finding{CapabilityID: capID, Status: StatusRequiresConnection, Binding: &bindingRef, Reason: "runtime connection required"}, nil
	}
	return Finding{CapabilityID: capID, Status: StatusReady, Binding: &bindingRef, Trust: "binding metadata verified"}, nil
}

func (r *Resolver) authorize(ctx context.Context, p ports.Principal, ref *ports.ArtifactRef) error {
	d, err := r.auth.Decide(ctx, p, ports.ActionRead, ref)
	if err != nil {
		return fmt.Errorf("authorize %s: %w", ref.ID, err)
	}
	if !d.Allowed {
		return fmt.Errorf("authorize %s: denied", ref.ID)
	}
	return nil
}

func (r *Resolver) persist(ctx context.Context, req Request, findings []Finding, status string) (Result, error) {
	id, err := newID()
	if err != nil {
		return Result{}, err
	}
	expires := leaseExpiry(r.clock.Now(), req.Lease).Unix()
	pfindings := make([]ports.Finding, len(findings))
	for i, f := range findings {
		pfindings[i] = ports.Finding{CapabilityID: f.CapabilityID, Status: f.Status, ConnectURL: f.ConnectURL}
	}
	result := Result{ID: id, Status: status, Skill: req.Skill, ExpiresAt: expires, Findings: findings}
	b, err := json.Marshal(result)
	if err != nil {
		return Result{}, fmt.Errorf("encode resolution: %w", err)
	}
	if len(b) > req.MaxBytes {
		return Result{}, fmt.Errorf("resolution requires %d bytes, budget is %d: budget exceeded", len(b), req.MaxBytes)
	}
	result.TokenEstimate = (len(b) + 3) / 4
	if err := r.store.Put(ctx, ports.Resolution{ID: id, Principal: req.Principal, Skill: req.Skill, ExpiresAt: expires, Findings: pfindings}); err != nil {
		return Result{}, fmt.Errorf("store resolution %s: %w", id, err)
	}
	return result, nil
}
