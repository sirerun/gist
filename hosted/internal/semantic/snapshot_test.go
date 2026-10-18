package semantic

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirerun/gist/catalog/directory"
)

func smallSnapshotFixture(t *testing.T) (*Service, *fakeEmbedder) {
	t.Helper()
	index, err := directory.DefaultPublic()
	if err != nil {
		t.Fatal(err)
	}
	raw := directory.DefaultSnapshot()
	var snap directory.Snapshot
	if err = json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	snap.Entries = snap.Entries[:2]
	index, err = directory.New(snap)
	if err != nil {
		t.Fatal(err)
	}
	// Empty admitted apps are valid; avoid serializing thousands of irrelevant
	// vectors when testing the artifact's trust boundary.
	apps, err := directory.ParseApps([]byte(`{"format":"gist.app-directory/v1","items":[]}`), index)
	if err != nil {
		t.Fatal(err)
	}
	embedder := &fakeEmbedder{}
	service := New(index, apps, embedder, func() bool { return true })
	if err = service.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service, embedder
}

func TestSnapshotRoundTripRequiresNoDocumentInference(t *testing.T) {
	source, f := smallSnapshotFixture(t)
	var body bytes.Buffer
	if err := source.WriteSnapshot(&body); err != nil {
		t.Fatal(err)
	}
	target := New(source.index, source.apps, f, func() bool { return true })
	calls := f.calls
	if err := target.LoadSnapshot(bytes.NewReader(body.Bytes())); err != nil {
		t.Fatal(err)
	}
	if f.calls != calls {
		t.Fatal("snapshot loading invoked inference")
	}
	if target.ready.Load().version != source.ready.Load().version {
		t.Fatal("generation changed")
	}
	original := source.ready.Load()
	if original.version != snapshotGeneration(f.ModelVersion(), source.index.Revision(), original.docs) {
		t.Fatal("generation algorithm changed")
	}
	for i, v := range target.ready.Load().vectors {
		if len(v) != 768 || v[0] != original.vectors[i][0] {
			t.Fatal("vector mismatch")
		}
	}
}

func TestSnapshotRejectsMutationWithoutPublishing(t *testing.T) {
	service, _ := smallSnapshotFixture(t)
	var body bytes.Buffer
	if err := service.WriteSnapshot(&body); err != nil {
		t.Fatal(err)
	}
	var artifact snapshotArtifact
	if err := json.Unmarshal(body.Bytes(), &artifact); err != nil {
		t.Fatal(err)
	}
	mutate := map[string]func(*snapshotArtifact){
		"format":     func(a *snapshotArtifact) { a.Format = "other" },
		"model":      func(a *snapshotArtifact) { a.ModelVersion = "other" },
		"corpus":     func(a *snapshotArtifact) { a.CorpusDigest = "other" },
		"generation": func(a *snapshotArtifact) { a.Generation = "other" },
		"count":      func(a *snapshotArtifact) { a.Rows = a.Rows[:1] },
		"order":      func(a *snapshotArtifact) { a.Rows[0], a.Rows[1] = a.Rows[1], a.Rows[0] },
		"text":       func(a *snapshotArtifact) { a.Rows[0].TextSHA256 = "other" },
		"dimension":  func(a *snapshotArtifact) { a.Rows[0].Vector = a.Rows[0].Vector[:767] },
		"norm":       func(a *snapshotArtifact) { a.Rows[0].Vector = make([]float64, 768) },
	}
	for name, change := range mutate {
		t.Run(name, func(t *testing.T) {
			var a snapshotArtifact
			if err := json.Unmarshal(body.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
			change(&a)
			encoded, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			old := service.ready.Load()
			if err = service.LoadSnapshot(bytes.NewReader(encoded)); err == nil {
				t.Fatal("mutated artifact accepted")
			}
			if service.ready.Load() != old {
				t.Fatal("failed load changed active generation")
			}
		})
	}
	for _, bad := range []string{
		strings.Replace(body.String(), `"format":`, `"format":"duplicate","format":`, 1),
		strings.Replace(body.String(), `"format":`, `"Format":`, 1),
		strings.Replace(body.String(), `"vector":[`, `"vector":[null,`, 1),
		body.String() + `{}`,
		`{"format":"unknown","unexpected":true}`,
	} {
		if err := service.LoadSnapshot(strings.NewReader(bad)); err == nil {
			t.Fatal("ambiguous schema accepted")
		}
	}
}

func TestCompressedSnapshotChecksTrailerBeforePublication(t *testing.T) {
	service, _ := smallSnapshotFixture(t)
	var raw, packed bytes.Buffer
	if err := service.WriteSnapshot(&raw); err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(&packed)
	if _, err := writer.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.json.gz")
	if err := os.WriteFile(path, packed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	service.ready.Store(nil)
	if err := service.LoadSnapshotFile(path); err != nil {
		t.Fatal(err)
	}
	previous := service.ready.Load()
	corrupt := append([]byte(nil), packed.Bytes()...)
	corrupt[len(corrupt)-8] ^= 1
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.LoadSnapshotFile(path); err == nil {
		t.Fatal("corrupt gzip checksum accepted")
	}
	if service.ready.Load() != previous {
		t.Fatal("corrupt gzip published snapshot")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Truncate((32 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if err = service.LoadSnapshotFile(path); err == nil {
		t.Fatal("oversized compressed artifact accepted")
	}
}
