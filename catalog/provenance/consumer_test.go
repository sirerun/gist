package provenance_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/provenance"
	"github.com/sirerun/gist/catalog/skills/inboxbriefing"
)

type resolver struct {
	state  provenance.TrustSnapshot
	calls  int
	mutate func(*provenance.TrustSnapshot, int)
	err    error
}

func (r *resolver) ResolvePublisherTrust(_ context.Context, query provenance.TrustQuery) (provenance.TrustSnapshot, error) {
	r.calls++
	state := r.state
	if r.mutate != nil {
		r.mutate(&state, r.calls)
	}
	if query != r.state.Query {
		return provenance.TrustSnapshot{}, errors.New("not enrolled")
	}
	return state, r.err
}

type fixture struct {
	archive  []byte
	subject  provenance.Subject
	envelope provenance.Envelope
	key      ed25519.PrivateKey
	trust    *resolver
	verifier provenance.Verifier
	limits   packages.Limits
	now      time.Time
}

func setup(t *testing.T) fixture {
	t.Helper()
	archive, err := inboxbriefing.Archive()
	if err != nil {
		t.Fatal(err)
	}
	limits := packages.Limits{MaxPackageBytes: 1 << 20, MaxFileBytes: 1 << 18, MaxFiles: 20}
	a, err := packages.ReadArchive(archive, limits)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := packages.Validate(a, limits)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive)
	schema := sha256.Sum256([]byte("caller-selected immutable skill schema fixture"))
	subject := provenance.Subject{
		WorkspaceID: "workspace-a", PublisherID: "publisher-a", ArtifactKind: "skill", ArtifactID: inboxbriefing.SkillID, Version: inboxbriefing.Version,
		SchemaID: "urn:gist:skill-manifest:fixture-v1", SchemaDigest: hex.EncodeToString(schema[:]), SourceRepository: "https://example.test/maintained-skills", SourceRevision: strings.Repeat("a", 40),
		ArchiveDigest: hex.EncodeToString(digest[:]), ManifestDigest: pkg.ManifestDigest, PackageDigest: pkg.PackageDigest,
	}
	now := time.Unix(1800000000, 0).UTC()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	statement := provenance.Statement{Format: provenance.Format, KeyID: "publisher-key-1", Subject: subject, IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	envelope, err := provenance.Sign(statement, key)
	if err != nil {
		t.Fatal(err)
	}
	trust := &resolver{state: provenance.TrustSnapshot{Query: provenance.TrustQuery{WorkspaceID: subject.WorkspaceID, PublisherID: subject.PublisherID, KeyID: statement.KeyID}, Generation: "generation-1", PublicKey: key.Public().(ed25519.PublicKey), CheckedAt: now.Add(-time.Second), ValidUntil: now.Add(time.Minute), NotBefore: now.Add(-time.Hour)}}
	return fixture{archive, subject, envelope, key, trust, provenance.Verifier{Trust: trust, Clock: func() time.Time { return now }, MaxTrustAge: 30 * time.Second}, limits, now}
}
func (f fixture) verify() (provenance.VerifiedSkill, error) {
	return f.verifier.VerifySkill(context.Background(), f.subject, f.envelope, f.archive, f.limits)
}
func (f *fixture) resign(t *testing.T) {
	t.Helper()
	e, err := provenance.Sign(f.envelope.Statement, f.key)
	if err != nil {
		t.Fatal(err)
	}
	f.envelope = e
}

func TestConsumerVerifiesMaintainedPackageAndEveryAsset(t *testing.T) {
	f := setup(t)
	result, err := f.verify()
	if err != nil {
		t.Fatal(err)
	}
	want, err := inboxbriefing.Files()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Package.Files) != len(want) {
		t.Fatal("incomplete package")
	}
	for path, content := range want {
		if !bytes.Equal(content, result.Package.Files[path]) {
			t.Fatalf("asset mismatch: %s", path)
		}
	}
	if result.Statement.Subject != f.subject || result.TrustGeneration != "generation-1" || !result.VerifiedAt.Equal(f.now) || f.trust.calls != 2 {
		t.Fatal("missing point-in-time provenance evidence")
	}
	if result.Package.Manifest.Trust != "operator_asserted" {
		t.Fatal("signature must not silently upgrade import/publication trust")
	}
}

func TestCanonicalStatementGoldenAndTransportOrderIndependence(t *testing.T) {
	f := setup(t)
	raw, err := provenance.CanonicalBytes(f.envelope.Statement)
	if err != nil {
		t.Fatal(err)
	}
	want := "gist.catalog.publisher-attestation/v1\x00" + `{"format":"gist.catalog.publisher-attestation/v1","key_id":"publisher-key-1","subject":{"workspace_id":"workspace-a","publisher_id":"publisher-a","artifact_kind":"skill","artifact_id":"zati/inbox-briefing","version":"1.0.0","schema_id":"urn:gist:skill-manifest:fixture-v1","schema_digest":"` + f.subject.SchemaDigest + `","source_repository":"https://example.test/maintained-skills","source_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","archive_digest":"` + f.subject.ArchiveDigest + `","manifest_digest":"` + f.subject.ManifestDigest + `","package_digest":"` + f.subject.PackageDigest + `"},"issued_at":1799999940,"expires_at":1800003600}`
	if string(raw) != want {
		t.Fatalf("canonical bytes changed:\n%s", raw)
	}
	// JSON formatting/key order are transport details, not signed bytes.
	reordered := `{"signature":` + mustJSON(t, f.envelope.Signature) + `,"statement":` + mustJSON(t, f.envelope.Statement) + `}`
	var decoded provenance.Envelope
	if err := json.Unmarshal([]byte(reordered), &decoded); err != nil {
		t.Fatal(err)
	}
	f.envelope = decoded
	if _, err := f.verify(); err != nil {
		t.Fatal(err)
	}
	again, err := provenance.Sign(f.envelope.Statement, f.key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again.Signature, f.envelope.Signature) {
		t.Fatal("deterministic signature changed")
	}
}
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func TestConsumerRejectsIdentityAndByteTampering(t *testing.T) {
	cases := map[string]func(*fixture){
		"workspace":                   func(f *fixture) { f.subject.WorkspaceID = "other" },
		"publisher":                   func(f *fixture) { f.subject.PublisherID = "other" },
		"artifact":                    func(f *fixture) { f.subject.ArtifactID = "other" },
		"version":                     func(f *fixture) { f.subject.Version = "2.0.0" },
		"schema":                      func(f *fixture) { f.subject.SchemaID = "other" },
		"schema digest":               func(f *fixture) { f.subject.SchemaDigest = strings.Repeat("b", 64) },
		"source":                      func(f *fixture) { f.subject.SourceRepository = "https://other.test" },
		"source revision":             func(f *fixture) { f.subject.SourceRevision = strings.Repeat("b", 40) },
		"archive pin":                 func(f *fixture) { f.subject.ArchiveDigest = strings.Repeat("b", 64) },
		"manifest pin":                func(f *fixture) { f.subject.ManifestDigest = strings.Repeat("b", 64) },
		"closure pin":                 func(f *fixture) { f.subject.PackageDigest = strings.Repeat("b", 64) },
		"archive bytes":               func(f *fixture) { f.archive = append([]byte(nil), f.archive...); f.archive[20] ^= 1 },
		"signature":                   func(f *fixture) { f.envelope.Signature[0] ^= 1 },
		"short signature":             func(f *fixture) { f.envelope.Signature = []byte{1} },
		"key substitution":            func(f *fixture) { f.envelope.Statement.KeyID = "publisher-key-2" },
		"signed statement alteration": func(f *fixture) { f.envelope.Statement.ExpiresAt++ },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			mutate(&f)
			if _, err := f.verify(); err == nil {
				t.Fatal("accepted tampered subject or bytes")
			}
		})
	}
}

func TestConsumerRejectsInvalidCurrentTrust(t *testing.T) {
	cases := map[string]func(*fixture){
		"missing resolver":         func(f *fixture) { f.verifier.Trust = nil },
		"missing clock":            func(f *fixture) { f.verifier.Clock = nil },
		"unbounded freshness":      func(f *fixture) { f.verifier.MaxTrustAge = 0 },
		"unknown lifecycle policy": func(f *fixture) { f.verifier.RetiredKeys = 42 },
		"resolver unavailable":     func(f *fixture) { f.trust.err = errors.New("unavailable") },
		"unknown publisher":        func(f *fixture) { f.trust.state.Query.PublisherID = "unknown" },
		"returned workspace mismatch": func(f *fixture) {
			f.trust.mutate = func(s *provenance.TrustSnapshot, _ int) { s.Query.WorkspaceID = "other" }
		},
		"returned publisher mismatch": func(f *fixture) {
			f.trust.mutate = func(s *provenance.TrustSnapshot, _ int) { s.Query.PublisherID = "other" }
		},
		"returned key mismatch": func(f *fixture) {
			f.trust.mutate = func(s *provenance.TrustSnapshot, _ int) { s.Query.KeyID = "other" }
		},
		"missing generation":      func(f *fixture) { f.trust.state.Generation = "" },
		"missing public key":      func(f *fixture) { f.trust.state.PublicKey = nil },
		"wrong length public key": func(f *fixture) { f.trust.state.PublicKey = make([]byte, 31) },
		"different public key": func(f *fixture) {
			f.trust.state.PublicKey = ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)
		},
		"publisher revoked":           func(f *fixture) { f.trust.state.PublisherRevoked = true },
		"key revoked":                 func(f *fixture) { f.trust.state.KeyRevoked = true },
		"missing authoritative check": func(f *fixture) { f.trust.state.CheckedAt = time.Time{} },
		"stale authoritative check":   func(f *fixture) { f.trust.state.CheckedAt = f.now.Add(-31 * time.Second) },
		"future check":                func(f *fixture) { f.trust.state.CheckedAt = f.now.Add(time.Second) },
		"expired trust":               func(f *fixture) { f.trust.state.ValidUntil = f.now },
		"missing trust expiry":        func(f *fixture) { f.trust.state.ValidUntil = time.Time{} },
		"missing key start":           func(f *fixture) { f.trust.state.NotBefore = time.Time{} },
		"key not active at issuance":  func(f *fixture) { f.trust.state.NotBefore = f.now },
		"key not active now":          func(f *fixture) { f.trust.state.NotBefore = f.now.Add(time.Second) },
		"retired default":             func(f *fixture) { f.trust.state.RetiredAt = f.now.Add(-time.Second) },
		"revoked during validation":   func(f *fixture) { f.trust.mutate = func(s *provenance.TrustSnapshot, n int) { s.KeyRevoked = n == 2 } },
		"publisher revoked during validation": func(f *fixture) {
			f.trust.mutate = func(s *provenance.TrustSnapshot, n int) { s.PublisherRevoked = n == 2 }
		},
		"generation changed during validation": func(f *fixture) {
			f.trust.mutate = func(s *provenance.TrustSnapshot, n int) {
				if n == 2 {
					s.Generation = "generation-2"
				}
			}
		},
		"expiry during validation": func(f *fixture) {
			calls := 0
			f.verifier.Clock = func() time.Time {
				calls++
				if calls > 1 {
					return f.now.Add(2 * time.Hour)
				}
				return f.now
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			mutate(&f)
			if _, err := f.verify(); err == nil {
				t.Fatal("accepted invalid trust")
			}
		})
	}
}

func TestHistoricalRetirementIsExplicitAndRevocationAlwaysWins(t *testing.T) {
	f := setup(t)
	f.trust.state.RetiredAt = f.now.Add(-time.Second)
	f.verifier.RetiredKeys = provenance.AllowIssuedBeforeRetirement
	if _, err := f.verify(); err != nil {
		t.Fatal(err)
	}
	f.trust.state.KeyRevoked = true
	if _, err := f.verify(); err == nil {
		t.Fatal("historical policy bypassed revocation")
	}
	f.trust.state.KeyRevoked = false
	f.envelope.Statement.IssuedAt = f.trust.state.RetiredAt.Unix()
	f.resign(t)
	if _, err := f.verify(); err == nil {
		t.Fatal("accepted issuance at retirement")
	}
}

func TestRotationRequiresExplicitNewEnrollment(t *testing.T) {
	f := setup(t)
	f.key = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32))
	f.envelope.Statement.KeyID = "publisher-key-2"
	f.resign(t)
	if _, err := f.verify(); err == nil {
		t.Fatal("unregistered rotated key accepted")
	}
	f.trust.state.Query.KeyID = "publisher-key-2"
	f.trust.state.PublicKey = f.key.Public().(ed25519.PublicKey)
	f.trust.state.Generation = "generation-2"
	if _, err := f.verify(); err != nil {
		t.Fatal(err)
	}
}

func TestAttestationTimeAndStatementValidation(t *testing.T) {
	for name, mutate := range map[string]func(*provenance.Statement){
		"expired":       func(s *provenance.Statement) { s.ExpiresAt = 1800000000 },
		"future issued": func(s *provenance.Statement) { s.IssuedAt = 1800000001 },
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			mutate(&f.envelope.Statement)
			f.resign(t)
			if _, err := f.verify(); err == nil {
				t.Fatal("accepted invalid time")
			}
		})
	}
	for name, mutate := range map[string]func(*provenance.Statement){
		"wrong domain version":  func(s *provenance.Statement) { s.Format = "other" },
		"negative issued":       func(s *provenance.Statement) { s.IssuedAt = -1 },
		"empty interval":        func(s *provenance.Statement) { s.ExpiresAt = s.IssuedAt },
		"missing key":           func(s *provenance.Statement) { s.KeyID = "" },
		"unknown artifact kind": func(s *provenance.Statement) { s.Subject.ArtifactKind = "tool" },
		"empty schema":          func(s *provenance.Statement) { s.Subject.SchemaID = "" },
		"mutable source":        func(s *provenance.Statement) { s.Subject.SourceRevision = "main" },
		"noncanonical digest":   func(s *provenance.Statement) { s.Subject.ManifestDigest = strings.Repeat("A", 64) },
		"oversized identity":    func(s *provenance.Statement) { s.Subject.PublisherID = strings.Repeat("x", 1025) },
		"invalid UTF8":          func(s *provenance.Statement) { s.Subject.PublisherID = string([]byte{0xff}) },
		"control bytes":         func(s *provenance.Statement) { s.KeyID = "key\n" },
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			mutate(&f.envelope.Statement)
			if _, err := provenance.Sign(f.envelope.Statement, f.key); err == nil {
				t.Fatal("signed invalid statement")
			}
		})
	}
}

// Re-sign altered claims and recompute outer ZIP pins: neither a signature nor
// an authentic publisher assertion can substitute for verifying actual content.
func TestSignedPackageStillRequiresCompleteManifestAndInventory(t *testing.T) {
	for _, which := range []string{"manifest pin", "closure pin", "artifact identity", "missing asset", "altered asset"} {
		t.Run(which, func(t *testing.T) {
			f := setup(t)
			switch which {
			case "manifest pin":
				f.subject.ManifestDigest = strings.Repeat("b", 64)
			case "closure pin":
				f.subject.PackageDigest = strings.Repeat("b", 64)
			case "artifact identity":
				f.subject.ArtifactID = "another/skill"
			default:
				files, err := inboxbriefing.Files()
				if err != nil {
					t.Fatal(err)
				}
				if which == "missing asset" {
					delete(files, inboxbriefing.TemplatePath)
				} else {
					files[inboxbriefing.TemplatePath] = []byte("altered template")
				}
				var out bytes.Buffer
				w := zip.NewWriter(&out)
				for path, data := range files {
					part, err := w.Create(path)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := part.Write(data); err != nil {
						t.Fatal(err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				f.archive = out.Bytes()
				d := sha256.Sum256(f.archive)
				f.subject.ArchiveDigest = hex.EncodeToString(d[:])
			}
			f.envelope.Statement.Subject = f.subject
			f.resign(t)
			if _, err := f.verify(); err == nil {
				t.Fatal("signature bypassed content verification")
			}
		})
	}
}

func TestExplicitBoundsAndCancellation(t *testing.T) {
	for _, which := range []string{"no bounds", "archive too large", "file too large", "too many files"} {
		t.Run(which, func(t *testing.T) {
			f := setup(t)
			switch which {
			case "no bounds":
				f.limits = packages.Limits{}
			case "archive too large":
				f.limits.MaxPackageBytes = len(f.archive) - 1
			case "file too large":
				f.limits.MaxFileBytes = 1
			case "too many files":
				f.limits.MaxFiles = 1
			}
			if _, err := f.verify(); err == nil {
				t.Fatal("ignored archive bounds")
			}
		})
	}
	f := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.verifier.VerifySkill(ctx, f.subject, f.envelope, f.archive, f.limits); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if f.trust.calls != 0 {
		t.Fatal("resolved trust despite cancellation")
	}
}

type brokenSigner struct{ public crypto.PublicKey }

func (s brokenSigner) Public() crypto.PublicKey { return s.public }
func (s brokenSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return []byte{1}, nil
}
func TestSignerRequiresEd25519AndValidReturnedSignature(t *testing.T) {
	f := setup(t)
	for _, signer := range []crypto.Signer{nil, ed25519.PrivateKey(nil), ed25519.PrivateKey(make([]byte, 3)), brokenSigner{public: []byte{1}}, brokenSigner{public: f.key.Public()}} {
		if _, err := provenance.Sign(f.envelope.Statement, signer); err == nil {
			t.Fatal("invalid signer accepted")
		}
	}
}
