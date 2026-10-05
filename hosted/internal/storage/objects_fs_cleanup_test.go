package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFSBackendReturnsTemporaryCleanupFailure(t *testing.T) {
	for _, failure := range []bool{false, true} {
		name := "committed"
		if failure {
			name = "commit_failure"
		}
		t.Run(name, func(t *testing.T) {
			cleanupFailure := errors.New("injected cleanup failure")
			root := t.TempDir()
			var removed string
			backend := &fsBackend{root: root, removeTemp: func(path string) error { removed = path; return cleanupFailure }}
			target := "object"
			if failure {
				target = "missing/object"
			}
			err := backend.put(context.Background(), target, []byte("complete object"))
			if !errors.Is(err, cleanupFailure) {
				t.Fatalf("cleanup failure discarded: %v", err)
			}
			if removed == "" || filepath.Dir(removed) != root {
				t.Fatalf("cleanup target was not owned temporary file: %q", removed)
			}
			if failure && !strings.Contains(err.Error(), "commit object") {
				t.Fatalf("primary failure discarded: %v", err)
			}
			if !failure {
				raw, readErr := os.ReadFile(filepath.Join(root, target))
				if readErr != nil || string(raw) != "complete object" {
					t.Fatalf("committed file: %q %v", raw, readErr)
				}
			}
		})
	}
}
