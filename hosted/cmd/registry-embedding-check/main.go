// registry-embedding-check qualifies the installed production runtime without
// contacting catalog storage or external services. It makes no relevance claim.
package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/sirerun/gist/catalog/directory"
	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/hosted/internal/embeddingruntime"
	"github.com/sirerun/gist/hosted/internal/semantic"
)

type receipt struct {
	Status              string  `json:"status"`
	Dimension           int     `json:"dimension"`
	ModelVersion        string  `json:"model_version"`
	QueryDocumentCosine float64 `json:"query_document_cosine"`
	ElapsedSeconds      float64 `json:"elapsed_seconds"`
	PublicIndexVerified bool    `json:"public_index_verified"`
	PublicAppMatches    int     `json:"public_app_matches"`
}

func installed(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	output := flag.String("write-public-index", "", "Generate the release public-corpus snapshot at this absolute path")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(1)
	}
	started := time.Now()
	budget := 180 * time.Second
	if *output != "" {
		budget = 1800 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cfg := embeddingruntime.Config{
		Enabled:          true,
		BinaryPath:       installed("GIST_EMBEDDING_BINARY", "/usr/local/bin/llama-server"),
		ModelPath:        installed("GIST_EMBEDDING_MODEL", "/models/embeddinggemma-2-BF16.gguf"),
		BinarySHA256File: installed("GIST_EMBEDDING_BINARY_SHA256_FILE", "/usr/local/share/llama-server.sha256"),
	}
	if *output != "" {
		cfg.BatchSlots = 8
		cfg.Threads = 8
	}
	result, err := check(ctx, cfg, *output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "embedding qualification failed:", err)
		os.Exit(1)
	}
	result.ElapsedSeconds = time.Since(started).Seconds()
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		os.Exit(1)
	}
}

func check(ctx context.Context, cfg embeddingruntime.Config, output string) (result receipt, err error) {
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
	index, err := directory.DefaultPublic()
	if err != nil {
		return result, errors.New("public corpus unavailable")
	}
	apps, err := directory.DefaultApps(index)
	if err != nil {
		return result, errors.New("public apps unavailable")
	}
	search := semantic.New(index, apps, runtime.Embedder, runtime.Healthy)
	if output != "" {
		if err := search.Warm(ctx); err != nil {
			return result, fmt.Errorf("public corpus qualification failed: %w", err)
		}
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return result, errors.New("snapshot output unavailable")
		}
		z := gzip.NewWriter(f)
		writeErr := search.WriteSnapshot(z)
		zipErr := z.Close()
		closeErr := f.Close()
		if writeErr != nil || zipErr != nil || closeErr != nil {
			return result, errors.New("snapshot output failed")
		}
	} else if err := search.LoadSnapshotFile(installed("GIST_EMBEDDING_SNAPSHOT", "/models/public-embeddings.json.gz")); err != nil {
		return result, fmt.Errorf("public snapshot qualification failed: %w", err)
	}
	found, err := search.SearchApps(ctx, directory.AppQuery{Text: "find leads for my startup", Limit: 10})
	if err != nil || found.SearchMode != "hybrid" || found.Total == 0 {
		return result, errors.New("public retrieval qualification failed")
	}
	return receipt{Status: "verified", Dimension: 768, ModelVersion: version, QueryDocumentCosine: cosine, PublicIndexVerified: true, PublicAppMatches: found.Total}, nil
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
