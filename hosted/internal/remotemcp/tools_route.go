package remotemcp

import (
	"encoding/json"
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/url"
)

func toolRoute(name string, a map[string]any) (string, string, []byte) {
	switch name {
	case "gist_discover":
		return "/v1/discover", "POST", marshal(a)
	case "gist_resolve":
		return "/v1/resolve", "POST", marshal(a)
	case "gist_connect":
		return "/v1/connections", "POST", marshal(a)
	case "gist_get":
		if refs, ok := a["references"]; ok {
			b, _ := json.Marshal(map[string]any{"references": refs, "max_bytes": a["max_bytes"]})
			return "/v1/artifacts/batch-get", "POST", b
		}
		kind, _ := a["kind"].(string)
		id, _ := a["id"].(string)
		version, _ := a["version"].(string)
		if id == "" || version == "" {
			return "", "", nil
		}
		esc := url.PathEscape(id) + "/versions/" + url.PathEscape(version)
		switch kind {
		case "manifest", "skill":
			return "/v1/skills/" + esc, "GET", nil
		case "package":
			return "/v1/skills/" + esc + "/package", "GET", nil
		case string(ports.KindTool):
			return "/v1/tools/" + esc, "GET", nil
		case string(ports.KindCapability):
			return "/v1/capabilities/" + esc, "GET", nil
		}
	}
	return "", "", nil
}
