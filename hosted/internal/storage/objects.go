package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// ObjectStore keeps content-addressed blobs in a blobBackend: a directory on
// disk (NewObjectStore) or an S3 bucket (OpenObjectStore with an s3:// root).
// The ref->digest index is an in-memory cache filled by Bind at publish time;
// the durable record of that mapping is the catalog (CatalogRecord.Digest),
// which Open consults on a cache miss when UseCatalog has been called, so
// bindings survive a restart.
type ObjectStore struct {
	blobs   blobBackend
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

// blobBackend stores immutable blobs under their sha256 hex name. Callers
// validate the name and the content before either method is reached.
type blobBackend interface {
	// put stores b under name. A name that is already present is left as it
	// is and reported as success, and a failed put leaves nothing visible.
	put(ctx context.Context, name string, b []byte) error
	// get returns the blob, or ErrNotFound when name is absent.
	get(ctx context.Context, name string) ([]byte, error)
}

// NewObjectStore opens a filesystem object store rooted at root.
func NewObjectStore(root string) (*ObjectStore, error) {
	fs, err := newFSBackend(root)
	if err != nil {
		return nil, err
	}
	return newObjectStore(fs), nil
}

// OpenObjectStore selects the backend from root: an s3://bucket/prefix value
// opens an S3 store with the default AWS credential chain, and anything else
// is a filesystem directory.
func OpenObjectStore(ctx context.Context, root string) (*ObjectStore, error) {
	if !isS3Root(root) {
		return NewObjectStore(root)
	}
	s3b, err := newS3BackendFromEnv(ctx, root)
	if err != nil {
		return nil, err
	}
	return newObjectStore(s3b), nil
}

func newObjectStore(b blobBackend) *ObjectStore {
	return &ObjectStore{blobs: b, refs: make(map[string]ports.Digest)}
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
	return s.blobs.put(ctx, digest.Value, b)
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
	b, err := s.blobs.get(ctx, digest.Value)
	if err != nil {
		return nil, err
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
