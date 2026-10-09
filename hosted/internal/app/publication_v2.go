package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/publicationv2"
	"github.com/sirerun/gist/hosted/internal/rest"
	"github.com/sirerun/gist/hosted/internal/storage"
)

// publicationV2 composes pure validation, trusted review evidence, current
// authorization fences and durable publication storage. It has no provider
// client and never fetches source URIs.
type publicationV2 struct {
	pool        *pgxpool.Pool
	catalog     ports.CatalogStore
	store       *storage.PublicationStore
	config      PublicationV2Config
	maxResponse int64
	maxRequest  int64
}

func (p *publicationV2) PublishV2(ctx context.Context, principal ports.Principal, kind ports.ArtifactKind, envelope []byte) (ports.PublicationResult, error) {
	limits := publicationv2.DefaultLimits()
	if p.maxRequest > 0 && limits.MaxEnvelopeBytes > p.maxRequest {
		limits.MaxEnvelopeBytes = p.maxRequest
	}
	prepared, err := publicationv2.Decode(kind, envelope, limits)
	if err != nil {
		return ports.PublicationResult{}, mapPublicationError(err)
	}
	evidence, evidenceBytes, err := p.trustedEvidence(ctx, principal, prepared)
	if err != nil {
		return ports.PublicationResult{}, mapPublicationError(err)
	}
	if !p.trustedReviewer(evidence.Reviewer) {
		return ports.PublicationResult{}, rest.ErrValidationFailed
	}
	if err := publicationv2.ValidateEvidence(prepared, principal, evidence, time.Now().UTC(), p.config.AllowSynthetic); err != nil {
		return ports.PublicationResult{}, rest.ErrValidationFailed
	}
	var protectedRefs []ports.ArtifactRef
	if kind == ports.KindSkill {
		refs, err := skillCapabilityRefs(ctx, p.catalog, principal, prepared)
		if err != nil {
			return ports.PublicationResult{}, mapPublicationError(err)
		}
		protectedRefs = append(protectedRefs, refs...)
	}
	if kind == ports.KindBinding {
		if p.config.BindingVerifier == nil || evidence.Conformance == nil {
			return ports.PublicationResult{}, rest.ErrPublicationDenied
		}
		refs, refList, err := bindingRecords(ctx, p.catalog, principal, prepared)
		if err != nil {
			return ports.PublicationResult{}, err
		}
		golden, err := base64.StdEncoding.Strict().DecodeString(evidence.Conformance.RetainedFixtureBase64)
		if err != nil {
			return ports.PublicationResult{}, publicationv2.ErrValidation
		}
		if err := p.config.BindingVerifier.VerifyPublicationBinding(ctx, refs, evidence, golden); err != nil {
			return ports.PublicationResult{}, err
		}
		protectedRefs = append(protectedRefs, refList...)
	}
	receipt, err := publicationv2.Receipt(prepared)
	if err != nil {
		return ports.PublicationResult{}, err
	}
	budget := prepared.MaxBytes
	if p.maxResponse > 0 && budget > p.maxResponse {
		budget = p.maxResponse
	}
	if budget <= 0 || int64(len(receipt)) > budget {
		return ports.PublicationResult{}, rest.ErrBudgetExceeded
	}
	fence := func(ctx context.Context, tx pgx.Tx) error {
		if err := currentPublicationAuthority(ctx, tx, principal, true); err != nil {
			return err
		}
		if err := currentNamespace(ctx, tx, principal, prepared.Ref.ID); err != nil {
			return err
		}
		for _, ref := range protectedRefs {
			var state string
			if err := tx.QueryRow(ctx, `SELECT state FROM catalog_versions WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 FOR SHARE`, ref.WorkspaceID, ref.Kind, ref.ID, ref.Version).Scan(&state); err != nil || state != "published" {
				return publicationv2.ErrValidation
			}
		}
		current, raw, err := p.trustedEvidenceTx(ctx, tx, principal, prepared)
		if err != nil {
			return err
		}
		if !bytesEqual(raw, evidenceBytes) || current.Reviewer != evidence.Reviewer {
			return publicationv2.ErrValidation
		}
		return publicationv2.ValidateEvidence(prepared, principal, current, time.Now().UTC(), p.config.AllowSynthetic)
	}
	result, err := p.store.Publish(ctx, principal, prepared, receipt, fence)
	if errors.Is(err, storage.ErrConflict) {
		return ports.PublicationResult{}, rest.ErrPublicationConflict
	}
	if errors.Is(err, storage.ErrPublicationBudget) {
		return ports.PublicationResult{}, rest.ErrBudgetExceeded
	}
	if err != nil {
		return ports.PublicationResult{}, mapPublicationError(err)
	}
	return result, nil
}

func (p *publicationV2) ReadV2(ctx context.Context, principal ports.Principal, ref ports.ArtifactRef, maxBytes int64, packageBody bool) (ports.PublicationRead, error) {
	got, err := p.store.Read(ctx, principal, ref, maxBytes, packageBody, func(ctx context.Context, tx pgx.Tx) error {
		return currentPublicationAuthority(ctx, tx, principal, false)
	})
	if errors.Is(err, storage.ErrNotFound) {
		return ports.PublicationRead{}, rest.ErrPublicationNotFound
	}
	if errors.Is(err, storage.ErrPublicationBudget) {
		return ports.PublicationRead{}, rest.ErrBudgetExceeded
	}
	if err != nil {
		return ports.PublicationRead{}, mapPublicationError(err)
	}
	return got, nil
}

func (p *publicationV2) trustedReviewer(reviewer ports.PublicationEvidenceReviewer) bool {
	for _, configured := range p.config.TrustedReviewers {
		if configured.Issuer == reviewer.Issuer && configured.Subject == reviewer.Subject {
			return true
		}
	}
	return false
}

func (p *publicationV2) trustedEvidence(ctx context.Context, principal ports.Principal, prepared ports.PreparedPublication) (ports.PublicationAdmissionEvidence, []byte, error) {
	var evidence ports.PublicationAdmissionEvidence
	var raw []byte
	err := storage.WithTenantPrincipal(ctx, p.pool, tenantPrincipal(principal), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		evidence, raw, err = p.trustedEvidenceTx(ctx, tx, principal, prepared)
		return err
	})
	return evidence, raw, err
}

func (p *publicationV2) trustedEvidenceTx(ctx context.Context, tx pgx.Tx, principal ports.Principal, prepared ports.PreparedPublication) (ports.PublicationAdmissionEvidence, []byte, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT evidence FROM review_records WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 AND decision='approved' ORDER BY reviewed_at DESC LIMIT 1 FOR SHARE`, principal.WorkspaceID, prepared.Ref.Kind, prepared.Ref.ID, prepared.Ref.Version).Scan(&raw)
	if err != nil {
		return ports.PublicationAdmissionEvidence{}, nil, rest.ErrValidationFailed
	}
	evidence, err := publicationv2.ValidateEvidenceJSON(raw)
	if err != nil {
		return ports.PublicationAdmissionEvidence{}, nil, mapPublicationError(err)
	}
	var reviewerID string
	err = tx.QueryRow(ctx, `SELECT reviewer_id FROM review_records WHERE workspace_id=$1 AND kind=$2 AND artifact_id=$3 AND version=$4 AND decision='approved' AND evidence=$5::jsonb ORDER BY reviewed_at DESC LIMIT 1 FOR SHARE`, principal.WorkspaceID, prepared.Ref.Kind, prepared.Ref.ID, prepared.Ref.Version, raw).Scan(&reviewerID)
	if err != nil || reviewerID != evidence.Reviewer.Subject || !p.trustedReviewer(evidence.Reviewer) {
		return ports.PublicationAdmissionEvidence{}, nil, rest.ErrValidationFailed
	}
	return evidence, raw, nil
}

func tenantPrincipal(p ports.Principal) storage.Tenant {
	return storage.Tenant{Issuer: p.Issuer, Subject: p.Subject, Audience: p.Audience, WorkspaceID: p.WorkspaceID, Scopes: p.Scopes, PolicyGeneration: p.PolicyGeneration}
}

func currentPublicationAuthority(ctx context.Context, tx pgx.Tx, p ports.Principal, publish bool) error {
	var issuer, subject, workspace, role string
	var generation, workspaceGeneration uint64
	var active bool
	var scopes []string
	var identityScopes []string
	var expiry time.Time
	var revoked *time.Time
	err := tx.QueryRow(ctx, `SELECT wi.issuer,wi.subject,wi.workspace_id,wi.policy_generation,wi.expires_at,wi.revoked_at,wi.scopes,wm.role,wm.scopes,wm.active,w.policy_generation FROM workload_identities wi JOIN workspace_memberships wm ON wm.workspace_id=wi.workspace_id AND wm.issuer=wi.issuer AND wm.subject=wi.subject JOIN workspaces w ON w.id=wi.workspace_id WHERE wi.issuer=$1 AND wi.subject=$2 AND wi.workspace_id=$3 FOR SHARE OF wi,wm,w`, p.Issuer, p.Subject, p.WorkspaceID).Scan(&issuer, &subject, &workspace, &generation, &expiry, &revoked, &identityScopes, &role, &scopes, &active, &workspaceGeneration)
	if err != nil || issuer != p.Issuer || subject != p.Subject || workspace != p.WorkspaceID || revoked != nil || !expiry.After(time.Now()) || !active || generation != p.PolicyGeneration || workspaceGeneration != p.PolicyGeneration {
		return rest.ErrPublicationDenied
	}
	action := ports.ActionRead
	if publish {
		action = ports.ActionPublish
	}
	if !containsScopes(scopes, []string{string(action)}) || !containsScopes(identityScopes, []string{string(action)}) || !containsScopes(p.Scopes, []string{string(action)}) {
		return rest.ErrPublicationDenied
	}
	if (publish || action == ports.ActionPublish) && role != "maintainer" {
		return rest.ErrPublicationDenied
	}
	return nil
}

func currentNamespace(ctx context.Context, tx pgx.Tx, p ports.Principal, id string) error {
	var prefix string
	err := tx.QueryRow(ctx, `SELECT prefix FROM namespace_reservations WHERE workspace_id=$1 AND owner_id=$2 AND status='active' AND ($3=prefix OR $3 LIKE prefix || '/%') ORDER BY length(prefix) DESC LIMIT 1 FOR SHARE`, p.WorkspaceID, p.Subject, id).Scan(&prefix)
	if err != nil || prefix == "" {
		return rest.ErrPublicationDenied
	}
	return nil
}

func bindingRecords(ctx context.Context, catalog ports.CatalogStore, p ports.Principal, prepared ports.PreparedPublication) ([]ports.CatalogRecord, []ports.ArtifactRef, error) {
	var doc struct {
		CapabilityRef string `json:"capability_ref"`
		ToolRef       string `json:"tool_ref"`
		ProviderRef   string `json:"provider_ref"`
	}
	if json.Unmarshal(prepared.Metadata, &doc) != nil {
		return nil, nil, publicationv2.ErrValidation
	}
	items := []struct {
		kind  ports.ArtifactKind
		value string
	}{{ports.KindCapability, doc.CapabilityRef}, {ports.KindTool, doc.ToolRef}, {ports.KindProvider, doc.ProviderRef}}
	var records []ports.CatalogRecord
	var refs []ports.ArtifactRef
	for _, item := range items {
		id, version, ok := strings.Cut(item.value, "@")
		if !ok || id == "" || version == "" || strings.Contains(version, "@") {
			return nil, nil, publicationv2.ErrValidation
		}
		ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: item.kind, ID: id, Version: version}
		record, err := catalog.Get(ctx, ref)
		if err != nil || record.Ref != ref || record.State != "published" {
			return nil, nil, publicationv2.ErrValidation
		}
		sum := sha256.Sum256(record.Metadata)
		got := hex.EncodeToString(sum[:])
		if record.Digest.Algorithm != "sha256" || record.Digest.Value != got || record.DocumentDigest.Algorithm != "sha256" || record.DocumentDigest.Value != got {
			return nil, nil, publicationv2.ErrValidation
		}
		records = append(records, record)
		refs = append(refs, ref)
	}
	return records, refs, nil
}

func skillCapabilityRefs(ctx context.Context, catalog ports.CatalogStore, p ports.Principal, prepared ports.PreparedPublication) ([]ports.ArtifactRef, error) {
	var manifest struct {
		Required []struct {
			ID      string `json:"id"`
			Version string `json:"contract_version"`
		} `json:"required_capabilities"`
	}
	if json.Unmarshal(prepared.Metadata, &manifest) != nil || len(manifest.Required) > 256 {
		return nil, publicationv2.ErrValidation
	}
	refs := make([]ports.ArtifactRef, 0, len(manifest.Required))
	seen := map[string]bool{}
	for _, required := range manifest.Required {
		ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: ports.KindCapability, ID: required.ID, Version: required.Version}
		key := ref.ID + "\x00" + ref.Version
		if required.ID == "" || required.Version == "" || seen[key] {
			return nil, publicationv2.ErrValidation
		}
		seen[key] = true
		record, err := catalog.Get(ctx, ref)
		if err != nil || record.Ref != ref || record.State != "published" {
			return nil, publicationv2.ErrValidation
		}
		sum := sha256.Sum256(record.Metadata)
		digest := hex.EncodeToString(sum[:])
		if record.Digest.Algorithm != "sha256" || record.Digest.Value != digest || record.DocumentDigest.Algorithm != "sha256" || record.DocumentDigest.Value != digest {
			return nil, publicationv2.ErrValidation
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mapPublicationError(err error) error {
	if errors.Is(err, publicationv2.ErrValidation) {
		return rest.ErrValidationFailed
	}
	if errors.Is(err, publicationv2.ErrBudgetExceeded) || errors.Is(err, storage.ErrPublicationBudget) {
		return rest.ErrBudgetExceeded
	}
	return err
}
