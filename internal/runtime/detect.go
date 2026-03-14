// Package runtime provides runtime detection for the polyglot executor.
package runtime

import (
	"fmt"
	"os/exec"
)

// Detect returns the binary path and arguments needed to execute code
// written in the given language. The returned args slice does not include
// the source file path; the caller must append it.
func Detect(lang string) (binaryPath string, args []string, err error) {
	switch lang {
	case "sh", "bash", "shell":
		p, err := exec.LookPath("sh")
		if err != nil {
			return "", nil, fmt.Errorf("runtime: sh not found: %w", err)
		}
		return p, nil, nil

	case "python", "python3":
		p, err := exec.LookPath("python3")
		if err != nil {
			return "", nil, fmt.Errorf("runtime: python3 not found: %w", err)
		}
		return p, nil, nil

	case "go":
		p, err := exec.LookPath("go")
		if err != nil {
			return "", nil, fmt.Errorf("runtime: go not found: %w", err)
		}
		return p, []string{"run"}, nil

	case "node", "javascript", "js":
		p, err := exec.LookPath("node")
		if err != nil {
			return "", nil, fmt.Errorf("runtime: node not found: %w", err)
		}
		return p, nil, nil

	default:
		return "", nil, fmt.Errorf("runtime: unsupported language %q", lang)
	}
}
