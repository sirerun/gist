package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPContractServerStatusAndErrorShape(t *testing.T) {
	server := newContractTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "budget_exceeded", "message": "bounded", "request_id": "r1", "retryable": false})
	}))
	if server == nil {
		t.Skip("sandbox does not permit httptest listeners")
	}
	t.Cleanup(server.Close)
	resp, err := http.Get(server.URL + "/v1/discover")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response: %s %s", resp.Status, resp.Header.Get("Content-Type"))
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"code", "message", "request_id", "retryable"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing error field %s", key)
		}
	}
}

func TestInvalidRequiredFieldFailsWireValidation(t *testing.T) {
	if err := ValidateResolution(ResolutionWire{Aggregate: FindingReady}); err == nil {
		t.Fatal("missing required findings accepted")
	}
}

func newContractTestServer(handler http.Handler) (server *httptest.Server) {
	defer func() {
		if recover() != nil {
			server = nil
		}
	}()
	return httptest.NewServer(handler)
}
