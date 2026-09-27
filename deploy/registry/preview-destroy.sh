#!/usr/bin/env bash
# Destroy the OAuth preview stack, fail-closed, and write sanitized evidence.
#
# Used by the end-of-run destroy stage, the manual destroy action, and the
# hourly TTL sweeper (ONLY_IF_EXPIRED=true). The stack itself is kept so its
# empty state remains inspectable evidence for O4; only its resources go.
set -euo pipefail

: "${PULUMI_STATE_BUCKET:?}"
STACK="${STACK:-preview}"
PROGRAM_DIR="${PROGRAM_DIR:-deploy/registry}"
REASON="${REASON:-manual}"
ONLY_IF_EXPIRED="${ONLY_IF_EXPIRED:-false}"
ATTEMPTS="${DESTROY_ATTEMPTS:-3}"
mkdir -p evidence

if [[ "$STACK" != "preview" ]]; then
  echo "refusing to destroy stack '$STACK': this script only tears down the 'preview' stack" >&2
  exit 1
fi

pulumi login "gs://${PULUMI_STATE_BUCKET}"
if ! pulumi stack select "$STACK" --cwd "$PROGRAM_DIR" 2>/dev/null; then
  echo "no preview stack exists; nothing to destroy"
  exit 0
fi

count_resources() {
  pulumi stack export --stack "$STACK" --cwd "$PROGRAM_DIR" \
    | jq '[.deployment.resources[]? | select(.type != "pulumi:pulumi:Stack" and (.type | startswith("pulumi:providers:") | not))] | length'
}

remaining="$(count_resources)"
cfg() { pulumi config get --stack "$STACK" --cwd "$PROGRAM_DIR" "gist-registry:$1" 2>/dev/null || true; }
run_id="$(cfg previewRunId)"
owner="$(cfg previewOwner)"
expires_at="$(cfg previewExpiresAt)"

if [[ "$remaining" == "0" ]]; then
  echo "preview stack has no resources; nothing to destroy"
  exit 0
fi

if [[ "$ONLY_IF_EXPIRED" == "true" ]]; then
  if [[ -z "$expires_at" ]]; then
    echo "preview has resources but no recorded expiry; treating as abandoned"
  elif ! python3 "$PROGRAM_DIR/preview.py" is-expired "$expires_at"; then
    echo "preview run ${run_id} (owner ${owner}) expires at ${expires_at}; not yet due"
    exit 0
  fi
fi

status=destroy_failed
for attempt in $(seq 1 "$ATTEMPTS"); do
  if pulumi destroy --stack "$STACK" --cwd "$PROGRAM_DIR" --yes --non-interactive --skip-preview; then
    remaining="$(count_resources)"
    if [[ "$remaining" == "0" ]]; then
      status=destroyed
      break
    fi
  fi
  echo "destroy attempt ${attempt} left ${remaining} resource(s); retrying" >&2
  sleep 30
done

jq -n --arg run_id "$run_id" --arg owner "$owner" --arg expires_at "$expires_at" --arg status "$status" \
  --arg reason "$REASON" --arg commit "${GITHUB_SHA:-unknown}" --argjson remaining "$remaining" \
  '{stack:"preview", run_id:$run_id, owner:$owner, expires_at:$expires_at, status:$status, reason:$reason,
    commit:$commit, remaining_resources:$remaining, destroyed_at:(now|todateiso8601)}' \
  > evidence/preview-teardown.json
cat evidence/preview-teardown.json

if [[ "$status" != "destroyed" ]]; then
  echo "::error::preview teardown failed with ${remaining} resource(s) remaining; owner ${owner} must clean up via this workflow before any M2b acceptance" >&2
  exit 1
fi
