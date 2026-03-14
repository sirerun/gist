package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type toolAdapter struct {
	Name            string
	DisplayName     string
	GlobalMCPPath   string
	ProjectMCPPath  string
	MCPKey          string
	GlobalInstPath  string
	ProjectInstPath string
	InstSentinel    string
}

var toolRegistry = map[string]toolAdapter{
	"claude": {
		Name:            "claude",
		DisplayName:     "Claude Code",
		GlobalMCPPath:   "~/.claude/mcp.json",
		ProjectMCPPath:  ".mcp.json",
		MCPKey:          "mcpServers",
		GlobalInstPath:  "~/.claude/CLAUDE.md",
		ProjectInstPath: "CLAUDE.md",
		InstSentinel:    "## Gist Context Management",
	},
	"gemini": {
		Name:            "gemini",
		DisplayName:     "Gemini CLI",
		GlobalMCPPath:   "~/.gemini/mcp.json",
		ProjectMCPPath:  ".gemini/mcp.json",
		MCPKey:          "mcpServers",
		GlobalInstPath:  "~/.gemini/GEMINI.md",
		ProjectInstPath: "GEMINI.md",
		InstSentinel:    "## Gist Context Management",
	},
	"cursor": {
		Name:            "cursor",
		DisplayName:     "Cursor",
		GlobalMCPPath:   "~/.cursor/mcp.json",
		ProjectMCPPath:  ".cursor/mcp.json",
		MCPKey:          "mcpServers",
		GlobalInstPath:  "~/.cursor/rules/gist.mdc",
		ProjectInstPath: ".cursor/rules/gist.mdc",
		InstSentinel:    "## Gist Context Management",
	},
	"copilot": {
		Name:            "copilot",
		DisplayName:     "VS Code Copilot",
		GlobalMCPPath:   "~/.vscode/mcp.json",
		ProjectMCPPath:  ".vscode/mcp.json",
		MCPKey:          "servers",
		GlobalInstPath:  "~/.github/copilot-instructions.md",
		ProjectInstPath: ".github/copilot-instructions.md",
		InstSentinel:    "## Gist Context Management",
	},
	"codex": {
		Name:            "codex",
		DisplayName:     "Codex CLI",
		GlobalMCPPath:   "~/.codex/config.toml",
		ProjectMCPPath:  ".codex/config.toml",
		MCPKey:          "mcp_servers",
		GlobalInstPath:  "",
		ProjectInstPath: "",
		InstSentinel:    "## Gist Context Management",
	},
}

const gistInstructions = `## Gist Context Management

Use the gist MCP tools (gist_index, gist_search, gist_stats) to manage context efficiently:

- When reading files over 5KB or receiving tool output over 5KB, index the content with gist_index (set a descriptive source label like the file path).
- Instead of re-reading indexed files, use gist_search to retrieve only the relevant snippets.
- When exploring a codebase (reading multiple files, grepping across directories), index results into gist and search across them.
- After completing a task, call gist_stats and briefly report bytes saved (e.g., "Gist: indexed 48.2 KB, returned 3.1 KB, saved 93.6%").
`

var setupCmd = &cobra.Command{
	Use:   "setup [tool]",
	Short: "Configure gist for an agentic coding tool",
	Long:  "Configure gist as an MCP server and add context management instructions for a supported agentic coding tool.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			names := make([]string, 0, len(toolRegistry))
			for name := range toolRegistry {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Fprintln(os.Stderr, "Supported tools:")
			for _, name := range names {
				adapter := toolRegistry[name]
				fmt.Fprintf(os.Stderr, "  %-10s %s\n", name, adapter.DisplayName)
			}
			fmt.Fprintln(os.Stderr, "\nUsage: gist setup <tool>")
			return nil
		}
		if len(args) > 1 {
			return fmt.Errorf("expected exactly one tool argument, got %d", len(args))
		}

		toolName := strings.ToLower(args[0])
		adapter, ok := toolRegistry[toolName]
		if !ok {
			names := make([]string, 0, len(toolRegistry))
			for name := range toolRegistry {
				names = append(names, name)
			}
			sort.Strings(names)
			return fmt.Errorf("unknown tool %q, supported tools: %s", toolName, strings.Join(names, ", "))
		}

		fmt.Fprintf(os.Stderr, "Setting up %s...\n", adapter.DisplayName)
		return nil
	},
}

func init() {
	setupCmd.Flags().Bool("project", false, "Configure for the current project instead of globally")
	setupCmd.Flags().Bool("uninstall", false, "Remove gist configuration")
	setupCmd.Flags().Bool("dry-run", false, "Preview changes without writing files")
	rootCmd.AddCommand(setupCmd)
}

func configureInstructions(path string, sentinel string, uninstall bool, dryRun bool) (changed bool, err error) {
	if path == "" {
		return false, nil
	}

	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("reading instructions file: %w", err)
	}
	text := string(content)

	if uninstall {
		idx := strings.Index(text, sentinel)
		if idx == -1 {
			return false, nil
		}
		before := text[:idx]
		after := text[idx+len(sentinel):]
		// Find the next ## heading after the sentinel
		nextHeading := strings.Index(after, "\n## ")
		if nextHeading != -1 {
			// Keep from the next heading onward (skip the newline before ##)
			after = after[nextHeading+1:]
		} else {
			after = ""
		}
		result := strings.TrimRight(before+after, "\n")
		if result != "" {
			result += "\n"
		}
		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] Would write to %s:\n%s", path, result)
			return true, nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return false, fmt.Errorf("creating directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
			return false, fmt.Errorf("writing instructions file: %w", err)
		}
		return true, nil
	}

	// Install
	if strings.Contains(text, sentinel) {
		return false, nil
	}
	result := text
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	result += "\n" + gistInstructions
	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] Would write to %s:\n%s", path, result)
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("creating directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
		return false, fmt.Errorf("writing instructions file: %w", err)
	}
	return true, nil
}

// configureMCP creates, updates, or removes the gist MCP server entry in a
// tool's config file. It handles both JSON format (claude, gemini, cursor,
// copilot) and TOML format (codex).
func configureMCP(path string, mcpKey string, gistPath string, uninstall bool, dryRun bool) (changed bool, err error) {
	if strings.HasSuffix(path, ".toml") {
		return configureMCPTOML(path, gistPath, uninstall, dryRun)
	}
	return configureMCPJSON(path, mcpKey, gistPath, uninstall, dryRun)
}

func configureMCPJSON(path string, mcpKey string, gistPath string, uninstall bool, dryRun bool) (bool, error) {
	var data map[string]any

	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		data = map[string]any{mcpKey: map[string]any{}}
	} else {
		if err := json.Unmarshal(raw, &data); err != nil {
			return false, fmt.Errorf("invalid JSON in %s: %w", path, err)
		}
	}

	serversRaw, ok := data[mcpKey]
	if !ok {
		serversRaw = map[string]any{}
		data[mcpKey] = serversRaw
	}
	servers, ok := serversRaw.(map[string]any)
	if !ok {
		return false, fmt.Errorf("expected %s to be an object in %s", mcpKey, path)
	}

	if uninstall {
		if _, exists := servers["gist"]; !exists {
			return false, nil
		}
		delete(servers, "gist")
	} else {
		if existing, exists := servers["gist"]; exists {
			if m, ok := existing.(map[string]any); ok {
				existingCmd, _ := m["command"].(string)
				existingArgs, _ := m["args"].([]any)
				if existingCmd == gistPath && len(existingArgs) == 1 {
					if arg, ok := existingArgs[0].(string); ok && arg == "serve" {
						return false, nil
					}
				}
			}
		}
		servers["gist"] = map[string]any{
			"command": gistPath,
			"args":    []string{"serve"},
		}
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] Would write to %s:\n%s", path, out)
		return true, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o644)
}

func configureMCPTOML(path string, gistPath string, uninstall bool, dryRun bool) (bool, error) {
	const sectionHeader = "[mcp_servers.gist]"

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	content := string(raw)

	hasSectionIdx := strings.Index(content, sectionHeader)
	hasSection := hasSectionIdx >= 0

	if uninstall {
		if !hasSection {
			return false, nil
		}
		start := hasSectionIdx
		rest := content[start+len(sectionHeader):]
		end := len(content)
		lines := strings.Split(rest, "\n")
		offset := start + len(sectionHeader)
		for i, line := range lines {
			if i == 0 {
				offset += len(line) + 1
				continue
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") {
				end = offset
				break
			}
			offset += len(line) + 1
		}
		newContent := content[:start] + content[end:]
		newContent = strings.TrimRight(newContent, "\n")
		if newContent != "" {
			newContent += "\n"
		}

		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] Would write to %s:\n%s", path, newContent)
			return true, nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return false, err
		}
		return true, os.WriteFile(path, []byte(newContent), 0o644)
	}

	// Install
	if hasSection {
		expectedLine := fmt.Sprintf("command = %q", gistPath)
		if strings.Contains(content, expectedLine) {
			return false, nil
		}
		// Different path: remove old section, then re-add.
		if _, err := configureMCPTOML(path, gistPath, true, false); err != nil {
			return false, err
		}
		raw, err = os.ReadFile(path)
		if err != nil {
			return false, err
		}
		content = string(raw)
	}

	section := fmt.Sprintf("%s\ncommand = %q\nargs = [\"serve\"]\n", sectionHeader, gistPath)
	newContent := content
	if newContent != "" && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += section

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] Would write to %s:\n%s", path, newContent)
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(newContent), 0o644)
}
