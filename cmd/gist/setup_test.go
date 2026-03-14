package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureInstructions(t *testing.T) {
	const sentinel = "## Gist Context Management"

	tests := []struct {
		name        string
		initial     *string // nil means file does not exist
		uninstall   bool
		wantChanged bool
		wantContent string
	}{
		{
			name:        "fresh install creates file with gist instructions",
			initial:     nil,
			uninstall:   false,
			wantChanged: true,
			wantContent: "\n" + gistInstructions,
		},
		{
			name:        "existing file without gist appends section with blank line separator",
			initial:     strPtr("# My Config\n\nSome existing content.\n"),
			uninstall:   false,
			wantChanged: true,
			wantContent: "# My Config\n\nSome existing content.\n\n" + gistInstructions,
		},
		{
			name:        "already has gist section is idempotent",
			initial:     strPtr("# My Config\n\n" + gistInstructions),
			uninstall:   false,
			wantChanged: false,
			wantContent: "# My Config\n\n" + gistInstructions,
		},
		{
			name:        "uninstall removes gist section and preserves content before it",
			initial:     strPtr("# My Config\n\nKeep this.\n\n" + gistInstructions),
			uninstall:   true,
			wantChanged: true,
			wantContent: "# My Config\n\nKeep this.\n",
		},
		{
			name:        "uninstall when not present returns no change",
			initial:     strPtr("# My Config\n\nNo gist here.\n"),
			uninstall:   true,
			wantChanged: false,
			wantContent: "# My Config\n\nNo gist here.\n",
		},
		{
			name: "uninstall preserves content after gist section",
			initial: strPtr("# My Config\n\n" + gistInstructions +
				"## Other Section\n\nOther content.\n"),
			uninstall:   true,
			wantChanged: true,
			wantContent: "# My Config\n\n## Other Section\n\nOther content.\n",
		},
		{
			name:        "empty path returns no change and no error",
			initial:     nil,
			uninstall:   false,
			wantChanged: false,
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string

			// Special case: empty path test
			if tt.name == "empty path returns no change and no error" {
				path = ""
			} else {
				dir := t.TempDir()
				path = filepath.Join(dir, "instructions.md")
				if tt.initial != nil {
					if err := os.WriteFile(path, []byte(*tt.initial), 0o644); err != nil {
						t.Fatalf("writing initial file: %v", err)
					}
				}
			}

			changed, err := configureInstructions(path, sentinel, tt.uninstall, false)
			if err != nil {
				t.Fatalf("configureInstructions() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}

			// For empty path, nothing to check on disk.
			if path == "" {
				return
			}

			got, err := os.ReadFile(path)
			if err != nil {
				// If file was never created and we expected empty content, that's fine.
				if os.IsNotExist(err) && tt.wantContent == "" {
					return
				}
				t.Fatalf("reading result file: %v", err)
			}

			if string(got) != tt.wantContent {
				t.Errorf("file content mismatch\ngot:\n%s\nwant:\n%s", quoteLines(string(got)), quoteLines(tt.wantContent))
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func quoteLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  | " + l
	}
	return strings.Join(lines, "\n")
}
