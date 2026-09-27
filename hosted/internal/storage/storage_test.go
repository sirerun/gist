package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

type digestCatalog struct {
	records map[string]ports.CatalogRecord
}

func (c digestCatalog) Get(_ context.Context, ref ports.ArtifactRef) (ports.CatalogRecord, error) {
	r, ok := c.records[refKey(ref)]
	if !ok {
		return ports.CatalogRecord{}, ErrNotFound
	}
	return r, nil
}

func (digestCatalog) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

func TestObjectStoreResolvesUnboundRefsFromCatalog(t *testing.T) {
	root := t.TempDir()
	raw := []byte("durable artifact")
	sum := sha256.Sum256(raw)
	digest := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	ref := ports.ArtifactRef{WorkspaceID: "w", Kind: "skill", ID: "a", Version: "1.0.0"}

	first, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Put(context.Background(), digest, bytes.NewReader(raw), int64(len(raw))); err != nil {
		t.Fatal(err)
	}
	if err := first.Bind(ref, digest); err != nil {
		t.Fatal(err)
	}

	// A fresh store over the same root models a process restart: the
	// in-memory bindings are gone, so only the catalog can resolve the ref.
	restarted, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Open(context.Background(), ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("without a catalog an unbound ref must be not found, got %v", err)
	}
	restarted.UseCatalog(digestCatalog{records: map[string]ports.CatalogRecord{refKey(ref): {Digest: digest}}})
	r, err := restarted.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("content = %q", got)
	}
	other := ref
	other.Version = "2.0.0"
	if _, err := restarted.Open(context.Background(), other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown ref must surface catalog not found, got %v", err)
	}
}
