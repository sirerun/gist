package main

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/sirerun/gist/mcp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server over stdio",
	Long:  "Start an MCP (Model Context Protocol) server that reads JSON-RPC 2.0 messages from stdin and writes responses to stdout.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		srv := mcp.NewServer(gistDB)
		if err := srv.Serve(ctx); err != nil && ctx.Err() == nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
