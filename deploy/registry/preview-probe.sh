#!/usr/bin/env bash
# Probe a deployed OAuth preview through its public origin.
#
# Proves the ingress reaches the application (an application OAuth challenge,
# not an infrastructure denial), TLS is valid for the preview host, HTTP
# redirects to HTTPS, and the protected-resource metadata names the preview
# audience. Sends no credentials.
set -euo pipefail

: "${ORIGIN:?}"
REQUIRE_PRM="${REQUIRE_PRM:-true}"
TLS_WAIT_SECONDS="${TLS_WAIT_SECONDS:-4500}"
PRM_PATH="/.well-known/oauth-protected-resource"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# Google-managed certificates take 15-60 minutes to provision after DNS resolves.
deadline=$((SECONDS + TLS_WAIT_SECONDS))
until curl --silent --fail --max-time 10 --output /dev/null "${ORIGIN}/healthz"; do
  if (( SECONDS >= deadline )); then
    echo "preview origin ${ORIGIN} did not serve /healthz over valid TLS within ${TLS_WAIT_SECONDS}s" >&2
    exit 1
  fi
  sleep 30
done
echo "PASS healthz over managed TLS"

host="${ORIGIN#https://}"
redirect="$(curl --silent --output /dev/null --write-out '%{http_code} %{redirect_url}' "http://${host}/healthz")"
case "$redirect" in
  "301 https://${host}/healthz"|"308 https://${host}/healthz") echo "PASS http redirects to https" ;;
  *) echo "FAIL http did not redirect to https: $redirect" >&2; exit 1 ;;
esac

# Unauthenticated MCP call must reach the application and get its bearer challenge.
status="$(curl --silent --output "$work/mcp.body" --dump-header "$work/mcp.headers" --write-out '%{http_code}' \
  --request POST --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' "${ORIGIN}/mcp")"
if [[ "$status" != "401" ]] || ! grep -qi '^www-authenticate: *bearer' "$work/mcp.headers"; then
  echo "FAIL /mcp returned $status without an application bearer challenge (infrastructure denial?)" >&2
  sed -n '1,20p' "$work/mcp.headers" >&2
  exit 1
fi
echo "PASS /mcp returns the application 401 bearer challenge"
if grep -qi 'resource_metadata=' "$work/mcp.headers"; then
  grep -qi "resource_metadata=\"${ORIGIN}${PRM_PATH}\"" "$work/mcp.headers" \
    || { echo "FAIL challenge resource_metadata does not point at ${ORIGIN}${PRM_PATH}" >&2; exit 1; }
  echo "PASS challenge advertises the preview protected-resource metadata"
fi

prm_status="$(curl --silent --output "$work/prm.json" --write-out '%{http_code}' "${ORIGIN}${PRM_PATH}")"
if [[ "$prm_status" == "200" ]] && jq -e --arg origin "$ORIGIN" '.resource == $origin' "$work/prm.json" >/dev/null; then
  echo "PASS protected-resource metadata names audience ${ORIGIN}"
elif [[ "$REQUIRE_PRM" == "true" ]]; then
  echo "FAIL protected-resource metadata missing or wrong audience (status ${prm_status})" >&2
  exit 1
else
  echo "WARN protected-resource metadata not served yet (status ${prm_status}); allowed only before I4"
fi
