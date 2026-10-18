package essentials_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirerun/gist/catalog/packages"
	"github.com/sirerun/gist/catalog/skills/essentials"
)

func TestCompleteScreeningPackages(t *testing.T) {
	if len(essentials.List()) != 5 {
		t.Fatal("expected five candidates")
	}
	for _, workflow := range essentials.List() {
		t.Run(workflow.Key, func(t *testing.T) {
			a, err := essentials.Archive(workflow.Key)
			if err != nil {
				t.Fatal(err)
			}
			b, err := essentials.Archive(workflow.Key)
			if err != nil || !bytes.Equal(a, b) {
				t.Fatal("archive nondeterministic", err)
			}
			archive, err := packages.ReadArchive(a, packages.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := packages.Validate(archive, packages.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			if pkg.Manifest.ID != workflow.SkillID || pkg.Manifest.Version != essentials.Version || pkg.Manifest.Publication.Status != "screening" {
				t.Fatal("candidate identity/status mismatch")
			}
			if len(pkg.Files) != 4 || len(pkg.Manifest.Inventory) != 3 {
				t.Fatal("incomplete package")
			}
			if err := packages.ScreenText(pkg); err != nil {
				t.Fatal(err)
			}
			var requirements struct {
				Format, Status string
				BindingStatus  string `json:"binding_status"`
				Requirements   []struct {
					ID            string `json:"id"`
					Version       string `json:"contract_version"`
					Status        string `json:"status"`
					BindingStatus string `json:"binding_status"`
					Class         string `json:"effect_class"`
				}
				Limits struct {
					MaxCalls  int `json:"max_calls"`
					MaxOutput int `json:"max_output_bytes"`
					Duration  int `json:"max_duration_seconds"`
					Retries   int `json:"automatic_retries"`
				}
			}
			if err := json.Unmarshal(pkg.Files["assets/requirements.json"], &requirements); err != nil {
				t.Fatal(err)
			}
			if requirements.Format != "gist.workflow-requirements.draft/v1" || requirements.Status != "draft" || requirements.BindingStatus != "not_implemented" || len(requirements.Requirements) == 0 {
				t.Fatal("draft boundary missing")
			}
			for _, r := range requirements.Requirements {
				if !strings.HasPrefix(r.ID, "draft/") || r.Version != essentials.DraftContractVersion || r.Status != "draft" || r.BindingStatus != "not_implemented" || r.Class != "read_only" {
					t.Fatal("unsafe or undocumented requirement")
				}
			}
			if requirements.Limits.MaxCalls < 1 || requirements.Limits.MaxCalls > 21 || requirements.Limits.MaxOutput < 1 || requirements.Limits.Duration != 120 || requirements.Limits.Retries != 0 {
				t.Fatal("workflow budgets missing")
			}
			pkg.Files["assets/outcome-template.md"][0] = '!'
			if _, err := packages.Validate(archive, packages.DefaultLimits()); err == nil {
				t.Fatal("mutated asset accepted")
			}
			fresh, err := essentials.Files(workflow.Key)
			if err != nil || fresh["assets/outcome-template.md"][0] == '!' {
				t.Fatal("embedded source mutated")
			}
		})
	}
}
func TestUnknownWorkflowCannotEscapePackage(t *testing.T) {
	for _, key := range []string{"", "../inboxbriefing", "sources/inbox-briefing", "/tmp/skill"} {
		if _, err := essentials.Files(key); err == nil {
			t.Fatal("unknown package accepted")
		}
	}
}
