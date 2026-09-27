"""Pulumi definition for the hosted Gist registry deployment surface.

Secret values are supplied out of band. This program creates references to
Secret Manager entries but never embeds credentials in source or state.

``gist-registry:deploymentMode`` selects the shape:

* ``target`` (default): the long-lived registry surface from O1.
* ``preview``: the short-lived OAuth preview from ADR 007 section 4 (task O2).
  It only runs on the stack named ``preview``, gets a unique public origin
  and audience behind a managed-TLS load balancer, uses synthetic data,
  references pre-created ``gist-registry-preview-*`` secrets by name only,
  creates no workload identity pool, and every resource is destroyable
  (no deletion protection, no bucket retention lock). See ``preview.py``.
"""

from __future__ import annotations

import json
import re
from typing import Any

import os
import sys

import pulumi
import pulumi_gcp as gcp

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import preview as preview_mod  # noqa: E402  (local module next to this program)


cfg = pulumi.Config("gist-registry")
project = cfg.require("project")
region = cfg.get("region") or "us-central1"
origin = cfg.require("publicOrigin")
image = cfg.require("containerImage")
repository = cfg.get("githubRepository") or "sirerun/gist"
state_bucket = cfg.require("stateBucket")
stack = pulumi.get_stack()
mode = cfg.get("deploymentMode") or "target"
if mode not in {"target", "preview"}:
    raise ValueError("deploymentMode must be 'target' or 'preview'")
is_preview = mode == "preview"

if not re.fullmatch(r"https://[^/?#*]+", origin):
    raise ValueError("publicOrigin must be one exact HTTPS origin without path, query, fragment, wildcard, or slash")
if "@sha256:" not in image or not (image.startswith("gcr.io/") or "-docker.pkg.dev/" in image):
    raise ValueError("containerImage must be an immutable registry reference by digest")

limits: dict[str, Any] = {
    "maxPackageBytes": 10 * 1024 * 1024,
    "maxExpandedPackageBytes": 50 * 1024 * 1024,
    "maxRequestBytes": 1 * 1024 * 1024,
    "maxResponseBytes": 2 * 1024 * 1024,
    "maxCatalogEntries": 100_000,
    "maxConcurrentRequests": 80,
    "maxDiscoveryResults": 50,
    "requestTimeoutSeconds": 30,
    "targetAvailabilityPercent": 99.5,
    "p95LatencyMilliseconds": 750,
    "errorRatePercent": 1.0,
    "backupRetentionDays": 14,
    "previewTtlHours": 24,
}
limits.update(cfg.get_object("limits") or {})

if is_preview:
    def _preview_get(key: str) -> Any:
        if key == "oauthRedirectUris":
            return cfg.get_object(key)
        return cfg.get(key)

    preview = preview_mod.parse_settings(_preview_get, stack, max_ttl_hours=float(limits["previewTtlHours"]))
    audience = preview.audience
    name_prefix = preview.prefix
    labels = preview.labels()
    redirect_uris = list(preview.redirect_uris)
else:
    preview_mod.validate_production_stack(stack)
    preview = None
    audience = cfg.get("resourceAudience") or origin
    if not re.fullmatch(r"https://[^/?#*]+", audience):
        raise ValueError("resourceAudience must be one exact HTTPS origin")
    name_prefix = "gist-registry"
    labels = {"service": "gist-registry"}
    redirect_uris = list(cfg.get_object("oauthRedirectUris") or [])
common = {"project": project, "region": region}
project_only = {"project": project}

network = gcp.compute.Network("registry-network", auto_create_subnetworks=False, project=project)
subnet = gcp.compute.Subnetwork(
    "registry-subnet", network=network.id, ip_cidr_range="10.42.0.0/24", region=region, project=project
)
peering_range = gcp.compute.GlobalAddress(
    "registry-private-service-range", purpose="VPC_PEERING", address_type="INTERNAL",
    prefix_length=16, network=network.id, project=project
)
gcp.servicenetworking.Connection(
    "registry-service-networking", network=network.id,
    reserved_peering_ranges=[peering_range.name], service="servicenetworking.googleapis.com",
)

runtime = gcp.serviceaccount.Account(
    "registry-runtime", account_id=f"{name_prefix}-runtime",
    display_name="Gist registry preview runtime" if is_preview else "Gist registry runtime", **project_only
)

bucket = gcp.storage.Bucket(
    "registry-objects", location=region, uniform_bucket_level_access=True,
    public_access_prevention="enforced", versioning=gcp.storage.BucketVersioningArgs(enabled=not is_preview),
    # A retention policy would block the preview's mandatory teardown.
    retention_policy=None if is_preview else gcp.storage.BucketRetentionPolicyArgs(retention_period=86400),
    force_destroy=is_preview,
    labels={**labels, "data": "synthetic-preview" if is_preview else "private-immutable-artifacts"}, project=project
)
gcp.storage.BucketIAMMember(
    "registry-runtime-object-reader", bucket=bucket.name, role="roles/storage.objectViewer",
    member=runtime.email.apply(lambda email: f"serviceAccount:{email}"),
)

sql = gcp.sql.DatabaseInstance(
    "registry-postgres", database_version="POSTGRES_16", region=region,
    deletion_protection=not is_preview,
    settings=gcp.sql.DatabaseInstanceSettingsArgs(
        tier="db-custom-1-3840" if is_preview else "db-custom-2-7680",
        availability_type="ZONAL" if is_preview else "REGIONAL", disk_type="PD_SSD",
        disk_size=10 if is_preview else 20, disk_autoresize=not is_preview,
        deletion_protection_enabled=not is_preview, user_labels=labels,
        backup_configuration=gcp.sql.DatabaseInstanceSettingsBackupConfigurationArgs(
            # Preview data is synthetic and destroyed within 24 hours.
            enabled=not is_preview, point_in_time_recovery_enabled=not is_preview, transaction_log_retention_days=7,
            backup_retention_settings=gcp.sql.DatabaseInstanceSettingsBackupConfigurationBackupRetentionSettingsArgs(
                retained_backups=int(limits["backupRetentionDays"]),
            ),
        ),
        ip_configuration=gcp.sql.DatabaseInstanceSettingsIpConfigurationArgs(
            ipv4_enabled=False, private_network=network.id,
        ),
    ), **project_only
)
database = gcp.sql.Database("registry", instance=sql.name, charset="UTF8", **project_only)
db_user = gcp.sql.User("registry-service-user", instance=sql.name, name="registry_service", **project_only)

secret_names = {
    "databasePassword": cfg.get("databasePasswordSecret") or "gist-registry-database-password",
    "issuer": cfg.get("issuerSecret") or "gist-registry-oauth-issuer",
    "signingKey": cfg.get("signingKeySecret") or "gist-registry-workload-signing-key",
}
if is_preview:
    # Preview secrets are pre-created, preview-only entries referenced by name.
    # The stack grants and, on destroy, removes the preview runtime's access;
    # it never creates, reads or versions secret payloads.
    secret_names = dict(preview.secret_names)
secret_ids: dict[str, Any] = {}
for key, name in secret_names.items():
    if is_preview:
        secret_ids[key] = name
        secret_ref: Any = f"projects/{project}/secrets/{name}"
    else:
        created = gcp.secretmanager.Secret(
            f"registry-{key}-secret", secret_id=name,
            replication=gcp.secretmanager.SecretReplicationArgs(auto={}), labels={"service": "gist-registry"}, **project_only
        )
        secret_ids[key] = created.secret_id
        secret_ref = created.id
    gcp.secretmanager.SecretIamMember(
        f"registry-{key}-access", secret_id=secret_ref,
        role="roles/secretmanager.secretAccessor", member=runtime.email.apply(lambda email: f"serviceAccount:{email}"),
        **project_only
    )

env = [
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_PUBLIC_ORIGIN", value=origin),
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_RESOURCE_AUDIENCE", value=audience),
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_DEPLOYMENT_MODE", value=mode),
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_OAUTH_REDIRECT_URIS", value=json.dumps(redirect_uris)),
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_DATA_BUCKET", value=bucket.name),
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_DATABASE_NAME", value=database.name),
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_DATABASE_USER", value=db_user.name),
    gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(name="GIST_LIMITS_JSON", value=json.dumps(limits, sort_keys=True)),
]
for key, env_name in (("databasePassword", "GIST_DATABASE_PASSWORD"), ("issuer", "GIST_OAUTH_ISSUER"), ("signingKey", "GIST_WORKLOAD_SIGNING_KEY")):
    env.append(gcp.cloudrunv2.ServiceTemplateContainerEnvArgs(
        name=env_name,
        value_source=gcp.cloudrunv2.ServiceTemplateContainerEnvValueSourceArgs(
            secret_key_ref=gcp.cloudrunv2.ServiceTemplateContainerEnvValueSourceSecretKeyRefArgs(
                secret=secret_ids[key], version="latest"
            )
        ),
    ))

startup = gcp.cloudrunv2.ServiceTemplateContainerStartupProbeArgs(
    http_get=gcp.cloudrunv2.ServiceTemplateContainerStartupProbeHttpGetArgs(path="/healthz", port=8080),
    initial_delay_seconds=5, period_seconds=10, timeout_seconds=3, failure_threshold=6,
)
liveness = gcp.cloudrunv2.ServiceTemplateContainerLivenessProbeArgs(
    http_get=gcp.cloudrunv2.ServiceTemplateContainerLivenessProbeHttpGetArgs(path="/healthz", port=8080),
    period_seconds=30, timeout_seconds=5, failure_threshold=3,
)
service = gcp.cloudrunv2.Service(
    "registry", location=region, deletion_protection=False, labels=labels,
    # The preview is reachable only through its managed-TLS load balancer origin.
    ingress="INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER" if is_preview else "INGRESS_TRAFFIC_ALL",
    template=gcp.cloudrunv2.ServiceTemplateArgs(
        service_account=runtime.email, timeout=f"{int(limits['requestTimeoutSeconds'])}s",
        max_instance_request_concurrency=int(limits["maxConcurrentRequests"]),
        scaling=gcp.cloudrunv2.ServiceTemplateScalingArgs(max_instance_count=2 if is_preview else 10, min_instance_count=0),
        vpc_access=gcp.cloudrunv2.ServiceTemplateVpcAccessArgs(network_interfaces=[
            gcp.cloudrunv2.ServiceTemplateVpcAccessNetworkInterfaceArgs(network=network.name, subnetwork=subnet.name)
        ]),
        containers=[gcp.cloudrunv2.ServiceTemplateContainerArgs(
            image=image, ports=gcp.cloudrunv2.ServiceTemplateContainerPortsArgs(container_port=8080),
            envs=env, startup_probe=startup, liveness_probe=liveness,
            resources=gcp.cloudrunv2.ServiceTemplateContainerResourcesArgs(limits={"cpu": "1", "memory": "512Mi"}),
        )],
    ), project=project
)
gcp.cloudrunv2.ServiceIamMember("registry-invoker", name=service.name, location=region, role="roles/run.invoker", member="allUsers", project=project)

migration = gcp.cloudrunv2.Job(
    "registry-migrations", location=region,
    template=gcp.cloudrunv2.JobTemplateArgs(template=gcp.cloudrunv2.JobTemplateTemplateArgs(
        service_account=runtime.email, max_retries=1,
        timeout=f"{int(limits['requestTimeoutSeconds']) * 2}s",
        containers=[gcp.cloudrunv2.JobTemplateTemplateContainerArgs(
            image=image, args=["migrate", "--path=/app/migrations"], envs=env,
        )],
    )), project=project
)

if not is_preview:
    # The preview reuses the long-lived CI identity path; pool ids are
    # project-global and soft-deleted for 30 days, so a per-run pool is unsafe.
    deployer = gcp.serviceaccount.Account(
        "registry-deployer", account_id="gist-registry-deployer", display_name="Gist registry CI deployer", **project_only
    )
    pool = gcp.iam.WorkloadIdentityPool("registry-github-pool", workload_identity_pool_id="gist-registry-github", **project_only)
    gcp.iam.WorkloadIdentityPoolProvider(
        "registry-github-provider", workload_identity_pool_id=pool.workload_identity_pool_id,
        workload_identity_pool_provider_id="github",
        oidc=gcp.iam.WorkloadIdentityPoolProviderOidcArgs(issuer_uri="https://token.actions.githubusercontent.com"),
        attribute_mapping={"google.subject": "assertion.sub", "attribute.repository": "assertion.repository", "attribute.ref": "assertion.ref"},
        attribute_condition=f"assertion.repository == '{repository}'", **project_only
    )
    gcp.serviceaccount.IAMMember(
        "registry-deployer-oidc", service_account_id=deployer.name, role="roles/iam.workloadIdentityUser",
        member=pulumi.Output.concat("principalSet://iam.googleapis.com/", pool.name, "/attribute.repository/", repository)
    )
    for role in ("roles/run.admin", "roles/cloudsql.client", "roles/storage.objectViewer", "roles/secretmanager.viewer"):
        gcp.projects.IAMMember(f"registry-deployer-{role.split('/')[-1]}", project=project, role=role, member=deployer.email.apply(lambda email: f"serviceAccount:{email}"))

error_metric = gcp.logging.Metric(
    "registry-request-errors", filter='resource.type="cloud_run_revision" AND jsonPayload.status >= 500',
    metric_descriptor=gcp.logging.MetricMetricDescriptorArgs(metric_kind="DELTA", value_type="INT64", unit="1"), project=project,
)
gcp.logging.Metric(
    "registry-request-latency", filter='resource.type="cloud_run_revision" AND jsonPayload.latency_ms >= 0',
    value_extractor="EXTRACT(jsonPayload.latency_ms)",
    metric_descriptor=gcp.logging.MetricMetricDescriptorArgs(metric_kind="DELTA", value_type="DISTRIBUTION", unit="ms"), project=project,
)
gcp.monitoring.AlertPolicy(
    "registry-error-budget", display_name="Gist registry error-rate guardrail", combiner="OR",
    conditions=[gcp.monitoring.AlertPolicyConditionArgs(
        display_name="5xx rate above configured baseline",
        condition_threshold=gcp.monitoring.AlertPolicyConditionConditionThresholdArgs(
            filter=error_metric.name.apply(lambda name: f'metric.type="logging.googleapis.com/user/{name}"'),
            comparison="COMPARISON_GT", threshold_value=float(limits["errorRatePercent"]), duration="300s",
            aggregations=[gcp.monitoring.AlertPolicyConditionConditionThresholdAggregationArgs(
                alignment_period="60s", per_series_aligner="ALIGN_RATE", cross_series_reducer="REDUCE_SUM"
            )],
        ),
    )], project=project,
)

pulumi.export("service_url", service.uri)
pulumi.export("migration_job_name", migration.name)
pulumi.export("runtime_service_account", runtime.email)
pulumi.export("deployment_mode", mode)
pulumi.export("public_origin", origin)
pulumi.export("resource_audience", audience)
pulumi.export("protected_resource_metadata_url", origin + preview_mod.PRM_PATH)
if not is_preview:
    pulumi.export("deployer_service_account", deployer.email)
else:
    ingress = preview_mod.build_ingress(preview, project, region, service)
    pulumi.export("preview_run_id", preview.run_id)
    pulumi.export("preview_owner", preview.owner)
    pulumi.export("preview_expires_at", preview.expires_at)
    pulumi.export("preview_ingress_ip", ingress["ingress_ip"])
    pulumi.export("preview_certificate", ingress["certificate"])
    pulumi.export("preview_dns_record", ingress["dns_record"])
pulumi.export("private_object_bucket", bucket.name)
pulumi.export("database_instance", sql.name)
pulumi.export("pulumi_state_backend", state_bucket)
pulumi.export("limits", json.dumps(limits, sort_keys=True))
