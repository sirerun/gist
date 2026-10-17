package migrate

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestAssetsAreContiguousImmutableAndDeterministic(t *testing.T) {
	files := fstest.MapFS{"002_second.sql": {Data: []byte("SELECT 2;")}, "001_first.sql": {Data: []byte("SELECT 1;")}}
	r, err := Load(files)
	if err != nil {
		t.Fatal(err)
	}
	if r.Migrations[0].Name != "001_first.sql" || len(r.ManifestSHA256()) != 64 {
		t.Fatal("unordered or unhashed assets")
	}
	old := r.ManifestSHA256()
	files["001_first.sql"] = &fstest.MapFile{Data: []byte("SELECT 3;")}
	changed, err := Load(files)
	if err != nil {
		t.Fatal(err)
	}
	if old == changed.ManifestSHA256() {
		t.Fatal("changed SQL did not change manifest")
	}
	for name, invalid := range map[string]fstest.MapFS{
		"gap":         {"002_second.sql": {Data: []byte("SELECT 2;")}},
		"duplicate":   {"001_a.sql": {Data: []byte("a")}, "001_b.sql": {Data: []byte("b")}},
		"unsafe name": {"../001_x.sql": {Data: []byte("x")}},
		"empty":       {"001_a.sql": {Data: []byte{}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(invalid); err == nil {
				t.Fatal("accepted invalid assets")
			}
		})
	}
}
func TestLedgerRefusesMissingUnknownAndDriftedAppliedVersions(t *testing.T) {
	r, err := Load(fstest.MapFS{"001_a.sql": {Data: []byte("SELECT 1")}, "002_b.sql": {Data: []byte("SELECT 2")}})
	if err != nil {
		t.Fatal(err)
	}
	first := Applied{Version: 1, Name: r.Migrations[0].Name, SHA256: r.Migrations[0].SHA256}
	if err := r.validate([]Applied{first}); err != nil {
		t.Fatal(err)
	}
	for _, entries := range [][]Applied{{{Version: 2}}, {{Version: 1, Name: first.Name, SHA256: strings.Repeat("0", 64)}}, {first, {Version: 3}}, {first, first, first}} {
		if r.validate(entries) == nil {
			t.Fatal("accepted invalid ledger")
		}
	}
}
