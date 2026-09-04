#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PAHO_DIR="${ROOT}/pahotransport"
BLE_DIR="${ROOT}/ble"

# shellcheck source=verify-tools.env
source "${SCRIPT_DIR}/verify-tools.env"

TMP_DIR="$(mktemp -d)"
WORKSPACE="${TMP_DIR}/go.work"
BLE_WORKSPACE="${TMP_DIR}/ble.go.work"
MQTT_CONTAINER=""

cleanup() {
  if [[ -n "${MQTT_CONTAINER}" ]]; then
    docker stop "${MQTT_CONTAINER}" >/dev/null 2>&1 || true
  fi
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT INT TERM

usage() {
  cat <<'EOF'
usage: scripts/verify.sh <command>

commands:
  quick          Run current-toolchain tests and vet for all modules.
  docs           Check documentation structure, API coverage, links, and example compilation.
  minimum-root   Test the root module with its minimum Go toolchain and CGO off.
  minimum-ble    Test the BLE module with its minimum Go toolchain and CGO off.
  minimum-ble-darwin
                 Test the CoreBluetooth backend with the BLE minimum Go toolchain.
  minimum-paho   Test the Paho module with its minimum Go toolchain and CGO off.
  current-root   Test the root module on the current supported Go toolchain.
  current-ble    Test the BLE module on the current platform and toolchain.
  race           Run race tests for all modules.
  vet            Run standalone vet for all modules.
  coverage       Enforce the public-package coverage floor.
  lint           Run the pinned linter for all modules.
  repository     Validate shell scripts, Git diffs, and GitHub workflow syntax.
  metadata       Verify checksums and tidy module metadata without changing the tree.
  security       Run the pinned vulnerability scanner for all modules.
  generated      Verify generated protobuf code and focused API tests.
  firmware       Verify the pinned firmware contract checkout.
  integration    Run broker-backed Paho tests and compile device-tagged tests.
  fuzz           Run the frame fuzz target for BUSYLIB_FUZZ_TIME or 5 minutes.
  history        Validate Conventional Commits after the release bootstrap commit.
  device         Run physical local and USB device tests; required variables must be set.
  ble-device     Run opt-in physical BLE data-plane and bonded-reconnect tests.
  all            Run every device-free local gate.
  release        Run all gates for root and Paho releases, including local and USB tests.
  ble-release    Run all gates for a BLE release, including BLE qualification.
EOF
}

phase() {
  printf '\n==> %s\n' "$1"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 2
  fi
}

ensure_workspace() {
  if [[ -f "${WORKSPACE}" ]]; then
    return
  fi
  require_command go
  GOTOOLCHAIN="${PAHO_MIN_GO_TOOLCHAIN}" GOWORK="${WORKSPACE}" \
    go work init "${PAHO_DIR}"
  GOTOOLCHAIN="${PAHO_MIN_GO_TOOLCHAIN}" GOWORK="${WORKSPACE}" \
    go work edit -replace "github.com/lxdb/busylib-go=${ROOT}"
}

ensure_ble_workspace() {
  if [[ -f "${BLE_WORKSPACE}" ]]; then
    return
  fi
  require_command go
  GOTOOLCHAIN="${BLE_MIN_GO_TOOLCHAIN}" GOWORK="${BLE_WORKSPACE}" \
    go work init "${BLE_DIR}"
  GOTOOLCHAIN="${BLE_MIN_GO_TOOLCHAIN}" GOWORK="${BLE_WORKSPACE}" \
    go work edit -replace "github.com/lxdb/busylib-go=${ROOT}"
}

root_go() {
  local toolchain="$1"
  shift
  require_command go
  (cd "${ROOT}" && GOTOOLCHAIN="${toolchain}" GOWORK=off go "$@")
}

paho_go() {
  local toolchain="$1"
  shift
  ensure_workspace
  (cd "${PAHO_DIR}" && GOTOOLCHAIN="${toolchain}" GOWORK="${WORKSPACE}" go "$@")
}

ble_go() {
  local toolchain="$1"
  shift
  ensure_ble_workspace
  (cd "${BLE_DIR}" && GOTOOLCHAIN="${toolchain}" GOWORK="${BLE_WORKSPACE}" go "$@")
}

run_quick() {
  phase "root tests and vet (${CURRENT_GO_TOOLCHAIN})"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./...
  root_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...
  phase "Paho tests and vet (${CURRENT_GO_TOOLCHAIN})"
  paho_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./...
  paho_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...
  phase "BLE tests and vet (${CURRENT_GO_TOOLCHAIN})"
  ble_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./...
  ble_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...
}

run_docs() {
  phase "documentation contracts"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./internal/docscheck
  phase "root examples (compile all; run examples with expected output)"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -run '^Example' ./...
  phase "Paho examples (compile all; run examples with expected output)"
  paho_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -run '^Example' ./...
  phase "BLE examples (compile only; physical examples have no expected output)"
  ble_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -run '^Example' ./...
}

run_minimum_root() {
  phase "root minimum toolchain (${ROOT_MIN_GO_TOOLCHAIN}, CGO disabled)"
  CGO_ENABLED=0 root_go "${ROOT_MIN_GO_TOOLCHAIN}" test -mod=readonly ./...
}

run_minimum_ble() {
  phase "BLE minimum toolchain (${BLE_MIN_GO_TOOLCHAIN}, CGO disabled)"
  CGO_ENABLED=0 ble_go "${BLE_MIN_GO_TOOLCHAIN}" test -mod=readonly ./...
}

run_minimum_ble_darwin() {
  if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "minimum-ble-darwin requires macOS" >&2
    exit 2
  fi
  phase "CoreBluetooth minimum toolchain (${BLE_MIN_GO_TOOLCHAIN}, CGO enabled)"
  CGO_ENABLED=1 ble_go "${BLE_MIN_GO_TOOLCHAIN}" test -mod=readonly ./...
  CGO_ENABLED=1 ble_go "${BLE_MIN_GO_TOOLCHAIN}" vet -mod=readonly ./...
}

run_minimum_paho() {
  phase "Paho minimum toolchain (${PAHO_MIN_GO_TOOLCHAIN}, CGO disabled)"
  CGO_ENABLED=0 paho_go "${PAHO_MIN_GO_TOOLCHAIN}" test -mod=readonly ./...
}

run_current_root() {
  phase "root current toolchain (${CURRENT_GO_TOOLCHAIN})"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./...
}

run_current_ble() {
  phase "BLE current platform and toolchain (${CURRENT_GO_TOOLCHAIN})"
  ble_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./...
  ble_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...
  if [[ "$(uname -s)" == "Darwin" ]]; then
    phase "BLE device-test compilation on macOS (${CURRENT_GO_TOOLCHAIN})"
    CGO_ENABLED=1 ble_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -run '^$' -tags=device ./...
    CGO_ENABLED=1 ble_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly -tags=device ./...
  fi
}

run_race() {
  phase "root race tests (${CURRENT_GO_TOOLCHAIN})"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -race -count=1 ./...
  phase "Paho race tests (${CURRENT_GO_TOOLCHAIN})"
  paho_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -race -count=1 ./...
  phase "BLE race tests (${CURRENT_GO_TOOLCHAIN})"
  ble_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -race -count=1 ./...
}

run_vet() {
  phase "root vet (${CURRENT_GO_TOOLCHAIN})"
  root_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...
  phase "Paho vet (${CURRENT_GO_TOOLCHAIN})"
  paho_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...
  phase "BLE vet (${CURRENT_GO_TOOLCHAIN})"
  ble_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...
}

run_coverage() {
  phase "public-package coverage"
  (cd "${ROOT}" && GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off GOFLAGS=-mod=readonly \
    "${SCRIPT_DIR}/test-coverage.sh")
}

run_lint() {
  phase "root lint (${GOLANGCI_LINT_VERSION}, ${LINT_GO_TOOLCHAIN})"
  root_go "${LINT_GO_TOOLCHAIN}" run \
    "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" run
  phase "Paho lint (${GOLANGCI_LINT_VERSION}, ${LINT_GO_TOOLCHAIN})"
  paho_go "${LINT_GO_TOOLCHAIN}" run \
    "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" run
  phase "BLE lint (${GOLANGCI_LINT_VERSION}, ${LINT_GO_TOOLCHAIN})"
  ble_go "${LINT_GO_TOOLCHAIN}" run \
    "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" run
}

run_repository() {
  phase "shell syntax"
  bash -n "${SCRIPT_DIR}"/*.sh
  phase "Git diff whitespace"
  (cd "${ROOT}" && git diff --check HEAD)
  phase "GitHub workflow syntax (${ACTIONLINT_VERSION})"
  root_go "${CURRENT_GO_TOOLCHAIN}" run \
    "github.com/rhysd/actionlint/cmd/actionlint@${ACTIONLINT_VERSION}"
}

run_metadata() {
  phase "root module metadata"
  root_go "${CURRENT_GO_TOOLCHAIN}" mod verify
  root_go "${CURRENT_GO_TOOLCHAIN}" mod tidy -diff

  phase "Paho module metadata"
  local copy_dir="${TMP_DIR}/pahotransport-tidy"
  cp -R "${PAHO_DIR}" "${copy_dir}"
  (
    cd "${copy_dir}"
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off \
      go mod edit -replace "github.com/lxdb/busylib-go=${ROOT}"
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off go mod tidy
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off go mod verify
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off \
      go mod edit -dropreplace github.com/lxdb/busylib-go
  )
  diff -u "${PAHO_DIR}/go.mod" "${copy_dir}/go.mod"
  grep -v '^github.com/lxdb/busylib-go ' "${PAHO_DIR}/go.sum" > "${TMP_DIR}/pahotransport.go.sum"
  grep -v '^github.com/lxdb/busylib-go ' "${copy_dir}/go.sum" > "${TMP_DIR}/pahotransport-copy.go.sum"
  diff -u "${TMP_DIR}/pahotransport.go.sum" "${TMP_DIR}/pahotransport-copy.go.sum"

  phase "BLE module metadata"
  copy_dir="${TMP_DIR}/ble-tidy"
  cp -R "${BLE_DIR}" "${copy_dir}"
  (
    cd "${copy_dir}"
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off \
      go mod edit -replace "github.com/lxdb/busylib-go=${ROOT}"
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off go mod tidy
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off go mod verify
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off \
      go mod edit -dropreplace github.com/lxdb/busylib-go
  )
  diff -u "${BLE_DIR}/go.mod" "${copy_dir}/go.mod"
  grep -v '^github.com/lxdb/busylib-go ' "${BLE_DIR}/go.sum" > "${TMP_DIR}/ble.go.sum"
  grep -v '^github.com/lxdb/busylib-go ' "${copy_dir}/go.sum" > "${TMP_DIR}/ble-copy.go.sum"
  diff -u "${TMP_DIR}/ble.go.sum" "${TMP_DIR}/ble-copy.go.sum"
}

run_security() {
  phase "root vulnerability scan (${GOVULNCHECK_VERSION}, ${CURRENT_GO_TOOLCHAIN})"
  root_go "${CURRENT_GO_TOOLCHAIN}" run \
    "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" ./...
  phase "Paho vulnerability scan (${GOVULNCHECK_VERSION}, ${CURRENT_GO_TOOLCHAIN})"
  paho_go "${CURRENT_GO_TOOLCHAIN}" run \
    "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" ./...
  phase "BLE vulnerability scan (${GOVULNCHECK_VERSION}, ${CURRENT_GO_TOOLCHAIN})"
  ble_go "${CURRENT_GO_TOOLCHAIN}" run \
    "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" ./...
}

run_generated() {
  local proto_source="${BUSYLIB_GO_PROTO_SRC:-${ROOT}/../busybar-protobuf}"
  if [[ "${proto_source}" != /* ]]; then
    proto_source="${ROOT}/${proto_source}"
  fi
  phase "generated protobuf contract"
  (cd "${ROOT}" && BUSYLIB_GO_PROTO_SRC="${proto_source}" \
    GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off \
    "${SCRIPT_DIR}/check-protobuf.sh")
  BUSYLIB_GO_PROTO_SRC="${proto_source}" \
    root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./internal/api/...
}

run_firmware() {
  local firmware_dir="${BUSYBAR_FIRMWARE_DIR:-}"
  if [[ -z "${firmware_dir}" && -d "${ROOT}/busybar-firmware" ]]; then
    firmware_dir="${ROOT}/busybar-firmware"
  fi
  if [[ -z "${firmware_dir}" ]]; then
    firmware_dir="${ROOT}/../busybar-firmware"
  fi
  if [[ ! -d "${firmware_dir}" ]]; then
    echo "BUSYBAR_FIRMWARE_DIR must point to a busybar-firmware checkout" >&2
    exit 2
  fi
  phase "firmware contract (${firmware_dir})"
  (cd "${ROOT}" && GOTOOLCHAIN="${CURRENT_GO_TOOLCHAIN}" GOWORK=off \
    BUSYBAR_FIRMWARE_DIR="${firmware_dir}" "${SCRIPT_DIR}/check-firmware-contract.sh")
}

wait_for_mqtt() {
  local attempt
  for attempt in {1..100}; do
    if docker logs "${MQTT_CONTAINER}" 2>&1 | grep -q 'mosquitto version .* running'; then
      return
    fi
    sleep 0.1
  done
  docker logs "${MQTT_CONTAINER}" >&2 || true
  echo "MQTT broker did not become ready" >&2
  exit 1
}

run_integration() {
  require_command docker
  local port="${BUSYLIB_MQTT_PORT:-18883}"
  MQTT_CONTAINER="busylib-go-mqtt-$$"
  phase "MQTT broker (${MOSQUITTO_IMAGE})"
  docker run -d --rm --name "${MQTT_CONTAINER}" \
    -p "127.0.0.1:${port}:1883" \
    -v "${PAHO_DIR}/testdata/mosquitto.conf:/mosquitto/config/mosquitto.conf:ro" \
    "${MOSQUITTO_IMAGE}" >/dev/null
  wait_for_mqtt

  phase "broker-backed Paho race tests"
  BUSYLIB_MQTT_BROKER_URL="mqtt://127.0.0.1:${port}" \
    paho_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -race -count=1 ./...
  paho_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly ./...

  phase "root device-tag compilation and vet"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -run '^$' -tags=device ./integration/device
  root_go "${CURRENT_GO_TOOLCHAIN}" vet -mod=readonly -tags=device ./integration/device
}

run_fuzz() {
  local fuzz_time="${BUSYLIB_FUZZ_TIME:-${DEFAULT_FUZZ_TIME}}"
  phase "frame fuzz target (${fuzz_time})"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly ./frame -run '^$' \
    -fuzz FuzzPixelsPreservesFrameInvariants -fuzztime="${fuzz_time}"
}

run_history() {
  local range="${BUSYLIB_COMMIT_RANGE:-}"
  if [[ -z "${range}" ]]; then
    local bootstrap_sha
    bootstrap_sha="$(awk -F'"' '/"bootstrap-sha"/ {print $4; exit}' "${ROOT}/release-please-config.json")"
    if [[ -z "${bootstrap_sha}" ]]; then
      echo "release-please-config.json does not define bootstrap-sha" >&2
      exit 1
    fi
    range="${bootstrap_sha}..HEAD"
  fi
  phase "Conventional Commit history (${range})"
  (cd "${ROOT}" && "${SCRIPT_DIR}/check-conventional-commits.sh" "${range}")
}

run_device() {
  if [[ -z "${BUSYBAR_BASE_URL:-}" ]]; then
    echo "BUSYBAR_BASE_URL is required for physical local-device verification" >&2
    exit 2
  fi
  if [[ -z "${BUSYBAR_USB_ADDRESS:-}" ]]; then
    echo "BUSYBAR_USB_ADDRESS is required for physical USB verification" >&2
    exit 2
  fi
  if [[ -z "${BUSYBAR_EXPECTED_FIRMWARE_VERSION:-}" ]]; then
    echo "BUSYBAR_EXPECTED_FIRMWARE_VERSION is required for physical local-device verification" >&2
    exit 2
  fi
  if [[ -z "${BUSYBAR_EXPECTED_API_VERSION:-}" ]]; then
    echo "BUSYBAR_EXPECTED_API_VERSION is required for physical local-device verification" >&2
    exit 2
  fi
  phase "physical local HTTP and WebSocket tests except selective clear"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -count=1 -tags=device \
    -run '^TestLocalDevice' -skip '^TestLocalDeviceSelectiveClear$' -v ./integration/device
  phase "physical USB tests"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -count=1 -tags=device \
    -run '^TestUSBDevice' -v ./integration/device
  phase "physical selective-clear test (final; firmware 1.2.3 restart risk)"
  root_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -count=1 -tags=device \
    -run '^TestLocalDeviceSelectiveClear$' -v ./integration/device
}

require_ble_device_identifier() {
  if [[ -z "${BUSYBAR_BLE_IDENTIFIER:-}" ]]; then
    echo "BUSYBAR_BLE_IDENTIFIER is required for physical BLE verification" >&2
    exit 2
  fi
}

run_ble_device() {
  require_ble_device_identifier
  phase "physical BLE data-plane test"
  ble_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -count=1 -tags=device \
    -run '^TestBLEDeviceDataPlane$' -v ./...
  phase "physical BLE bonded-reconnect qualification (30 cycles)"
  ble_go "${CURRENT_GO_TOOLCHAIN}" test -mod=readonly -count=30 -tags=device \
    -run '^TestBLEDeviceBondedReconnect$' -v ./...
}

require_ble_release_inputs() {
  require_ble_device_identifier
  if [[ "${BUSYBAR_BLE_WRITE_TEST:-}" != "1" ]]; then
    echo "BUSYBAR_BLE_WRITE_TEST=1 is required for BLE release qualification" >&2
    exit 2
  fi
}

run_all() {
  run_repository
  run_docs
  run_history
  run_minimum_root
  run_minimum_ble
  run_minimum_paho
  run_current_root
  run_current_ble
  run_race
  run_vet
  run_coverage
  run_lint
  run_metadata
  run_security
  run_generated
  run_firmware
  run_integration
  run_fuzz
}

case "${1:-}" in
  quick) run_quick ;;
  docs) run_docs ;;
  minimum-root) run_minimum_root ;;
  minimum-ble) run_minimum_ble ;;
  minimum-ble-darwin) run_minimum_ble_darwin ;;
  minimum-paho) run_minimum_paho ;;
  current-root) run_current_root ;;
  current-ble) run_current_ble ;;
  race) run_race ;;
  vet) run_vet ;;
  coverage) run_coverage ;;
  lint) run_lint ;;
  repository) run_repository ;;
  metadata) run_metadata ;;
  security) run_security ;;
  generated) run_generated ;;
  firmware) run_firmware ;;
  integration) run_integration ;;
  fuzz) run_fuzz ;;
  history) run_history ;;
  device) run_device ;;
  ble-device) run_ble_device ;;
  all) run_all ;;
  release)
    run_all
    run_device
    ;;
  ble-release)
    require_ble_release_inputs
    run_all
    run_minimum_ble_darwin
    run_ble_device
    ;;
  -h|--help|help) usage ;;
  *)
    usage >&2
    exit 2
    ;;
esac
