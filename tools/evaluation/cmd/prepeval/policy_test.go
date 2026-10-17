package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/tools/evaluation/fixture"
)

func reviewedPolicy(t *testing.T) (taskPolicy, []byte) {
	t.Helper()
	p, err := policyTemplate()
	if err != nil {
		t.Fatal(err)
	}
	// This independent test author admission is not generated from corpus/gold.
	p.Entries[0].IntentSHA256 = fixture.Hash([]byte("zati/inbox-briefing Summarize unread inbox email")).Value
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, raw
}
func TestExactPolicyScope(t *testing.T) {
	p, raw := reviewedPolicy(t)
	loaded, err := decodePolicy(raw, fixture.Hash(raw).Value)
	if err != nil {
		t.Fatal(err)
	}
	e := p.Entries[0]
	q := preparation.TaskAdmissionRequest{Intent: "zati/inbox-briefing Summarize unread inbox email", Principal: e.Principal, RuntimeID: e.RuntimeID, RecipeVersion: e.RecipeVersion, Skill: e.Skill, RequiredCapabilities: e.RequiredCapabilities, ScopeDigest: "scope"}
	decision, err := loaded.AdmitTask(context.Background(), q)
	if err != nil || !decision.Admitted {
		t.Fatalf("not admitted: %+v %v", decision, err)
	}
	for _, field := range []string{"intent", "subject", "scope", "generation", "workspace", "runtime", "recipe", "digest", "manifest", "capability"} {
		t.Run(field, func(t *testing.T) {
			changed := q
			changed.RequiredCapabilities = append(changed.RequiredCapabilities[:0:0], q.RequiredCapabilities...)
			switch field {
			case "intent":
				changed.Intent += "!"
			case "subject":
				changed.Principal.Subject += "!"
			case "scope":
				changed.Principal.Scopes = []string{"extra"}
			case "generation":
				changed.Principal.PolicyGeneration++
			case "workspace":
				changed.Principal.WorkspaceID += "!"
			case "runtime":
				changed.RuntimeID += "!"
			case "recipe":
				changed.RecipeVersion += "!"
			case "digest":
				changed.Skill.Digest.Value = "tamper"
			case "manifest":
				changed.Skill.ManifestDigest.Value = "tamper"
			case "capability":
				changed.RequiredCapabilities[0].Version = "2.0.0"
			}
			d, err := loaded.AdmitTask(context.Background(), changed)
			if err != nil || d.Admitted {
				t.Fatalf("accepted changed %s", field)
			}
		})
	}
}
func TestPolicyFileFailClosed(t *testing.T) {
	_, raw := reviewedPolicy(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sha := fixture.Hash(raw).Value
	if _, err := loadPolicy(path, sha); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPolicy(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("tampered digest accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPolicy(link, sha); err == nil {
		t.Fatal("symlink accepted")
	}
	for _, bad := range [][]byte{[]byte(`{"schema":"unknown"}`), append(raw, []byte(` {}`)...), []byte(strings.Replace(string(raw), `"schema":`, `"expected_status":"ready","schema":`, 1)), []byte(strings.Repeat(" ", maxPolicyBytes+1))} {
		if _, err := decodePolicy(bad, fixture.Hash(bad).Value); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
}
func TestCLIHelper(t *testing.T) {
	if os.Getenv("GIST_PREPEVAL_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("prepeval", flag.ContinueOnError)
	if err := run(); err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
		os.Exit(2)
	}
	os.Exit(0)
}
func cli(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestCLIHelper", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "GIST_PREPEVAL_HELPER=1")
	return cmd.CombinedOutput()
}
func TestCLIExternalPolicyAndConfiguration(t *testing.T) {
	_, raw := reviewedPolicy(t)
	dir := t.TempDir()
	policy := filepath.Join(dir, "policy.json")
	inputs := filepath.Join(dir, "inputs.jsonl")
	if err := os.WriteFile(policy, raw, 0600); err != nil {
		t.Fatal(err)
	}
	input := `{"split":"development","id":"independent-test","family":"test","stratum":"test","intent":"zati/inbox-briefing Summarize unread inbox email","fixture":{"status":"ready"}}` + "\n"
	if err := os.WriteFile(inputs, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	sha := fixture.Hash(raw).Value
	for _, k := range []string{"4", "8", "16"} {
		out, err := cli(t, "--source", strings.Repeat("a", 40), "--inputs", inputs, "--task-policy", policy, "--task-policy-sha256", sha, "--top-k", k)
		if err != nil {
			t.Fatalf("CLI: %v %s", err, out)
		}
		var row attempt
		if err := json.Unmarshal([]byte(strings.Split(string(out), "\n")[0]), &row); err != nil {
			t.Fatal(err)
		}
		if row.Status != "ready" || row.PolicyVersion != "sha256:"+sha {
			t.Fatalf("external policy not used: %+v", row)
		}
	}
	// Default two literals do not admit this independent request.
	out, err := cli(t, "--source", strings.Repeat("a", 40), "--inputs", inputs)
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	var denied attempt
	if err := json.Unmarshal([]byte(strings.Split(string(out), "\n")[0]), &denied); err != nil {
		t.Fatal(err)
	}
	if denied.Status != "requires_selection" {
		t.Fatalf("default broadened: %+v", denied)
	}
	// Failure must occur before touching this nonexistent held-out input.
	for _, args := range [][]string{{"--top-k", "7"}, {"--task-policy", policy, "--task-policy-sha256", strings.Repeat("0", 64)}} {
		out, err := cli(t, append([]string{"--source", strings.Repeat("a", 40), "--phase", "heldout", "--inputs", filepath.Join(dir, "DO_NOT_OPEN")}, args...)...)
		if err == nil || strings.Contains(string(out), "DO_NOT_OPEN") {
			t.Fatalf("invalid configuration read input: %s", out)
		}
	}
}
