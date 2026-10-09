//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
	v2.MaintenanceTargets = []MaintenanceTarget{{Issuer: compositionIssuer, Subject: "missing-maintainer", WorkspaceID: compositionWorkspace}}
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
