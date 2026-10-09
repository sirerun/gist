package app

import "testing"

func TestPublicationResponseBudgetClampsClientServerAndHardLimit(t *testing.T) {
	tests := []struct {
		name   string
		client int64
		server int64
		want   int64
	}{
		{name: "client lower", client: 1024, server: 4096, want: 1024},
		{name: "server lower", client: 4096, server: 1024, want: 1024},
		{name: "hard ceiling", client: 200 << 20, server: 150 << 20, want: 100 << 20},
		{name: "disabled server limit still hard bounded", client: 200 << 20, want: 100 << 20},
		{name: "zero client budget", client: 0, server: 4096, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicationResponseBudget(tc.client, tc.server); got != tc.want {
				t.Fatalf("publicationResponseBudget(%d, %d) = %d, want %d", tc.client, tc.server, got, tc.want)
			}
		})
	}
}
