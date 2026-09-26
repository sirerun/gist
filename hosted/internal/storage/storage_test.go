package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestObjectStoreVerifiesImmutableContent(t *testing.T) {
	root := t.TempDir()
	store, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	b := []byte("immutable package")
	sum := sha256.Sum256(b)
	digest := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	ref := ports.ArtifactRef{WorkspaceID: "ws-a", Kind: ports.KindSkill, ID: "skill/a", Version: "1.0.0"}
	if err := store.Put(context.Background(), digest, bytesReader(b), int64(len(b))); err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(ref, digest); err != nil {
		t.Fatal(err)
	}
	r, err := store.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(b) {
		t.Fatalf("got %q", got)
	}
	if err := store.Put(context.Background(), digest, bytesReader([]byte("changed")), 7); err == nil {
		t.Fatal("expected digest mismatch")
	}
}
func bytesReader(b []byte) io.Reader { return &byteReader{b: b} }

type byteReader struct{ b []byte }

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}
