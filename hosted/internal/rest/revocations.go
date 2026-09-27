package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// ErrArtifactRevoked is the 409 artifact_revoked answer ADR 005 gives for a
// revoked pinned record after authorization. Adapters outside this package
// return (or wrap) it to surface a revocation.
var ErrArtifactRevoked = appError("artifact_revoked", "Artifact revoked", 409, false)

// ErrNotFound is the uniform 404 for a missing or foreign private record.
var ErrNotFound = appError("not_found", "Not found", 404, false)

// RevocationNotice is the immutable notice for one revoked artifact version.
type RevocationNotice struct {
	Ref       ports.ArtifactRef
	RevokedAt int64
}

// ArtifactRevoker records a revocation for an existing artifact version in
// the principal's workspace. It never creates a catalog version. A repeat for
// an already revoked version returns the original notice. A missing or
// foreign target returns ErrNotFound.
type ArtifactRevoker interface {
	RevokeArtifact(ctx context.Context, p ports.Principal, ref ports.ArtifactRef) (RevocationNotice, error)
}

var registryKey = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)

// publishRevocation serves POST /v1/publish/revocations. The body follows
// publish.schema.json; artifact names the exact target version. The workspace
// comes only from the authenticated principal.
func (h *Handler) publishRevocation(w http.ResponseWriter, r *http.Request, p ports.Principal) error {
	if err := h.authorize(r.Context(), p, ports.ActionPublish, nil); err != nil {
		return err
	}
	raw, err := readBody(r, h.limits.MaxBodyBytes)
	if err != nil {
		return err
	}
	var in struct {
		Artifact *struct {
			Kind    ports.ArtifactKind `json:"kind"`
			ID      string             `json:"id"`
			Version string             `json:"version"`
			Reason  string             `json:"reason"`
		} `json:"artifact"`
		MaxBytes       int    `json:"max_bytes"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || in.Artifact == nil || in.MaxBytes < 1 || in.IdempotencyKey == "" {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	a := in.Artifact
	switch a.Kind {
	case ports.KindSkill, ports.KindCapability, ports.KindTool, ports.KindProvider, ports.KindBinding, ports.KindTaxonomy:
	default:
		return appError("validation_failed", "Invalid request", 422, false)
	}
	if !registryKey.MatchString(a.ID) || a.Version == "" || len(a.Version) > maxVersionLength || len(a.Reason) > 1024 {
		return appError("validation_failed", "Invalid request", 422, false)
	}
	ref := ports.ArtifactRef{WorkspaceID: p.WorkspaceID, Kind: a.Kind, ID: a.ID, Version: a.Version}
	if err := h.authorize(r.Context(), p, ports.ActionPublish, &ref); err != nil {
		return err
	}
	if h.s.Revocations == nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	notice, err := h.s.Revocations.RevokeArtifact(r.Context(), p, ref)
	if err != nil {
		return err
	}
	out, err := json.Marshal(map[string]any{
		"notice": map[string]any{
			"kind":       string(notice.Ref.Kind),
			"id":         notice.Ref.ID,
			"version":    notice.Ref.Version,
			"state":      "revoked",
			"revoked_at": notice.RevokedAt,
		},
	})
	if err != nil {
		return appError("service_unavailable", "Service unavailable", 503, true)
	}
	if len(out) > in.MaxBytes {
		return appError("budget_exceeded", "Response exceeds the requested byte budget", 413, false)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(out)
	return err
}
