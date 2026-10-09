package ports

import "context"

// V2Publisher validates, admits, and persists a v2 publication. Body is the
// exact canonical receipt bytes persisted for replay; Created controls the
// HTTP status and is deliberately not part of the receipt.
type V2Publisher interface {
	PublishV2(context.Context, Principal, ArtifactKind, []byte) (PublicationResult, error)
}

// PublicationResult contains the publisher-owned immutable receipt and its
// digest domains. Optional skill-only digests are zero-valued for other kinds.
type PublicationResult struct {
	Body           []byte
	Created        bool
	ArtifactDigest Digest
	ManifestDigest Digest
	PackageDigest  Digest
}

// V2Reader returns exact retained bytes after current catalog-read policy and
// tenant/revocation checks. packageBody selects original ZIP bytes for skills.
type V2Reader interface {
	ReadV2(context.Context, Principal, ArtifactRef, int64, bool) (PublicationRead, error)
}

// PublicationRead contains the exact response body and its independent digest
// domains. ManifestDigest and PackageDigest are present only for skills.
type PublicationRead struct {
	Body           []byte
	ContentType    string
	ArtifactDigest Digest
	BodyDigest     Digest
	ManifestDigest Digest
	PackageDigest  Digest
}

// PreparedPublication is the byte-preserving decoder output consumed by app
// and storage. Ref identifies kind/id/version; WorkspaceID is supplied later
// from the authenticated Principal. Artifact holds the original ZIP for a
// skill or original document bytes otherwise. Metadata holds original
// manifest bytes for a skill or original document bytes otherwise. No JSON
// reserialization is permitted. IdempotencyKey and MaxBytes are client input.
type PreparedPublication struct {
	Ref            ArtifactRef
	IdempotencyKey string
	MaxBytes       int64
	Artifact       []byte
	Metadata       []byte
	ArtifactDigest Digest
	DocumentDigest Digest
	ManifestDigest Digest
	PackageDigest  Digest
	// AdmissionEvidence is populated only by app after trusted server review
	// validation and rechecked by its transaction fence. Decoder output leaves
	// it nil; it is never accepted from a client. Taxonomy projections retain
	// the exact qualified license instead of inventing a placeholder.
	AdmissionEvidence *PublicationAdmissionEvidence
}

// PublicationReceipt is the fixed successful HTTP receipt shape. Its Body is
// serialized and persisted by the publisher. It contains no workspace,
// replay, or timestamp fields.
type PublicationReceipt struct {
	Kind           ArtifactKind `json:"kind"`
	ID             string       `json:"id"`
	Version        string       `json:"version"`
	ArtifactDigest string       `json:"artifact_digest"`
	ManifestDigest string       `json:"manifest_digest,omitempty"`
	PackageDigest  string       `json:"package_digest,omitempty"`
}

// PublicationAdmissionEvidence is server-only evidence decoded from a
// trusted approved review record. Its JSON shape mirrors
// admission-evidence.schema.json; digest values are sha256:hex strings and
// timestamps remain strings for separate parsing. Retained base64 fields are
// the schema's explicit representation of source/grant/fixture bytes; no
// decoded byte fields act as source identity. This type is never client input.
type PublicationAdmissionEvidence struct {
	EvidenceVersion       string                            `json:"evidence_version"`
	WorkspaceID           string                            `json:"workspace_id"`
	Artifact              PublicationEvidenceArtifact       `json:"artifact"`
	Source                PublicationEvidenceSource         `json:"source"`
	Rights                PublicationEvidenceRights         `json:"rights"`
	Reviewer              PublicationEvidenceReviewer       `json:"reviewer"`
	Synthetic             bool                              `json:"synthetic"`
	OriginPublisher       *PublicationEvidenceOrigin        `json:"origin_publisher,omitempty"`
	ApprovedSupportState  string                            `json:"approved_support_state,omitempty"`
	ProviderQualification *PublicationProviderQualification `json:"provider_qualification,omitempty"`
	Conformance           *PublicationConformance           `json:"conformance,omitempty"`
	TaxonomyGrant         *PublicationTaxonomyGrant         `json:"taxonomy_grant,omitempty"`
}

type PublicationEvidenceArtifact struct {
	Kind           ArtifactKind `json:"kind"`
	ID             string       `json:"id"`
	Version        string       `json:"version"`
	ArtifactDigest string       `json:"artifact_digest"`
}

type PublicationEvidenceSource struct {
	URI                 string                       `json:"uri"`
	CapturedAt          string                       `json:"captured_at"`
	Collector           PublicationEvidenceCollector `json:"collector"`
	CaptureDigest       string                       `json:"capture_digest"`
	RetainedBytesBase64 string                       `json:"retained_bytes_base64"`
	MediaType           string                       `json:"media_type"`
}

type PublicationEvidenceCollector struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type PublicationEvidenceRights struct {
	License             string   `json:"license"`
	Redistribution      bool     `json:"redistribution"`
	Scope               string   `json:"scope"`
	GrantDigest         string   `json:"grant_digest"`
	AllowedUse          []string `json:"allowed_use"`
	RetainedGrantBase64 string   `json:"retained_grant_base64"`
}

type PublicationEvidenceReviewer struct {
	Issuer     string `json:"issuer"`
	Subject    string `json:"subject"`
	ReviewedAt string `json:"reviewed_at"`
}

type PublicationEvidenceOrigin struct {
	ID             string `json:"id"`
	SourceRevision string `json:"source_revision,omitempty"`
}

type PublicationProviderQualification struct {
	ReceiptRef    string `json:"receipt_ref"`
	ReceiptDigest string `json:"receipt_digest"`
	ValidUntil    string `json:"valid_until"`
}

type PublicationConformance struct {
	AdapterVersion        string   `json:"adapter_version"`
	ExecutorID            string   `json:"executor_id"`
	ExecutorRevision      string   `json:"executor_revision"`
	FixtureDigest         string   `json:"fixture_digest"`
	RetainedFixtureBase64 string   `json:"retained_fixture_base64"`
	ExecutedCaseCount     int      `json:"executed_case_count"`
	ResultDigest          string   `json:"result_digest"`
	ExactRefs             []string `json:"exact_refs"`
	Passed                bool     `json:"passed"`
}

type PublicationTaxonomyGrant struct {
	TaxonomyID    string   `json:"taxonomy_id"`
	Edition       string   `json:"edition"`
	AllowedFields []string `json:"allowed_fields"`
}

// PublicationBindingVerifier is implemented only by a configured, qualified
// offline executor. An unavailable verifier must fail closed; a generic
// success stub does not satisfy this contract.
type PublicationBindingVerifier interface {
	VerifyPublicationBinding(context.Context, []CatalogRecord, PublicationAdmissionEvidence, []byte) error
}
