package runtime

import (
	"os/exec"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		lang      string
		wantBin   string // expected binary name (basename)
		wantArgs  []string
		wantErr   bool
		skipCheck string // binary to check availability
	}{
		{
			name:      "sh",
			lang:      "sh",
			wantBin:   "sh",
			skipCheck: "sh",
		},
		{
			name:      "bash alias",
			lang:      "bash",
			wantBin:   "sh",
			skipCheck: "sh",
		},
		{
			name:      "shell alias",
			lang:      "shell",
			wantBin:   "sh",
			skipCheck: "sh",
		},
		{
			name:      "python",
			lang:      "python",
			wantBin:   "python3",
			skipCheck: "python3",
		},
		{
			name:      "python3",
			lang:      "python3",
			wantBin:   "python3",
			skipCheck: "python3",
		},
		{
			name:      "go",
			lang:      "go",
			wantBin:   "go",
			wantArgs:  []string{"run"},
			skipCheck: "go",
		},
		{
			name:      "node",
			lang:      "node",
			wantBin:   "node",
			skipCheck: "node",
		},
		{
			name:      "javascript alias",
			lang:      "javascript",
			wantBin:   "node",
			skipCheck: "node",
		},
		{
			name:      "js alias",
			lang:      "js",
			wantBin:   "node",
			skipCheck: "node",
		},
		{
			name:    "unsupported language",
			lang:    "ruby",
			wantErr: true,
		},
		{
			name:    "empty language",
			lang:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipCheck != "" {
				if _, err := exec.LookPath(tt.skipCheck); err != nil {
					t.Skipf("%s not available on PATH", tt.skipCheck)
				}
			}

			bin, args, err := Detect(tt.lang)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if bin == "" {
				t.Fatal("binary path is empty")
			}

			// Verify args match.
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tt.wantArgs)
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Fatalf("args[%d] = %q, want %q", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}
