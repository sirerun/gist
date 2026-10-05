package ports

import "context"

// ArtifactPin retains immutable digest identity without altering frozen v1 ports.
type ArtifactPin struct {
	Ref            ArtifactRef
	Digest         Digest
	ManifestDigest Digest
}

type PinnedFinding struct {
	CapabilityID, Status, ConnectURL string
	BindingRef                       ArtifactRef
	Closure                          []ArtifactPin
	Required                         bool
	Provenance                       string
}

// PinnedResolution persists the canonical root and complete resolved closure.
// Frozen Resolution/ResolutionStore stay available to legacy consumers.
type PinnedResolution struct {
	ID        string
	Principal Principal
	Skill     ArtifactRef
	SkillPin  ArtifactPin
	ExpiresAt int64
	Findings  []PinnedFinding
}

type PinnedResolutionStore interface {
	PutPinnedResolution(context.Context, PinnedResolution) error
	GetPinnedResolution(context.Context, Cursor) (PinnedResolution, error)
}
