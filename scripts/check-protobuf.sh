#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

BUSYLIB_GO_PROTO_OUT="${TMP_DIR}" "${ROOT}/scripts/generate-protobuf.sh"
# Package documentation is maintained by hand; compare generated files only.
diff -ru -x doc.go "${ROOT}/proto" "${TMP_DIR}/proto"
