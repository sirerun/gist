package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestNativeHelperProcess(t *testing.T) {
	if os.Getenv("GIST_NATIVE_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("nativevectors", flag.ContinueOnError)
	if err := run(); err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
		os.Exit(2)
	}
	os.Exit(0)
}
func TestOversizedCLIInputMakesNoNativeRequests(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	input := filepath.Join(dir, "input.jsonl")
	raw, err := json.Marshal(map[string]any{"Origin": server.URL, "APIKey": strings.Repeat("k", 64), "BinarySHA256": strings.Repeat("a", 64), "ModelSHA256": "68bae29d62fb8c7d23e98d21fd4662753ddd636e6b62b8a70dfe059a9844f216", "Dimension": 256, "Alias": "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	line := `{"purpose":"query","texts":["probe"]}` + "\n"
	if err := os.WriteFile(input, []byte(strings.Repeat(line, (1<<20)/len(line)+2)), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestNativeHelperProcess", "--", "--config", config, "--input", input)
	cmd.Env = append(os.Environ(), "GIST_NATIVE_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "bounded regular input") || calls.Load() != 0 {
		t.Fatalf("oversized CLI input reached native runtime: %s", out)
	}
}
