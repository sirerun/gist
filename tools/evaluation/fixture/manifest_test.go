package fixture_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirerun/gist/tools/evaluation/fixture"
)

func frozen(t *testing.T) ([]byte, fixture.Manifest) {
	t.Helper()
	c, e := fixture.New("ready")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := fixture.Export(c)
	if e != nil {
		t.Fatal(e)
	}
	var m fixture.Manifest
	if e = json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	return raw, m
}
func TestFrozenManifestRoundtripUsesRealPublicSDK(t *testing.T) {
	raw, _ := frozen(t)
	path := filepath.Join(t.TempDir(), "catalog.json")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	c, e := fixture.LoadManifest(path, fixture.Hash(raw).Value)
	if e != nil {
		t.Fatal(e)
	}
	s, e := c.Service()
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.Prepare(context.Background(), fixture.Request("Summarize unread inbox email"))
	if e != nil || out.Status != "ready" || out.Selected == nil {
		t.Fatalf("real SDK roundtrip failed: %+v %v", out, e)
	}
}
func TestManifestRejectsMalformedTamperWorkspaceAndBudgets(t *testing.T) {
	for _, which := range []string{"malformed", "wrong workspace", "tamper", "duplicate", "too many records", "oversize", "wrong digest"} {
		t.Run(which, func(t *testing.T) {
			raw, m := frozen(t)
			switch which {
			case "malformed":
				raw = []byte("not JSON")
			case "wrong workspace":
				m.WorkspaceID = "foreign"
			case "tamper":
				m.Records[0].Artifact = []byte("altered bytes")
			case "duplicate":
				m.Records = append(m.Records, m.Records[0])
			case "too many records":
				m.Records = make([]fixture.ManifestRecord, fixture.MaxManifestRecords+1)
			case "oversize":
				raw = []byte(strings.Repeat("x", fixture.MaxManifestBytes+1))
			case "wrong digest":
				m.Records[0].Record.Digest.Value = strings.Repeat("b", 64)
			}
			if which != "malformed" && which != "oversize" {
				var e error
				raw, e = json.Marshal(m)
				if e != nil {
					t.Fatal(e)
				}
			}
			if _, e := fixture.DecodeManifest(raw); e == nil {
				t.Fatal("invalid frozen catalog accepted")
			}
		})
	}
}
func TestManifestPathAndContentPinAreExplicit(t *testing.T) {
	raw, _ := frozen(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := fixture.LoadManifest(path, strings.Repeat("b", 64)); e == nil {
		t.Fatal("content hash not enforced")
	}
	link := filepath.Join(dir, "symlink.json")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, e := fixture.LoadManifest(link, fixture.Hash(raw).Value); e == nil {
		t.Fatal("symlink manifest accepted")
	}
}
