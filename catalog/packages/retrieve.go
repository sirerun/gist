package packages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/sirerun/gist/catalog/ports"
)

var ErrDenied = errors.New("packages: retrieval denied")
var ErrBudgetExceeded = errors.New("packages: retrieval budget exceeded")

type Envelope struct {
	Record ports.CatalogRecord
	Bytes  []byte
	Size   int64
}
type Retriever struct {
	Authorizer ports.Authorizer
	Catalog    ports.CatalogStore
	Artifacts  ports.ArtifactStore
}

func (r Retriever) Retrieve(ctx context.Context, principal ports.Principal, ref ports.ArtifactRef, maxBytes int64) (Envelope, error) {
	if r.Authorizer == nil || r.Catalog == nil || r.Artifacts == nil {
		return Envelope{}, errors.New("packages: incomplete retrieval service")
	}
	decision, err := r.Authorizer.Decide(ctx, principal, ports.ActionRead, &ref)
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: authorize retrieval: %w", err)
	}
	if !decision.Allowed {
		return Envelope{}, ErrDenied
	}
	record, err := r.Catalog.Get(ctx, ref)
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: load record: %w", err)
	}
	if principal.WorkspaceID == "" || ref.WorkspaceID != principal.WorkspaceID || record.Ref != ref || (record.State != "published" && record.State != "deprecated") {
		return Envelope{}, errors.New("packages: artifact identity, workspace or admission invalid")
	}
	reader, err := r.Artifacts.Open(ctx, ref)
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: open artifact: %w", err)
	}
	defer reader.Close()
	if maxBytes <= 0 {
		maxBytes = 64 << 20
	}
	b, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: read artifact: %w", err)
	}
	if int64(len(b)) > maxBytes {
		return Envelope{}, ErrBudgetExceeded
	}
	return Envelope{Record: record, Bytes: b, Size: int64(len(b))}, nil
}

// RetrieveSkill verifies a skill against caller-trusted immutable pins. It does
// not authenticate a publisher signature; callers establish pin trust separately.
func (r Retriever) RetrieveSkill(ctx context.Context, principal ports.Principal, expected ports.ArtifactPin, limits Limits) (Package, error) {
	if principal.Subject == "" || principal.WorkspaceID == "" || expected.Ref.WorkspaceID != principal.WorkspaceID || expected.Ref.Kind != ports.KindSkill || expected.Ref.ID == "" || expected.Ref.Version == "" {
		return Package{}, fmt.Errorf("packages: invalid skill identity or workspace")
	}
	if !validPinDigest(expected.Digest) || !validPinDigest(expected.ManifestDigest) {
		return Package{}, fmt.Errorf("packages: trusted sha256 pins required")
	}
	if limits.MaxPackageBytes <= 0 || limits.MaxFileBytes <= 0 || limits.MaxFiles <= 0 {
		return Package{}, fmt.Errorf("packages: invalid archive limits")
	}
	envelope, err := r.Retrieve(ctx, principal, expected.Ref, int64(limits.MaxPackageBytes))
	if err != nil {
		return Package{}, err
	}
	record := envelope.Record
	if record.Ref != expected.Ref || (record.State != "published" && record.State != "deprecated") || record.Digest != expected.Digest || record.ManifestDigest != expected.ManifestDigest {
		return Package{}, fmt.Errorf("packages: current record does not match admitted trusted pin")
	}
	// Artifact digest identifies retained ZIP bytes for this public API.
	sum := sha256.Sum256(envelope.Bytes)
	if hex.EncodeToString(sum[:]) != expected.Digest.Value {
		return Package{}, fmt.Errorf("packages: artifact digest mismatch")
	}
	archive, err := ReadArchive(envelope.Bytes, limits)
	if err != nil {
		return Package{}, err
	}
	pkg, err := Validate(archive, limits)
	if err != nil {
		return Package{}, err
	}
	if pkg.Manifest.ID != expected.Ref.ID || pkg.Manifest.Version != expected.Ref.Version || pkg.ManifestDigest != expected.ManifestDigest.Value {
		return Package{}, fmt.Errorf("packages: manifest identity or digest mismatch")
	}
	if record.PackageDigest.Algorithm != "" && (record.PackageDigest.Algorithm != "sha256" || record.PackageDigest.Value != pkg.PackageDigest) {
		return Package{}, fmt.Errorf("packages: package closure mismatch")
	}
	// Recheck after I/O so a revocation observed during retrieval cannot escape.
	current, err := r.Catalog.Get(ctx, expected.Ref)
	if err != nil {
		return Package{}, err
	}
	if current.Ref != record.Ref || current.Digest != record.Digest || current.ManifestDigest != record.ManifestDigest || current.PackageDigest != record.PackageDigest || (current.State != "published" && current.State != "deprecated") {
		return Package{}, fmt.Errorf("packages: artifact changed during retrieval")
	}
	decision, err := r.Authorizer.Decide(ctx, principal, ports.ActionRead, &expected.Ref)
	if err != nil {
		return Package{}, err
	}
	if !decision.Allowed {
		return Package{}, ErrDenied
	}
	return pkg, nil
}
func validPinDigest(d ports.Digest) bool {
	if d.Algorithm != "sha256" || len(d.Value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(d.Value)
	return err == nil && hex.EncodeToString(decoded) == d.Value
}
