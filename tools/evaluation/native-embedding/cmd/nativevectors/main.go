// nativevectors emits actual Go HTTP adapter results for frozen synthetic inputs.
// The runtime process and private config are explicitly caller-owned.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/sirerun/gist/catalog/embedding/llamacpp"
	"github.com/sirerun/gist/catalog/preparation"
	"io"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	config := flag.String("config", "", "private runtime config file, never log contents")
	input := flag.String("input", "", "frozen explicit synthetic purpose/text JSONL")
	dimension := flag.Int("dimension", 768, "256 or768")
	flag.Parse()
	raw, err := os.ReadFile(*config)
	if err != nil {
		return errors.New("nativevectors: private config unavailable")
	}
	var c llamacpp.Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return errors.New("nativevectors: invalid private config")
	}
	c.Dimension = *dimension
	e, err := llamacpp.New(c)
	if err != nil {
		return err
	}
	defer e.Close()
	f, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(io.LimitReader(f, 1<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	entries := []map[string]any{}
	timings := []float64{}
	for scanner.Scan() {
		var row struct {
			Purpose string   `json:"purpose"`
			Texts   []string `json:"texts"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return errors.New("nativevectors: invalid synthetic request")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		started := time.Now()
		got, err := e.Embed(ctx, preparation.EmbeddingRequest{ModelVersion: e.ModelVersion(), Purpose: row.Purpose, Texts: row.Texts})
		cancel()
		if err != nil {
			return err
		}
		timings = append(timings, time.Since(started).Seconds())
		for i, text := range row.Texts {
			entries = append(entries, map[string]any{"purpose": row.Purpose, "text": text, "vector": got.Vectors[i]})
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"model_version": e.ModelVersion(), "entries": entries, "adapter_request_seconds": timings})
}
