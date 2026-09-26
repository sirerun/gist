//go:build integration

package wiring

import "testing"

func TestC1HTTPAndRemoteMCPBoundaries(t *testing.T) {
	requireFixture(t)
	t.Fatal("configured boundary runner is unavailable in this sandbox")
}
