//go:build integration

package wiring

import "testing"

func TestTwoTenantIsolationOracle(t *testing.T) {
	requireFixture(t)
	t.Fatal("two-tenant PostgreSQL/object-store fixture is unavailable in this sandbox")
}
