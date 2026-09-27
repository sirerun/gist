#!/usr/bin/env bash
# Configure the dedicated `preview` Pulumi stack for one OAuth preview run.
#
# Called by .github/workflows/registry-preview.yml. Every value arrives through
# the environment; secrets are referenced by Secret Manager name only and no
# secret payload is read or written here.
set -euo pipefail

: "${PULUMI_STATE_BUCKET:?}" "${GCP_PROJECT:?}" "${PRODUCTION_ORIGIN:?}" "${DNS_ZONE:?}"
: "${ORIGIN:?}" "${RUN_ID:?}" "${EXPIRES_AT:?}" "${IMAGE_REF:?}" "${OWNER:?}"
STACK="${STACK:-preview}"
PROGRAM_DIR="${PROGRAM_DIR:-deploy/registry}"
REGION="${REGION:-us-central1}"
DNS_PROJECT="${DNS_PROJECT:-$GCP_PROJECT}"
REDIRECT_URIS="${REDIRECT_URIS:-[]}"

if [[ "$STACK" != "preview" ]]; then
  echo "refusing to configure stack '$STACK': the OAuth preview only uses the 'preview' stack" >&2
  exit 1
fi

# PULUMI_BACKEND_URL outranks Pulumi.yaml and the stored login, so every job
# reads and writes the same shared state (never runner-local files).
backend="gs://${PULUMI_STATE_BUCKET}"
if [[ -n "${PULUMI_BACKEND_URL:-}" && "$PULUMI_BACKEND_URL" != "$backend" ]]; then
  echo "refusing to run: PULUMI_BACKEND_URL does not point at the shared preview state bucket" >&2
  exit 1
fi
export PULUMI_BACKEND_URL="$backend"
pulumi login "$backend"
pulumi stack select "$STACK" --create --cwd "$PROGRAM_DIR"

set_cfg() { pulumi config set --stack "$STACK" --cwd "$PROGRAM_DIR" "gist-registry:$1" "$2"; }
set_cfg deploymentMode preview
set_cfg project "$GCP_PROJECT"
set_cfg region "$REGION"
set_cfg publicOrigin "$ORIGIN"
set_cfg resourceAudience "$ORIGIN"
set_cfg productionOrigin "$PRODUCTION_ORIGIN"
set_cfg containerImage "$IMAGE_REF"
set_cfg githubRepository "${GITHUB_REPOSITORY:-sirerun/gist}"
set_cfg stateBucket "gs://${PULUMI_STATE_BUCKET}"
set_cfg previewRunId "$RUN_ID"
set_cfg previewOwner "$OWNER"
set_cfg previewExpiresAt "$EXPIRES_AT"
set_cfg previewDnsZone "$DNS_ZONE"
set_cfg previewDnsProject "$DNS_PROJECT"
set_cfg databasePasswordSecret gist-registry-preview-database-password
set_cfg issuerSecret gist-registry-preview-oauth-issuer
set_cfg signingKeySecret gist-registry-preview-workload-signing-key
set_cfg consentSecretSecret gist-registry-preview-oauth-consent-secret
# A JSON array string; the program reads it with Config.get_object.
jq -e 'type == "array" and all(.[]; type == "string")' <<<"$REDIRECT_URIS" >/dev/null \
  || { echo "GIST_PREVIEW_OAUTH_REDIRECT_URIS must be a JSON array of exact HTTPS URIs" >&2; exit 1; }
set_cfg oauthRedirectUris "$REDIRECT_URIS"

echo "configured stack $STACK for run $RUN_ID (origin $ORIGIN, expires $EXPIRES_AT, owner $OWNER)"
