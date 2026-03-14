package main

import (
	"testing"
	"time"
)

func TestGenerateDoc(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"small", 500},
		{"medium", 5000},
		{"large", 50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := generateDoc(tt.size)
			if len(doc) > tt.size {
				t.Errorf("generateDoc(%d) produced %d bytes, want <= %d", tt.size, len(doc), tt.size)
			}
			// Allow some slack — should be at least 80% of target.
			minSize := tt.size * 80 / 100
			if len(doc) < minSize {
				t.Errorf("generateDoc(%d) produced %d bytes, want >= %d", tt.size, len(doc), minSize)
			}
		})
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		values []time.Duration
		p      float64
		want   time.Duration
	}{
		{
			name:   "empty",
			values: nil,
			p:      0.5,
			want:   0,
		},
		{
			name:   "single",
			values: []time.Duration{10 * time.Millisecond},
			p:      0.99,
			want:   10 * time.Millisecond,
		},
		{
			name:   "p50 of ten",
			values: []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			p:      0.50,
			want:   6,
		},
		{
			name:   "p95 of twenty",
			values: func() []time.Duration {
				d := make([]time.Duration, 20)
				for i := range d {
					d[i] = time.Duration(i+1) * time.Millisecond
				}
				return d
			}(),
			p:    0.95,
			want: 20 * time.Millisecond,
		},
		{
			name:   "p0",
			values: []time.Duration{5, 10, 15},
			p:      0.0,
			want:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.values, tt.p)
			if got != tt.want {
				t.Errorf("percentile(%v, %.2f) = %v, want %v", tt.values, tt.p, got, tt.want)
			}
		})
	}
}

func TestGenerateQuery(t *testing.T) {
	for i := 0; i < 50; i++ {
		q := generateQuery()
		if q == "" {
			t.Fatal("generateQuery() returned empty string")
		}
	}
}
