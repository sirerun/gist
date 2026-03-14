package main

import (
	"context"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestCheckGoRuntime(t *testing.T) {
	r := checkGoRuntime()
	if !r.OK {
		t.Fatal("expected OK")
	}
	if r.Name != "Go runtime" {
		t.Fatalf("unexpected name: %s", r.Name)
	}
	if r.Message != runtime.Version() {
		t.Fatalf("expected %s, got %s", runtime.Version(), r.Message)
	}
}

func TestCheckBuildInfo(t *testing.T) {
	r := checkBuildInfo()
	// In tests, build info should be available.
	if r.Name != "Build info" {
		t.Fatalf("unexpected name: %s", r.Name)
	}
	// During go test, ReadBuildInfo returns true with module info.
	info, ok := debug.ReadBuildInfo()
	if ok && r.OK {
		if !strings.Contains(r.Message, info.Main.Path) {
			t.Fatalf("expected message to contain module path %q, got %q", info.Main.Path, r.Message)
		}
	}
}

func TestCheckPlatform(t *testing.T) {
	r := checkPlatform()
	if !r.OK {
		t.Fatal("expected OK")
	}
	if r.Name != "Platform" {
		t.Fatalf("unexpected name: %s", r.Name)
	}
	expected := runtime.GOOS + "/" + runtime.GOARCH
	if r.Message != expected {
		t.Fatalf("expected %s, got %s", expected, r.Message)
	}
}

func TestCheckPostgresNoDSN(t *testing.T) {
	r := checkPostgres(context.Background(), "")
	if r.OK {
		t.Fatal("expected not OK with empty DSN")
	}
	if !r.Skipped {
		t.Fatal("expected skipped with empty DSN")
	}
	if !strings.Contains(r.Message, "not configured") {
		t.Fatalf("unexpected message: %s", r.Message)
	}
}

func TestCheckPostgresBadDSN(t *testing.T) {
	r := checkPostgres(context.Background(), "postgres://invalid:5432/nonexistent?connect_timeout=1")
	if r.OK {
		t.Fatal("expected not OK with bad DSN")
	}
	if r.Skipped {
		t.Fatal("expected not skipped with bad DSN")
	}
	if !strings.Contains(r.Message, "connection failed") {
		t.Fatalf("unexpected message: %s", r.Message)
	}
}

func TestCheckPgTrgmNoDSN(t *testing.T) {
	r := checkPgTrgm(context.Background(), "")
	if r.OK {
		t.Fatal("expected not OK with empty DSN")
	}
	if !r.Skipped {
		t.Fatal("expected skipped with empty DSN")
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name     string
		result   CheckResult
		contains string
	}{
		{
			name:     "ok result",
			result:   CheckResult{Name: "Test", OK: true, Message: "good"},
			contains: "[✓] Test: good",
		},
		{
			name:     "failed result",
			result:   CheckResult{Name: "Test", OK: false, Message: "bad"},
			contains: "[✗] Test: bad",
		},
		{
			name:     "skipped result",
			result:   CheckResult{Name: "Test", Skipped: true, Message: "skip"},
			contains: "[-] Test: skip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResult(tt.result)
			if got != tt.contains {
				t.Fatalf("expected %q, got %q", tt.contains, got)
			}
		})
	}
}
