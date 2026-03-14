package gist

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name       string
		lang       string
		code       string
		wantStdout string
		wantExit   int
		wantErr    string
		skipCheck  string
	}{
		{
			name:       "sh echo",
			lang:       "sh",
			code:       "echo hello",
			wantStdout: "hello",
			skipCheck:  "sh",
		},
		{
			name:       "bash alias",
			lang:       "bash",
			code:       "echo from-bash",
			wantStdout: "from-bash",
			skipCheck:  "sh",
		},
		{
			name:       "shell alias",
			lang:       "shell",
			code:       "echo from-shell",
			wantStdout: "from-shell",
			skipCheck:  "sh",
		},
		{
			name:      "sh exit code",
			lang:      "sh",
			code:      "exit 42",
			wantExit:  42,
			skipCheck: "sh",
		},
		{
			name:    "unsupported language",
			lang:    "ruby",
			code:    "puts 'hi'",
			wantErr: "unsupported language",
		},
		{
			name:    "policy violation",
			lang:    "sh",
			code:    "sudo rm -rf /",
			wantErr: "denied pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipCheck != "" {
				if _, err := exec.LookPath(tt.skipCheck); err != nil {
					t.Skipf("%s not available on PATH", tt.skipCheck)
				}
			}

			dir := t.TempDir()
			executor := NewExecutor(dir)

			result, err := executor.Execute(context.Background(), tt.lang, tt.code)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.ExitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", result.ExitCode, tt.wantExit)
			}

			if tt.wantStdout != "" {
				got := strings.TrimSpace(result.Stdout)
				if got != tt.wantStdout {
					t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
				}
			}

			if result.Duration <= 0 {
				t.Error("duration should be positive")
			}
		})
	}
}

func TestExecuteTimeout(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available on PATH")
	}

	dir := t.TempDir()
	executor := NewExecutor(dir, WithTimeout(100*time.Millisecond))

	result, err := executor.Execute(context.Background(), "sh", "sleep 5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1 for timeout", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "timed out") {
		t.Errorf("stderr = %q, want 'timed out'", result.Stderr)
	}
}

func TestExecuteWithCustomPolicy(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available on PATH")
	}

	dir := t.TempDir()

	// Policy that only allows echo.
	policy := &Policy{
		AllowedCommands: []string{"echo"},
		MaxOutputBytes:  1024,
	}
	executor := NewExecutor(dir, WithPolicy(policy))

	// Allowed command.
	result, err := executor.Execute(context.Background(), "sh", "echo allowed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "allowed" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "allowed")
	}

	// Disallowed command.
	_, err = executor.Execute(context.Background(), "sh", "ls /")
	if err == nil {
		t.Fatal("expected error for disallowed command")
	}
	if !strings.Contains(err.Error(), "not in the allowed list") {
		t.Fatalf("error %q does not mention allowed list", err.Error())
	}
}

func TestExecuteOutputTruncation(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available on PATH")
	}

	dir := t.TempDir()
	policy := &Policy{
		MaxOutputBytes: 10,
	}
	executor := NewExecutor(dir, WithPolicy(policy))

	// Generate output longer than MaxOutputBytes.
	result, err := executor.Execute(context.Background(), "sh", "echo 'this is a long output that should be truncated'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Stdout) > 10 {
		t.Errorf("stdout length = %d, want <= 10", len(result.Stdout))
	}
}

func TestNewExecutorDefaults(t *testing.T) {
	e := NewExecutor("/tmp")
	if e.timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", e.timeout)
	}
	if e.policy == nil {
		t.Error("default policy is nil")
	}
	if e.projectRoot != "/tmp" {
		t.Errorf("projectRoot = %q, want /tmp", e.projectRoot)
	}
}
