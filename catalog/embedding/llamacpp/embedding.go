// Package llamacpp adapts an explicitly caller-owned, integrity-verified local
// llama-server to preparation.Embedder. It never starts or downloads a runtime.
package llamacpp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sirerun/gist/catalog/preparation"
)

const EngineCommit = "620d2225cc436ab0b001e3200e590e3b286ef9ab"
const ModelSHA256 = "68bae29d62fb8c7d23e98d21fd4662753ddd636e6b62b8a70dfe059a9844f216"
const QueryPrefix = "task: search result | query: "
const DocumentPrefix = "title: none | text: "
const MaxTokens = 8192
const MaxBatchTokens = 32768
const MaxTexts = 64
const MaxTextBytes = 32768
const MaxBatchTextBytes = 262144
const maxResponseBytes = 2 << 20
const pipeline = "embeddinggemma2-v1:mean:project768:l2:bos-eos:parse-special:prefix-v1:complete8192:batch32768"

// Config describes a runtime verified and owned by the caller. Digests are
// caller assertions about that owned process, not remote identity attestations.
// APIKey is private custody material; never log Config or request bodies.
type Config struct {
	Origin       string
	APIKey       string
	BinarySHA256 string
	ModelSHA256  string
	Dimension    int
	Alias        string
}

type Embedder struct {
	origin, key, alias, version string
	dimension                   int
	client                      *http.Client
}

var _ preparation.Embedder = (*Embedder)(nil)

func New(c Config) (*Embedder, error) {
	u, err := url.Parse(c.Origin)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" {
		return nil, errors.New("llamacpp: explicit numeric loopback HTTP origin required")
	}
	address, err := netip.ParseAddr(u.Hostname())
	port, perr := strconv.Atoi(u.Port())
	if err != nil || !address.IsLoopback() || perr != nil || port < 1 || port > 65535 {
		return nil, errors.New("llamacpp: explicit numeric loopback HTTP origin required")
	}
	if !digest(c.BinarySHA256) || c.ModelSHA256 != ModelSHA256 || (c.Dimension != 256 && c.Dimension != 768) || len(c.Alias) == 0 || len(c.Alias) > 128 || strings.Contains(c.Alias, ",") || len(c.APIKey) < 32 || len(c.APIKey) > 512 {
		return nil, errors.New("llamacpp: invalid pinned runtime configuration")
	}
	for _, s := range []string{c.APIKey, c.Alias} {
		for _, r := range s {
			if r < 33 || r > 126 {
				return nil, errors.New("llamacpp: invalid runtime alias or private key")
			}
		}
	}
	// A private owned transport disables environment proxies, redirects,
	// keepalive implicit replay and HTTP/2; no automatic retries are introduced.
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, DisableKeepAlives: true, ForceAttemptHTTP2: false, MaxResponseHeaderBytes: 8192, ResponseHeaderTimeout: 30 * time.Second}
	identity := EngineCommit + ":" + c.BinarySHA256 + ":" + c.ModelSHA256 + ":" + pipeline + ":" + QueryPrefix + ":" + DocumentPrefix + ":" + strconv.Itoa(c.Dimension)
	sum := sha256.Sum256([]byte(identity))
	return &Embedder{origin: strings.TrimSuffix(c.Origin, "/"), key: c.APIKey, alias: c.Alias, dimension: c.Dimension, version: "llamacpp-embeddinggemma2:sha256:" + hex.EncodeToString(sum[:]), client: &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("llamacpp: redirects refused") }}}, nil
}

func (e *Embedder) ModelVersion() string { return e.version }

// Close releases idle transport resources; process lifecycle remains caller-owned.
func (e *Embedder) Close() { e.client.CloseIdleConnections() }

func (e *Embedder) Embed(ctx context.Context, q preparation.EmbeddingRequest) (preparation.EmbeddingResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	empty := preparation.EmbeddingResult{}
	if q.ModelVersion != e.version || len(q.Texts) == 0 || len(q.Texts) > MaxTexts {
		return empty, errors.New("llamacpp: model identity or batch mismatch")
	}
	prefix := ""
	switch q.Purpose {
	case "query":
		prefix = QueryPrefix
	case "document":
		prefix = DocumentPrefix
	default:
		return empty, errors.New("llamacpp: unsupported embedding purpose")
	}
	texts := make([]string, len(q.Texts))
	size := 0
	for i, text := range q.Texts {
		size += len(text)
		if !utf8.ValidString(text) || len(text) > MaxTextBytes || size > MaxBatchTextBytes || strings.HasPrefix(text, QueryPrefix) || strings.HasPrefix(text, DocumentPrefix) {
			return empty, errors.New("llamacpp: text budget or prefix policy violation")
		}
		texts[i] = prefix + text
	}
	total := 0
	// Tokenize every complete prefixed input before sending any inference request.
	for _, text := range texts {
		var tokens struct {
			Tokens []int `json:"tokens"`
		}
		if err := e.post(ctx, "/tokenize", map[string]any{"content": text, "add_special": true, "parse_special": true, "with_pieces": false}, &tokens); err != nil {
			return empty, err
		}
		if len(tokens.Tokens) < 2 || len(tokens.Tokens) > MaxTokens || tokens.Tokens[0] != 2 || tokens.Tokens[len(tokens.Tokens)-1] != 1 {
			return empty, errors.New("llamacpp: complete tokenizer bounds or BOS/EOS mismatch")
		}
		for _, id := range tokens.Tokens {
			if id < 0 || id >= 262144 {
				return empty, errors.New("llamacpp: invalid token identity")
			}
		}
		total += len(tokens.Tokens)
		if total > MaxBatchTokens {
			return empty, errors.New("llamacpp: total token budget exceeded")
		}
	}
	var response struct {
		Model  string `json:"model"`
		Object string `json:"object"`
		Data   []struct {
			Index     *int      `json:"index"`
			Object    string    `json:"object"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	// Omit request.model: upstream otherwise echoes the caller's value rather
	// than reporting its configured alias. Owned binary/model verification is
	// still required; the response alias is only an additional consistency check.
	if err := e.post(ctx, "/v1/embeddings", map[string]any{"input": texts, "encoding_format": "float", "embd_normalize": 2}, &response); err != nil {
		return empty, err
	}
	if response.Model != e.alias || response.Object != "list" || len(response.Data) != len(texts) || response.Usage.PromptTokens != total || response.Usage.TotalTokens != total {
		return empty, errors.New("llamacpp: response identity/count/usage mismatch")
	}
	result := preparation.EmbeddingResult{ModelVersion: e.version, Vectors: make([][]float64, len(texts))}
	seen := make([]bool, len(texts))
	for _, item := range response.Data {
		if item.Index == nil || *item.Index < 0 || *item.Index >= len(texts) || seen[*item.Index] || item.Object != "embedding" || len(item.Embedding) != 768 {
			return empty, errors.New("llamacpp: invalid indexed vector response")
		}
		seen[*item.Index] = true
		fullNorm := 0.0
		for _, v := range item.Embedding {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return empty, errors.New("llamacpp: nonfinite vector")
			}
			fullNorm += v * v
		}
		if fullNorm <= 0 || math.IsInf(fullNorm, 0) || math.Abs(math.Sqrt(fullNorm)-1) > 1e-5 {
			return empty, errors.New("llamacpp: invalid normalized native vector")
		}
		vector := append([]float64(nil), item.Embedding[:e.dimension]...)
		norm := 0.0
		for _, v := range vector {
			norm += v * v
		}
		if norm <= 0 {
			return empty, errors.New("llamacpp: zero truncated vector")
		}
		norm = math.Sqrt(norm)
		for i := range vector {
			vector[i] /= norm
		}
		result.Vectors[*item.Index] = vector
	}
	return result, nil
}

func digest(s string) bool {
	raw, err := hex.DecodeString(s)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == s
}
func (e *Embedder) post(ctx context.Context, path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return errors.New("llamacpp: invalid local request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.origin+path, bytes.NewReader(raw))
	if err != nil {
		return errors.New("llamacpp: invalid local request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.key)
	res, err := e.client.Do(req)
	if err != nil {
		return errors.New("llamacpp: local runtime request failed")
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return errors.New("llamacpp: local runtime refused request")
	}
	raw, err = io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(raw) > maxResponseBytes {
		return errors.New("llamacpp: bounded response read failed")
	}
	if err := strictKeys(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return errors.New("llamacpp: ambiguous response schema")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("llamacpp: invalid response schema")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("llamacpp: trailing response bytes")
	}
	return nil
}

// All native protocol field names are canonical lowercase. Reject aliases and
// duplicate keys before encoding/json can silently reinterpret a response.
func strictKeys(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || !asciiKey(name) || seen[name] {
				return errors.New("ambiguous response key")
			}
			seen[name] = true
			if err := strictKeys(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := strictKeys(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid response delimiter")
	}
	_, err = d.Token()
	return err
}

func asciiKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '_' {
			return false
		}
	}
	return true
}
