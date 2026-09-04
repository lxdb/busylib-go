# Compatibility

Compatibility depends on the Go module, firmware API contract, selected platform, generated protocol code, and configured resource limits. A successful build does not prove access to a physical BUSY Bar.

## Canonical sources

| Contract | Canonical source |
| --- | --- |
| Root Go version | [`go.mod`](../../go.mod) |
| Paho adapter Go version and root dependency | [`pahotransport/go.mod`](../../pahotransport/go.mod) |
| Verification toolchains and pinned tools | [`scripts/verify-tools.env`](../../scripts/verify-tools.env) |
| Firmware API contract and audited revisions | [`internal/api/testdata/firmware-contract.json`](../../internal/api/testdata/firmware-contract.json) |
| Protobuf repository and source revision | [`scripts/protobuf-source.env`](../../scripts/protobuf-source.env) |
| Generated protobuf package and digest mapping | [`scripts/protobuf-packages.tsv`](../../scripts/protobuf-packages.tsv) |
| CI operating systems and checks | [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) |
| Release modules and tag formats | [`release-please-config.json`](../../release-please-config.json) |

Read these files for exact version values. Documentation describes the meaning of each boundary and does not duplicate values that change independently.

## Go modules

The repository contains two independently versioned modules. The root library and optional Paho adapter can require different minimum Go versions. Test both modules when a shared transport interface changes.

Use `GOWORK=off` when verifying the Paho module against the root version declared in its `go.mod`. A local workspace replacement can otherwise hide an invalid published dependency.

## Firmware API

The local client discovers the API semantic version, caches it, and sends it in `X-API-Sem-Ver`. The recorded firmware contract covers HTTP operations, the status stream, frames, snapshots, optional tools, and remote MQTT behavior.

`busylib.WithVersionNegotiation(busylib.VersionNegotiationDisabled)` supports endpoints that do not implement discovery. It omits version discovery and the header. It does not validate or translate an incompatible response schema.

## Generated protobuf code

Files under `proto/` are generated. Do not edit `.pb.go` files by hand. The generation scripts verify the source license, mapped schema digests, and complete schema inventory before writing output.

The default source is the sibling `../busybar-protobuf` checkout. `BUSYLIB_GO_PROTO_SRC` selects another authorized checkout.

## Platforms and physical devices

The CI workflow defines the operating systems covered by device-free tests. Physical HTTP, WebSocket, MQTT, media, and USB behavior requires separate integration evidence.

The USB package uses the device's USB network interface. A passing build does not prove host routing, permissions, prompt compatibility, or firmware behavior.

## Default safety limits

| Boundary | Default | Configuration or owner |
| --- | ---: | --- |
| Buffered local HTTP response | 1 MiB | `busylib.DefaultMaxResponseBytes`; override with `busylib.WithMaxResponseBytes` |
| Encoded image input | 32 MiB | `convert.DefaultMaxInputBytes` |
| Decoded image pixels | 16,777,216 | `convert.DefaultMaxSourcePixels` |
| Audio output | 64 MiB | `audio.DefaultMaxOutputBytes` |
| Animation ZIP input | 32 MiB | `animation.DefaultMaxInputBytes` |
| Animation output | 64 MiB | `animation.DefaultMaxOutputBytes` |
| Remote MQTT payload | 1 MiB | `remote.DefaultMaxMessageBytes` |
| Encoded or decoded frame payload | 16 KiB | `frame.MaxPayloadSize` |

These limits bound memory or processing exposure. They are not performance targets. Raise a limit only for a measured input requirement, and keep the caller's context bounded.

## Change a compatibility boundary

1. Change the canonical source.
2. Add or update the contract test.
3. Regenerate owned artifacts when required.
4. Update this page only when the compatibility meaning or source location changes.
5. Run the applicable checks in [Development](../maintainers/development.md) and [Testing](../maintainers/testing.md).
