#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT}"
go run ./internal/api/cmd/openapi-inventory \
  -input internal/api/testdata/busybar-f21-openapi-1.0.0-rc.yaml \
  -output internal/api/testdata/operations.json
