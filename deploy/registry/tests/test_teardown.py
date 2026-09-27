"""Offline regression tests for preview state, teardown and workflow wiring (O2).

Run from the repository root:

    python -m unittest discover -s deploy/registry/tests -v

The teardown tests run ``preview-destroy.sh`` against a fake ``pulumi`` on
PATH, so they need bash and jq but no Pulumi login, backend or credentials.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import stat
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path

import yaml

HERE = Path(__file__).resolve().parent
PROGRAM_DIR = HERE.parent
REPO = PROGRAM_DIR.parents[1]
WORKFLOW = REPO / ".github/workflows/registry-preview.yml"
OIDC_JOBS = {"plan", "create", "destroy", "ttl-sweep"}

FAKE_PULUMI = textwrap.dedent(
    """\
    #!/usr/bin/env bash
    printf '%s|%s\\n' "${PULUMI_BACKEND_URL:-}" "$*" >> "$FAKE/calls"
    case "$1 ${2:-}" in
      "login "*) exit 0 ;;
      "stack ls") [[ -f "$FAKE/ls_fail" ]] && exit 1; cat "$FAKE/stacks" ;;
      "stack select") exit 0 ;;
      "stack export")
        if [[ -f "$FAKE/destroyed" ]]; then echo '{"deployment":{"resources":[]}}'; else cat "$FAKE/export"; fi ;;
      "stack output") cat "$FAKE/outputs" ;;
      "destroy "*) touch "$FAKE/destroyed" ;;
      "config "*) echo "stack config is runner-local and must not be read" >&2; exit 3 ;;
      *) echo "unexpected pulumi call: $*" >&2; exit 4 ;;
    esac
    """
)

LIVE_RESOURCES = {
    "deployment": {
        "resources": [
            {"type": "pulumi:pulumi:Stack"},
            {"type": "pulumi:providers:gcp"},
            {"type": "gcp:cloudrunv2/service:Service"},
            {"type": "gcp:compute/globalAddress:GlobalAddress"},
        ]
    }
}


def load_workflow() -> dict:
    return yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))


def secret_derived_outputs(workflow: dict) -> list[str]:
    """Return job outputs whose value is (transitively) built from a secret.

    GitHub silently drops a job output that contains a secret value, so any
    such output arrives empty in dependent jobs. Taint starts at step env vars
    whose expression references ``secrets.`` and flows through shell
    assignments in the step's script.
    """

    offenders = []
    for job_name, job in (workflow.get("jobs") or {}).items():
        steps = {s.get("id"): s for s in job.get("steps", []) if s.get("id")}
        for out_name, expr in (job.get("outputs") or {}).items():
            expr = str(expr)
            if "secrets." in expr:
                offenders.append(f"{job_name}.{out_name}")
                continue
            ref = re.search(r"steps\.([\w-]+)\.outputs\.([\w-]+)", expr)
            if not ref or ref.group(1) not in steps:
                continue
            step = steps[ref.group(1)]
            tainted = {k for k, v in (step.get("env") or {}).items() if "secrets." in str(v)}
            tainted |= {k for k, v in (job.get("env") or {}).items() if "secrets." in str(v)}
            script = str(step.get("run", ""))
            assignments = re.findall(r"^\s*([A-Za-z_]\w*)=(.*)$", script, re.MULTILINE)
            changed = True
            while changed:
                changed = False
                for name, rhs in assignments:
                    if name not in tainted and any(re.search(rf"\${{?{t}\b", rhs) for t in tainted):
                        tainted.add(name)
                        changed = True
            for line in re.findall(rf"{re.escape(ref.group(2))}=([^\"'\n]*)", script):
                if "secrets." in line or any(re.search(rf"\${{?{t}\b", line) for t in tainted):
                    offenders.append(f"{job_name}.{out_name}")
                    break
    return offenders


class BackendTests(unittest.TestCase):
    def test_pulumi_yaml_does_not_pin_a_backend(self):
        # A project backend.url outranks the stored `pulumi login`, so a pinned
        # file:// backend sent every job's state to its own runner.
        project = yaml.safe_load((PROGRAM_DIR / "Pulumi.yaml").read_text(encoding="utf-8"))
        self.assertNotIn("backend", project)

    def test_every_pulumi_job_sets_the_shared_backend_url(self):
        for name, job in load_workflow()["jobs"].items():
            uses_pulumi = any("pulumi/actions" in str(s.get("uses", "")) for s in job.get("steps", []))
            if not uses_pulumi:
                continue
            url = str((job.get("env") or {}).get("PULUMI_BACKEND_URL", ""))
            self.assertTrue(url.startswith("gs://"), f"job {name} must set PULUMI_BACKEND_URL to the gs:// bucket")

    def test_scripts_export_the_backend_url(self):
        for script in ("preview-configure.sh", "preview-destroy.sh"):
            text = (PROGRAM_DIR / script).read_text(encoding="utf-8")
            self.assertIn('export PULUMI_BACKEND_URL="$backend"', text, script)


class WorkflowLintTests(unittest.TestCase):
    def test_no_job_output_is_derived_from_a_secret(self):
        self.assertEqual(secret_derived_outputs(load_workflow()), [])

    def test_lint_flags_secret_derived_outputs(self):
        bad = yaml.safe_load(
            textwrap.dedent(
                """\
                jobs:
                  plan:
                    outputs:
                      origin: ${{ steps.meta.outputs.origin }}
                      image_ref: ${{ steps.image.outputs.image_ref }}
                      run_id: ${{ steps.meta.outputs.run_id }}
                    steps:
                      - id: meta
                        env:
                          PREVIEW_DOMAIN: ${{ secrets.GIST_PREVIEW_BASE_DOMAIN }}
                        run: |
                          run_id="r1"
                          echo "run_id=$run_id" >> "$GITHUB_OUTPUT"
                          echo "origin=https://preview-${run_id}.${PREVIEW_DOMAIN}" >> "$GITHUB_OUTPUT"
                      - id: image
                        env:
                          GCP_PROJECT: ${{ secrets.GCP_PREVIEW_PROJECT }}
                        run: |
                          base="us-docker.pkg.dev/${GCP_PROJECT}/r/i"
                          echo "image_ref=${base}@sha256:0" >> "$GITHUB_OUTPUT"
                """
            )
        )
        self.assertEqual(sorted(secret_derived_outputs(bad)), ["plan.image_ref", "plan.origin"])

    def test_id_token_is_granted_only_to_jobs_that_need_it(self):
        workflow = load_workflow()
        self.assertNotIn("id-token", workflow.get("permissions", {}))
        for name, job in workflow["jobs"].items():
            granted = (job.get("permissions") or {}).get("id-token") == "write"
            self.assertEqual(granted, name in OIDC_JOBS, f"job {name} id-token grant")

    def test_sweep_and_manual_destroy_use_distinct_concurrency_groups(self):
        concurrency = load_workflow()["concurrency"]
        group = str(concurrency["group"])
        self.assertFalse(concurrency.get("cancel-in-progress", False))
        # A manual destroy is keyed by its own run id, so no later run can
        # replace it while it is pending; sweeps get their own group.
        self.assertIn("ttl-sweep", group)
        self.assertRegex(group, r"destroy-\{0\}'\s*,\s*github\.run_id")

    def test_end_of_run_destroy_expects_the_created_stack(self):
        destroy = load_workflow()["jobs"]["destroy"]
        step = next(s for s in destroy["steps"] if s.get("name") == "Destroy (fail-closed)")
        self.assertIn("needs.create.result != 'skipped'", str(step["env"]["EXPECT_STACK"]))


@unittest.skipUnless(shutil.which("bash") and shutil.which("jq"), "bash and jq are required")
class TeardownScriptTests(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.tmp)
        self.fake = self.tmp / "fake"
        self.fake.mkdir()
        bindir = self.tmp / "bin"
        bindir.mkdir()
        pulumi = bindir / "pulumi"
        pulumi.write_text(FAKE_PULUMI, encoding="utf-8")
        pulumi.chmod(pulumi.stat().st_mode | stat.S_IEXEC)
        self.env = {
            **{k: v for k, v in os.environ.items() if not k.startswith("PULUMI_")},
            "PATH": f"{bindir}{os.pathsep}{os.environ['PATH']}",
            "FAKE": str(self.fake),
            "PULUMI_STATE_BUCKET": "example-preview-state",
            "PROGRAM_DIR": str(PROGRAM_DIR),
            "DESTROY_RETRY_SLEEP": "0",
        }
        self.stacks([{"name": "organization/gist-registry/preview"}])
        (self.fake / "export").write_text(json.dumps(LIVE_RESOURCES), encoding="utf-8")
        self.outputs("2999-01-01T00:00:00Z")

    def stacks(self, value):
        (self.fake / "stacks").write_text(json.dumps(value), encoding="utf-8")

    def outputs(self, expires_at):
        doc = {"preview_run_id": "r42", "preview_owner": "octocat"}
        if expires_at:
            doc["preview_expires_at"] = expires_at
        (self.fake / "outputs").write_text(json.dumps(doc), encoding="utf-8")

    def run_destroy(self, **env):
        return subprocess.run(
            ["bash", str(PROGRAM_DIR / "preview-destroy.sh")],
            cwd=self.tmp, env={**self.env, **env}, capture_output=True, text=True, check=False,
        )

    def calls(self):
        path = self.fake / "calls"
        return path.read_text(encoding="utf-8").splitlines() if path.exists() else []

    def destroyed(self):
        return any("|destroy " in c for c in self.calls())

    def test_missing_stack_after_create_fails(self):
        self.stacks([])
        result = self.run_destroy(EXPECT_STACK="true", REASON="end-of-run")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn("missing from the shared backend", result.stderr)

    def test_missing_stack_without_create_is_a_no_op(self):
        self.stacks([])
        result = self.run_destroy(REASON="manual")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(self.destroyed())

    def test_backend_listing_error_fails_instead_of_reporting_no_stack(self):
        (self.fake / "ls_fail").touch()
        result = self.run_destroy(REASON="manual")
        self.assertNotEqual(result.returncode, 0)

    def test_every_pulumi_call_uses_the_shared_backend(self):
        self.run_destroy(REASON="manual")
        calls = self.calls()
        self.assertTrue(calls)
        for call in calls:
            self.assertTrue(call.startswith("gs://example-preview-state|"), call)

    def test_conflicting_backend_url_is_refused(self):
        result = self.run_destroy(PULUMI_BACKEND_URL="file://.pulumi-state")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_sweep_keeps_a_preview_whose_output_expiry_is_in_the_future(self):
        # No Pulumi.preview.yaml exists in a fresh checkout; the expiry must come
        # from stack outputs, and a future expiry must not be destroyed.
        self.assertFalse((PROGRAM_DIR / "Pulumi.preview.yaml").exists())
        result = self.run_destroy(ONLY_IF_EXPIRED="true", REASON="ttl-expired")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("not yet due", result.stdout)
        self.assertFalse(self.destroyed())
        self.assertFalse(any("|config " in c for c in self.calls()))

    def test_sweep_destroys_an_expired_preview_and_records_owner(self):
        self.outputs("2020-01-01T00:00:00Z")
        result = self.run_destroy(ONLY_IF_EXPIRED="true", REASON="ttl-expired")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(self.destroyed())
        record = json.loads((self.tmp / "evidence/preview-teardown.json").read_text(encoding="utf-8"))
        self.assertEqual((record["run_id"], record["owner"], record["status"]), ("r42", "octocat", "destroyed"))
        self.assertEqual(record["expires_at"], "2020-01-01T00:00:00Z")


if __name__ == "__main__":
    unittest.main()
