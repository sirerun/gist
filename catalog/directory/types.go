// Package directory provides public discovery metadata. It never connects to or
// executes servers and never holds account credentials.
package directory

import "time"

const SnapshotFormat = "gist.directory.snapshot/v1"
const ExportFormat = "gist.directory.entry/v1"

type Source struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Revision    string    `json:"revision"`
	SHA256      string    `json:"sha256"`
	RetrievedAt time.Time `json:"retrieved_at"`
	License     string    `json:"license,omitempty"`
	Attribution string    `json:"attribution"`
}
type RecordSource struct {
	ID      string `json:"id"`
	EntryID string `json:"entry_id"`
	URL     string `json:"url"`
}
type Remote struct {
	URL       string         `json:"url"`
	Transport string         `json:"transport"`
	Check     *EndpointCheck `json:"check,omitempty"`
}

// EndpointCheck records an unauthenticated endpoint reachability probe. It is
// intentionally separate from Qualification, which requires an actual client
// workflow and pinned tool/schema evidence.
type EndpointCheck struct {
	CheckedAt  time.Time `json:"checked_at"`
	HTTPStatus int       `json:"http_status"`
	Result     string    `json:"result"` // mcp-initialize, endpoint-responded, authentication-required or payment-required
}
type Package struct {
	Registry   string `json:"registry"`
	Identifier string `json:"identifier"`
	Version    string `json:"version,omitempty"`
	Runtime    string `json:"runtime,omitempty"`
}
type OpenAPI struct {
	SpecURL      string `json:"spec_url"`
	MintGuideURL string `json:"mint_guide_url"`
}
type Auth struct {
	Type            string   `json:"type"`
	CredentialNames []string `json:"credential_names,omitempty"`
	SetupURL        string   `json:"setup_url,omitempty"`
}
type Skill struct {
	SourceDigest string `json:"source_digest,omitempty"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	URL          string `json:"url,omitempty"`
	Status       string `json:"status"`
}
type Qualification struct {
	DescriptorDigest string    `json:"descriptor_digest"`
	ClientVersion    string    `json:"client_version"`
	ToolDigest       string    `json:"tool_digest"`
	SchemaDigest     string    `json:"schema_digest"`
	SkillDigest      string    `json:"skill_digest"`
	Client           string    `json:"client"`
	Workflow         string    `json:"workflow"`
	Version          string    `json:"version"`
	EvidenceURL      string    `json:"evidence_url"`
	CheckedAt        time.Time `json:"checked_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	Result           string    `json:"result"`
}
type Record struct {
	ID                  string          `json:"id"`
	Kind                string          `json:"kind"` // mcp or openapi
	ServiceID           string          `json:"service_id"`
	Title               string          `json:"title"`
	Description         string          `json:"description"`
	Version             string          `json:"version,omitempty"`
	Tags                []string        `json:"tags"`
	Source              RecordSource    `json:"source"`
	Homepage            string          `json:"homepage,omitempty"`
	Repository          string          `json:"repository,omitempty"`
	Remotes             []Remote        `json:"remotes,omitempty"`
	Packages            []Package       `json:"packages,omitempty"`
	OpenAPI             *OpenAPI        `json:"openapi,omitempty"`
	Auth                Auth            `json:"auth"`
	Skills              []Skill         `json:"skills,omitempty"`
	Verification        []Qualification `json:"verification,omitempty"`
	Recommended         bool            `json:"recommended"`
	Official            bool            `json:"official"` // vendor evidence, never registry membership
	OfficialEvidenceURL string          `json:"official_evidence_url,omitempty"`
	ServerListingURL    string          `json:"server_listing_url,omitempty"`
	FeaturedSelection   bool            `json:"featured_selection,omitempty"` // operator-curated only
	Featured            bool            `json:"featured"`                     // computed by Index using current time
}
type Snapshot struct {
	Format      string    `json:"format"`
	Revision    string    `json:"revision"`
	GeneratedAt time.Time `json:"generated_at"`
	Sources     []Source  `json:"sources"`
	Entries     []Record  `json:"entries"`
}
type Query struct {
	Text   string
	Kind   string
	Tag    string
	Cursor string
	Limit  int
}
type Result struct {
	Items            []Record `json:"items"`
	NextCursor       string   `json:"next_cursor,omitempty"`
	Total            int      `json:"total"`
	SnapshotRevision string   `json:"snapshot_revision"`
}
type Export struct {
	Format string `json:"format"`
	Record Record `json:"record"`
}
