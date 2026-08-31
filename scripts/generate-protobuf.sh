#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROTO_SRC="${BUSYLIB_GO_PROTO_SRC:-${ROOT}/../busybar-protobuf}"
OUT_DIR="${BUSYLIB_GO_PROTO_OUT:-${ROOT}}"
MANIFEST="${SCRIPT_DIR}/protobuf-packages.tsv"

# shellcheck source=protobuf-tools.env
source "${SCRIPT_DIR}/protobuf-tools.env"
# shellcheck source=protobuf-source.env
source "${SCRIPT_DIR}/protobuf-source.env"

if [[ ! -d "${PROTO_SRC}" ]]; then
  echo "BUSYLIB_GO_PROTO_SRC must point to a busybar-protobuf checkout: ${PROTO_SRC}" >&2
  exit 2
fi
if [[ ! -f "${PROTO_SRC}/LICENSE.md" ]] ||
  ! cmp -s "${ROOT}/LICENSES/busybar-protobuf-MIT.txt" "${PROTO_SRC}/LICENSE.md"; then
  echo "busybar-protobuf LICENSE.md does not match LICENSES/busybar-protobuf-MIT.txt" >&2
  exit 1
fi
if ! command -v shasum >/dev/null 2>&1; then
  echo "required command not found: shasum" >&2
  exit 2
fi
if [[ ! -f "${PROTO_SRC}/frame.options" ]]; then
  echo "busybar-protobuf checkout is missing frame.options" >&2
  exit 1
fi
frame_options_sha256="$(shasum -a 256 "${PROTO_SRC}/frame.options" | awk '{print $1}')"
if [[ "${frame_options_sha256}" != "${PROTOBUF_FRAME_OPTIONS_SHA256}" ]]; then
  echo "protobuf source digest mismatch for frame.options" >&2
  exit 1
fi

MODULE="$(cd "${ROOT}" && GOTOOLCHAIN="${PROTOC_GEN_GO_TOOLCHAIN}" go list -m)"
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

GOTOOLCHAIN="${PROTOC_GEN_GO_TOOLCHAIN}" GOBIN="${TOOL_DIR}" \
  GOCACHE="${BUSYLIB_GO_GOCACHE:-${TMP_DIR}/gocache}" \
  go install "google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}"

if [[ "$("${TOOL_DIR}/protoc-gen-go" --version)" != "protoc-gen-go ${PROTOC_GEN_GO_VERSION}" ]]; then
  echo "failed to install protoc-gen-go ${PROTOC_GEN_GO_VERSION}" >&2
  exit 1
fi

manifest_paths="${TMP_DIR}/manifest-protos.txt"
discovered_paths="${TMP_DIR}/discovered-protos.txt"
go_opts=()
proto_files=()

while IFS=$'\t' read -r proto_path package_suffix expected_sha256; do
  [[ -z "${proto_path}" || "${proto_path}" == \#* ]] && continue
  if [[ -z "${package_suffix}" || ! "${expected_sha256}" =~ ^[0-9a-f]{64}$ ]]; then
    echo "invalid package mapping or SHA-256 for ${proto_path}" >&2
    exit 1
  fi
  if [[ ! -f "${PROTO_SRC}/${proto_path}" ]]; then
    echo "manifest references missing proto: ${proto_path}" >&2
    exit 1
  fi
  actual_sha256="$(shasum -a 256 "${PROTO_SRC}/${proto_path}" | awk '{print $1}')"
  if [[ "${actual_sha256}" != "${expected_sha256}" ]]; then
    echo "protobuf source digest mismatch for ${proto_path}" >&2
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
