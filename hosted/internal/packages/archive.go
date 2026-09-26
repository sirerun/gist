package packages

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

type Limits struct{ MaxPackageBytes, MaxFileBytes, MaxFiles int }

func DefaultLimits() Limits {
	return Limits{MaxPackageBytes: 10 << 20, MaxFileBytes: 2 << 20, MaxFiles: 256}
}

type Archive struct{ Files map[string][]byte }

func ReadArchive(data []byte, limits Limits) (Archive, error) {
	if limits.MaxPackageBytes <= 0 || limits.MaxFileBytes <= 0 || limits.MaxFiles <= 0 {
		return Archive{}, errors.New("packages: invalid archive limits")
	}
	if len(data) > limits.MaxPackageBytes {
		return Archive{}, errors.New("packages: archive exceeds package limit")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Archive{}, fmt.Errorf("open archive: %w", err)
	}
	if len(zr.File) > limits.MaxFiles {
		return Archive{}, errors.New("packages: archive has too many files")
	}
	files := make(map[string][]byte, len(zr.File))
	total := 0
	for _, file := range zr.File {
		name, err := safeArchivePath(file.Name)
		if err != nil {
			return Archive{}, err
		}
		if file.FileInfo().Mode()&0o170000 == 0o120000 {
			return Archive{}, fmt.Errorf("packages: symlink is not allowed: %q", name)
		}
		if file.FileInfo().IsDir() {
			return Archive{}, fmt.Errorf("packages: directory member is not allowed: %q", name)
		}
		if file.UncompressedSize64 > uint64(limits.MaxFileBytes) {
			return Archive{}, fmt.Errorf("packages: file exceeds limit: %q", name)
		}
		r, err := file.Open()
		if err != nil {
			return Archive{}, fmt.Errorf("open archive member %q: %w", name, err)
		}
		b, readErr := io.ReadAll(io.LimitReader(r, int64(limits.MaxFileBytes)+1))
		closeErr := r.Close()
		if readErr != nil {
			return Archive{}, fmt.Errorf("read archive member %q: %w", name, readErr)
		}
		if closeErr != nil {
			return Archive{}, fmt.Errorf("close archive member %q: %w", name, closeErr)
		}
		if len(b) > limits.MaxFileBytes {
			return Archive{}, fmt.Errorf("packages: file exceeds limit: %q", name)
		}
		total += len(b)
		if total > limits.MaxPackageBytes {
			return Archive{}, errors.New("packages: expanded archive exceeds package limit")
		}
		if _, exists := files[name]; exists {
			return Archive{}, fmt.Errorf("packages: duplicate member: %q", name)
		}
		files[name] = b
	}
	return Archive{Files: files}, nil
}

func safeArchivePath(name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("packages: unsafe archive path %q", name)
	}
	clean := path.Clean(name)
	if clean != name || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("packages: traversal archive path %q", name)
	}
	for _, segment := range strings.Split(clean, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("packages: unsafe archive path %q", name)
		}
	}
	return clean, nil
}
