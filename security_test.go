package gist

import (
	"strings"
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p == nil {
		t.Fatal("DefaultPolicy returned nil")
	}
	if len(p.DeniedPatterns) == 0 {
		t.Fatal("DefaultPolicy has no denied patterns")
	}
	if p.MaxOutputBytes == 0 {
		t.Fatal("DefaultPolicy has no max output bytes")
	}
}

func TestPolicyCheck(t *testing.T) {
	tests := []struct {
		name    string
		policy  *Policy
		lang    string
		code    string
		wantErr string
	}{
		{
			name:   "nil policy allows everything",
			policy: nil,
			lang:   "sh",
			code:   "rm -rf /",
		},
		{
			name:    "denied pattern rm -rf /",
			policy:  DefaultPolicy(),
			lang:    "sh",
			code:    "rm -rf /",
			wantErr: "denied pattern",
		},
		{
			name:    "denied pattern sudo",
			policy:  DefaultPolicy(),
			lang:    "sh",
			code:    "sudo apt-get install foo",
			wantErr: "denied pattern",
		},
		{
			name:    "denied pattern chmod 777",
			policy:  DefaultPolicy(),
			lang:    "bash",
			code:    "chmod 777 /etc/passwd",
			wantErr: "denied pattern",
		},
		{
			name:    "denied pattern case insensitive",
			policy:  DefaultPolicy(),
			lang:    "sh",
			code:    "SUDO apt-get update",
			wantErr: "denied pattern",
		},
		{
			name:    "denied pattern fork bomb",
			policy:  DefaultPolicy(),
			lang:    "sh",
			code:    ":(){:|:&};:",
			wantErr: "denied pattern",
		},
		{
			name:   "safe shell command",
			policy: DefaultPolicy(),
			lang:   "sh",
			code:   "echo hello world",
		},
		{
			name:   "safe python code",
			policy: DefaultPolicy(),
			lang:   "python",
			code:   "print('hello')",
		},
		{
			name:   "safe go code",
			policy: DefaultPolicy(),
			lang:   "go",
			code:   "package main\nfunc main() {}",
		},
		{
			name: "allowed commands restricts shell",
			policy: &Policy{
				AllowedCommands: []string{"echo", "cat"},
			},
			lang:    "sh",
			code:    "ls -la",
			wantErr: "not in the allowed list",
		},
		{
			name: "allowed commands permits listed command",
			policy: &Policy{
				AllowedCommands: []string{"echo", "cat"},
			},
			lang: "sh",
			code: "echo hello",
		},
		{
			name: "allowed commands not enforced for non-shell",
			policy: &Policy{
				AllowedCommands: []string{"echo"},
			},
			lang: "python",
			code: "import os; os.system('ls')",
		},
		{
			name:   "empty code",
			policy: DefaultPolicy(),
			lang:   "sh",
			code:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Check(tt.lang, tt.code)
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
		})
	}
}
