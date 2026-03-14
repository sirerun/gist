package main

import (
	"context"
	"fmt"

	"github.com/sirerun/gist"
	"github.com/spf13/cobra"
)

var (
	searchLimit  int
	searchSource string
	searchBudget int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search indexed content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		query := args[0]

		var opts []gist.SearchOption
		opts = append(opts, gist.WithLimit(searchLimit))
		if searchSource != "" {
			opts = append(opts, gist.WithSourceFilter(searchSource))
		}
		if searchBudget > 0 {
			opts = append(opts, gist.WithBudget(searchBudget))
		}

		results, err := gistDB.Search(ctx, query, opts...)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		for i, r := range results {
			snippet := r.Snippet
			if len(snippet) > 100 {
				snippet = snippet[:100] + "..."
			}
			fmt.Printf("[%d] score=%.4f source=%s\n    %s\n    %s\n\n", i+1, r.Score, r.Source, r.Title, snippet)
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().IntVar(&searchLimit, "limit", 5, "maximum number of results")
	searchCmd.Flags().StringVar(&searchSource, "source", "", "filter by source label")
	searchCmd.Flags().IntVar(&searchBudget, "budget", 0, "token budget for results")
	rootCmd.AddCommand(searchCmd)
}
