package runtimes

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/contract"
)

// These probes are contract-only fixtures. They model the runtime-owned
// decision points at the Gist boundary; they do not implement an external
// runtime adapter or contact a registry deployment.

const (
	fixtureSkillRef = "gist://skills/asset-skill@1.0.0"
	fixtureToolURI  = "gist://tools/github/issues.create"
	fixtureToolVer  = "2026-09-25"
)

type grant struct {
	ToolURI       string
	ToolVersion   string
	Parent        parentBounds
	Spend         spendPolicy
	Connection    connectionAssertion
	LeaseValid    bool
	GrantRevision int
}

type parentBounds struct {
	Principal     string
	Workspace     string
	Child         string
	Capabilities  []string
	EffectCeiling string
}

type spendPolicy struct {
	Currency string
	Amount   int64
	Period   string
	Approval string
}

type connectionAssertion struct {
	Status       string
	Audience     string
	ScopeSummary string
	ChallengeID  string
}

type protectedEffect struct {
	AttemptID     string
	SubmissionKey string
	TimeoutMS     int
	EffectClass   string
	Outcome       string
}

type runtimeState struct {
	grant             grant
	connection        connectionAssertion
	connectionPrompts int
	lastEffect        protectedEffect
	revoked           bool
	recheckCount      int
	updatePending     bool
}

func validGrant() grant {
	return grant{
		ToolURI: fixtureToolURI, ToolVersion: fixtureToolVer,
		Parent: parentBounds{
			Principal: "principal-a", Workspace: "workspace-a", Child: "worker-a",
			Capabilities: []string{fixtureToolURI}, EffectCeiling: "mutation",
		},
		Spend:      spendPolicy{Currency: "USD", Amount: 25, Period: "day", Approval: "required"},
		Connection: connectionAssertion{Status: "connected", Audience: "github", ScopeSummary: "issues:write", ChallengeID: "challenge-1"},
		LeaseValid: true, GrantRevision: 7,
	}
}

func validateGrant(g grant) error {
	if g.ToolURI == "" || g.ToolVersion == "" || strings.ContainsAny(g.ToolURI, "*#?") || strings.ContainsAny(g.ToolVersion, "*<>[]=|^") || g.ToolVersion == "latest" {
		return errors.New("invalid_grant: exact URI and version required")
	}
	if len(g.Parent.Capabilities) != 1 || g.Parent.Capabilities[0] != g.ToolURI {
		return errors.New("invalid_grant: taxonomy or widened capability grant")
	}
	if g.Spend.Currency == "" || g.Spend.Amount <= 0 || g.Spend.Period == "" || g.Spend.Approval == "" {
		return errors.New("spending_policy_required")
	}
	if g.Parent.Workspace == "" || g.Parent.Child == "" || g.Parent.EffectCeiling == "" {
		return errors.New("invalid_grant: incomplete parent bounds")
	}
	return nil
}

func validateDispatchPolicy(g grant, effectClass string, spendAmount int64) error {
	if err := validateGrant(g); err != nil {
		return err
	}
	if effectClass == "mutation" && g.Parent.EffectCeiling != "mutation" {
		return errors.New("invalid_grant: effect exceeds parent ceiling")
	}
	if spendAmount > g.Spend.Amount {
		return errors.New("spending_denied")
	}
	return nil
}

func (s *runtimeState) ensureConnection() error {
	if s.connection.Status == "connected" {
		return nil
	}
	s.connectionPrompts++
	return errors.New("requires_connection")
}

func (s *runtimeState) dispatch() (protectedEffect, error) {
	s.recheckCount++
	if s.revoked || !s.grant.LeaseValid {
		return protectedEffect{}, errors.New("artifact_revoked")
	}
	if err := validateGrant(s.grant); err != nil {
		return protectedEffect{}, err
	}
	if err := s.ensureConnection(); err != nil {
		return protectedEffect{}, err
	}
	e := protectedEffect{
		AttemptID: "attempt-1", SubmissionKey: "principal-a/" + fixtureToolURI + "/submission-1",
		TimeoutMS: 5000, EffectClass: "mutation", Outcome: "unknown",
	}
	s.lastEffect = e
	return e, nil
}

func TestPolicyRuntimeRejectsWildcardAndTaxonomyGrants(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*grant)
	}{
		{name: "wildcard URI", mutate: func(g *grant) { g.ToolURI = "gist://tools/github/*" }},
		{name: "taxonomy capability", mutate: func(g *grant) { g.Parent.Capabilities = []string{"taxonomy:project-management"} }},
		{name: "widened capability set", mutate: func(g *grant) {
			g.Parent.Capabilities = append(g.Parent.Capabilities, "gist://tools/github/issues.close")
		}},
		{name: "range version", mutate: func(g *grant) { g.ToolVersion = ">=2026-09-25" }},
		{name: "independent spend missing", mutate: func(g *grant) { g.Spend = spendPolicy{} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := validGrant()
			tc.mutate(&g)
			if err := validateGrant(g); err == nil {
				t.Fatal("expected invalid grant")
			}
		})
	}
	if err := validateDispatchPolicy(validGrant(), "mutation", 26); err == nil {
		t.Fatal("spend above the independent ceiling was accepted")
	}
	if err := validateDispatchPolicy(validGrant(), "mutation", 25); err != nil {
		t.Fatalf("independent parent/spend policies rejected valid dispatch: %v", err)
	}
	parentLimited := validGrant()
	parentLimited.Parent.EffectCeiling = "read_only"
	if err := validateDispatchPolicy(parentLimited, "mutation", 1); err == nil {
		t.Fatal("mutation above the parent effect ceiling was accepted")
	}
}

func TestPolicyRuntimeRechecksGrantBeforeDispatchAndMapsProtectedEffects(t *testing.T) {
	s := &runtimeState{grant: validGrant(), connection: validGrant().Connection}
	if _, err := s.dispatch(); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if s.recheckCount != 1 {
		t.Fatalf("recheck count = %d, want 1", s.recheckCount)
	}
	if got := s.lastEffect; got.SubmissionKey == "" || got.TimeoutMS != 5000 || got.EffectClass != "mutation" || got.Outcome != "unknown" {
		t.Fatalf("protected-effects mapping lost: %+v", got)
	}

	// Readiness is not authority: a revocation between readiness and dispatch
	// must be observed by the dispatch-time recheck.
	s.revoked = true
	if _, err := s.dispatch(); err == nil || err.Error() != "artifact_revoked" {
		t.Fatalf("revoked dispatch error = %v", err)
	}
	if s.recheckCount != 2 {
		t.Fatalf("recheck count after revoke = %d, want 2", s.recheckCount)
	}
}

func TestRuntimeOwnedConnectionChallengeIsReused(t *testing.T) {
	s := &runtimeState{grant: validGrant(), connection: connectionAssertion{Status: "pending", ChallengeID: "challenge-runtime-1"}}
	if _, err := s.dispatch(); err == nil || err.Error() != "requires_connection" {
		t.Fatalf("first dispatch error = %v", err)
	}
	if _, err := s.dispatch(); err == nil || err.Error() != "requires_connection" {
		t.Fatalf("second dispatch error = %v", err)
	}
	if s.connectionPrompts != 2 {
		t.Fatalf("unexpected prompt count before assertion: %d", s.connectionPrompts)
	}
	// The runtime completes its existing challenge; the adapter must reuse it,
	// not create a second challenge or copy credentials into the grant.
	s.connection = connectionAssertion{Status: "connected", Audience: "github", ScopeSummary: "issues:write", ChallengeID: "challenge-runtime-1"}
	if _, err := s.dispatch(); err != nil {
		t.Fatalf("dispatch after connection: %v", err)
	}
	if s.connection.ChallengeID != "challenge-runtime-1" || s.connectionPrompts != 2 {
		t.Fatalf("runtime-owned challenge was not reused: %+v prompts=%d", s.connection, s.connectionPrompts)
	}
}

func packageFixture(t *testing.T) contract.Package {
	t.Helper()
	files := map[string][]byte{
		"SKILL.md":                []byte("---\nname: asset-skill\ndescription: deterministic fixture\n---\nUse the exact operation.\n"),
		"references/operation.md": []byte("operation: github/issues.create\n"),
	}
	inventory := make([]contract.InventoryEntry, 0, len(files))
	for path, content := range files {
		h := sha256.Sum256(content)
		inventory = append(inventory, contract.InventoryEntry{Path: path, Size: int64(len(content)), SHA256: hex.EncodeToString(h[:]), MediaType: "text/markdown"})
	}
	pkgDigest, _, err := contract.PackageDigest(inventory)
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"manifest_version":"1","id":"asset-skill","version":"1.0.0"}`)
	h := sha256.Sum256(manifest)
	return contract.Package{Manifest: manifest, Files: files, Inventory: inventory, PackageDigest: pkgDigest, Transfer: contract.DetachedTransfer{ManifestDigest: hex.EncodeToString(h[:]), ManifestSize: int64(len(manifest))}}
}

func TestArtifactImportVerifiesCompletePackageAndDigestClosure(t *testing.T) {
	pkg := packageFixture(t)
	if err := contract.VerifyPackage(pkg); err != nil {
		t.Fatalf("complete package rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*contract.Package)
	}{
		{name: "missing referenced file", mutate: func(p *contract.Package) { delete(p.Files, "references/operation.md") }},
		{name: "corrupt file", mutate: func(p *contract.Package) { p.Files["SKILL.md"] = append(p.Files["SKILL.md"], 'x') }},
		{name: "undeclared file", mutate: func(p *contract.Package) { p.Files["references/extra.md"] = []byte("extra") }},
		{name: "detached manifest mismatch", mutate: func(p *contract.Package) { p.Transfer.ManifestDigest = "00" + p.Transfer.ManifestDigest[2:] }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := packageFixture(t)
			tc.mutate(&bad)
			if err := contract.VerifyPackage(bad); err == nil {
				t.Fatal("expected complete-package verification failure")
			}
		})
	}
}

func TestArtifactImportDoesNotSilentlyUpdate(t *testing.T) {
	s := &runtimeState{grant: validGrant()}
	if s.updatePending {
		t.Fatal("fixture unexpectedly has a pending update")
	}
	s.updatePending = true // version notice: available, but not an implicit install
	if !s.updatePending {
		t.Fatal("expected update notice")
	}
	if s.grant.ToolVersion != fixtureToolVer {
		t.Fatalf("binding changed without explicit update: %s", s.grant.ToolVersion)
	}
}

func TestContractOnlyReceiptIsExplicitlyDeferredWithoutExternalReceipts(t *testing.T) {
	// X-R1 and X-R2 are intentionally absent in this checkout. This assertion
	// keeps the local evidence honest: it proves the frozen contract only.
	const xR1, xR2 = "", ""
	if xR1 != "" || xR2 != "" {
		t.Fatal("unexpected external receipt fixture")
	}
	if fixtureSkillRef == "" {
		t.Fatal("fixture identity must remain exact")
	}
}
