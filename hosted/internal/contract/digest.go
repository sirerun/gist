package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type InventoryEntry struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type"`
}

func CanonicalInventory(entries []InventoryEntry) ([]byte, error) {
	copyEntries := append([]InventoryEntry(nil), entries...)
	sort.Slice(copyEntries, func(i, j int) bool { return copyEntries[i].Path < copyEntries[j].Path })
	seen := make(map[string]struct{}, len(copyEntries))
	for _, e := range copyEntries {
		if e.Path == "" || e.Size < 0 || e.MediaType == "" {
			return nil, fmt.Errorf("invalid inventory entry %q", e.Path)
		}
		if _, ok := seen[e.Path]; ok {
			return nil, fmt.Errorf("duplicate inventory path %q", e.Path)
		}
		seen[e.Path] = struct{}{}
		if err := validatePath(e.Path); err != nil {
			return nil, err
		}
	}
	// Field order is the RFC 8785 order for this fixed object shape.
	var out []byte
	out = append(out, '[')
	for i, e := range copyEntries {
		if i > 0 {
			out = append(out, ',')
		}
		path, _ := json.Marshal(e.Path)
		media, _ := json.Marshal(e.MediaType)
		digest, _ := json.Marshal(e.SHA256)
		out = append(out, `{"media_type":`...)
		out = append(out, media...)
		out = append(out, `,"path":`...)
		out = append(out, path...)
		out = append(out, `,"sha256":`...)
		out = append(out, digest...)
		out = append(out, fmt.Sprintf(`,"size":%d}`, e.Size)...)
	}
	return append(out, ']'), nil
}

func PackageDigest(entries []InventoryEntry) (string, []byte, error) {
	b, err := CanonicalInventory(entries)
	if err != nil {
		return "", nil, err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), b, nil
}

func validSHA(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func validatePath(path string) error {
	if path[0] == '/' || path[len(path)-1] == '/' {
		return fmt.Errorf("invalid absolute or directory path %q", path)
	}
	parts := splitPath(path)
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return fmt.Errorf("invalid inventory path %q", path)
		}
	}
	return nil
}
func splitPath(s string) []string {
	var p []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '/' {
			p = append(p, s[start:i])
			start = i + 1
		}
	}
	return p
}
