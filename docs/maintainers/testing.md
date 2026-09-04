# Test busylib-go

`scripts/verify.sh` is the executable verification contract for local development and CI. Workflow files select runners and call its named phases.

## Select a verification phase

| Command | Evidence produced | External requirements |
| --- | --- | --- |
| `scripts/verify.sh quick` | Current-toolchain tests and vet for both modules | Go toolchain and module cache or network |
| `scripts/verify.sh docs` | Documentation structure, links, API coverage, and examples | Go toolchain |
| `scripts/verify.sh minimum-root` | Root tests with the minimum Go toolchain and CGO disabled | Minimum Go toolchain |
| `scripts/verify.sh minimum-paho` | Adapter tests with its minimum Go toolchain and CGO disabled | Minimum Go toolchain |
| `scripts/verify.sh current-root` | Root tests with the current supported toolchain | Current Go toolchain |
| `scripts/verify.sh race` | Race-enabled tests for both modules | CGO-capable Go environment |
| `scripts/verify.sh vet` | Standalone vet for both modules | Go toolchain |
| `scripts/verify.sh coverage` | Public-package coverage floor | Go toolchain |
| `scripts/verify.sh lint` | Pinned linter for both modules | Tool download or cache |
| `scripts/verify.sh repository` | Shell syntax, Git whitespace, and workflow syntax | Git and tool download or cache |
| `scripts/verify.sh metadata` | Checksums and tidy module metadata without changing the checkout | Module cache or network |
| `scripts/verify.sh security` | Pinned vulnerability scan for both modules | Vulnerability data access |
| `scripts/verify.sh generated` | Generated protobuf and focused protocol checks | `protoc` and protobuf source checkout |
| `scripts/verify.sh firmware` | Pinned firmware contract checks | Firmware checkout |
| `scripts/verify.sh integration` | Broker-backed Paho tests and device-tag compilation | Docker and pinned Mosquitto image |
| `scripts/verify.sh fuzz` | Scheduled frame fuzz target | Go toolchain and configured time |
| `scripts/verify.sh history` | Conventional Commit history after bootstrap | Git history |
| `scripts/verify.sh device` | Physical HTTP, WebSocket, media, and USB behavior | BUSY Bar and both addresses |
| `scripts/verify.sh all` | Every device-free local gate | All device-free requirements above |
| `scripts/verify.sh release` | All gates followed by physical-device tests | Complete release environment |

## Run complete device-free verification

The complete harness requires Go, Docker, `protoc` at the version declared in `scripts/protobuf-tools.env`, access to pinned tools and vulnerability data, and the protobuf and firmware source checkouts.

```sh
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware scripts/verify.sh all
```

Set `BUSYLIB_GO_PROTO_SRC` when the protobuf checkout is not at `../busybar-protobuf`. Set `BUSYLIB_FUZZ_TIME` only for a focused diagnostic run; release evidence uses the declared default.

## Run physical-device verification

Set both addresses so the harness cannot pass through skipped device tests:

```sh
BUSYBAR_BASE_URL=http://device-address \
BUSYBAR_USB_ADDRESS=device-usb-address \
scripts/verify.sh device
```

Set `BUSYBAR_ACCESS_KEY` only when the local HTTP API requires it. The device phase verifies local HTTP snapshots, WebSocket lifecycle, USB commands, and media upload, read-back, and cleanup under a unique application name.

A hosted build can compile device-tagged tests but cannot prove device reachability, firmware compatibility, or media behavior. Record the device model, firmware version, exact command, and result without credentials or device tokens. See the [device test README](../../integration/device/README.md) for the test boundary.

## Write useful tests

Test observable contracts. Use exact counts when the contract defines an exact set, synchronization barriers for concurrent behavior, and deadlines only as failure bounds. A sleep does not prove that concurrent work completed.

For remote transports, cover connection loss, reconnection, restored subscriptions, oversized messages, slow consumers, and concurrent close. For one-shot streams, cover start, terminal completion, cleanup errors, repeated `Wait`, and cancellation.

## Understand GitHub-only evidence

CodeQL, pull-request dependency review, pull-request title validation, and release publication depend on GitHub event or service state. They supplement the local harness and do not replace it. A release candidate needs the applicable local evidence and GitHub service checks.
