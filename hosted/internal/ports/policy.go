package ports

import "context"

type Action string

const (
	ActionRead    Action = "catalog:read"
	ActionPublish Action = "catalog:publish"
	// ActionIdentityRevoke revokes a workload identity in the caller's own
	// workspace. It requires the identity:revoke scope and the maintainer role.
	ActionIdentityRevoke Action = "identity:revoke"
)

type Principal struct {
	Issuer           string
	Subject          string
	Audience         string
	WorkspaceID      string
	Scopes           []string
	PolicyGeneration uint64
	SubjectType      string
}

type Decision struct {
	Allowed bool
	Status  int
	Reason  string
}

type Authorizer interface {
	Decide(context.Context, Principal, Action, *ArtifactRef) (Decision, error)
}

type Cursor struct {
	ID            string
	PrincipalHash string
	WorkspaceID   string
	ExpiresAt     int64
}

type Resolution struct {
	ID        string
	Principal Principal
	Skill     ArtifactRef
	SkillPin  ArtifactPin
	ExpiresAt int64
	Findings  []Finding
}

// ArtifactPin is the immutable artifact identity retained with a resolution.
type ArtifactPin struct {
	Ref            ArtifactRef
	Digest         Digest
	ManifestDigest Digest
}

type Finding struct {
	CapabilityID, Status, ConnectURL string
	BindingRef                       ArtifactRef
	Closure                          []ArtifactPin
	Required                         bool
	Provenance                       string
}

type ResolutionStore interface {
	Put(context.Context, Resolution) error
	Get(context.Context, Cursor) (Resolution, error)
}
