package app

import (
	"strings"
	"testing"
)

func TestMaintenanceTargetsRejectInvalidUnicodeAuthority(t *testing.T) {
	cases := [][]byte{
		[]byte(`[{"workspace_id":"workspace","subject":"\ud800"}]`),
		[]byte(`[{"workspace_id":"workspace","subject":"\udc00"}]`),
		[]byte(`[{"workspace_id":"workspace","subject":"\ud800x"}]`),
		[]byte(`[{"workspace_id":"workspace","subject":"\udc00\ud800"}]`),
		append(append([]byte(`[{"workspace_id":"workspace","subject":"`), 0xff), []byte(`"}]`)...),
	}
	for _, raw := range cases {
		if _, err := DecodeMaintenanceTargets(raw); err == nil {
			t.Error("malformed Unicode authority accepted")
		}
	}
}
func TestMaintenanceTargetsAcceptValidUnicodeAndLiteralEscapes(t *testing.T) {
	for _, raw := range []string{`[{"workspace_id":"workspace","subject":"\ud83d\ude00"}]`, `[{"workspace_id":"workspace","subject":"\\ud800"}]`} {
		got, err := DecodeMaintenanceTargets([]byte(raw))
		if err != nil || len(got) != 1 || strings.ContainsRune(got[0].Subject, '\ufffd') {
			t.Fatalf("valid Unicode authority rejected: %v", err)
		}
	}
}
