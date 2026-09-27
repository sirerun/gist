"""Offline tests for the registry preview IaC (task O2).

Run from the repository root:

    python -m unittest discover -s deploy/registry/tests -v

The settings and manifest tests use only the standard library. The program
tests run ``__main__.py`` under Pulumi's in-process mocks, so they need the
packages from ``deploy/registry/requirements.txt`` but no Pulumi login, stack,
backend, or cloud credentials. They are skipped when Pulumi is not installed.
"""

from __future__ import annotations

import datetime as dt
import json
import runpy
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
PROGRAM_DIR = HERE.parent
REPO = PROGRAM_DIR.parents[1]
sys.path.insert(0, str(PROGRAM_DIR))

import preview  # noqa: E402

NOW = dt.datetime(2026, 9, 26, 12, 0, 0, tzinfo=dt.timezone.utc)

BASE = {
    "project": "example-registry-project",
    "previewRunId": "r42",
    "previewOwner": "octocat",
    "previewExpiresAt": "2026-09-27T06:00:00Z",
    "publicOrigin": "https://preview-r42.registry-preview.example.invalid",
    "resourceAudience": "https://preview-r42.registry-preview.example.invalid",
    "productionOrigin": "https://registry.example.invalid",
    "previewDnsZone": "registry-preview-zone",
    "oauthRedirectUris": ["https://client.example.invalid/oauth/callback"],
    "databasePasswordSecret": "gist-registry-preview-database-password",
    "issuerSecret": "gist-registry-preview-oauth-issuer",
    "signingKeySecret": "gist-registry-preview-workload-signing-key",
}


def settings(**overrides):
    values = {**BASE, **overrides}
    return preview.parse_settings(lambda key: values.get(key), "preview", now=NOW)


class PreviewSettingsTest(unittest.TestCase):
    def test_valid_settings(self):
        s = settings()
        self.assertEqual(s.audience, s.origin)
        self.assertNotEqual(s.origin, s.production_origin)
        self.assertEqual(s.protected_resource_metadata_url, s.origin + "/.well-known/oauth-protected-resource")
        self.assertLessEqual(len(s.prefix + "-runtime"), 30, "service account id limit")
        self.assertEqual(s.labels()["environment"], "preview")

    def test_rejects_non_preview_stack(self):
        with self.assertRaises(preview.PreviewConfigError):
            preview.parse_settings(lambda key: BASE.get(key), "production", now=NOW)

    def test_rejects_production_origin_reuse(self):
        with self.assertRaises(preview.PreviewConfigError):
            settings(publicOrigin=BASE["productionOrigin"], resourceAudience=BASE["productionOrigin"])

    def test_rejects_audience_different_from_origin(self):
        with self.assertRaises(preview.PreviewConfigError):
            settings(resourceAudience="https://registry.example.invalid")

    def test_origin_must_carry_run_id(self):
        with self.assertRaises(preview.PreviewConfigError):
            settings(publicOrigin="https://preview-other.example.invalid", resourceAudience=None)

    def test_rejects_origin_with_path_or_slash(self):
        for bad in ("https://preview-r42.example.invalid/", "https://preview-r42.example.invalid/x", "http://preview-r42.example.invalid"):
            with self.subTest(bad=bad), self.assertRaises(preview.PreviewConfigError):
                settings(publicOrigin=bad, resourceAudience=None)

    def test_ttl_is_bounded_to_24_hours(self):
        with self.assertRaises(preview.PreviewConfigError):
            settings(previewExpiresAt="2026-09-27T12:00:01Z")
        with self.assertRaises(preview.PreviewConfigError):
            settings(previewExpiresAt="2026-09-26T11:59:59Z")
        with self.assertRaises(preview.PreviewConfigError):
            preview.parse_settings(lambda key: BASE.get(key), "preview", now=NOW, max_ttl_hours=48)

    def test_secrets_must_be_preview_scoped(self):
        with self.assertRaises(preview.PreviewConfigError):
            settings(databasePasswordSecret="gist-registry-database-password")

    def test_redirects_are_exact_and_not_production(self):
        with self.assertRaises(preview.PreviewConfigError):
            settings(oauthRedirectUris=["https://*.example.invalid/cb"])
        with self.assertRaises(preview.PreviewConfigError):
            settings(oauthRedirectUris=["https://registry.example.invalid/cb"])

    def test_preview_named_stack_requires_preview_mode(self):
        with self.assertRaises(preview.PreviewConfigError):
            preview.validate_production_stack("preview")
        preview.validate_production_stack("production")

    def test_expiry_helpers(self):
        self.assertEqual(preview.expiry_after(24, now=NOW), "2026-09-27T12:00:00Z")
        with self.assertRaises(preview.PreviewConfigError):
            preview.expiry_after(25, now=NOW)
        self.assertTrue(preview.is_expired("2026-09-26T12:00:00Z", now=NOW))
        self.assertFalse(preview.is_expired("2026-09-26T12:00:01Z", now=NOW))


class PreviewManifestTest(unittest.TestCase):
    def manifest(self, **overrides):
        document = {
            "stack": "preview", "run_id": "r42", "owner": "octocat",
            "created_at": "2026-09-26T12:00:00Z", "expires_at": "2026-09-27T12:00:00Z",
            "origin": BASE["publicOrigin"], "audience": BASE["publicOrigin"],
            "protected_resource_metadata_url": BASE["publicOrigin"] + preview.PRM_PATH,
            "production_origin": BASE["productionOrigin"], "commit": "a" * 40, "config_hash": "b" * 64,
            "status": "created",
        }
        document.update(overrides)
        return document

    def test_valid_manifest(self):
        preview.validate_manifest(self.manifest())

    def test_example_manifest_file_is_valid(self):
        path = PROGRAM_DIR / "preview.manifest.example.json"
        preview.validate_manifest(json.loads(path.read_text(encoding="utf-8")))

    def test_manifest_rejects_long_lifetime_and_credentials(self):
        with self.assertRaises(preview.PreviewConfigError):
            preview.validate_manifest(self.manifest(expires_at="2026-09-28T12:00:00Z"))
        with self.assertRaises(preview.PreviewConfigError):
            preview.validate_manifest(self.manifest(evidence={"access_token": "x"}))
        with self.assertRaises(preview.PreviewConfigError):
            preview.validate_manifest(self.manifest(audience=BASE["productionOrigin"]))

    def test_manifest_schema_file_matches_validator(self):
        schema = json.loads((PROGRAM_DIR / "preview.manifest.schema.json").read_text(encoding="utf-8"))
        self.assertEqual(set(schema["required"]), preview.MANIFEST_KEYS)
        self.assertEqual(set(schema["properties"]["status"]["enum"]), preview.MANIFEST_STATUSES)


try:
    import pulumi  # noqa: F401
    import pulumi_gcp  # noqa: F401
except ImportError:  # pragma: no cover - exercised only without IaC deps
    pulumi = None


@unittest.skipIf(pulumi is None, "pulumi and pulumi-gcp are not installed")
class ProgramMockTest(unittest.TestCase):
    """Run __main__.py under Pulumi mocks and inspect the declared resources."""

    def run_program(self, stack: str, config: dict[str, object]):
        import pulumi
        import pulumi.runtime

        created: list[pulumi.runtime.MockResourceArgs] = []

        class Mocks(pulumi.runtime.Mocks):
            def new_resource(self, args):
                created.append(args)
                outputs = dict(args.inputs)
                outputs.setdefault("name", args.name)
                if args.typ == "gcp:serviceaccount/account:Account":
                    outputs["email"] = f"{args.inputs['accountId']}@example.iam.gserviceaccount.com"
                if args.typ == "gcp:compute/globalAddress:GlobalAddress":
                    outputs["address"] = "203.0.113.10"
                if args.typ == "gcp:cloudrunv2/service:Service":
                    outputs["uri"] = "https://registry-mock.a.run.app"
                return [args.name + "_id", outputs]

            def call(self, args):
                return {}

        pulumi.runtime.set_mocks(Mocks(), project="gist-registry", stack=stack, preview=False)
        pulumi.runtime.set_all_config(
            {f"gist-registry:{k}": (v if isinstance(v, str) else json.dumps(v)) for k, v in config.items()}
        )
        sys.modules.pop("preview", None)
        runpy.run_path(str(PROGRAM_DIR / "__main__.py"), run_name="__pulumi_program__")
        # Wait until every resource registration reached the mocks.
        from pulumi.runtime.stack import _sync_await, wait_for_rpcs

        _sync_await(wait_for_rpcs())
        return created

    def common(self):
        return {
            "project": "example-registry-project",
            "containerImage": "us-central1-docker.pkg.dev/example-registry-project/registry/hosted@sha256:" + "0" * 64,
            "stateBucket": "gs://example-state",
        }

    def by_type(self, created, typ):
        return [r for r in created if r.typ == typ]

    def test_preview_program(self):
        expires = preview.expiry_after(12)
        config = {**self.common(), **BASE, "deploymentMode": "preview", "previewExpiresAt": expires}
        created = self.run_program("preview", config)
        types = {r.typ for r in created}

        for required in (
            "gcp:compute/managedSslCertificate:ManagedSslCertificate",
            "gcp:compute/targetHttpsProxy:TargetHttpsProxy",
            "gcp:compute/regionNetworkEndpointGroup:RegionNetworkEndpointGroup",
            "gcp:dns/recordSet:RecordSet",
        ):
            self.assertIn(required, types)
        for forbidden in (
            "gcp:secretmanager/secret:Secret",
            "gcp:iam/workloadIdentityPool:WorkloadIdentityPool",
            "gcp:iam/workloadIdentityPoolProvider:WorkloadIdentityPoolProvider",
        ):
            self.assertNotIn(forbidden, types, "preview must not create " + forbidden)

        cert = self.by_type(created, "gcp:compute/managedSslCertificate:ManagedSslCertificate")[0]
        self.assertEqual(cert.inputs["managed"]["domains"], ["preview-r42.registry-preview.example.invalid"])

        service = self.by_type(created, "gcp:cloudrunv2/service:Service")[0]
        self.assertEqual(service.inputs["ingress"], "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER")
        envs = {e["name"]: e for e in service.inputs["template"]["containers"][0]["envs"]}
        self.assertEqual(envs["GIST_RESOURCE_AUDIENCE"]["value"], BASE["publicOrigin"])
        self.assertEqual(envs["GIST_PUBLIC_ORIGIN"]["value"], BASE["publicOrigin"])
        self.assertEqual(envs["GIST_DEPLOYMENT_MODE"]["value"], "preview")
        for name in ("GIST_DATABASE_PASSWORD", "GIST_OAUTH_ISSUER", "GIST_WORKLOAD_SIGNING_KEY"):
            ref = envs[name]["valueSource"]["secretKeyRef"]["secret"]
            self.assertTrue(ref.startswith("gist-registry-preview-"), ref)
            self.assertNotIn("value", envs[name])

        sql = self.by_type(created, "gcp:sql/databaseInstance:DatabaseInstance")[0]
        self.assertFalse(sql.inputs["deletionProtection"])
        self.assertFalse(sql.inputs["settings"]["ipConfiguration"]["ipv4Enabled"])

        bucket = self.by_type(created, "gcp:storage/bucket:Bucket")[0]
        self.assertTrue(bucket.inputs["forceDestroy"])
        self.assertNotIn("retentionPolicy", bucket.inputs)
        self.assertEqual(bucket.inputs["publicAccessPrevention"], "enforced")

        account = self.by_type(created, "gcp:serviceaccount/account:Account")[0]
        self.assertEqual(account.inputs["accountId"], "gist-prv-r42-runtime")

    def test_target_program_unchanged(self):
        config = {**self.common(), "publicOrigin": "https://registry.example.invalid"}
        created = self.run_program("production", config)
        types = {r.typ for r in created}
        self.assertIn("gcp:iam/workloadIdentityPool:WorkloadIdentityPool", types)
        self.assertIn("gcp:secretmanager/secret:Secret", types)
        self.assertNotIn("gcp:compute/managedSslCertificate:ManagedSslCertificate", types)
        sql = self.by_type(created, "gcp:sql/databaseInstance:DatabaseInstance")[0]
        self.assertTrue(sql.inputs["deletionProtection"])
        bucket = self.by_type(created, "gcp:storage/bucket:Bucket")[0]
        self.assertIn("retentionPolicy", bucket.inputs)
        service = self.by_type(created, "gcp:cloudrunv2/service:Service")[0]
        self.assertEqual(service.inputs["ingress"], "INGRESS_TRAFFIC_ALL")


class CheckerTest(unittest.TestCase):
    def test_check_iac_preview_passes(self):
        sys.path.insert(0, str(REPO / "scripts" / "registry"))
        import check

        self.assertEqual(check.main(["iac", "--preview"]), 0)


if __name__ == "__main__":
    unittest.main()
