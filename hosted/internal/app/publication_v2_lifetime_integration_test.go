//go:build integration

package app

import (
	"context"
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
