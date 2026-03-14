package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testGistPath = "/usr/local/bin/gist"

func TestConfigureMCPJSON(t *testing.T) {
	tests := []struct {
		name        string
		mcpKey      string
		existing    string // empty means file does not exist
		gistPath    string
		uninstall   bool
		wantChanged bool
		wantErr     bool
		check       func(t *testing.T, path string)
	}{
		{
			name:        "fresh install creates file with gist entry",
			mcpKey:      "mcpServers",
			gistPath:    testGistPath,
			wantChanged: true,
			check: func(t *testing.T, path string) {
				data := readJSONFile(t, path)
				servers := jsonObj(t, data, "mcpServers")
				gist := jsonObj(t, servers, "gist")
				if gist["command"] != testGistPath {
					t.Errorf("command = %v, want %v", gist["command"], testGistPath)
				}
				args, ok := gist["args"].([]any)
				if !ok || len(args) != 1 || args[0] != "serve" {
					t.Errorf("args = %v, want [serve]", gist["args"])
				}
			},
		},
		{
			name:     "existing file with other servers preserves them",
			mcpKey:   "mcpServers",
			gistPath: testGistPath,
			existing: `{
  "mcpServers": {
    "other": {
      "command": "/usr/bin/other",
      "args": ["run"]
    }
  }
}`,
			wantChanged: true,
			check: func(t *testing.T, path string) {
				data := readJSONFile(t, path)
				servers := jsonObj(t, data, "mcpServers")
				if _, ok := servers["other"]; !ok {
					t.Error("other server entry was removed")
				}
				if _, ok := servers["gist"]; !ok {
					t.Error("gist entry was not added")
				}
			},
		},
		{
			name:     "already configured with same path is idempotent",
			mcpKey:   "mcpServers",
			gistPath: testGistPath,
			existing: `{
  "mcpServers": {
    "gist": {
      "command": "/usr/local/bin/gist",
      "args": ["serve"]
    }
  }
}`,
			wantChanged: false,
		},
		{
			name:     "different gist path updates entry",
			mcpKey:   "mcpServers",
			gistPath: "/new/path/gist",
			existing: `{
  "mcpServers": {
    "gist": {
      "command": "/old/path/gist",
      "args": ["serve"]
    }
  }
}`,
			wantChanged: true,
			check: func(t *testing.T, path string) {
				data := readJSONFile(t, path)
				servers := jsonObj(t, data, "mcpServers")
				gist := jsonObj(t, servers, "gist")
				if gist["command"] != "/new/path/gist" {
					t.Errorf("command = %v, want /new/path/gist", gist["command"])
				}
			},
		},
		{
			name:      "uninstall removes gist entry preserves others",
			mcpKey:    "mcpServers",
			gistPath:  testGistPath,
			uninstall: true,
			existing: `{
  "mcpServers": {
    "gist": {
      "command": "/usr/local/bin/gist",
      "args": ["serve"]
    },
    "other": {
      "command": "/usr/bin/other",
      "args": ["run"]
    }
  }
}`,
			wantChanged: true,
			check: func(t *testing.T, path string) {
				data := readJSONFile(t, path)
				servers := jsonObj(t, data, "mcpServers")
				if _, ok := servers["gist"]; ok {
					t.Error("gist entry was not removed")
				}
				if _, ok := servers["other"]; !ok {
					t.Error("other server entry was removed")
				}
			},
		},
		{
			name:      "uninstall when not present is no-op",
			mcpKey:    "mcpServers",
			gistPath:  testGistPath,
			uninstall: true,
			existing: `{
  "mcpServers": {
    "other": {
      "command": "/usr/bin/other",
      "args": ["run"]
    }
  }
}`,
			wantChanged: false,
		},
		{
			name:     "malformed JSON returns error",
			mcpKey:   "mcpServers",
			gistPath: testGistPath,
			existing: `{not valid json`,
			wantErr:  true,
		},
		{
			name:        "servers key for copilot variant",
			mcpKey:      "servers",
			gistPath:    testGistPath,
			wantChanged: true,
			check: func(t *testing.T, path string) {
				data := readJSONFile(t, path)
				servers := jsonObj(t, data, "servers")
				gist := jsonObj(t, servers, "gist")
				if gist["command"] != testGistPath {
					t.Errorf("command = %v, want %v", gist["command"], testGistPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "mcp.json")

			if tt.existing != "" {
				if err := os.WriteFile(path, []byte(tt.existing), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			changed, err := configureMCPJSON(path, tt.mcpKey, tt.gistPath, tt.uninstall, false)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if tt.check != nil {
				tt.check(t, path)
			}
		})
	}
}

func TestConfigureMCPTOML(t *testing.T) {
	tests := []struct {
		name        string
		existing    string
		gistPath    string
		uninstall   bool
		wantChanged bool
		check       func(t *testing.T, path string)
	}{
		{
			name:        "fresh install creates file with gist section",
			gistPath:    testGistPath,
			wantChanged: true,
			check: func(t *testing.T, path string) {
				content := readFile(t, path)
				for _, want := range []string{
					"[mcp_servers.gist]",
					`command = "/usr/local/bin/gist"`,
					`args = ["serve"]`,
				} {
					if !strings.Contains(content, want) {
						t.Errorf("file missing %q", want)
					}
				}
			},
		},
		{
			name:        "existing file with other sections preserves them",
			gistPath:    testGistPath,
			existing:    "[mcp_servers.other]\ncommand = \"/usr/bin/other\"\nargs = [\"run\"]\n",
			wantChanged: true,
			check: func(t *testing.T, path string) {
				content := readFile(t, path)
				if !strings.Contains(content, "[mcp_servers.other]") {
					t.Error("other section was removed")
				}
				if !strings.Contains(content, "[mcp_servers.gist]") {
					t.Error("gist section was not added")
				}
			},
		},
		{
			name:        "already configured with same path is idempotent",
			gistPath:    testGistPath,
			existing:    "[mcp_servers.gist]\ncommand = \"/usr/local/bin/gist\"\nargs = [\"serve\"]\n",
			wantChanged: false,
		},
		{
			name:        "uninstall removes gist section",
			gistPath:    testGistPath,
			uninstall:   true,
			existing:    "[mcp_servers.other]\ncommand = \"/usr/bin/other\"\n[mcp_servers.gist]\ncommand = \"/usr/local/bin/gist\"\nargs = [\"serve\"]\n",
			wantChanged: true,
			check: func(t *testing.T, path string) {
				content := readFile(t, path)
				if strings.Contains(content, "[mcp_servers.gist]") {
					t.Error("gist section was not removed")
				}
				if !strings.Contains(content, "[mcp_servers.other]") {
					t.Error("other section was removed")
				}
			},
		},
		{
			name:        "uninstall when not present is no-op",
			gistPath:    testGistPath,
			uninstall:   true,
			existing:    "[mcp_servers.other]\ncommand = \"/usr/bin/other\"\n",
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")

			if tt.existing != "" {
				if err := os.WriteFile(path, []byte(tt.existing), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			changed, err := configureMCPTOML(path, tt.gistPath, tt.uninstall, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if tt.check != nil {
				tt.check(t, path)
			}
		})
	}
}

func TestConfigureMCP(t *testing.T) {
	t.Run("routes JSON to configureMCPJSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "mcp.json")

		changed, err := configureMCP(path, "mcpServers", testGistPath, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected changed=true for fresh JSON install")
		}
		data := readJSONFile(t, path)
		servers := jsonObj(t, data, "mcpServers")
		if _, ok := servers["gist"]; !ok {
			t.Error("gist entry not found")
		}
	})

	t.Run("routes TOML to configureMCPTOML", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")

		changed, err := configureMCP(path, "mcp_servers", testGistPath, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected changed=true for fresh TOML install")
		}
		content := readFile(t, path)
		if !strings.Contains(content, "[mcp_servers.gist]") {
			t.Error("gist section not found in TOML")
		}
	})
}

// helpers

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshalling %s: %v", path, err)
	}
	return data
}

func jsonObj(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := parent[key]
	if !ok {
		t.Fatalf("key %q not found", key)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("key %q is not an object", key)
	}
	return m
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}
