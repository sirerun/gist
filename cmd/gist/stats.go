package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show indexing and search statistics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := gistDB.Stats()
		fmt.Printf("Bytes indexed:  %d\n", s.BytesIndexed)
		fmt.Printf("Bytes returned: %d\n", s.BytesReturned)
		fmt.Printf("Bytes saved:    %d (%.1f%%)\n", s.BytesSaved, s.SavedPercent)
		fmt.Printf("Sources:        %d\n", s.SourceCount)
		fmt.Printf("Chunks:         %d\n", s.ChunkCount)
		fmt.Printf("Searches:       %d\n", s.SearchCount)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
