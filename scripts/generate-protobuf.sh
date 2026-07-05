#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROTO_SRC="${BUSYLIB_GO_PROTO_SRC:-${ROOT}/internal/protosrc/bsb-protobuf}"
OUT_DIR="${BUSYLIB_GO_PROTO_OUT:-${ROOT}}"
MANIFEST="${SCRIPT_DIR}/protobuf-packages.tsv"

# shellcheck source=protobuf-tools.env
source "${SCRIPT_DIR}/protobuf-tools.env"

MODULE="$(cd "${ROOT}" && go list -m)"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

if [[ "$(protoc --version)" != "${PROTOC_VERSION}" ]]; then
  echo "expected ${PROTOC_VERSION}; found $(protoc --version)" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"

TOOL_DIR="${TMP_DIR}/bin"
mkdir -p "${TOOL_DIR}"

if command -v protoc-gen-go >/dev/null 2>&1 &&
  [[ "$(protoc-gen-go --version)" == "protoc-gen-go ${PROTOC_GEN_GO_VERSION}" ]]; then
  cp "$(command -v protoc-gen-go)" "${TOOL_DIR}/protoc-gen-go"
else
  GOBIN="${TOOL_DIR}" GOCACHE="${BUSYLIB_GO_GOCACHE:-${TMP_DIR}/gocache}" \
    go install "google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}"

  if [[ "$("${TOOL_DIR}/protoc-gen-go" --version)" != "protoc-gen-go ${PROTOC_GEN_GO_VERSION}" ]]; then
    echo "failed to install protoc-gen-go ${PROTOC_GEN_GO_VERSION}" >&2
    exit 1
  fi
fi

manifest_paths="${TMP_DIR}/manifest-protos.txt"
discovered_paths="${TMP_DIR}/discovered-protos.txt"
go_opts=()
proto_files=()

while IFS=$'\t' read -r proto_path package_suffix; do
  [[ -z "${proto_path}" || "${proto_path}" == \#* ]] && continue
  if [[ -z "${package_suffix}" ]]; then
    echo "missing Go package suffix for ${proto_path}" >&2
    exit 1
  fi
  if [[ ! -f "${PROTO_SRC}/${proto_path}" ]]; then
    echo "manifest references missing proto: ${proto_path}" >&2
    exit 1
  fi
  printf '%s\n' "${proto_path}" >> "${manifest_paths}"
  go_opts+=("--go_opt=M${proto_path}=${MODULE}/${package_suffix}")
  proto_files+=("${proto_path}")
done < "${MANIFEST}"

(cd "${PROTO_SRC}" && find . -name '*.proto' -print | sed 's#^\./##' | sort) > "${discovered_paths}"
sort -o "${manifest_paths}" "${manifest_paths}"

if ! diff -u "${manifest_paths}" "${discovered_paths}" >&2; then
  echo "protobuf manifest does not match discovered .proto files" >&2
  exit 1
fi

PATH="${TOOL_DIR}:${PATH}" protoc \
  -I "${PROTO_SRC}" \
  --go_out="${OUT_DIR}" \
  --go_opt="module=${MODULE}" \
  "${go_opts[@]}" \
  "${proto_files[@]}"
