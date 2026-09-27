#!/usr/bin/env bash
# Destroy the OAuth preview stack, fail-closed, and write sanitized evidence.
#
# Used by the end-of-run destroy stage, the manual destroy action, and the
# hourly TTL sweeper (ONLY_IF_EXPIRED=true). The stack itself is kept so its
# empty state remains inspectable evidence for O4; only its resources go.
#
# State always lives in the shared gs:// backend. PULUMI_BACKEND_URL is exported
# because it outranks both Pulumi.yaml and the stored `pulumi login`, so a job
# can never fall back to runner-local state and report a false "nothing to
# destroy". EXPECT_STACK=true (set when this run created the stack) turns a
# missing stack into a failure instead of a green no-op.
set -euo pipefail

: "${PULUMI_STATE_BUCKET:?}"
STACK="${STACK:-preview}"
PROGRAM_DIR="${PROGRAM_DIR:-deploy/registry}"
REASON="${REASON:-manual}"
ONLY_IF_EXPIRED="${ONLY_IF_EXPIRED:-false}"
EXPECT_STACK="${EXPECT_STACK:-false}"
ATTEMPTS="${DESTROY_ATTEMPTS:-3}"
RETRY_SLEEP="${DESTROY_RETRY_SLEEP:-30}"
mkdir -p evidence

if [[ "$STACK" != "preview" ]]; then
  echo "refusing to destroy stack '$STACK': this script only tears down the 'preview' stack" >&2
  exit 1
fi

backend="gs://${PULUMI_STATE_BUCKET}"
if [[ -n "${PULUMI_BACKEND_URL:-}" && "$PULUMI_BACKEND_URL" != "$backend" ]]; then
  echo "refusing to run: PULUMI_BACKEND_URL does not point at the shared preview state bucket" >&2
  exit 1
fi
export PULUMI_BACKEND_URL="$backend"
pulumi login "$backend"

# List stacks explicitly so an auth or backend error fails the job instead of
# being mistaken for "no stack".
stacks="$(pulumi stack ls --json --cwd "$PROGRAM_DIR")"
if ! jq -e --arg s "$STACK" 'any(.[]; .name == $s or (.name | endswith("/" + $s)))' <<<"$stacks" >/dev/null; then
  if [[ "$EXPECT_STACK" == "true" ]]; then
    echo "::error::this run created the preview stack but it is missing from the shared backend; resources may be orphaned" >&2
    exit 1
  fi
  echo "no preview stack exists in the shared backend; nothing to destroy"
  exit 0
fi
pulumi stack select "$STACK" --cwd "$PROGRAM_DIR"

count_resources() {
  pulumi stack export --stack "$STACK" --cwd "$PROGRAM_DIR" \
    | jq '[.deployment.resources[]? | select(.type != "pulumi:pulumi:Stack" and (.type | startswith("pulumi:providers:") | not))] | length'
}

# Owner, run id and expiry come from stack outputs in the shared backend. The
# stack config file (Pulumi.preview.yaml) is runner-local and not committed, so
# a fresh checkout would read it as empty.
outputs="$(pulumi stack output --json --stack "$STACK" --cwd "$PROGRAM_DIR")"
out() { jq -r --arg k "$1" '.[$k] // "" | tostring' <<<"$outputs"; }
run_id="$(out preview_run_id)"
owner="$(out preview_owner)"
expires_at="$(out preview_expires_at)"

remaining="$(count_resources)"
if [[ "$remaining" == "0" ]]; then
  echo "preview stack has no resources; nothing to destroy"
  exit 0
fi

if [[ "$ONLY_IF_EXPIRED" == "true" ]]; then
  if [[ -z "$expires_at" ]]; then
    echo "preview has resources but no recorded expiry output; treating as abandoned"
  elif python3 "$PROGRAM_DIR/preview.py" is-expired "$expires_at"; then
    echo "preview run ${run_id} (owner ${owner}) expired at ${expires_at}; destroying"
  else
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
  sleep "$RETRY_SLEEP"
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
