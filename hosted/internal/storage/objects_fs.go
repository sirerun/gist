package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	path, err := f.objectPath(name, true)
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(existing, b) {
			return errors.New("objects: immutable key already contains different bytes")
		}
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
	if err = os.Link(tmpName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, readErr := os.ReadFile(path)
			if readErr == nil && bytes.Equal(existing, b) {
				return nil
			}
			if readErr != nil {
				return fmt.Errorf("verify existing object: %w", readErr)
			}
			return errors.New("objects: immutable key already contains different bytes")
		}
		return fmt.Errorf("commit object: %w", err)
	}
	return nil
}

func (f *fsBackend) get(_ context.Context, name string) ([]byte, error) {
	path, err := f.objectPath(name, false)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return b, nil
}

func (f *fsBackend) objectPath(name string, create bool) (string, error) {
	if !validLowerHex(name) && !validOwnedObjectKey(name) {
		return "", errors.New("objects: invalid filesystem key")
	}
	root, err := filepath.Abs(f.root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if resolvedRoot != root {
		return "", errors.New("objects: symlink in configured object root")
	}
	if err := rejectSymlink(root); err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	if !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return "", errors.New("objects: path escapes root")
	}
	parent := filepath.Dir(path)
	// Inspect the existing chain before making any directory. In particular,
	// never call MkdirAll through an attacker-controlled v2 parent symlink.
	var missing []string
	for dir := parent; dir != root; dir = filepath.Dir(dir) {
		if err := rejectSymlink(dir); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			missing = append(missing, dir)
		}
	}
	if create {
		for i := len(missing) - 1; i >= 0; i-- {
			if err := os.Mkdir(missing[i], 0o700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if err := rejectSymlink(missing[i]); err != nil {
				return "", err
			}
		}
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("objects: symlink object rejected")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("objects: unsafe filesystem path")
	}
	return nil
}

func (f *fsBackend) delete(_ context.Context, name string) error {
	path, err := f.objectPath(name, false)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	} else {
		return err
	}
}
