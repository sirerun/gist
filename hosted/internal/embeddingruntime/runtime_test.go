package embeddingruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDisabledAndInvalidConfiguration(t *testing.T) {
	r, err := Start(context.Background(), Config{})
	if r != nil || err != nil {
		t.Fatal("disabled runtime should not start")
	}
	if _, err = Start(context.Background(), Config{Enabled: true}); err == nil {
		t.Fatal("relative inputs accepted")
	}
}
func TestStageVerifiedBytes(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("verified"), 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("verified"))
	digest := hex.EncodeToString(hash[:])
	out := filepath.Join(dir, "staged")
	if err := stageEmbeddingFile(context.Background(), input, out, digest, 0500); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil || string(body) != "verified" {
		t.Fatal("staged executable changed")
	}
	if err := stageEmbeddingFile(context.Background(), input, filepath.Join(dir, "bad"), digest, 0500); err == nil {
		t.Fatal("digest mismatch accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := stageEmbeddingFile(ctx, input, filepath.Join(dir, "cancelled"), digest, 0500); err == nil {
		t.Fatal("cancellation ignored")
	}
}
func TestBindProofExactLine(t *testing.T) {
	expected := "srv  llama_server: listening on http://127.0.0.1:1234"
	p := &embeddingBindProof{expected: expected, bound: make(chan struct{})}
	for _, bad := range []string{"prefix " + expected + "\n", expected + " extra\n", strings.Repeat("x", 513) + expected + "\n"} {
		_, _ = p.Write([]byte(bad))
		select {
		case <-p.bound:
			t.Fatal("noncanonical proof accepted")
		default:
		}
	}
	_, _ = p.Write([]byte(expected[:10]))
	_, _ = p.Write([]byte(expected[10:] + "\n"))
	select {
	case <-p.bound:
	default:
		t.Fatal("canonical proof missing")
	}
}
func TestNoRequestWithoutOwnedBindProof(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	child := &embeddingChild{done: make(chan struct{}), bound: make(chan struct{})}
	if err := waitEmbeddingReady(ctx, child, server.URL, "private-key", "alias"); err == nil {
		t.Fatal("unowned listener accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("request before bind proof")
	}
}
func TestReadinessRequiresAuthenticationAndAlias(t *testing.T) {
	for _, mode := range []string{"good", "public", "wrong-alias"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode != "public" && r.Header.Get("Authorization") == "" {
					w.WriteHeader(401)
					return
				}
				alias := "owned"
				if mode == "wrong-alias" {
					alias = "foreign"
				}
				_, _ = w.Write([]byte(`{"data":[{"id":"` + alias + `"}]}`))
			}))
			defer server.Close()
			bound := make(chan struct{})
			close(bound)
			child := &embeddingChild{done: make(chan struct{}), bound: bound}
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
			defer cancel()
			err := waitEmbeddingReady(ctx, child, server.URL, "private-key", "owned")
			if (err == nil) != (mode == "good") {
				t.Fatalf("unexpected readiness: %v", err)
			}
		})
	}
}
func TestChildStopReaps(t *testing.T) {
	child, err := startEmbeddingChild("/bin/sh", []string{"-c", "printf 'srv  llama_server: listening on http://127.0.0.1:1234\\n' >&2; sleep 20"}, t.TempDir(), "http://127.0.0.1:1234")
	if err != nil {
		t.Fatal(err)
	}
	defer child.stop()
	select {
	case <-child.bound:
	case <-time.After(time.Second):
		t.Fatal("no bind proof")
	}
	child.stop()
	if child.alive() {
		t.Fatal("child not reaped")
	}
}
