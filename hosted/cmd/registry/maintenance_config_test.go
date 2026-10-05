package main

import (
	"strings"
	"testing"
)

func TestMaintenanceTargetsRejectAmbiguousAuthority(t *testing.T) {
	for _, raw := range []string{
		`[{"workspace_id":"first","workspace_id":"other","subject":"actor"}]`,
		`[{"workspace_id":"first","subject":"first","subject":"other"}]`,
		`[{"workspace_id":"first","WORKSPACE_ID":"other","subject":"actor"}]`,
		`[{"workspace_id":"first","subject":"private-marker","unknown":true}]`,
		`[{"workspace_id":"first","subject":"actor"}] {}`,
		`null`, `[]`,
	} {
		_, err := maintenanceTargets(raw)
		if err == nil || strings.Contains(err.Error(), "private-marker") {
			t.Errorf("ambiguous maintenance authority accepted or leaked: %v", err)
		}
	}
}
func TestMaintenanceTargetsAcceptCanonicalPairs(t *testing.T) {
	got, err := maintenanceTargets(`[{"workspace_id":"first","subject":"actor"}]`)
	if err != nil || len(got) != 1 || got[0].WorkspaceID != "first" || got[0].Subject != "actor" {
		t.Fatalf("valid target rejected: %v", err)
	}
}
