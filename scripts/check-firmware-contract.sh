#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -z "${BUSYBAR_FIRMWARE_DIR:-}" ]]; then
  echo "BUSYBAR_FIRMWARE_DIR must point to a busybar-firmware checkout" >&2
  exit 2
fi

cd "${ROOT}"
exec go run ./internal/api/cmd/firmware-contract-check \
  -firmware-dir "${BUSYBAR_FIRMWARE_DIR}"
