package main

import (
	"fmt"
	"os"

	"github.com/sirerun/gist"
	"github.com/spf13/cobra"
)

var (
	dsn    string
	gistDB *gist.Gist
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gist",
	Short: "Context intelligence for LLM applications",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if dsn == "" {
			dsn = os.Getenv("GIST_DSN")
		}
		if dsn == "" {
			return fmt.Errorf("--dsn flag or GIST_DSN environment variable is required")
		}
		g, err := gist.New(gist.WithPostgres(dsn))
		if err != nil {
			return fmt.Errorf("connecting to database: %w", err)
		}
		gistDB = g
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if gistDB != nil {
			_ = gistDB.Close()
		}
	},
}


func init() {
	rootCmd.PersistentFlags().StringVar(&dsn, "dsn", "", "PostgreSQL DSN (also reads GIST_DSN env var)")
}
