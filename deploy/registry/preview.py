"""Short-lived OAuth preview settings and public ingress for the registry stack.

ADR 007 section 4 defines the preview: one Pulumi stack named ``preview``,
a unique temporary public origin and resource audience per run, synthetic
data only, a hard maximum lifetime of 24 hours, and a reviewed destroy stage
that also runs on failure and from a TTL sweeper.

The validation helpers in this module use only the standard library so the
workflow and the unit tests can run them without Pulumi or cloud credentials.
``build_ingress`` imports the Pulumi providers lazily and is only called by
``__main__.py`` when ``deploymentMode`` is ``preview``.

Command-line use (no cloud access):

    python3 preview.py is-expired 2026-09-27T12:00:00Z   # exit 0 when expired
    python3 preview.py expiry 24                         # print now + 24h
    python3 preview.py validate-manifest preview.manifest.json
"""

from __future__ import annotations

import datetime as _dt
import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Callable, Mapping

PREVIEW_STACK = "preview"
MAX_TTL_HOURS = 24
PREVIEW_SECRET_PREFIX = "gist-registry-preview-"
PRM_PATH = "/.well-known/oauth-protected-resource"

_ORIGIN = re.compile(r"https://([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)")
_RUN_ID = re.compile(r"[a-z0-9]{1,12}")
_EXPIRY = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z")
_LABEL_VALUE = re.compile(r"[^a-z0-9_-]")
_SECRET_ID = re.compile(r"[A-Za-z0-9_-]{1,255}")


class PreviewConfigError(ValueError):
    """Raised when a preview configuration would break isolation or TTL rules."""


@dataclass(frozen=True)
class PreviewSettings:
    run_id: str
    owner: str
    expires_at: str
    origin: str
    audience: str
    production_origin: str
    dns_zone: str
    dns_project: str
    redirect_uris: tuple[str, ...]
    secret_names: Mapping[str, str] = field(default_factory=dict)

    @property
    def host(self) -> str:
        return self.origin.removeprefix("https://")

    @property
    def prefix(self) -> str:
        """Prefix for globally named resources (service account ids <= 30 chars)."""
        return f"gist-prv-{self.run_id}"

    @property
    def protected_resource_metadata_url(self) -> str:
        return self.origin + PRM_PATH

    def labels(self) -> dict[str, str]:
        return {
            "service": "gist-registry",
            "environment": "preview",
            "gist-preview-run": self.run_id,
            "gist-preview-owner": label_value(self.owner),
            "gist-preview-expires": label_value(self.expires_at.replace(":", "").replace("-", "")),
        }


def utcnow() -> _dt.datetime:
    return _dt.datetime.now(_dt.timezone.utc)


def parse_expiry(value: str) -> _dt.datetime:
    if not isinstance(value, str) or not _EXPIRY.fullmatch(value):
        raise PreviewConfigError("previewExpiresAt must be an RFC 3339 UTC timestamp like 2026-09-27T12:00:00Z")
    return _dt.datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=_dt.timezone.utc)


def format_expiry(moment: _dt.datetime) -> str:
    return moment.astimezone(_dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def expiry_after(hours: float, now: _dt.datetime | None = None, max_hours: float = MAX_TTL_HOURS) -> str:
    if not 0 < hours <= max_hours:
        raise PreviewConfigError(f"preview TTL must be greater than 0 and at most {max_hours} hours")
    return format_expiry((now or utcnow()) + _dt.timedelta(hours=hours))


def is_expired(expires_at: str, now: _dt.datetime | None = None) -> bool:
    return parse_expiry(expires_at) <= (now or utcnow())


def label_value(value: str) -> str:
    cleaned = _LABEL_VALUE.sub("-", value.lower())[:63]
    return cleaned or "unknown"


def validate_origin(value: str, label: str) -> str:
    if not isinstance(value, str) or not _ORIGIN.fullmatch(value):
        raise PreviewConfigError(f"{label} must be one exact lowercase HTTPS origin without port, path, query, fragment, wildcard or trailing slash")
    return value


def parse_settings(
    get: Callable[[str], Any],
    stack: str,
    now: _dt.datetime | None = None,
    max_ttl_hours: float = MAX_TTL_HOURS,
) -> PreviewSettings:
    """Build and validate preview settings from a config getter.

    ``get`` returns the configured value for a key in the ``gist-registry``
    namespace, or ``None``. Every rule here fails closed.
    """

    if stack != PREVIEW_STACK:
        raise PreviewConfigError(f"deploymentMode=preview is only allowed on the '{PREVIEW_STACK}' stack, not '{stack}'")
    if max_ttl_hours > MAX_TTL_HOURS:
        raise PreviewConfigError(f"limits.previewTtlHours may not exceed the ADR 007 hard maximum of {MAX_TTL_HOURS}")

    def need(key: str) -> Any:
        value = get(key)
        if value is None or value == "" or value == []:
            raise PreviewConfigError(f"preview mode requires gist-registry:{key}")
        return value

    run_id = str(need("previewRunId"))
    if not _RUN_ID.fullmatch(run_id):
        raise PreviewConfigError("previewRunId must be 1-12 lowercase letters or digits")
    owner = str(need("previewOwner"))
    expires_at = str(need("previewExpiresAt"))
    expiry = parse_expiry(expires_at)
    current = now or utcnow()
    if expiry <= current:
        raise PreviewConfigError("previewExpiresAt is already in the past")
    if expiry - current > _dt.timedelta(hours=max_ttl_hours):
        raise PreviewConfigError(f"previewExpiresAt is more than {max_ttl_hours} hours away")

    origin = validate_origin(str(need("publicOrigin")), "publicOrigin")
    audience = validate_origin(str(get("resourceAudience") or origin), "resourceAudience")
    if audience != origin:
        raise PreviewConfigError("preview resourceAudience must equal the preview publicOrigin")
    production_origin = validate_origin(str(need("productionOrigin")), "productionOrigin")
    if production_origin in {origin, audience}:
        raise PreviewConfigError("preview origin and audience must differ from the production origin")
    host = origin.removeprefix("https://")
    production_host = production_origin.removeprefix("https://")
    if host == production_host or not host.startswith(f"preview-{run_id}."):
        raise PreviewConfigError(f"preview origin host must start with 'preview-{run_id}.' so every run has a unique origin")

    dns_zone = str(need("previewDnsZone"))
    dns_project = str(get("previewDnsProject") or need("project"))

    redirects_raw = get("oauthRedirectUris") or []
    if isinstance(redirects_raw, str):
        redirects_raw = json.loads(redirects_raw)
    if not isinstance(redirects_raw, list):
        raise PreviewConfigError("oauthRedirectUris must be a list of exact HTTPS redirect URIs")
    redirects: list[str] = []
    for uri in redirects_raw:
        if not isinstance(uri, str) or not uri.startswith("https://") or "*" in uri or "#" in uri:
            raise PreviewConfigError(f"redirect URI must be exact HTTPS without wildcard or fragment: {uri!r}")
        if uri.startswith(production_origin + "/") or uri == production_origin:
            raise PreviewConfigError("preview redirect URIs may not point at the production origin")
        redirects.append(uri)

    secret_names = {
        "databasePassword": str(need("databasePasswordSecret")),
        "issuer": str(need("issuerSecret")),
        "signingKey": str(need("signingKeySecret")),
        "consentSecret": str(need("consentSecretSecret")),
    }
    for key, name in secret_names.items():
        if not _SECRET_ID.fullmatch(name) or not name.startswith(PREVIEW_SECRET_PREFIX):
            raise PreviewConfigError(f"preview secret {key} must be a secret id starting with '{PREVIEW_SECRET_PREFIX}', got {name!r}")

    return PreviewSettings(
        run_id=run_id, owner=owner, expires_at=expires_at, origin=origin, audience=audience,
        production_origin=production_origin, dns_zone=dns_zone, dns_project=dns_project,
        redirect_uris=tuple(redirects), secret_names=secret_names,
    )


def validate_production_stack(stack: str) -> None:
    if stack == PREVIEW_STACK or stack.startswith(f"{PREVIEW_STACK}-"):
        raise PreviewConfigError("preview-named stacks must set gist-registry:deploymentMode=preview")


def build_ingress(settings: PreviewSettings, project: str, region: str, service: Any) -> dict[str, Any]:
    """Declare the preview's public HTTPS origin in front of the Cloud Run service.

    Global external Application Load Balancer, Google-managed certificate for
    the preview host, HTTP-to-HTTPS redirect, and one A record in an existing
    Cloud DNS zone. Every resource belongs to the preview stack and is removed
    by ``pulumi destroy``; the zone itself is never created or modified.
    """

    import pulumi_gcp as gcp

    p = settings.prefix
    project_only = {"project": project}
    address = gcp.compute.GlobalAddress(f"{p}-ip", **project_only)
    neg = gcp.compute.RegionNetworkEndpointGroup(
        f"{p}-neg", region=region, network_endpoint_type="SERVERLESS",
        cloud_run=gcp.compute.RegionNetworkEndpointGroupCloudRunArgs(service=service.name), **project_only,
    )
    backend = gcp.compute.BackendService(
        f"{p}-backend", load_balancing_scheme="EXTERNAL_MANAGED", protocol="HTTPS",
        backends=[gcp.compute.BackendServiceBackendArgs(group=neg.id)],
        log_config=gcp.compute.BackendServiceLogConfigArgs(enable=True, sample_rate=1.0), **project_only,
    )
    url_map = gcp.compute.URLMap(f"{p}-https", default_service=backend.id, **project_only)
    certificate = gcp.compute.ManagedSslCertificate(
        f"{p}-cert", managed=gcp.compute.ManagedSslCertificateManagedArgs(domains=[settings.host]), **project_only,
    )
    https_proxy = gcp.compute.TargetHttpsProxy(
        f"{p}-https-proxy", url_map=url_map.id, ssl_certificates=[certificate.id], **project_only,
    )
    gcp.compute.GlobalForwardingRule(
        f"{p}-https-rule", ip_address=address.address, port_range="443", target=https_proxy.id,
        load_balancing_scheme="EXTERNAL_MANAGED", labels=settings.labels(), **project_only,
    )
    redirect_map = gcp.compute.URLMap(
        f"{p}-http-redirect",
        default_url_redirect=gcp.compute.URLMapDefaultUrlRedirectArgs(
            https_redirect=True, strip_query=False, redirect_response_code="MOVED_PERMANENTLY_DEFAULT",
        ), **project_only,
    )
    http_proxy = gcp.compute.TargetHttpProxy(f"{p}-http-proxy", url_map=redirect_map.id, **project_only)
    gcp.compute.GlobalForwardingRule(
        f"{p}-http-rule", ip_address=address.address, port_range="80", target=http_proxy.id,
        load_balancing_scheme="EXTERNAL_MANAGED", labels=settings.labels(), **project_only,
    )
    record = gcp.dns.RecordSet(
        f"{p}-dns", managed_zone=settings.dns_zone, name=f"{settings.host}.", type="A", ttl=60,
        rrdatas=[address.address], project=settings.dns_project,
    )
    return {"ingress_ip": address.address, "certificate": certificate.name, "dns_record": record.name,
            "url_map": url_map.name, "https_proxy": https_proxy.name}


MANIFEST_KEYS = {
    "stack", "run_id", "owner", "expires_at", "origin", "audience", "protected_resource_metadata_url",
    "production_origin", "commit", "config_hash", "status",
}
MANIFEST_STATUSES = {"created", "tested", "destroyed", "destroy_failed", "create_failed"}


def validate_manifest(document: Mapping[str, Any], now: _dt.datetime | None = None) -> None:
    """Validate the per-run preview record the workflow emits (owner, expiry, origin)."""

    missing = sorted(MANIFEST_KEYS - set(document))
    if missing:
        raise PreviewConfigError(f"preview manifest is missing {', '.join(missing)}")
    extra = sorted(set(document) - MANIFEST_KEYS - {"created_at", "destroyed_at", "evidence"})
    if extra:
        raise PreviewConfigError(f"preview manifest has unexpected keys {', '.join(extra)}")
    if document["stack"] != PREVIEW_STACK:
        raise PreviewConfigError("preview manifest stack must be 'preview'")
    if not _RUN_ID.fullmatch(str(document["run_id"])):
        raise PreviewConfigError("preview manifest run_id is invalid")
    if not str(document["owner"]).strip():
        raise PreviewConfigError("preview manifest owner must be non-empty")
    expiry = parse_expiry(str(document["expires_at"]))
    created = document.get("created_at")
    if created is not None and expiry - parse_expiry(str(created)) > _dt.timedelta(hours=MAX_TTL_HOURS):
        raise PreviewConfigError("preview manifest lifetime exceeds 24 hours")
    origin = validate_origin(str(document["origin"]), "origin")
    if document["audience"] != origin:
        raise PreviewConfigError("preview manifest audience must equal its origin")
    if document["protected_resource_metadata_url"] != origin + PRM_PATH:
        raise PreviewConfigError("preview manifest protected_resource_metadata_url must be origin + " + PRM_PATH)
    if validate_origin(str(document["production_origin"]), "production_origin") == origin:
        raise PreviewConfigError("preview manifest origin must differ from production")
    if not re.fullmatch(r"[0-9a-f]{40}", str(document["commit"])):
        raise PreviewConfigError("preview manifest commit must be a 40-hex git SHA")
    if not re.fullmatch(r"[0-9a-f]{64}", str(document["config_hash"])):
        raise PreviewConfigError("preview manifest config_hash must be a SHA-256 hex digest")
    if document["status"] not in MANIFEST_STATUSES:
        raise PreviewConfigError(f"preview manifest status must be one of {sorted(MANIFEST_STATUSES)}")
    text = json.dumps(document).lower()
    for marker in ("secret_data", "access_token", "refresh_token", "private_key", "password\":"):
        if marker in text:
            raise PreviewConfigError(f"preview manifest must not carry credential material ({marker})")


def _main(argv: list[str]) -> int:
    if len(argv) == 2 and argv[0] == "is-expired":
        return 0 if is_expired(argv[1]) else 1
    if len(argv) == 2 and argv[0] == "expiry":
        print(expiry_after(float(argv[1])))
        return 0
    if len(argv) == 2 and argv[0] == "validate-manifest":
        validate_manifest(json.loads(Path(argv[1]).read_text(encoding="utf-8")))
        print("preview manifest: PASS")
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    try:
        raise SystemExit(_main(sys.argv[1:]))
    except PreviewConfigError as exc:
        print(f"preview: FAIL: {exc}", file=sys.stderr)
        raise SystemExit(3)
