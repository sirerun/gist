package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sirerun/gist"
	"github.com/spf13/cobra"
)

var (
	benchDocs    int
	benchDocSize int
	benchSearchN int
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Run performance benchmarks",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Override root PersistentPreRunE: bench can run chunk-only without DSN.
		if dsn == "" {
			dsn = os.Getenv("GIST_DSN")
		}
		if dsn != "" {
			g, err := gist.New(gist.WithPostgres(dsn))
			if err != nil {
				return fmt.Errorf("connecting to database: %w", err)
			}
			gistDB = g
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		totalBytes := float64(benchDocs) * float64(benchDocSize)
		fmt.Println("Gist Benchmark")
		fmt.Println("==============")
		fmt.Printf("Documents: %d × %.1f KB = %.1f KB total\n\n",
			benchDocs, float64(benchDocSize)/1024, totalBytes/1024)

		// Generate synthetic documents.
		docs := make([]string, benchDocs)
		for i := range docs {
			docs[i] = generateDoc(benchDocSize)
		}

		// Chunk throughput.
		chunkStart := time.Now()
		for _, doc := range docs {
			if _, err := gist.ChunkContent(doc); err != nil {
				return fmt.Errorf("chunking: %w", err)
			}
		}
		chunkDur := time.Since(chunkStart)
		chunkMBs := (totalBytes / (1024 * 1024)) / chunkDur.Seconds()
		fmt.Printf("Chunk throughput:    %.1f MB/s (%d docs in %.1fms)\n",
			chunkMBs, benchDocs, float64(chunkDur.Microseconds())/1000)

		if gistDB == nil {
			fmt.Println("\nNo DSN provided — skipping index and search benchmarks.")
			return nil
		}

		// Index throughput.
		indexStart := time.Now()
		for i, doc := range docs {
			_, err := gistDB.Index(ctx, doc, gist.WithSource(fmt.Sprintf("bench-%d", i)))
			if err != nil {
				return fmt.Errorf("indexing: %w", err)
			}
		}
		indexDur := time.Since(indexStart)
		indexMBs := (totalBytes / (1024 * 1024)) / indexDur.Seconds()
		fmt.Printf("Index throughput:    %.1f MB/s (%d docs in %.1fms)\n",
			indexMBs, benchDocs, float64(indexDur.Microseconds())/1000)

		// Search latency.
		queries := make([]string, benchSearchN)
		for i := range queries {
			queries[i] = generateQuery()
		}

		latencies := make([]time.Duration, benchSearchN)
		for i, q := range queries {
			start := time.Now()
			if _, err := gistDB.Search(ctx, q); err != nil {
				return fmt.Errorf("search: %w", err)
			}
			latencies[i] = time.Since(start)
		}
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

		fmt.Printf("Search latency (%d queries):\n", benchSearchN)
		fmt.Printf("  p50:  %.2fms\n", float64(percentile(latencies, 0.50).Microseconds())/1000)
		fmt.Printf("  p95:  %.2fms\n", float64(percentile(latencies, 0.95).Microseconds())/1000)
		fmt.Printf("  p99:  %.2fms\n", float64(percentile(latencies, 0.99).Microseconds())/1000)

		return nil
	},
}

func init() {
	benchCmd.Flags().IntVar(&benchDocs, "docs", 100, "number of documents to generate")
	benchCmd.Flags().IntVar(&benchDocSize, "doc-size", 10000, "bytes per document")
	benchCmd.Flags().IntVar(&benchSearchN, "searches", 100, "number of search queries to run")
	rootCmd.AddCommand(benchCmd)
}

// percentile returns the value at the given percentile p (0.0–1.0) from a
// sorted slice of durations.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// wordList is a small vocabulary used for generating synthetic documents and queries.
var wordList = []string{
	"server", "database", "configuration", "deployment", "monitoring",
	"authentication", "middleware", "endpoint", "container", "network",
	"protocol", "encryption", "pipeline", "scheduler", "cluster",
	"storage", "migration", "replication", "partition", "transaction",
	"function", "variable", "interface", "module", "package",
	"algorithm", "structure", "framework", "library", "service",
	"request", "response", "handler", "router", "controller",
	"template", "component", "instance", "resource", "platform",
}

// generateDoc creates a synthetic markdown document of approximately sizeBytes.
func generateDoc(sizeBytes int) string {
	var b strings.Builder
	sections := []string{"Overview", "Configuration", "Usage", "API Reference", "Troubleshooting"}
	sectionIdx := 0

	for b.Len() < sizeBytes {
		// Add a heading.
		heading := sections[sectionIdx%len(sections)]
		sectionIdx++
		fmt.Fprintf(&b, "## %s\n\n", heading)

		// Add a prose paragraph.
		for i := 0; i < 5 && b.Len() < sizeBytes; i++ {
			sentenceLen := 8 + rand.Intn(12)
			words := make([]string, sentenceLen)
			for j := range words {
				words[j] = wordList[rand.Intn(len(wordList))]
			}
			words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
			fmt.Fprintf(&b, "%s. ", strings.Join(words, " "))
		}
		b.WriteString("\n\n")

		// Add a code block every other section.
		if sectionIdx%2 == 0 && b.Len() < sizeBytes {
			b.WriteString("```go\n")
			for i := 0; i < 5 && b.Len() < sizeBytes; i++ {
				varName := wordList[rand.Intn(len(wordList))]
				fmt.Fprintf(&b, "func %s() error {\n\treturn nil\n}\n\n", varName)
			}
			b.WriteString("```\n\n")
		}
	}

	result := b.String()
	if len(result) > sizeBytes {
		result = result[:sizeBytes]
	}
	return result
}

// generateQuery returns a random 1-3 word query from the word list.
func generateQuery() string {
	n := 1 + rand.Intn(3)
	words := make([]string, n)
	for i := range words {
		words[i] = wordList[rand.Intn(len(wordList))]
	}
	return strings.Join(words, " ")
}
