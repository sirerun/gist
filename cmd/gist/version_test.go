package main

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestVersionCmd(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "gist version ") {
		t.Errorf("expected output to start with 'gist version ', got: %s", out)
	}
	if !strings.Contains(out, runtime.Version()) {
		t.Errorf("expected output to contain Go version %s, got: %s", runtime.Version(), out)
	}
	if !strings.Contains(out, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("expected output to contain %s/%s, got: %s", runtime.GOOS, runtime.GOARCH, out)
	}
}

func TestVersionCmdDoesNotRequireDSN(t *testing.T) {
	t.Setenv("GIST_DSN", "")

	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command should not require --dsn, but got error: %v", err)
	}
}
