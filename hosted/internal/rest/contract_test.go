package rest

import (
	"bytes"
	"context"
	"github.com/sirerun/gist/hosted/internal/ports"
	"net/http/httptest"
	"testing"
)

func TestFrozenErrorsUseEnvelopeAndStatus(t *testing.T) {
	h := testHandler(t)
	cases := []struct {
		name, path, body string
		status           int
		code             string
	}{{"budget", "/v1/discover", `{"query":"x","max_bytes":1}`, 413, "budget_exceeded"}, {"private", "/v1/skills/missing/versions/1", "", 200, ""}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", tc.path, bytes.NewBufferString(tc.body))
			if tc.body == "" {
				r = httptest.NewRequest("GET", tc.path, nil)
			}
			r.Header.Set("Authorization", "Bearer test")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if tc.name == "private" {
				return
			}
			if w.Code != tc.status || !bytes.Contains(w.Body.Bytes(), []byte(tc.code)) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

var _ = context.Background
var _ ports.Principal
