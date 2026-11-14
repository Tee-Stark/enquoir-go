#!/usr/bin/env bash
# Re-vendors api/openapi.yaml from the backend repo. Run after any backend
# API change, before contract_test.go can confirm the SDK still matches it.
#
# Usage: scripts/sync-openapi.sh [path-to-enquoir-backend-repo]
# Defaults to ../enquoir, matching this repo's sibling layout.
#
# scripts/sync-openapi.sh --check [path-to-enquoir-backend-repo] exits
# non-zero if the vendored copy has drifted from the backend's copy,
# without touching either file - the check CI runs on every push.
set -euo pipefail

vendored="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/openapi/openapi.yaml"

check=false
if [[ "${1:-}" == "--check" ]]; then
  check=true
  shift
fi
backend_repo="${1:-../enquoir}"
source="$backend_repo/api/openapi.yaml"

if [[ ! -f "$source" ]]; then
  echo "sync-openapi.sh: $source not found" >&2
  echo "usage: scripts/sync-openapi.sh [--check] [path-to-enquoir-backend-repo]" >&2
  exit 1
fi

if "$check"; then
  if ! diff -q "$source" "$vendored" >/dev/null 2>&1; then
    echo "sync-openapi --check: $vendored has drifted from $source" >&2
    echo "run scripts/sync-openapi.sh to re-vendor" >&2
    exit 1
  fi
  echo "sync-openapi --check: up to date"
  exit 0
fi

cp "$source" "$vendored"
echo "vendored $source -> $vendored"
