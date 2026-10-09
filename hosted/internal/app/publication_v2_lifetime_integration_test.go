//go:build integration

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestPublicationV2ShutdownClosesObjectStore(t *testing.T) {
	h := startMatrixHarness(t, nil)
	h.publish(t, "skill")
	a := h.artifacts["skill"].prepared
	var key string
	if err := h.fixture.adminPool.QueryRow(h.ctx, `SELECT object_key FROM catalog_versions WHERE workspace_id=$1 AND kind='skill' AND artifact_id=$2 AND version=$3`, compositionWorkspace, a.Ref.ID, a.Ref.Version).Scan(&key); err != nil {
		t.Fatal(err)
	}
	reader, err := h.instance.objects.OpenOwned(h.ctx, key, a.ArtifactDigest, int64(len(a.Artifact)))
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.instance.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	reader, err = h.instance.objects.OpenOwned(h.ctx, key, a.ArtifactDigest, int64(len(a.Artifact)))
	if err == nil {
		if closeErr := reader.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		t.Fatal("owned filesystem remained open after completed app shutdown")
	}
	// Repeated shutdown must return the retained completed cleanup result.
	if err := h.instance.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Test-only Linux descriptor observation qualifies startup resource cleanup
// without reaching into the storage adapter's private root representation.
func TestPublicationV2FailedConstructionClosesObjectRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("descriptor observation requires Linux")
	}
	h := startMatrixHarness(t, nil)
	count := func() int {
		t.Helper()
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range entries {
			target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
			if err == nil && target == h.instance.cfg.ObjectStoreRoot {
				n++
			}
		}
		return n
	}
	before := count()
	if before != 1 {
		t.Fatalf("fixture owns %d root handles, want1", before)
	}
	cfg := h.instance.cfg
	v2 := *cfg.PublicationV2
	v2.MaintenanceTargets = []MaintenanceTarget{{Subject: "missing-maintainer", WorkspaceID: compositionWorkspace}}
	cfg.PublicationV2 = &v2
	if instance, err := New(h.ctx, cfg); err == nil || instance != nil {
		if instance != nil {
			_ = instance.Shutdown(context.Background())
		}
		t.Fatalf("unqualified maintenance actor unexpectedly constructed app: %v", err)
	}
	if after := count(); after != before {
		t.Fatalf("failed construction retained object handles: before%d after%d", before, after)
	}
}

func TestPublicationV2ForcedShutdownWaitsForObjectReader(t *testing.T) {
	h := startMatrixHarness(t, nil)
	h.publish(t, "skill")
	a := h.artifacts["skill"].prepared
	var key string
	if err := h.fixture.adminPool.QueryRow(h.ctx, `SELECT object_key FROM catalog_versions WHERE workspace_id=$1 AND kind='skill' AND artifact_id=$2 AND version=$3`, compositionWorkspace, a.Ref.ID, a.Ref.Version).Scan(&key); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	readDone := make(chan error, 1)
	// Use the actual composed request boundary with a controlled slow reader.
	// Like a backend read already in progress, it completes independently of
	// connection cancellation and still needs the app-owned filesystem root.
	route := h.instance.Handler().(requestContext)
	route.rest = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		reader, err := h.instance.objects.OpenOwned(context.Background(), key, a.ArtifactDigest, int64(len(a.Artifact)))
		if err == nil {
			var got []byte
			got, err = io.ReadAll(reader)
			err = errors.Join(err, reader.Close())
			if err == nil && !bytes.Equal(got, a.Artifact) {
				err = errors.New("in-flight read changed bytes")
			}
		}
		readDone <- err
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewTLSServer(route)
	defer func() { unblock(); server.Close() }()
	// Shutdown the actual server serving this admitted slow handler.
	h.instance.server = server.Config
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := server.Client().Get(server.URL + "/v2/artifacts/skill")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("reader did not enter HTTP handler")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := h.instance.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown caller error=%v", err)
	}
	h.shutdownExpected = context.DeadlineExceeded
	select {
	case <-h.instance.shutdownDone:
		t.Fatal("cleanup completed while active object reader was still admitted")
	case <-time.After(40 * time.Millisecond):
	}
	// New requests are denied before dispatch rather than touching closed or
	// draining service state.
	denied := httptest.NewRecorder()
	h.instance.Handler().ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/oauth/jwks", nil))
	if denied.Code != http.StatusServiceUnavailable {
		t.Fatalf("late admission status=%d", denied.Code)
	}
	unblock()
	select {
	case err := <-readDone:
		if err != nil {
			t.Fatalf("in-flight owned read failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reader did not finish")
	}
	later, cancelLater := context.WithTimeout(context.Background(), time.Second)
	defer cancelLater()
	if err := h.instance.Shutdown(later); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("retained shutdown error=%v", err)
	}
	reader, err := h.instance.objects.OpenOwned(context.Background(), key, a.ArtifactDigest, int64(len(a.Artifact)))
	if err == nil {
		_ = reader.Close()
		t.Fatal("root stayed open after active handler completed")
	}
	select {
	case <-clientDone:
	case <-time.After(time.Second):
		t.Fatal("forced-close client did not finish")
	}
}
