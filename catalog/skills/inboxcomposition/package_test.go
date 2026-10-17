package inboxcomposition_test

import (
	"bytes"
	"testing"

	"github.com/sirerun/gist/catalog/composition"
	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/skills/inboxcomposition"
)

func TestImmutablePackageAndContracts(t *testing.T) {
	a, e := inboxcomposition.Archive()
	if e != nil {
		t.Fatal(e)
	}
	b, e := inboxcomposition.Archive()
	if e != nil || !bytes.Equal(a, b) {
		t.Fatal("archive not deterministic", e)
	}
	archive, e := packages.ReadArchive(a, packages.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	pkg, e := packages.Validate(archive, packages.DefaultLimits())
	if e != nil {
		t.Fatal(e)
	}
	if pkg.Manifest.Version != "2.0.0" {
		t.Fatal("new immutable version required")
	}
	d, e := composition.Parse(pkg.Files[composition.AssetPath])
	if e != nil {
		t.Fatal(e)
	}
	contracts := map[string]composition.Contract{}
	for _, r := range d.Requirements {
		contract, e := composition.ParseContract(pkg.Files["assets/"+r.Key+"-contract.json"])
		if e != nil {
			t.Fatal(e)
		}
		contracts[r.Key] = contract
	}
	if e = composition.Validate(d, contracts); e != nil {
		t.Fatal(e)
	}
	pkg.Files[composition.AssetPath][0] = 'x'
	if _, e = packages.Validate(archive, packages.DefaultLimits()); e == nil {
		t.Fatal("mutation escaped inventory digest")
	}
}
