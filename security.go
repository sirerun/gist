package gist

import (
	"fmt"
	"strings"
)

// Policy defines execution constraints for the polyglot executor.
type Policy struct {
	// AllowedCommands restricts which top-level commands may appear in code.
	// An empty list means all commands are allowed (subject to DeniedPatterns).
	AllowedCommands []string

	// DeniedPatterns lists substrings that must not appear in code.
	DeniedPatterns []string

	// MaxOutputBytes caps the combined stdout+stderr captured from execution.
	// Zero means no limit.
	MaxOutputBytes int
}

// DefaultPolicy returns a Policy that blocks obviously dangerous patterns.
func DefaultPolicy() *Policy {
	return &Policy{
		DeniedPatterns: []string{
			"rm -rf /",
			"rm -rf /*",
			"sudo ",
			"chmod 777",
			"mkfs.",
			":(){:|:&};:",
			"> /dev/sda",
			"dd if=/dev/zero",
			"curl | sh",
			"curl | bash",
			"wget | sh",
			"wget | bash",
		},
		MaxOutputBytes: 1 << 20, // 1 MB
	}
}

// Check validates code against the policy before execution.
// It returns an error if the code violates any policy constraint.
func (p *Policy) Check(lang string, code string) error {
	if p == nil {
		return nil
	}

	lower := strings.ToLower(code)

	// Check denied patterns.
	for _, pattern := range p.DeniedPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return fmt.Errorf("policy: code contains denied pattern %q", pattern)
		}
	}

	// Check allowed commands (only for shell languages).
	if len(p.AllowedCommands) > 0 && isShellLang(lang) {
		firstWord := firstToken(code)
		if firstWord != "" {
			allowed := false
			for _, cmd := range p.AllowedCommands {
				if firstWord == cmd {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("policy: command %q is not in the allowed list", firstWord)
			}
		}
	}

	return nil
}

func isShellLang(lang string) bool {
	switch lang {
	case "sh", "bash", "shell":
		return true
	}
	return false
}

func firstToken(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	fields := strings.Fields(code)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
