package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOversizedValidPrefixCannotBeSilentlyEmbedded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.jsonl")
	// Every full line is valid; LimitReader plus Scanner previously accepted
	// a complete prefix while silently losing the remaining valid request.
	line := `{"purpose":"query","texts":["independent synthetic probe"]}` + "\n"
	raw := strings.Repeat(line, (1<<20)/len(line)+2)
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := readInput(path)
	if err == nil || snapshot != nil {
		t.Fatal("oversized input exposed a valid prefix")
	}
}
func TestSingleSnapshotInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.jsonl")
	raw := []byte("{}\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readInput(path)
	if err != nil || string(got) != string(raw) {
		t.Fatal("valid snapshot rejected")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readInput(link); err == nil {
		t.Fatal("symlink accepted")
	}
}
