package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/sirerun/gist"
)

func TestStatsMissesFlag(t *testing.T) {
	if statsCmd.Flags().Lookup("misses") == nil {
		t.Fatal("stats command missing --misses flag")
	}

	origGist := gistDB
	t.Cleanup(func() { gistDB = origGist })

	g, err := gist.New()
	if err != nil {
		t.Fatalf("creating gist: %v", err)
	}
	defer g.Close()
	gistDB = g

	g.Index(context.Background(), "completely unrelated indexed content here", gist.WithSource("test"))
	g.Search(context.Background(), "zzz_no_such_term_zzz")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"stats", "--misses"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"exact_KB", 1024, "1.0 KB"},
		{"fractional_KB", 1536, "1.5 KB"},
		{"exact_MB", 1048576, "1.0 MB"},
		{"fractional_MB", 1572864, "1.5 MB"},
		{"negative", -1, "0 B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBytes(tt.in)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
