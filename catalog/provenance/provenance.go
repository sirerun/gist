// Package provenance authenticates caller-selected publisher attestations over
// immutable skill packages. It starts no service, discovers no trust, stores no
// keys, and grants no import, publication or execution authority.
package provenance

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/sirerun/gist/catalog/packages"
)

const Format = "gist.catalog.publisher-attestation/v1"
const domain = "gist.catalog.publisher-attestation/v1\x00"

// Subject is the complete, immutable identity a consumer expects. Every field
// must come from the consumer's admitted catalog/policy, not merely be copied
// from an untrusted envelope. Digests are lowercase SHA-256 hex; ArchiveDigest
// identifies the retained ZIP, ManifestDigest its exact manifest bytes, and
// PackageDigest the existing canonical complete-file inventory. SchemaDigest
// pins the caller-selected schema; schema conformance remains caller-owned.
// SourceRevision must be a lowercase 40- or 64-hex immutable Git commit ID.
type Subject struct {
	WorkspaceID      string `json:"workspace_id"`
	PublisherID      string `json:"publisher_id"`
	ArtifactKind     string `json:"artifact_kind"`
	ArtifactID       string `json:"artifact_id"`
	Version          string `json:"version"`
	SchemaID         string `json:"schema_id"`
	SchemaDigest     string `json:"schema_digest"`
	SourceRepository string `json:"source_repository"`
	SourceRevision   string `json:"source_revision"`
	ArchiveDigest    string `json:"archive_digest"`
	ManifestDigest   string `json:"manifest_digest"`
	PackageDigest    string `json:"package_digest"`
}

// Statement uses whole Unix seconds, no clock-skew allowance, maps or floating
// point values. KeyID is signed so changing an envelope's key identity cannot
// reuse a signature. Format and canonical field order are frozen for v1.
type Statement struct {
	Format    string  `json:"format"`
	KeyID     string  `json:"key_id"`
	Subject   Subject `json:"subject"`
	IssuedAt  int64   `json:"issued_at"`
	ExpiresAt int64   `json:"expires_at"`
}

type Envelope struct {
	Statement Statement `json:"statement"`
	Signature []byte    `json:"signature"`
}

// CanonicalBytes returns domain || compact JSON of the fixed Statement/Subject
// shapes, in their declared field order, using encoding/json string escaping.
// This is this format's canonical encoding, not a general RFC8785 JSON encoder.
// Verifiers reconstruct it from typed fields; raw envelope serialization is not
// signed. Unknown transport fields must never be treated as verified metadata.
func CanonicalBytes(statement Statement) ([]byte, error) {
	if err := validateStatement(statement); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(statement)
	if err != nil {
		return nil, fmt.Errorf("provenance: canonical statement: %w", err)
	}
	return append([]byte(domain), raw...), nil
}

// Sign asks a caller-owned Ed25519 signer to sign the canonical statement. The
// caller controls key custody and authenticates the publisher before signing.
// Signing does not enroll the key or establish consumer trust.
func Sign(statement Statement, signer crypto.Signer) (Envelope, error) {
	if key, ok := signer.(ed25519.PrivateKey); ok && len(key) != ed25519.PrivateKeySize {
		return Envelope{}, errors.New("provenance: invalid Ed25519 private key")
	}
	if signer == nil {
		return Envelope{}, errors.New("provenance: signer required")
	}
	public, ok := signer.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return Envelope{}, errors.New("provenance: Ed25519 signer required")
	}
	raw, err := CanonicalBytes(statement)
	if err != nil {
		return Envelope{}, err
	}
	signature, err := signer.Sign(rand.Reader, raw, crypto.Hash(0))
	if err != nil {
		return Envelope{}, fmt.Errorf("provenance: sign: %w", err)
	}
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(public, raw, signature) {
		return Envelope{}, errors.New("provenance: signer returned invalid signature")
	}
	return Envelope{Statement: statement, Signature: append([]byte(nil), signature...)}, nil
}

// TrustQuery names a publisher/key in exactly one consumer workspace. A resolver
// must authenticate its source and return the CURRENT authoritative state for
// this query, including publisher and key revocation. Never resolve from keys
// supplied by an envelope or use this API as unauthenticated network discovery.
type TrustQuery struct{ WorkspaceID, PublisherID, KeyID string }

type TrustResolver interface {
	ResolvePublisherTrust(context.Context, TrustQuery) (TrustSnapshot, error)
}

// TrustSnapshot contains only public verification material. Generation changes
// whenever relevant trust/identity/lifecycle policy changes; it is not a key ID.
// CheckedAt records authoritative freshness, not when an old cache was read.
// ValidUntil bounds that evidence; MaxTrustAge independently limits its age.
// NotBefore, CheckedAt and ValidUntil are mandatory. RetiredAt zero means active.
// Revocation of either publisher or key always rejects historical signatures.
type TrustSnapshot struct {
	Query            TrustQuery
	Generation       string
	PublicKey        ed25519.PublicKey
	CheckedAt        time.Time
	ValidUntil       time.Time
	NotBefore        time.Time
	RetiredAt        time.Time
	PublisherRevoked bool
	KeyRevoked       bool
}

type RetiredKeyPolicy uint8

const (
	// RejectRetired is the conservative default, including old signatures.
	RejectRetired RetiredKeyPolicy = iota
	// AllowIssuedBeforeRetirement permits still-unexpired attestations whose
	// issuance preceded retirement. Current authoritative non-revocation, key
	// validity at issuance and all normal freshness checks remain mandatory.
	AllowIssuedBeforeRetirement
)

type Verifier struct {
	Trust       TrustResolver
	Clock       func() time.Time
	MaxTrustAge time.Duration
	RetiredKeys RetiredKeyPolicy
}

// VerifiedSkill preserves the verified package without upgrading its manifest
// trust field. Evidence authenticates the signed publisher assertion, not the
// truth of its source claim, schema conformance, safety or execution permission.
// Consumers still enforce current catalog authorization/admission/revocation.
// Double trust checking detects observed changes, not atomic revocation: evidence
// is point-in-time at VerifiedAt, and must not become an offline permission cache.
type VerifiedSkill struct {
	Package         packages.Package
	Statement       Statement
	TrustGeneration string
	VerifiedAt      time.Time
}

// VerifySkill authenticates the caller-selected subject and every declared skill
// byte. It checks current publisher trust before and after archive validation.
// Expected must be independently selected by the caller; matching self-reported
// identity alone is not enrollment. Archive parsing has explicit caller limits.
func (v Verifier) VerifySkill(ctx context.Context, expected Subject, envelope Envelope, archive []byte, limits packages.Limits) (VerifiedSkill, error) {
	if v.Trust == nil || v.Clock == nil || v.MaxTrustAge <= 0 || (v.RetiredKeys != RejectRetired && v.RetiredKeys != AllowIssuedBeforeRetirement) {
		return VerifiedSkill{}, errors.New("provenance: explicit trust, clock, freshness and lifecycle policy required")
	}
	if err := ctx.Err(); err != nil {
		return VerifiedSkill{}, err
	}
	if envelope.Statement.Subject != expected {
		return VerifiedSkill{}, errors.New("provenance: unexpected subject")
	}
	raw, err := CanonicalBytes(envelope.Statement)
	if err != nil {
		return VerifiedSkill{}, err
	}
	if len(envelope.Signature) != ed25519.SignatureSize {
		return VerifiedSkill{}, errors.New("provenance: invalid signature size")
	}
	query := TrustQuery{expected.WorkspaceID, expected.PublisherID, envelope.Statement.KeyID}
	initial, _, err := v.check(ctx, query, envelope, raw)
	if err != nil {
		return VerifiedSkill{}, err
	}
	if limits.MaxPackageBytes <= 0 || limits.MaxFiles <= 0 || limits.MaxFileBytes <= 0 || len(archive) > limits.MaxPackageBytes {
		return VerifiedSkill{}, errors.New("provenance: explicit archive limits required")
	}
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != expected.ArchiveDigest {
		return VerifiedSkill{}, errors.New("provenance: archive digest mismatch")
	}
	parsed, err := packages.ReadArchive(archive, limits)
	if err != nil {
		return VerifiedSkill{}, fmt.Errorf("provenance: read skill: %w", err)
	}
	pkg, err := packages.Validate(parsed, limits)
	if err != nil {
		return VerifiedSkill{}, fmt.Errorf("provenance: validate skill: %w", err)
	}
	if pkg.Manifest.ID != expected.ArtifactID || pkg.Manifest.Version != expected.Version || pkg.ManifestDigest != expected.ManifestDigest || pkg.PackageDigest != expected.PackageDigest {
		return VerifiedSkill{}, errors.New("provenance: package identity or closure mismatch")
	}
	current, verifiedAt, err := v.check(ctx, query, envelope, raw)
	if err != nil {
		return VerifiedSkill{}, err
	}
	if current.Generation != initial.Generation || !current.PublicKey.Equal(initial.PublicKey) {
		return VerifiedSkill{}, errors.New("provenance: trust changed during verification")
	}
	return VerifiedSkill{Package: pkg, Statement: envelope.Statement, TrustGeneration: current.Generation, VerifiedAt: verifiedAt}, nil
}

func (v Verifier) check(ctx context.Context, query TrustQuery, envelope Envelope, raw []byte) (TrustSnapshot, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return TrustSnapshot{}, time.Time{}, err
	}
	trust, err := v.Trust.ResolvePublisherTrust(ctx, query)
	if err != nil {
		return TrustSnapshot{}, time.Time{}, fmt.Errorf("provenance: resolve current trust: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return TrustSnapshot{}, time.Time{}, err
	}
	now := v.Clock()
	if trust.Query != query || !validText(trust.Generation) || len(trust.PublicKey) != ed25519.PublicKeySize {
		return TrustSnapshot{}, time.Time{}, errors.New("provenance: unknown publisher/key or mismatched trust scope")
	}
	if trust.CheckedAt.IsZero() || trust.ValidUntil.IsZero() || trust.NotBefore.IsZero() || trust.CheckedAt.After(now) || !trust.ValidUntil.After(now) || now.Sub(trust.CheckedAt) > v.MaxTrustAge || !trust.ValidUntil.After(trust.CheckedAt) {
		return TrustSnapshot{}, time.Time{}, errors.New("provenance: stale or invalid trust evidence")
	}
	if trust.PublisherRevoked || trust.KeyRevoked {
		return TrustSnapshot{}, time.Time{}, errors.New("provenance: publisher or key revoked")
	}
	issued := time.Unix(envelope.Statement.IssuedAt, 0)
	expires := time.Unix(envelope.Statement.ExpiresAt, 0)
	if issued.After(now) || !expires.After(now) || issued.Before(trust.NotBefore) || trust.NotBefore.After(now) {
		return TrustSnapshot{}, time.Time{}, errors.New("provenance: attestation expired, future-issued or outside key validity")
	}
	if !trust.RetiredAt.IsZero() {
		if !trust.RetiredAt.After(trust.NotBefore) {
			return TrustSnapshot{}, time.Time{}, errors.New("provenance: invalid key lifecycle")
		}
		if !now.Before(trust.RetiredAt) && (v.RetiredKeys != AllowIssuedBeforeRetirement || !issued.Before(trust.RetiredAt)) {
			return TrustSnapshot{}, time.Time{}, errors.New("provenance: retired key not eligible")
		}
		if !issued.Before(trust.RetiredAt) {
			return TrustSnapshot{}, time.Time{}, errors.New("provenance: signed after retirement")
		}
	}
	if !ed25519.Verify(trust.PublicKey, raw, envelope.Signature) {
		return TrustSnapshot{}, time.Time{}, errors.New("provenance: invalid publisher signature")
	}
	trust.PublicKey = append(ed25519.PublicKey(nil), trust.PublicKey...)
	return trust, now, nil
}

func validateStatement(s Statement) error {
	if s.Format != Format || !validText(s.KeyID) || s.IssuedAt <= 0 || s.ExpiresAt <= s.IssuedAt {
		return errors.New("provenance: invalid statement format, key or interval")
	}
	subject := s.Subject
	if subject.ArtifactKind != "skill" {
		return errors.New("provenance: unsupported artifact kind")
	}
	for _, value := range []string{subject.WorkspaceID, subject.PublisherID, subject.ArtifactID, subject.Version, subject.SchemaID, subject.SourceRepository, subject.SourceRevision} {
		if !validText(value) {
			return errors.New("provenance: complete immutable subject required")
		}
	}
	for _, value := range []string{subject.ArchiveDigest, subject.ManifestDigest, subject.PackageDigest, subject.SchemaDigest} {
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != value {
			return errors.New("provenance: lowercase SHA-256 pins required")
		}
	}
	revision, err := hex.DecodeString(subject.SourceRevision)
	if err != nil || (len(revision) != 20 && len(revision) != 32) || hex.EncodeToString(revision) != subject.SourceRevision {
		return errors.New("provenance: immutable lowercase source commit required")
	}
	return nil
}

func validText(value string) bool {
	if value == "" || len(value) > 1024 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	return strings.IndexFunc(value, unicode.IsControl) < 0
}
