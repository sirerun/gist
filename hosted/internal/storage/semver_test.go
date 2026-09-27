package storage

import (
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.10.0", "1.9.0", 1},
		{"1.9.0", "1.10.0", -1},
		{"2.0.0", "10.0.0", -1},
		{"1.0.0", "1.0.0", 0},
		{"1.0.0+build.1", "1.0.0+build.2", 0},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1},
		{"1.0.0-rc.1", "1.0.0-beta.11", 1},
		{"not-a-version", "1.0.0", 1},
	}
	for _, tc := range cases {
		if got := compareSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestVersionStoreListsInSemverOrder(t *testing.T) {
	s := NewVersionStore()
	for _, v := range []string{"1.10.0", "1.2.0", "1.9.0", "1.10.0-rc.1", "0.1.0"} {
		ref := ports.ArtifactRef{WorkspaceID: "w", Kind: "skill", ID: "a", Version: v}
		if err := s.Add(Version{Ref: ref}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, v := range s.List("w", "skill", "a") {
		got = append(got, v.Ref.Version)
	}
	want := []string{"0.1.0", "1.2.0", "1.9.0", "1.10.0-rc.1", "1.10.0"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
