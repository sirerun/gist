package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// fsBackend keeps blobs as files named by digest under root. A put writes a
// temp file and renames it into place, so a reader never sees a partial blob.
type fsBackend struct {
	root string
	mu   sync.Mutex
	// Optional per-instance fault seam; production uses os.Remove.
	removeTemp func(string) error
}

func newFSBackend(root string) (*fsBackend, error) {
	if root == "" {
		return nil, errors.New("objects: empty root")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create object root: %w", err)
	}
	return &fsBackend{root: root}, nil
}

func (f *fsBackend) put(_ context.Context, name string, b []byte) (returnErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := filepath.Join(f.root, name)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat object: %w", err)
	}
	tmp, err := os.CreateTemp(f.root, ".upload-")
	if err != nil {
		return fmt.Errorf("create object temp: %w", err)
	}
	tmpName := tmp.Name()
	removeTemp := f.removeTemp
	if removeTemp == nil {
		removeTemp = os.Remove
	}
	defer func() {
		if cleanupErr := removeTemp(tmpName); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary object: %w", cleanupErr))
		}
	}()
	if _, err = tmp.Write(b); err != nil {
		closeErr := tmp.Close()
		return fmt.Errorf("write object: %w", errors.Join(err, closeErr))
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close object: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("commit object: %w", err)
	}
	return nil
}

func (f *fsBackend) get(_ context.Context, name string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(f.root, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return b, nil
}
