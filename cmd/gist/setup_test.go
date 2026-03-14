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

func TestSetupE2E(t *testing.T) {
	for toolName, adapter := range toolRegistry {
		adapter := adapter
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()

			mcpPath := strings.Replace(adapter.GlobalMCPPath, "~", home, 1)
			instPath := strings.Replace(adapter.GlobalInstPath, "~", home, 1)

			isJSON := !strings.HasSuffix(mcpPath, ".toml")
			hasInst := instPath != ""

			// --- Step 1: Fresh setup ---
			mcpChanged, err := configureMCP(mcpPath, adapter.MCPKey, testGistPath, false, false)
			if err != nil {
				t.Fatalf("fresh configureMCP: %v", err)
			}
			if !mcpChanged {
				t.Error("fresh configureMCP: expected changed=true")
			}

			var instChanged bool
			if hasInst {
				instChanged, err = configureInstructions(instPath, adapter.InstSentinel, false, false)
				if err != nil {
					t.Fatalf("fresh configureInstructions: %v", err)
				}
				if !instChanged {
					t.Error("fresh configureInstructions: expected changed=true")
				}
			}

			// Verify MCP file content
			if isJSON {
				data := readJSONFile(t, mcpPath)
				servers := jsonObj(t, data, adapter.MCPKey)
				gist := jsonObj(t, servers, "gist")
				if gist["command"] != testGistPath {
					t.Errorf("MCP command = %v, want %v", gist["command"], testGistPath)
				}
				args, ok := gist["args"].([]any)
				if !ok || len(args) != 1 || args[0] != "serve" {
					t.Errorf("MCP args = %v, want [serve]", gist["args"])
				}
			} else {
				content := readFile(t, mcpPath)
				for _, want := range []string{
					"[mcp_servers.gist]",
					`command = "` + testGistPath + `"`,
					`args = ["serve"]`,
				} {
					if !strings.Contains(content, want) {
						t.Errorf("TOML missing %q", want)
					}
				}
			}

			// Verify instructions file content
			if hasInst {
				content := readFile(t, instPath)
				if !strings.Contains(content, adapter.InstSentinel) {
					t.Errorf("instructions missing sentinel %q", adapter.InstSentinel)
				}
				if !strings.Contains(content, "gist_index") {
					t.Error("instructions missing gist_index reference")
				}
			}

			// --- Step 2: Idempotent re-run ---
			mcpChanged, err = configureMCP(mcpPath, adapter.MCPKey, testGistPath, false, false)
			if err != nil {
				t.Fatalf("idempotent configureMCP: %v", err)
			}
			if mcpChanged {
				t.Error("idempotent configureMCP: expected changed=false")
			}

			if hasInst {
				instChanged, err = configureInstructions(instPath, adapter.InstSentinel, false, false)
				if err != nil {
					t.Fatalf("idempotent configureInstructions: %v", err)
				}
				if instChanged {
					t.Error("idempotent configureInstructions: expected changed=false")
				}
			}

			// --- Step 3: Uninstall ---
			mcpChanged, err = configureMCP(mcpPath, adapter.MCPKey, testGistPath, true, false)
			if err != nil {
				t.Fatalf("uninstall configureMCP: %v", err)
			}
			if !mcpChanged {
				t.Error("uninstall configureMCP: expected changed=true")
			}

			if hasInst {
				instChanged, err = configureInstructions(instPath, adapter.InstSentinel, true, false)
				if err != nil {
					t.Fatalf("uninstall configureInstructions: %v", err)
				}
				if !instChanged {
					t.Error("uninstall configureInstructions: expected changed=true")
				}
			}

			// Verify gist entry removed from MCP
			if isJSON {
				data := readJSONFile(t, mcpPath)
				servers := jsonObj(t, data, adapter.MCPKey)
				if _, ok := servers["gist"]; ok {
					t.Error("uninstall: gist entry still present in MCP JSON")
				}
			} else {
				content := readFile(t, mcpPath)
				if strings.Contains(content, "[mcp_servers.gist]") {
					t.Error("uninstall: gist section still present in TOML")
				}
			}

			// Verify instructions removed
			if hasInst {
				content := readFile(t, instPath)
				if strings.Contains(content, adapter.InstSentinel) {
					t.Error("uninstall: instructions sentinel still present")
				}
			}

			// Codex: verify no instructions file involvement
			if !hasInst {
				if _, err := os.Stat(instPath); instPath == "" {
					// expected: no instructions path for codex
				} else if !os.IsNotExist(err) && err != nil {
					t.Errorf("unexpected instructions file state: %v", err)
				}
			}
		})
	}
}
