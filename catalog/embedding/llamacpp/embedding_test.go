package llamacpp_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirerun/gist/catalog/embedding/llamacpp"
	"github.com/sirerun/gist/catalog/preparation"
)

const key = "private-test-key-never-leak-0123456789"

func config(origin string, dim int) llamacpp.Config {
	return llamacpp.Config{Origin: origin, APIKey: key, BinarySHA256: strings.Repeat("a", 64), ModelSHA256: llamacpp.ModelSHA256, Dimension: dim, Alias: "qualified-test"}
}
func vector() []float64 { v := make([]float64, 768); v[0] = .6; v[767] = .8; return v }
func payload(n int) map[string]any {
	data := []any{}
	for i := n - 1; i >= 0; i-- {
		data = append(data, map[string]any{"index": i, "object": "embedding", "embedding": vector()})
	}
	return map[string]any{"model": "qualified-test", "object": "list", "data": data, "usage": map[string]any{"prompt_tokens": 3 * n, "total_tokens": 3 * n}}
}
func request(e *llamacpp.Embedder, texts ...string) preparation.EmbeddingRequest {
	return preparation.EmbeddingRequest{ModelVersion: e.ModelVersion(), Purpose: "query", Texts: texts}
}
func TestPublicAdapterProtocolAndDimensions(t *testing.T) {
	for _, dim := range []int{256, 768} {
		t.Run(string(rune(dim)), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+key {
					t.Error("missing private authentication")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				switch r.URL.Path {
				case "/tokenize":
					if body["content"] != llamacpp.QueryPrefix+"one" && body["content"] != llamacpp.QueryPrefix+"two" {
						t.Error("prefix/text changed")
					}
					if body["add_special"] != true || body["parse_special"] != true || body["with_pieces"] != false {
						t.Error("wrong tokenization settings")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"tokens": []int{2, 42, 1}})
				case "/v1/embeddings":
					calls++
					if _, ok := body["model"]; ok {
						t.Error("model echo masquerades as identity")
					}
					if body["embd_normalize"] != float64(2) {
						t.Error("normalization not explicit")
					}
					_ = json.NewEncoder(w).Encode(payload(2))
				default:
					t.Error("unexpected route")
				}
			}))
			defer server.Close()
			e, err := llamacpp.New(config(server.URL, dim))
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			got, err := e.Embed(context.Background(), request(e, "one", "two"))
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(got.Vectors) != 2 || len(got.Vectors[0]) != dim || got.ModelVersion != e.ModelVersion() {
				t.Fatal("protocol output mismatch")
			}
			want := .6
			if dim == 256 {
				want = 1
			}
			if math.Abs(got.Vectors[0][0]-want) > 1e-12 {
				t.Fatal("truncate/renormalize mismatch")
			}
		})
	}
}
func TestMalformedRuntimeResponsesFailClosed(t *testing.T) {
	for _, kind := range []string{"missing-index", "duplicate-index", "wrong-model", "wrong-count", "wrong-dimension", "zero", "wrong-norm", "unknown", "alias-key", "duplicate-key", "unicode-key", "trailing", "oversize", "status"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/tokenize" {
					_, _ = w.Write([]byte(`{"tokens":[2,42,1]}`))
					return
				}
				p := payload(2)
				data := p["data"].([]any)
				switch kind {
				case "missing-index":
					delete(data[0].(map[string]any), "index")
				case "duplicate-index":
					data[0].(map[string]any)["index"] = 0
				case "wrong-model":
					p["model"] = "foreign"
				case "wrong-count":
					p["data"] = data[:1]
				case "wrong-dimension":
					data[0].(map[string]any)["embedding"] = []float64{1}
				case "zero":
					data[0].(map[string]any)["embedding"] = make([]float64, 768)
				case "wrong-norm":
					v := vector()
					v[0] = 4
					data[0].(map[string]any)["embedding"] = v
				case "unknown":
					p["secret"] = key
				case "status":
					w.WriteHeader(500)
					_, _ = w.Write([]byte(key))
					return
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat(" ", (2<<20)+1)))
					return
				}
				raw, _ := json.Marshal(p)
				switch kind {
				case "alias-key":
					raw = []byte(strings.Replace(string(raw), `"model":`, `"MODEL":`, 1))
				case "duplicate-key":
					raw = []byte(strings.Replace(string(raw), `"model":`, `"model":"alias","model":`, 1))
				case "unicode-key":
					raw = []byte(strings.Replace(string(raw), `"total_tokens":`, `"total_tokenſ":`, 1))
				case "trailing":
					raw = append(raw, []byte(" {}")...)
				}
				_, _ = w.Write(raw)
			}))
			defer server.Close()
			e, err := llamacpp.New(config(server.URL, 256))
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			got, err := e.Embed(context.Background(), request(e, "one", "two"))
			if err == nil || len(got.Vectors) != 0 {
				t.Fatal("malformed response exposed vectors")
			}
			if strings.Contains(err.Error(), key) {
				t.Fatal("private runtime body leaked")
			}
		})
	}
}
func TestBoundsBeforeInference(t *testing.T) {
	for _, kind := range []string{"input-overflow", "total-overflow", "bad-bos", "bad-eos", "negative-token", "cancelled", "prefixed", "unknown-purpose", "stale-version", "text-overflow", "batch-overflow"} {
		t.Run(kind, func(t *testing.T) {
			infer := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/tokenize" {
					infer++
					_ = json.NewEncoder(w).Encode(payload(1))
					return
				}
				tokens := []int{2, 42, 1}
				switch kind {
				case "input-overflow":
					tokens = make([]int, 8193)
					tokens[0] = 2
					tokens[len(tokens)-1] = 1
				case "total-overflow":
					tokens = make([]int, 8192)
					tokens[0] = 2
					tokens[len(tokens)-1] = 1
				case "bad-bos":
					tokens[0] = 3
				case "bad-eos":
					tokens[2] = 3
				case "negative-token":
					tokens[1] = -1
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"tokens": tokens})
			}))
			defer server.Close()
			e, err := llamacpp.New(config(server.URL, 256))
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			q := request(e, "one")
			ctx := context.Background()
			switch kind {
			case "total-overflow":
				q.Texts = []string{"a", "b", "c", "d", "e"}
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "prefixed":
				q.Texts[0] = llamacpp.QueryPrefix + "one"
			case "unknown-purpose":
				q.Purpose = "other"
			case "stale-version":
				q.ModelVersion = "stale"
			case "text-overflow":
				q.Texts[0] = strings.Repeat("x", llamacpp.MaxTextBytes+1)
			case "batch-overflow":
				q.Texts = make([]string, 65)
			}
			got, err := e.Embed(ctx, q)
			if err == nil || len(got.Vectors) != 0 || infer != 0 {
				t.Fatal("invalid request reached inference")
			}
		})
	}
}
func TestExplicitLocalConfigurationAndIdentity(t *testing.T) {
	for _, origin := range []string{"https://127.0.0.1:1", "http://localhost:1", "http://192.0.2.1:1", "http://127.0.0.1", "http://127.0.0.1:1/path", "http://user:secret@127.0.0.1:1", "http://127.0.0.1:1?secret=x", "http://127.0.0.1:1?", "http://127.0.0.1:1#"} {
		if _, err := llamacpp.New(config(origin, 256)); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	c := config("http://127.0.0.1:1", 256)
	a, err := llamacpp.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	c.Dimension = 768
	b, err := llamacpp.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.ModelVersion() == b.ModelVersion() {
		t.Fatal("dimensions share identity")
	}
	c.Dimension = 256
	c.BinarySHA256 = strings.Repeat("b", 64)
	d, err := llamacpp.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if a.ModelVersion() == d.ModelVersion() {
		t.Fatal("binary change shares identity")
	}
}
func TestRedirectNeverContactsForeignServer(t *testing.T) {
	foreignCalls := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { foreignCalls++ }))
	defer foreign.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	e, err := llamacpp.New(config(server.URL, 256))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	_, err = e.Embed(context.Background(), request(e, "one"))
	if err == nil || foreignCalls != 0 {
		t.Fatal("redirect followed")
	}
}

func TestNullNumericResponsesAreNotZero(t *testing.T) {
	for _, kind := range []string{"token-null", "token-string", "embedding-null", "embedding-string"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/tokenize" {
					raw := `{"tokens":[2,42,1]}`
					if kind == "token-null" {
						raw = `{"tokens":[2,null,1]}`
					}
					if kind == "token-string" {
						raw = `{"tokens":[2,"42",1]}`
					}
					_, _ = w.Write([]byte(raw))
					return
				}
				p := payload(1)
				raw, _ := json.Marshal(p)
				// The existing unit vector has zero coordinates. Replacing one by null
				// must be rejected even though float64's ordinary decoder would make zero.
				if kind == "embedding-null" {
					raw = []byte(strings.Replace(string(raw), ",0,", ",null,", 1))
				}
				if kind == "embedding-string" {
					raw = []byte(strings.Replace(string(raw), ",0,", `,"0",`, 1))
				}
				_, _ = w.Write(raw)
			}))
			defer server.Close()
			e, err := llamacpp.New(config(server.URL, 256))
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			got, err := e.Embed(context.Background(), request(e, "one"))
			if err == nil || len(got.Vectors) != 0 {
				t.Fatalf("coerced invalid numeric %s", kind)
			}
		})
	}
}
