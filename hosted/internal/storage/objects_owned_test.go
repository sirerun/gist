package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestOwnedObjectExactKeyLifecycle(t *testing.T) {
	store, err := NewObjectStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("opaque owned v2 bytes")
	sum := sha256.Sum256(body)
	digest := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	key := "v2/0123456789abcdef0123456789abcdef/" + digest.Value
	if err := store.StageOwned(context.Background(), key, body, digest); err != nil {
		t.Fatal(err)
	}
	r, err := store.OpenOwned(context.Background(), key, digest, int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	if string(got) != string(body) {
		t.Fatalf("owned read = %q", got)
	}
	if err := store.StageOwned(context.Background(), key, []byte("changed"), digest); err == nil {
		t.Fatal("expected digest mismatch")
	}
	if _, err := store.OpenOwned(context.Background(), strings.Replace(key, digest.Value, strings.Repeat("0", 64), 1), digest, int64(len(body))); err == nil {
		t.Fatal("expected owned key suffix mismatch")
	}
	if err := store.DeleteOwned(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenOwned(context.Background(), key, digest, int64(len(body))); !errors.Is(err, ErrNotFound) {
		t.Fatalf("open after delete = %v", err)
	}
	if err := store.DeleteOwned(context.Background(), "../../outside"); err == nil {
		t.Fatal("expected unsafe key rejection")
	}
}

func TestFSOwnedPutRejectsSymlinkParentBeforeCreatingChildren(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "v2")); err != nil {
		t.Fatal(err)
	}
	backend, err := newFSBackend(root)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("owned")
	sum := sha256.Sum256(body)
	digest := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	key := "v2/0123456789abcdef0123456789abcdef/" + digest.Value
	if err := (&ObjectStore{blobs: backend}).StageOwned(context.Background(), key, body, digest); err == nil {
		t.Fatal("expected symlink parent rejection")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink target was mutated: %v", entries)
	}
}
