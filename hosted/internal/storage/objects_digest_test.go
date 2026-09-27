package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// A digest value reaches Open from the catalog row, so anything that is not a
// 64-character lowercase sha256 hex name must be refused before it becomes a
// path component.
func TestOpenRejectsMalformedDigestValues(t *testing.T) {
	root := t.TempDir()
	s, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	// A readable file outside the naming scheme, so a traversal would succeed
	// if the value were joined unchecked.
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := []string{
		"",
		"secret",
		"../secret",
		"../../etc/passwd",
		strings.Repeat("a", 63),
		strings.Repeat("a", 65),
		strings.Repeat("A", 64),
		strings.Repeat("a", 62) + "/.",
		strings.Repeat("g", 64),
	}
	for i, v := range bad {
		ref := ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "a", Version: string(rune('a' + i))}
		if err := s.Bind(ref, ports.Digest{Algorithm: "sha256", Value: v}); err != nil {
			t.Fatal(err)
		}
		rc, err := s.Open(context.Background(), ref)
		if err == nil {
			_ = rc.Close()
			t.Fatalf("Open accepted digest %q", v)
		}
		if errors.Is(err, ErrNotFound) {
			t.Fatalf("digest %q reached the filesystem (got ErrNotFound)", v)
		}
	}
}

func TestOpenAcceptsWellFormedDigest(t *testing.T) {
	s, err := NewObjectStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ref := ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "a", Version: "1"}
	if err := s.Bind(ref, ports.Digest{Algorithm: "sha256", Value: strings.Repeat("0123456789abcdef", 4)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(context.Background(), ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("well-formed but absent digest: err = %v, want ErrNotFound", err)
	}
}
