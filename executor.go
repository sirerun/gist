package gist

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirerun/gist/internal/runtime"
)

// ExecResult holds the output of a subprocess execution.
type ExecResult struct {
	// Stdout is the captured standard output.
	Stdout string
	// Stderr is the captured standard error.
	Stderr string
	// ExitCode is the process exit code.
	ExitCode int
	// Duration is the wall-clock time the execution took.
	Duration time.Duration
}

// Executor runs code snippets as subprocesses in a controlled environment.
type Executor struct {
	projectRoot string
	timeout     time.Duration
	policy      *Policy
}

// ExecutorOption configures an Executor.
type ExecutorOption func(*Executor)

// WithTimeout sets the maximum execution duration.
func WithTimeout(d time.Duration) ExecutorOption {
	return func(e *Executor) {
		if d > 0 {
			e.timeout = d
		}
	}
}

// WithPolicy sets the security policy for the executor.
func WithPolicy(p *Policy) ExecutorOption {
	return func(e *Executor) {
		e.policy = p
	}
}

// NewExecutor creates an Executor rooted at projectRoot.
// If no policy is provided, DefaultPolicy is used.
func NewExecutor(projectRoot string, opts ...ExecutorOption) *Executor {
	e := &Executor{
		projectRoot: projectRoot,
		timeout:     30 * time.Second,
		policy:      DefaultPolicy(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute writes code to a temporary file in the project root, determines
// the appropriate runtime, and runs the code as a subprocess. It uses
// exec.Command with array arguments — no shell interpolation.
func (e *Executor) Execute(ctx context.Context, lang string, code string) (*ExecResult, error) {
	// Validate against policy.
	if err := e.policy.Check(lang, code); err != nil {
		return nil, err
	}

	// Determine runtime.
	binPath, baseArgs, err := runtime.Detect(lang)
	if err != nil {
		return nil, err
	}

	// Choose temp file extension.
	ext := langExtension(lang)

	// Write code to temp file inside the project root.
	tmpFile, err := os.CreateTemp(e.projectRoot, "gist-exec-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("executor: failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("executor: failed to write code: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("executor: failed to close temp file: %w", err)
	}

	// Ensure the temp file is within project root (path safety).
	absRoot, err := filepath.Abs(e.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("executor: failed to resolve project root: %w", err)
	}
	absTmp, err := filepath.Abs(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("executor: failed to resolve temp path: %w", err)
	}
	if _, err := filepath.Rel(absRoot, absTmp); err != nil {
		return nil, fmt.Errorf("executor: temp file escapes project root: %w", err)
	}

	// Build command with array args (no shell interpolation).
	args := make([]string, 0, len(baseArgs)+1)
	args = append(args, baseArgs...)
	args = append(args, absTmp)

	// Apply timeout.
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, binPath, args...)
	cmd.Dir = e.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return &ExecResult{
				Stdout:   truncateOutput(stdout.String(), e.policy.MaxOutputBytes),
				Stderr:   "execution timed out",
				ExitCode: -1,
				Duration: duration,
			}, nil
		} else if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("executor: failed to run command: %w", runErr)
		}
	}

	return &ExecResult{
		Stdout:   truncateOutput(stdout.String(), e.policy.MaxOutputBytes),
		Stderr:   truncateOutput(stderr.String(), e.policy.MaxOutputBytes),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

func langExtension(lang string) string {
	switch lang {
	case "sh", "bash", "shell":
		return ".sh"
	case "python", "python3":
		return ".py"
	case "go":
		return ".go"
	case "node", "javascript", "js":
		return ".js"
	default:
		return ".tmp"
	}
}

func truncateOutput(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes]
}
