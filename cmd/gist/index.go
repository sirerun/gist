package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirerun/gist"
	"github.com/spf13/cobra"
)

var indexFormat string

var indexCmd = &cobra.Command{
	Use:   "index <file> [file...]",
	Short: "Index files for search",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		var format gist.Format
		switch strings.ToLower(indexFormat) {
		case "markdown", "md":
			format = gist.FormatMarkdown
		case "plaintext", "plain", "text":
			format = gist.FormatPlainText
		default:
			format = gist.FormatMarkdown
		}

		for _, file := range args {
			data, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading %s: %v\n", file, err)
				continue
			}
			result, err := gistDB.Index(ctx, string(data),
				gist.WithSource(file),
				gist.WithFormat(format),
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error indexing %s: %v\n", file, err)
				continue
			}
			fmt.Printf("Indexed %s: %d chunks (%d code)\n", file, result.TotalChunks, result.CodeChunks)
		}
		return nil
	},
}

func init() {
	indexCmd.Flags().StringVar(&indexFormat, "format", "markdown", "content format (markdown, plaintext)")
	rootCmd.AddCommand(indexCmd)
}
