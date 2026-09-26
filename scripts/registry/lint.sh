#!/bin/sh
set -eu

milestone=${1:-}
if [ -z "$milestone" ]; then
  echo "usage: $0 M1|M2a|M2b|M3" >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "lint: go is required" >&2; exit 1; }
command -v gofmt >/dev/null 2>&1 || { echo "lint: gofmt is required" >&2; exit 1; }
command -v goimports >/dev/null 2>&1 || { echo "lint: goimports is required" >&2; exit 1; }
command -v golangci-lint >/dev/null 2>&1 || { echo "lint: pinned golangci-lint is required" >&2; exit 1; }

echo "lint tool versions"
go version
gofmt --version 2>&1 | head -n 1
goimports -version 2>&1 | head -n 1
golangci-lint version | head -n 1

owned_go=$(find "$repo_root/hosted/cmd/registry" "$repo_root/hosted/internal/app" "$repo_root/hosted/acceptance/wiring" -type f -name '*.go' -print 2>/dev/null || true)
if [ -n "$owned_go" ]; then
  [ -z "$(printf '%s\n' "$owned_go" | xargs gofmt -l)" ] || { echo "lint: gofmt changes required" >&2; exit 1; }
  [ -z "$(printf '%s\n' "$owned_go" | xargs goimports -l)" ] || { echo "lint: goimports changes required" >&2; exit 1; }
fi

if [ -f "$repo_root/go.mod" ]; then
  (cd "$repo_root" && GOWORK=off go vet ./...)
fi
if [ -f "$repo_root/hosted/go.mod" ]; then
  (cd "$repo_root/hosted" && GOWORK=off GOFLAGS=-mod=readonly go vet ./...)
fi

python3 "$script_dir/check.py" contracts
python3 -m compileall -q "$script_dir"
echo "lint: PASS ($milestone)"
