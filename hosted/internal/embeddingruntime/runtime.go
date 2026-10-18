// Package embeddingruntime owns a pinned private loopback embedding process.
// Runtime inputs are installed by deployment; this package never downloads them.
package embeddingruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirerun/gist/catalog/embedding/llamacpp"
)

type Config struct {
	Enabled          bool
	BinaryPath       string
	ModelPath        string
	BinarySHA256File string
}

// Runtime remains usable only while Healthy reports true. Callers must fall
// back to lexical retrieval on any Embed error or child failure.
type Runtime struct {
	Embedder *llamacpp.Embedder
	child    *embeddingChild
	session  string
	closed   chan struct{}
	once     sync.Once
	closeErr error
}

func (r *Runtime) Healthy() bool         { return r != nil && r.child != nil && r.child.alive() }
func (r *Runtime) Done() <-chan struct{} { return r.child.done }
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		r.child.stop()
		r.Embedder.Close()
		if os.RemoveAll(r.session) != nil {
			r.closeErr = errors.New("embedding runtime: private cleanup failed")
		}
		close(r.closed)
	})
	return r.closeErr
}

// Start returns nil, nil when disabled. Its startup budget includes digest
// verification, staging and authenticated readiness. ctx owns the full lifetime.
func Start(ctx context.Context, cfg Config) (_ *Runtime, resultErr error) {
	if !cfg.Enabled {
		return nil, nil
	}
	startup, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	for _, p := range []string{cfg.BinaryPath, cfg.ModelPath, cfg.BinarySHA256File} {
		if !filepath.IsAbs(p) {
			return nil, errors.New("embedding runtime: absolute installed paths required")
		}
	}
	digestFile, err := os.Open(cfg.BinarySHA256File)
	if err != nil {
		return nil, errors.New("embedding runtime: binary digest unavailable")
	}
	digestBytes, readErr := io.ReadAll(io.LimitReader(digestFile, 257))
	closeErr := digestFile.Close()
	if readErr != nil || closeErr != nil || len(digestBytes) > 256 {
		return nil, errors.New("embedding runtime: binary digest unavailable")
	}
	digest := strings.TrimSpace(string(digestBytes))
	if !embeddingDigest(digest) {
		return nil, errors.New("embedding runtime: invalid binary digest")
	}
	session, err := os.MkdirTemp("", "gist-embedding-")
	if err != nil {
		return nil, errors.New("embedding runtime: private staging unavailable")
	}
	var child *embeddingChild
	success := false
	defer func() {
		if !success {
			if child != nil {
				child.stop()
			}
			_ = os.RemoveAll(session)
		}
	}()
	binary := filepath.Join(session, "llama-server")
	model := filepath.Join(session, "model.gguf")
	if err = stageEmbeddingFile(startup, cfg.BinaryPath, binary, digest, 0500); err != nil {
		return nil, err
	}
	if err = stageEmbeddingFile(startup, cfg.ModelPath, model, llamacpp.ModelSHA256, 0400); err != nil {
		return nil, err
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return nil, errors.New("embedding runtime: authentication unavailable")
	}
	key := hex.EncodeToString(random)
	keyFile := filepath.Join(session, "api-key")
	if err = os.WriteFile(keyFile, []byte(key), 0600); err != nil {
		return nil, errors.New("embedding runtime: authentication unavailable")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("embedding runtime: loopback port unavailable")
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	aliasDigest := sha256.Sum256([]byte(key))
	alias := "gist-embedding-" + hex.EncodeToString(aliasDigest[:8])
	origin := fmt.Sprintf("http://127.0.0.1:%d", port)
	args := []string{"--model", model, "--embedding", "--pooling", "mean", "--embd-normalize", "2", "--ctx-size", "8192", "--batch-size", "8192", "--ubatch-size", "8192", "--parallel", "1", "--threads", "2", "--threads-batch", "2", "--n-gpu-layers", "0", "--no-warmup", "--no-log-prefix", "--no-log-timestamps", "--log-colors", "off", "--host", "127.0.0.1", "--port", fmt.Sprint(port), "--alias", alias, "--api-key-file", keyFile}
	if startup.Err() != nil {
		return nil, errors.New("embedding runtime: startup cancelled")
	}
	child, err = startEmbeddingChild(binary, args, session, origin)
	if err != nil {
		return nil, err
	}
	if err = waitEmbeddingReady(startup, child, origin, key, alias); err != nil {
		return nil, err
	}
	embedder, err := llamacpp.New(llamacpp.Config{Origin: origin, APIKey: key, BinarySHA256: digest, ModelSHA256: llamacpp.ModelSHA256, Dimension: 768, Alias: alias})
	if err != nil {
		return nil, errors.New("embedding runtime: invalid identity")
	}
	r := &Runtime{Embedder: embedder, child: child, session: session, closed: make(chan struct{})}
	success = true
	go func() {
		select {
		case <-ctx.Done():
		case <-child.done:
		case <-r.closed:
			return
		}
		_ = r.Close()
	}()
	return r, nil
}

func embeddingDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && strings.ToLower(s) == s
}

// Copy then verify the actual staged bytes, rather than executing a path after
// hashing it. The session is private and removed only after the child is reaped.
func stageEmbeddingFile(ctx context.Context, source, target, digest string, mode os.FileMode) error {
	info, err := os.Lstat(source)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("embedding runtime: runtime input must be a regular file")
	}
	in, err := os.Open(source)
	if err != nil {
		return errors.New("embedding runtime: runtime input unavailable")
	}
	defer func() { _ = in.Close() }()
	opened, err := in.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return errors.New("embedding runtime: runtime input changed")
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("embedding runtime: private staging failed")
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, h), embeddingContextReader{ctx: ctx, reader: in})
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || hex.EncodeToString(h.Sum(nil)) != digest {
		return errors.New("embedding runtime: runtime digest mismatch")
	}
	if err = os.Chmod(target, mode); err != nil {
		return errors.New("embedding runtime: private staging failed")
	}
	return nil
}

type embeddingContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r embeddingContextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(b)
}

// Log bytes are drained and discarded, never surfaced in errors: server output
// can contain local paths or request data. No pipe can block shutdown.
type embeddingDiscard struct{}

func (embeddingDiscard) Write(b []byte) (int, error) { return len(b), nil }

// Only the verified child stderr pipe can produce this exact bind-success
// event. Public HTTP metadata is never consulted before exclusive bind proof.
// Pinned llama.cpp disables SO_REUSEPORT by default; no reuse-port flag is used.
type embeddingBindProof struct {
	expected string
	line     []byte
	overflow bool
	once     sync.Once
	bound    chan struct{}
}

func (p *embeddingBindProof) Write(body []byte) (int, error) {
	for _, b := range body {
		if b == '\n' {
			if !p.overflow && string(p.line) == p.expected {
				p.once.Do(func() { close(p.bound) })
			}
			p.line = p.line[:0]
			p.overflow = false
		} else if !p.overflow {
			if len(p.line) >= 512 {
				p.line = p.line[:0]
				p.overflow = true
			} else {
				p.line = append(p.line, b)
			}
		}
	}
	return len(body), nil
}

type embeddingChild struct {
	cmd   *exec.Cmd
	done  chan struct{}
	bound <-chan struct{}
}

func startEmbeddingChild(binary string, args []string, dir, origin string) (*embeddingChild, error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = []string{"LANG=C", "LC_ALL=C", "HOME=" + dir, "TMPDIR=" + dir}
	cmd.Stdout = embeddingDiscard{}
	proof := &embeddingBindProof{expected: "srv  llama_server: listening on " + origin, bound: make(chan struct{})}
	cmd.Stderr = proof
	configureEmbeddingProcess(cmd)
	if err := cmd.Start(); err != nil {
		return nil, errors.New("embedding runtime: runtime could not start")
	}
	child := &embeddingChild{cmd: cmd, done: make(chan struct{}), bound: proof.bound}
	go func() { _ = cmd.Wait(); close(child.done) }()
	return child, nil
}
func (c *embeddingChild) alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}
func (c *embeddingChild) stop() {
	if !c.alive() {
		return
	}
	signalEmbeddingProcess(c.cmd, false)
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-c.done:
		return
	case <-timer.C:
	}
	signalEmbeddingProcess(c.cmd, true)
	<-c.done
}

func waitEmbeddingReady(ctx context.Context, child *embeddingChild, origin, key, alias string) error {
	select {
	case <-ctx.Done():
		return errors.New("embedding runtime: owned bind proof deadline exceeded")
	case <-child.done:
		return errors.New("embedding runtime: owned runtime exited before bind proof")
	case <-child.bound:
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !child.alive() {
			return errors.New("embedding runtime: owned runtime exited before readiness")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/v1/models", nil)
		if err != nil {
			return errors.New("embedding runtime: invalid readiness request")
		}
		req.Header.Set("Authorization", "Bearer "+key)
		response, err := client.Do(req)
		if err == nil {
			var models struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 65537))
			_ = response.Body.Close()
			if readErr == nil && len(body) <= 65536 && response.StatusCode == http.StatusOK && json.Unmarshal(body, &models) == nil && child.alive() {
				for _, m := range models.Data {
					if m.ID == alias && embeddingRejectsAnonymous(ctx, client, origin) && child.alive() {
						return nil
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return errors.New("embedding runtime: authenticated readiness deadline exceeded")
		case <-child.done:
			return errors.New("embedding runtime: owned runtime exited before readiness")
		case <-ticker.C:
		}
	}
}

// Readiness is a private-key challenge, not public health or an echoed alias.
// A listener that answers the same public model metadata is insufficient.
func embeddingRejectsAnonymous(ctx context.Context, client *http.Client, origin string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/v1/models", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 65536))
	return response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden
}
