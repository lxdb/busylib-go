#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COVERAGE_FILE="${ROOT}/coverage.out"
RAW_FILE="$(mktemp)"

cleanup() {
  rm -f "${RAW_FILE}"
}
trap cleanup EXIT

cd "${ROOT}"
# Measure each package through its own tests. Cross-package instrumentation adds
# synthetic uncovered blocks for packages that a different test binary imports.
go test -coverprofile="${RAW_FILE}" ./...
awk '
  NR == 1 { mode = $0; next }
  # The publication floor covers handwritten public packages. Internal tooling
  # and generated protobuf packages still run above but do not dilute this API metric.
  $1 ~ /\/proto\// || $1 ~ /\/internal\// { next }
  {
    statements[$1] = $2
    if ($3 > 0) {
      covered[$1] = 1
    }
  }
  END {
    print mode
    for (block in statements) {
      print block, statements[block], covered[block] + 0
    }
  }
' "${RAW_FILE}" > "${COVERAGE_FILE}"

coverage="$(go tool cover -func="${COVERAGE_FILE}" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
minimum="${BUSYLIB_MIN_COVERAGE:-75.0}"

awk -v coverage="${coverage}" -v minimum="${minimum}" 'BEGIN {
  if (coverage + 0 < minimum + 0) {
    printf "public package coverage %.1f%% is below %.1f%%\n", coverage, minimum > "/dev/stderr"
    exit 1
  }
  printf "public package coverage %.1f%% meets %.1f%%\n", coverage, minimum
}'
