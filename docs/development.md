# Development

Run repository checks from the root of an authorized checkout. `scripts/verify.sh` is the executable verification contract for local development and CI; the workflow files select machines and invoke its named phases.

## Fast feedback

Install a Go toolchain with automatic toolchain downloads enabled, then run:

```sh
scripts/verify.sh quick
```

The harness reads exact Go, linter, vulnerability-scanner, broker-image, and fuzz versions from `scripts/verify-tools.env`. It creates the temporary Paho workspace outside the repository and never adds a local replacement to a committed module file.

Run a focused phase while developing:

```sh
scripts/verify.sh lint
scripts/verify.sh repository
scripts/verify.sh metadata
scripts/verify.sh security
scripts/verify.sh integration
```

Tests should assert observable behavior. Use exact counts when the contract defines an exact set, synchronization barriers for concurrent behavior, and deadlines only as failure bounds. A sleep is not evidence that concurrent work completed correctly.

## Complete device-free verification

The complete device-free harness requires Go, Docker, `protoc` at the version declared in `scripts/protobuf-tools.env`, network access for pinned tools and vulnerability data, and sibling `../busybar-protobuf` and `../busybar-firmware` checkouts. Set `BUSYLIB_GO_PROTO_SRC` or `BUSYBAR_FIRMWARE_DIR` to use another path.

```sh
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware scripts/verify.sh all
```

`all` validates repository and workflow syntax, Conventional Commit history, minimum and current Go versions, tests, race behavior, vet, coverage, lint, module metadata, known vulnerabilities, generated protobufs, the firmware contract, broker-backed integration, device-tag compilation, and the scheduled fuzz target. Set `BUSYLIB_FUZZ_TIME` only for a focused diagnostic run; release evidence uses the declared default.

For changes that span both modules, the harness tests the adapter against the current root checkout in a disposable workspace. Before releasing an adapter change, also run the workspace-disabled consumer check from [Releasing](releasing.md) against the root version declared in `pahotransport/go.mod`.

## Physical-device verification

Set both physical-device addresses so the tests cannot silently skip, then run:

```sh
BUSYBAR_BASE_URL=http://device-address \
BUSYBAR_USB_ADDRESS=device-usb-address \
scripts/verify.sh device
```

`BUSYBAR_ACCESS_KEY` is optional when the local HTTP API requires it. The harness fails before testing if either required address is absent.

The device phase also uploads media under a unique application name, verifies its contents, and removes the test assets.

## GitHub-only services

CodeQL, pull-request dependency review, pull-request title validation, and release publication depend on GitHub event or service state. They remain supplemental workflow checks; they do not replace `scripts/verify.sh`. A release candidate requires both a passing local release harness and the applicable GitHub service checks.

## Documentation changes

Keep repository-wide guidance in `docs/`, package contracts in Go comments, and executable usage in examples. Follow the documentation rules in [CONTRIBUTING.md](../CONTRIBUTING.md). Update a copied value only when that page is the declared owner of the value.
