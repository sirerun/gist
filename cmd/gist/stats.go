package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func formatBytes(b int64) string {
	if b < 0 {
		return "0 B"
	}
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1048576 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024.0)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/1048576.0)
}

var showMisses bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show indexing and search statistics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := gistDB.Stats()
		fmt.Printf("Bytes indexed:  %s\n", formatBytes(s.BytesIndexed))
		fmt.Printf("Bytes returned: %s\n", formatBytes(s.BytesReturned))
		fmt.Printf("Bytes saved:    %s (%.1f%%)\n", formatBytes(s.BytesSaved), s.SavedPercent)
		fmt.Printf("Sources:        %d\n", s.SourceCount)
		fmt.Printf("Chunks:         %d\n", s.ChunkCount)
		fmt.Printf("Searches:       %d\n", s.SearchCount)
		fmt.Printf("Misses:         %d (%.1f%%)\n", s.MissCount, s.MissRate*100)

		if showMisses {
			fmt.Println()
			fmt.Println("Miss breakdown:")
			for class, count := range gistDB.MissClassBreakdown() {
				fmt.Printf("  %-24s %d\n", class, count)
			}

			fmt.Println()
			fmt.Println("Recent misses:")
			for _, m := range gistDB.Misses(10) {
				fmt.Printf("  [%s] %q (class=%s, floor=%.2f)\n", m.Time.Format("2006-01-02T15:04:05"), m.Query, m.Class, m.ScoreFloor)
			}
		}
		return nil
	},
}

func init() {
	statsCmd.Flags().BoolVar(&showMisses, "misses", false, "Show retrieval miss telemetry breakdown and recent misses")
	rootCmd.AddCommand(statsCmd)
}
