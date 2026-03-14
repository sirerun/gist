package main

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"exact_KB", 1024, "1.0 KB"},
		{"fractional_KB", 1536, "1.5 KB"},
		{"exact_MB", 1048576, "1.0 MB"},
		{"fractional_MB", 1572864, "1.5 MB"},
		{"negative", -1, "0 B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBytes(tt.in)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
