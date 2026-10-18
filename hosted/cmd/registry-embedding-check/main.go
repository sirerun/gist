// registry-embedding-check qualifies the installed production runtime without
// contacting catalog storage or external services. It makes no relevance claim.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/hosted/internal/embeddingruntime"
)

type receipt struct {
	Status              string  `json:"status"`
	Dimension           int     `json:"dimension"`
	ModelVersion        string  `json:"model_version"`
	QueryDocumentCosine float64 `json:"query_document_cosine"`
	ElapsedSeconds      float64 `json:"elapsed_seconds"`
}

func installed(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	result, err := check(ctx, embeddingruntime.Config{
		Enabled:          true,
		BinaryPath:       installed("GIST_EMBEDDING_BINARY", "/usr/local/bin/llama-server"),
		ModelPath:        installed("GIST_EMBEDDING_MODEL", "/models/embeddinggemma-2-BF16.gguf"),
		BinarySHA256File: installed("GIST_EMBEDDING_BINARY_SHA256_FILE", "/usr/local/share/llama-server.sha256"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "embedding qualification failed")
		os.Exit(1)
	}
	result.ElapsedSeconds = time.Since(started).Seconds()
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		os.Exit(1)
	}
}

func check(ctx context.Context, cfg embeddingruntime.Config) (result receipt, err error) {
	runtime, err := embeddingruntime.Start(ctx, cfg)
	if err != nil || runtime == nil {
		return result, errors.New("runtime unavailable")
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			err = errors.New("runtime cleanup failed")
		}
	}()
	version := runtime.Embedder.ModelVersion()
	query, err := runtime.Embedder.Embed(ctx, preparation.EmbeddingRequest{ModelVersion: version, Purpose: "query", Texts: []string{"Find tools that summarize email messages."}})
	if err != nil || query.ModelVersion != version {
		return result, errors.New("query inference failed")
	}
	document, err := runtime.Embedder.Embed(ctx, preparation.EmbeddingRequest{ModelVersion: version, Purpose: "document", Texts: []string{"Email tools can read and summarize inbox messages."}})
	if err != nil || document.ModelVersion != version {
		return result, errors.New("document inference failed")
	}
	cosine, err := validatePair(query.Vectors, document.Vectors)
	if err != nil || !runtime.Healthy() {
		return result, errors.New("embedding validation failed")
	}
	return receipt{Status: "verified", Dimension: 768, ModelVersion: version, QueryDocumentCosine: cosine}, nil
}

// Validate numerical output, not semantic relevance: no minimum similarity is
// imposed on this synthetic pair. An all-zero, truncated or nonfinite vector
// must never qualify merely because its response had a successful HTTP status.
func validatePair(query, document [][]float64) (float64, error) {
	if len(query) != 1 || len(document) != 1 {
		return 0, errors.New("expected one vector per purpose")
	}
	norm := func(vector []float64) (float64, error) {
		if len(vector) != 768 {
			return 0, errors.New("expected dimension 768")
		}
		var sum float64
		for _, component := range vector {
			v := float64(component)
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return 0, errors.New("nonfinite component")
			}
			sum += v * v
		}
		if math.Abs(sum-1) > 0.001 {
			return 0, errors.New("expected unit normalization")
		}
		return math.Sqrt(sum), nil
	}
	qn, err := norm(query[0])
	if err != nil {
		return 0, err
	}
	dn, err := norm(document[0])
	if err != nil {
		return 0, err
	}
	var dot float64
	for i, q := range query[0] {
		dot += float64(q) * float64(document[0][i])
	}
	cosine := dot / (qn * dn)
	if math.IsNaN(cosine) || math.IsInf(cosine, 0) || math.Abs(cosine) > 1.000001 {
		return 0, errors.New("invalid cosine")
	}
	return cosine, nil
}
