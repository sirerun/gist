package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandHelp(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOut  string
		wantErr  bool
	}{
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantOut: "Context intelligence for LLM applications",
		},
		{
			name:    "index help",
			args:    []string{"index", "--help"},
			wantOut: "Index files for search",
		},
		{
			name:    "search help",
			args:    []string{"search", "--help"},
			wantOut: "Search indexed content",
		},
		{
			name:    "stats help",
			args:    []string{"stats", "--help"},
			wantOut: "Show indexing and search statistics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs(tt.args)

			err := rootCmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			out := buf.String()
			if !bytes.Contains([]byte(out), []byte(tt.wantOut)) {
				t.Errorf("output does not contain %q:\n%s", tt.wantOut, out)
			}
		})
	}
}

func TestRootCommandFallsBackToInMemory(t *testing.T) {
	// Reset dsn to empty to test in-memory fallback.
	orig := dsn
	dsn = ""
	origGist := gistDB
	t.Cleanup(func() {
		dsn = orig
		gistDB = origGist
	})
	t.Setenv("GIST_DSN", "")

	cmd := &cobra.Command{
		Use: "test-stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.PersistentPreRunE = rootCmd.PersistentPreRunE
	cmd.SetArgs([]string{})
	cmd.SetOut(new(bytes.Buffer))
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gistDB == nil {
		t.Fatal("expected gistDB to be set with in-memory store")
	}
}

func TestIndexCommandFlags(t *testing.T) {
	f := indexCmd.Flags()
	if f.Lookup("format") == nil {
		t.Error("index command missing --format flag")
	}
}

func TestSearchCommandFlags(t *testing.T) {
	tests := []struct {
		flag string
	}{
		{"limit"},
		{"source"},
		{"budget"},
	}
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			if searchCmd.Flags().Lookup(tt.flag) == nil {
				t.Errorf("search command missing --%s flag", tt.flag)
			}
		})
	}
}

func TestSubcommandsRegistered(t *testing.T) {
	cmds := rootCmd.Commands()
	names := make(map[string]bool)
	for _, c := range cmds {
		names[c.Name()] = true
	}

	for _, want := range []string{"index", "search", "stats"} {
		if !names[want] {
			t.Errorf("subcommand %q not registered on root", want)
		}
	}
}
