package remotemcp

var toolNames = []string{"gist_discover", "gist_get", "gist_resolve", "gist_connect"}

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{"name": "gist_discover", "description": "Find authorized Gist registry candidates.", "inputSchema": map[string]any{"type": "object"}},
		{"name": "gist_get", "description": "Retrieve a complete exact-version registry artifact or bounded batch.", "inputSchema": map[string]any{"type": "object"}},
		{"name": "gist_resolve", "description": "Resolve a pinned skill's capabilities.", "inputSchema": map[string]any{"type": "object"}},
		{"name": "gist_connect", "description": "Start or inspect a runtime-owned connection.", "inputSchema": map[string]any{"type": "object"}},
	}
}
