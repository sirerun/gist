package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// ObjectStore keeps content-addressed blobs on disk. The ref->digest index is
// an in-memory cache filled by Bind at publish time; the durable record of that
// mapping is the catalog (CatalogRecord.Digest), which Open consults on a cache
// miss when UseCatalog has been called, so bindings survive a restart.
type ObjectStore struct {
	root    string
	mu      sync.RWMutex
	refs    map[string]ports.Digest
	catalog ports.CatalogStore
}

// UseCatalog makes the catalog the durable fallback for ref->digest lookups.
func (s *ObjectStore) UseCatalog(c ports.CatalogStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catalog = c
}

func NewObjectStore(root string) (*ObjectStore, error) {
	if root == "" {
		return nil, errors.New("objects: empty root")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create object root: %w", err)
	}
	return &ObjectStore{root: root, refs: make(map[string]ports.Digest)}, nil
}

func (s *ObjectStore) Put(ctx context.Context, digest ports.Digest, r io.Reader, size int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if digest.Algorithm != "sha256" || len(digest.Value) != 64 {
		return errors.New("objects: only sha256 digests are accepted")
	}
	if size < 0 {
		return errors.New("objects: negative size")
	}
	b, err := io.ReadAll(io.LimitReader(r, size+1))
	if err != nil {
		return fmt.Errorf("read object: %w", err)
	}
	if int64(len(b)) != size {
		return fmt.Errorf("object size mismatch: got %d want %d", len(b), size)
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != digest.Value {
		return errors.New("object digest mismatch")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, digest.Value)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat object: %w", err)
	}
	tmp, err := os.CreateTemp(s.root, ".upload-")
	if err != nil {
		return fmt.Errorf("create object temp: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write object: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close object: %w", err)
	}
	if err = os.Rename(name, path); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("commit object: %w", err)
	}
	return nil
}

func (s *ObjectStore) Bind(ref ports.ArtifactRef, digest ports.Digest) error {
	if ref.WorkspaceID == "" || ref.ID == "" || ref.Version == "" {
		return errors.New("objects: incomplete reference")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refs[refKey(ref)] = digest
	return nil
}
func (s *ObjectStore) Open(ctx context.Context, ref ports.ArtifactRef) (ports.ArtifactReader, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	digest, ok := s.refs[refKey(ref)]
	catalog := s.catalog
	s.mu.RUnlock()
	if !ok {
		if catalog == nil {
			return nil, ErrNotFound
		}
		record, err := catalog.Get(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("objects: resolve %s from catalog: %w", refKey(ref), err)
		}
		digest = record.Digest
	}
	if digest.Algorithm != "sha256" {
		return nil, errors.New("objects: unsupported digest")
	}
	// digest.Value may come from the catalog row, so it is checked before it
	// becomes a path component: only a sha256 hex name can address an object.
	if !validSHA256Hex(digest.Value) {
		return nil, errors.New("objects: invalid digest")
	}
	b, err := os.ReadFile(filepath.Join(s.root, digest.Value))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return &readSeekCloser{Reader: bytes.NewReader(b)}, nil
}
func refKey(ref ports.ArtifactRef) string {
	return ref.WorkspaceID + "/" + string(ref.Kind) + "/" + ref.ID + "/" + ref.Version
}

type readSeekCloser struct{ *bytes.Reader }

func (r *readSeekCloser) Close() error { return nil }

var _ ports.ArtifactStore = (*ObjectStore)(nil)

// validSHA256Hex reports whether v is exactly 64 lowercase hex characters.
func validSHA256Hex(v string) bool {
	if len(v) != 64 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
