package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type fsBackend struct {
	root       string
	rootFD     *os.Root
	mu         sync.Mutex
	removeTemp func(string) error
}

func newFSBackend(root string) (*fsBackend, error) {
	if root == "" {
		return nil, errors.New("objects: empty root")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := mkdirRootNoSymlink(abs); err != nil {
		return nil, fmt.Errorf("create object root: %w", err)
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	return &fsBackend{root: abs, rootFD: r}, nil
}

// mkdirRootNoSymlink walks from the filesystem root, rejecting every existing
// link or non-directory before creating the next component. This prevents
// MkdirAll from following a configured-root symlink before validation.
func mkdirRootNoSymlink(path string) error {
	vol := filepath.VolumeName(path)
	rest := strings.TrimPrefix(path, vol)
	cur := vol + string(os.PathSeparator)
	for _, part := range strings.Split(strings.Trim(rest, string(os.PathSeparator)), string(os.PathSeparator)) {
		if part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(cur, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
			info, err = os.Lstat(cur)
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("objects: symlink or non-directory in configured object root")
		}
	}
	return nil
}

func (f *fsBackend) put(_ context.Context, name string, b []byte) (retErr error) {
	if validOwnedObjectKey(name) {
		return f.putOwned(name, b)
	}
	if strings.HasPrefix(name, "v2/") {
		return errors.New("objects: invalid owned filesystem key")
	}
	// Legacy backend behavior remains digest-path based. Public ObjectStore.Put
	// validates the digest; direct arbitrary names are retained as fault seams.
	f.mu.Lock()
	defer f.mu.Unlock()
	path := filepath.Join(f.root, name)
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
		if e := removeTemp(tmpName); e != nil && !errors.Is(e, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove temporary object: %w", e))
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
			existing, e := os.ReadFile(path)
			if e == nil && bytes.Equal(existing, b) {
				return nil
			}
			if e != nil {
				return fmt.Errorf("verify existing object: %w", e)
			}
			return errors.New("objects: immutable key already contains different bytes")
		}
		return fmt.Errorf("commit object: %w", err)
	}
	return nil
}

func (f *fsBackend) get(_ context.Context, name string) ([]byte, error) {
	if validOwnedObjectKey(name) {
		return f.getOwned(name)
	}
	if strings.HasPrefix(name, "v2/") {
		return nil, errors.New("objects: invalid owned filesystem key")
	}
	b, err := os.ReadFile(filepath.Join(f.root, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return b, nil
}

// ownedPathCheck rejects symlinks and special files before performing any
// v2 mutation. All subsequent accesses use the pinned os.Root descriptor.
func (f *fsBackend) ownedPathCheck(name string) error {
	if !validOwnedObjectKey(name) {
		return errors.New("objects: invalid filesystem key")
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		info, err := f.rootFD.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("objects: symlink in owned object path")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return errors.New("objects: non-directory in owned object path")
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return errors.New("objects: non-regular owned object")
		}
	}
	return nil
}

func (f *fsBackend) putOwned(name string, b []byte) (retErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rootFD == nil {
		return errors.New("objects: filesystem root is not pinned")
	}
	if err := f.ownedPathCheck(name); err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	parent := strings.Join(parts[:2], "/")
	// Make each directory only after the full existing chain was checked.
	if _, err := f.rootFD.Lstat("v2"); errors.Is(err, os.ErrNotExist) {
		if err = f.rootFD.Mkdir("v2", 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	if _, err := f.rootFD.Lstat(parent); errors.Is(err, os.ErrNotExist) {
		if err = f.rootFD.Mkdir(parent, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	if err := f.ownedPathCheck(name); err != nil {
		return err
	}
	if existing, err := f.rootFD.ReadFile(name); err == nil {
		if bytes.Equal(existing, b) {
			return nil
		}
		return errors.New("objects: immutable key already contains different bytes")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var tmpName string
	var tmp *os.File
	for range 10 {
		var n [12]byte
		if _, err := rand.Read(n[:]); err != nil {
			return err
		}
		tmpName = parent + "/.upload-" + hex.EncodeToString(n[:])
		var err error
		tmp, err = f.rootFD.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create object temp: %w", err)
		}
	}
	if tmp == nil {
		return errors.New("objects: cannot allocate temporary object")
	}
	defer func() {
		if err := f.rootFD.Remove(tmpName); err != nil && !errors.Is(err, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove temporary object: %w", err))
		}
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write object: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close object: %w", err)
	}
	if err := f.rootFD.Link(tmpName, name); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, e := f.rootFD.ReadFile(name)
			if e == nil && bytes.Equal(existing, b) {
				return nil
			}
			if e != nil {
				return e
			}
			return errors.New("objects: immutable key already contains different bytes")
		}
		return fmt.Errorf("commit object: %w", err)
	}
	return nil
}

func (f *fsBackend) getOwned(name string) ([]byte, error) {
	if f.rootFD == nil {
		return nil, errors.New("objects: filesystem root is not pinned")
	}
	if err := f.ownedPathCheck(name); err != nil {
		return nil, err
	}
	b, err := f.rootFD.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return b, err
}

func (f *fsBackend) getLimit(_ context.Context, name string, limit int64) ([]byte, error) {
	if f.rootFD == nil {
		return nil, errors.New("objects: filesystem root is not pinned")
	}
	if err := f.ownedPathCheck(name); err != nil {
		return nil, err
	}
	file, err := f.rootFD.Open(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, limit))
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	return data, err
}

func (f *fsBackend) delete(_ context.Context, name string) error {
	if !validOwnedObjectKey(name) {
		return errors.New("objects: invalid owned filesystem key")
	}
	if f.rootFD == nil {
		return errors.New("objects: filesystem root is not pinned")
	}
	if err := f.ownedPathCheck(name); err != nil {
		return err
	}
	err := f.rootFD.Remove(name)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}
