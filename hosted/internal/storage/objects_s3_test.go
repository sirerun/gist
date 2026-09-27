package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/sirerun/gist/hosted/internal/ports"
)

// fakeS3 is an in-process S3 that implements path-style PUT and GET with the
// If-None-Match: * conditional put and x-amz-checksum-sha256 verification.
type fakeS3 struct {
	mu        sync.Mutex
	objects   map[string][]byte // "bucket/key" -> body
	puts      int
	conflicts int // answer this many next PUTs with 409
	getStatus int // when set, every GET fails with this status
}

func newFakeS3(t *testing.T) (*fakeS3, *s3.Client) {
	t.Helper()
	f := &fakeS3{objects: map[string][]byte{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(srv.URL),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("AKIDTEST", "secret", ""),
		HTTPClient:   srv.Client(),
	})
	return f, client
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.TrimPrefix(r.URL.Path, "/")
	switch r.Method {
	case http.MethodPut:
		f.puts++
		if f.conflicts > 0 {
			f.conflicts--
			s3Error(w, http.StatusConflict, "ConditionalRequestConflict")
			return
		}
		if r.Header.Get("If-None-Match") != "*" {
			s3Error(w, http.StatusBadRequest, "MissingConditionalPut")
			return
		}
		if r.Header.Get("X-Amz-Acl") != "" {
			s3Error(w, http.StatusBadRequest, "UnexpectedACL")
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
			s3Error(w, http.StatusBadRequest, "UnsupportedEncoding")
			return
		}
		sum := sha256.Sum256(body)
		if r.Header.Get("X-Amz-Checksum-Sha256") != base64.StdEncoding.EncodeToString(sum[:]) {
			s3Error(w, http.StatusBadRequest, "BadDigest")
			return
		}
		if _, ok := f.objects[key]; ok {
			s3Error(w, http.StatusPreconditionFailed, "PreconditionFailed")
			return
		}
		f.objects[key] = body
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		if f.getStatus != 0 {
			s3Error(w, f.getStatus, "InternalError")
			return
		}
		body, ok := f.objects[key]
		if !ok {
			s3Error(w, http.StatusNotFound, "NoSuchKey")
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	default:
		s3Error(w, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

func s3Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>`+code+`</Code><Message>fake</Message></Error>`)
}

func sha256Digest(b []byte) ports.Digest {
	sum := sha256.Sum256(b)
	return ports.Digest{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
}

// TestObjectStoreBackendsShareSemantics runs one contract against both the
// filesystem and the S3 backend.
func TestObjectStoreBackendsShareSemantics(t *testing.T) {
	backends := map[string]func(t *testing.T) *ObjectStore{
		"filesystem": func(t *testing.T) *ObjectStore {
			s, err := NewObjectStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			return s
		},
		"s3": func(t *testing.T) *ObjectStore {
			_, client := newFakeS3(t)
			return newObjectStore(newS3Backend(client, "bucket", "registry/objects"))
		},
	}
	for name, open := range backends {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := open(t)
			raw := []byte("immutable package")
			d := sha256Digest(raw)
			ref := ports.ArtifactRef{WorkspaceID: "w", Kind: ports.KindSkill, ID: "a", Version: "1.0.0"}

			if err := s.Bind(ref, d); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Open(ctx, ref); !errors.Is(err, ErrNotFound) {
				t.Fatalf("absent blob: err = %v, want ErrNotFound", err)
			}
			if err := s.Put(ctx, d, bytes.NewReader(raw), int64(len(raw))); err != nil {
				t.Fatal(err)
			}
			// A second put of the same content is a no-op success.
			if err := s.Put(ctx, d, bytes.NewReader(raw), int64(len(raw))); err != nil {
				t.Fatalf("idempotent put: %v", err)
			}
			r, err := s.Open(ctx, ref)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(r)
			_ = r.Close()
			if !bytes.Equal(got, raw) {
				t.Fatalf("content = %q", got)
			}
			if err := s.Put(ctx, d, bytes.NewReader([]byte("changed!!")), 9); err == nil {
				t.Fatal("expected digest mismatch")
			}
			if err := s.Put(ctx, d, bytes.NewReader(raw), int64(len(raw))-1); err == nil {
				t.Fatal("expected size mismatch")
			}
			if err := s.Put(ctx, ports.Digest{Algorithm: "md5", Value: d.Value}, bytes.NewReader(raw), int64(len(raw))); err == nil {
				t.Fatal("expected non-sha256 rejection")
			}
		})
	}
}

func TestS3BackendKeysAndConditionalPut(t *testing.T) {
	ctx := context.Background()
	raw := []byte("blob")
	d := sha256Digest(raw)
	for _, tc := range []struct{ prefix, want string }{
		{"", "bucket/" + d.Value},
		{"registry/objects", "bucket/registry/objects/" + d.Value},
	} {
		fake, client := newFakeS3(t)
		b := newS3Backend(client, "bucket", tc.prefix)
		if err := b.put(ctx, d.Value, raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := fake.objects[tc.want]; !ok {
			t.Fatalf("prefix %q: stored keys %v, want %s", tc.prefix, keys(fake.objects), tc.want)
		}
		// The second put reaches S3, gets 412 and reports success without
		// overwriting.
		fake.objects[tc.want] = []byte("sentinel")
		if err := b.put(ctx, d.Value, raw); err != nil {
			t.Fatalf("412 must mean already present: %v", err)
		}
		if string(fake.objects[tc.want]) != "sentinel" || fake.puts != 2 {
			t.Fatalf("conditional put overwrote or skipped: %q after %d puts", fake.objects[tc.want], fake.puts)
		}
	}
}

func TestS3BackendRetriesConditionalConflict(t *testing.T) {
	ctx := context.Background()
	raw := []byte("contended")
	d := sha256Digest(raw)

	fake, client := newFakeS3(t)
	fake.conflicts = s3PutAttempts - 1
	if err := newS3Backend(client, "bucket", "").put(ctx, d.Value, raw); err != nil {
		t.Fatalf("put after transient 409s: %v", err)
	}

	fake, client = newFakeS3(t)
	fake.conflicts = s3PutAttempts
	if err := newS3Backend(client, "bucket", "").put(ctx, d.Value, raw); err == nil {
		t.Fatal("persistent 409 must fail")
	}
	if len(fake.objects) != 0 {
		t.Fatal("failed put left an object")
	}
}

func TestS3BackendGetErrors(t *testing.T) {
	ctx := context.Background()
	fake, client := newFakeS3(t)
	b := newS3Backend(client, "bucket", "p")
	name := strings.Repeat("ab", 32)
	if _, err := b.get(ctx, name); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key: err = %v, want ErrNotFound", err)
	}
	fake.getStatus = http.StatusForbidden
	if _, err := b.get(ctx, name); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("403 must be a plain error, got %v", err)
	}
}

func TestParseS3Root(t *testing.T) {
	for _, tc := range []struct{ root, bucket, prefix string }{
		{"s3://bucket", "bucket", ""},
		{"s3://bucket/", "bucket", ""},
		{"s3://bucket/objects", "bucket", "objects"},
		{"s3://bucket/a/b/", "bucket", "a/b"},
	} {
		b, p, err := parseS3Root(tc.root)
		if err != nil || b != tc.bucket || p != tc.prefix {
			t.Fatalf("%s: got (%q, %q, %v)", tc.root, b, p, err)
		}
	}
	for _, bad := range []string{
		"s3://",
		"s3:///prefix",
		"s3://bucket/a//b",
		"s3://bucket/../x",
		"s3://bucket/./x",
		"s3://bucket?region=x",
		"s3://bucket/p#f",
		"s3://user@bucket/p",
		"s3://bucket:9000/p",
		"https://bucket/p",
		"s3://bu%zzcket",
	} {
		if _, _, err := parseS3Root(bad); err == nil {
			t.Fatalf("%s: accepted", bad)
		}
	}
}

func TestOpenObjectStoreSelectsBackend(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenObjectStore(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.blobs.(*fsBackend); !ok {
		t.Fatalf("directory root selected %T", s.blobs)
	}
	if _, err := OpenObjectStore(context.Background(), "s3://"); err == nil {
		t.Fatal("s3 root without bucket accepted")
	}
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent")
	s, err = OpenObjectStore(context.Background(), "s3://bucket/objects")
	if err != nil {
		t.Fatal(err)
	}
	b, ok := s.blobs.(*s3Backend)
	if !ok || b.bucket != "bucket" || b.prefix != "objects" {
		t.Fatalf("s3 root selected %#v", s.blobs)
	}
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	if _, err := OpenObjectStore(context.Background(), "s3://bucket"); err == nil {
		t.Fatal("s3 root without a region accepted")
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
