package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/sirerun/gist/hosted/internal/ports"
)

func ownedFixture(body []byte) (string, ports.Digest) {
	sum := sha256.Sum256(body)
	d := ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	return "v2/0123456789abcdef0123456789abcdef/" + d.Value, d
}

func TestOwnedV2CreateCollisionAcrossFSInstancesAndExactDelete(t *testing.T) {
	root := t.TempDir()
	a, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("atomic immutable payload")
	key, d := ownedFixture(body)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, s := range []*ObjectStore{a, b} {
		wg.Add(1)
		go func(s *ObjectStore) { defer wg.Done(); errs <- s.StageOwned(context.Background(), key, body, d) }(s)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if err := b.StageOwned(context.Background(), key, []byte("different"), sha256Digest([]byte("different"))); err == nil {
		t.Fatal("conflicting content accepted")
	}
	if err := a.DeleteOwned(context.Background(), "v2/ffffffffffffffffffffffffffffffff/"+strings.Repeat("0", 64)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing exact owned delete = %v", err)
	}
	if err := a.DeleteOwned(context.Background(), "../../global"); err == nil {
		t.Fatal("traversal delete accepted")
	}
	if _, err := b.OpenOwned(context.Background(), key, d, int64(len(body))); err != nil {
		t.Fatalf("restart read: %v", err)
	}
}

func TestOwnedV2RejectsParentAndLeafSymlinksWithoutOutsideAccess(t *testing.T) {
	body := []byte("target")
	key, d := ownedFixture(body)
	t.Run("parent", func(t *testing.T) {
		root, out := t.TempDir(), t.TempDir()
		if err := os.Symlink(out, filepath.Join(root, "v2")); err != nil {
			t.Fatal(err)
		}
		s, err := NewObjectStore(root)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.StageOwned(context.Background(), key, body, d); err == nil {
			t.Fatal("parent symlink accepted")
		}
		if entries, _ := os.ReadDir(out); len(entries) != 0 {
			t.Fatalf("outside parent mutated: %v", entries)
		}
	})
	t.Run("leaf", func(t *testing.T) {
		root, out := t.TempDir(), t.TempDir()
		s, err := NewObjectStore(root)
		if err != nil {
			t.Fatal(err)
		}
		leaf := filepath.Join(root, filepath.FromSlash(key))
		if err = os.MkdirAll(filepath.Dir(leaf), 0700); err != nil {
			t.Fatal(err)
		}
		canary := filepath.Join(out, "canary")
		if err = os.WriteFile(canary, []byte("safe"), 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.Symlink(canary, leaf); err != nil {
			t.Fatal(err)
		}
		if err = s.StageOwned(context.Background(), key, body, d); err == nil {
			t.Fatal("leaf symlink accepted")
		}
		if err = s.DeleteOwned(context.Background(), key); err == nil {
			t.Fatal("leaf symlink deleted")
		}
		got, err := os.ReadFile(canary)
		if err != nil || string(got) != "safe" {
			t.Fatalf("outside canary changed: %q %v", got, err)
		}
	})
}

func TestObjectStoreRejectsConfiguredRootSymlink(t *testing.T) {
	base, outside := t.TempDir(), t.TempDir()
	link := filepath.Join(base, "root-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := NewObjectStore(link); err == nil {
		t.Fatal("configured root symlink accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside root changed: %v %v", entries, err)
	}
}

func TestOwnedV2CatalogReadsRecheckCurrentRecordAndExactSize(t *testing.T) {
	ctx := context.Background()
	body := []byte("v2 stored bytes")
	key, d := ownedFixture(body)
	s, err := NewObjectStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.StageOwned(ctx, key, body, d); err != nil {
		t.Fatal(err)
	}
	ref := ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "a", Version: "1.0.0"}
	record := ports.CatalogRecord{Ref: ref, State: "published", Digest: d, ObjectKey: key, ArtifactSize: int64(len(body))}
	catalog := digestCatalog{records: map[string]ports.CatalogRecord{refKey(ref): record}}
	s.UseCatalog(catalog)
	if err = s.Bind(ref, d); err != nil {
		t.Fatal(err)
	}
	r, err := s.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	_ = r.Close()
	if !bytes.Equal(got, body) {
		t.Fatalf("read %q", got)
	}
	revoked := record
	revoked.State = "revoked"
	s.UseCatalog(digestCatalog{records: map[string]ports.CatalogRecord{refKey(ref): revoked}})
	if _, err = s.Open(ctx, ref); !errors.Is(err, ErrRevoked) {
		t.Fatalf("cached revoked durable row = %v", err)
	}
	// Same digest with a wrong durable length must fail closed.
	catalog.records[refKey(ref)] = ports.CatalogRecord{Ref: ref, State: "published", Digest: d, ObjectKey: key, ArtifactSize: int64(len(body) + 1)}
	s.UseCatalog(catalog)
	if _, err = s.Open(ctx, ref); err == nil {
		t.Fatal("wrong catalog length accepted")
	}
	// A corrupt object with the expected length also fails the digest check.
	fs := s.blobs.(*fsBackend)
	if err = fs.rootFD.WriteFile(key, []byte(strings.Repeat("x", len(body))), 0600); err != nil {
		t.Fatal(err)
	}
	catalog.records[refKey(ref)] = record
	if _, err = s.Open(ctx, ref); err == nil {
		t.Fatal("corrupt owned object accepted")
	}
}

type ownedS3Fake struct {
	putErr    error
	get       []byte
	gotKey    string
	deleted   []string
	putStatus int
	gets      int
}

func (f *ownedS3Fake) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.gotKey = *in.Key
	return nil, f.putErr
}
func (f *ownedS3Fake) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.gotKey = *in.Key
	f.gets++
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f.get))}, nil
}
func (f *ownedS3Fake) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.deleted = append(f.deleted, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func TestOwnedS3ConditionalCollisionAndExactConfiguredPrefix(t *testing.T) {
	ctx := context.Background()
	data := []byte("v2")
	key, d := ownedFixture(data)
	_ = d
	for _, tc := range []struct {
		name     string
		existing []byte
		wantErr  bool
	}{{"same", data, false}, {"different", []byte("other"), true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &ownedS3Fake{putErr: &smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusPreconditionFailed}}, Err: errors.New("precondition failed")}, get: tc.existing}
			b := newS3Backend(f, "bucket", "root/prefix")
			err := b.put(ctx, key, data)
			if (err != nil) != tc.wantErr {
				t.Fatalf("put err %v", err)
			}
			want := "root/prefix/" + key
			if f.gotKey != want {
				t.Fatalf("key %q want %q", f.gotKey, want)
			}
			if f.gets != 1 {
				t.Fatalf("conditional verify GET count %d", f.gets)
			}
			if err = b.delete(ctx, key); err != nil {
				t.Fatal(err)
			}
			if len(f.deleted) != 1 || f.deleted[0] != want {
				t.Fatalf("delete keys %v", f.deleted)
			}
		})
	}
	if len((&ownedS3Fake{}).deleted) != 0 {
		t.Fatal("unexpected global listing or cleanup")
	}
}
