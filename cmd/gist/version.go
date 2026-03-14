package main

import (
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version can be set via ldflags: -X main.version=v0.1.0
var version string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of gist",
	Run: func(cmd *cobra.Command, args []string) {
		v := version
		if v == "" {
			if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
				v = info.Main.Version
			} else {
				v = "(devel)"
			}
		}
		cmd.Printf("gist version %s (%s %s/%s)\n", v, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	// Override PersistentPreRunE so version doesn't require --dsn.
	versionCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	rootCmd.AddCommand(versionCmd)
}
