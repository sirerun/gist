package otelboot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartNoopWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	shutdown, err := Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("Enabled() true without endpoint")
	}
}

func TestWrapHandlerPassthroughWhenDisabled(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := WrapHandler(inner)
	if h == nil {
		t.Fatal("nil handler")
	}
	// Identity when disabled: same handler value (no recover/otel wrapper).
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusTeapot {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestRecoverHandler(t *testing.T) {
	h := RecoverHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("x") }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != 500 {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestParseEndpoint(t *testing.T) {
	cases := []struct {
		in     string
		target string
		secure bool
		ok     bool
	}{
		{"bora-alloy:4317", "bora-alloy:4317", false, true},
		{"http://bora-alloy:4317", "bora-alloy:4317", false, true},
		{"https://collector.example:4317", "collector.example:4317", true, true},
		{"https://collector.example:4317/v1/traces", "", false, false},
		{"grpc://alloy:4317", "alloy:4317", false, true},
	}
	for _, tc := range cases {
		target, secure, err := parseEndpoint(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("%q: unexpected err %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: expected err", tc.in)
		}
		if tc.ok && (target != tc.target || secure != tc.secure) {
			t.Fatalf("%q: got %q secure=%v", tc.in, target, secure)
		}
	}
}

func TestRejectUnsafeTarget(t *testing.T) {
	if err := rejectUnsafeTarget("169.254.169.254:4317"); err == nil {
		t.Fatal("expected reject metadata IP")
	}
	if err := rejectUnsafeTarget("127.0.0.1:4317"); err != nil {
		t.Fatalf("loopback should be allowed: %v", err)
	}
	if err := rejectUnsafeTarget("bora-alloy.observability.svc:4317"); err != nil {
		t.Fatalf("cluster DNS should be allowed: %v", err)
	}
}
