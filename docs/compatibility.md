# Compatibility

This page records the compatibility contract that maintainers must check when a toolchain, firmware API, generated artifact, platform, or resource limit changes.

## Canonical sources

| Contract | Canonical source |
| --- | --- |
| Root Go toolchain | `go.mod` |
| Paho adapter Go toolchain | `pahotransport/go.mod` |
| Firmware API contract and audited revisions | `internal/api/testdata/firmware-contract.json` |
| Generated protobuf source mapping | `scripts/protobuf-packages.tsv` |
| CI operating systems and commands | `.github/workflows/ci.yml` |
| Release module order and tag format | `release-please-config.json` |

Documentation must point to these files instead of copying values that can drift without a reader noticing.

## Go toolchains

The repository contains two Go modules. The root module and the optional Paho adapter can require different minimum Go versions. Read each module’s `go.mod` before building it, and test both modules when a shared interface changes.

## Firmware API

The local client discovers the device API semantic version from `/api/version`, caches it, and sends it in `X-API-Sem-Ver`. `internal/api/testdata/firmware-contract.json` records the audited firmware revision, protobuf revision, API version, operations, status stream, frame, snapshot, optional-tool, and remote MQTT contracts.

Callers can disable discovery with `busylib.WithVersionNegotiation(busylib.VersionNegotiationDisabled)` for an endpoint that does not implement this contract. Disabling negotiation omits the version header; it does not make an incompatible response schema safe.

## Generated protobuf code

Files under `proto` are generated from the copied upstream sources in `internal/protosrc/bsb-protobuf`. Do not edit generated `.pb.go` files by hand. Regenerate them with the repository scripts, review the recorded source revision and package mapping, and run the focused protobuf tests.

The generated code remains subject to the redistribution condition in [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).

## Platforms

The standard CI workflow tests the root module on Linux and macOS. Windows behavior is not covered by that workflow. Physical-device behavior is covered separately by opt-in integration tests and cannot be inferred from a passing unit-test job.

The USB CLI uses the device’s USB network interface. A successful build proves that the client compiles for the tested host; it does not prove host routing, device access, prompt compatibility, or firmware compatibility.

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

These are safety boundaries, not performance targets. Change a default only with tests that cover rejection at the boundary and successful handling below it.

## What a compatibility change requires

1. Change the canonical source.
2. Update or add contract tests.
3. Regenerate artifacts when the source contract requires it.
4. Update this page only when the compatibility meaning or source location changes.
5. Run the verification commands in [Development](development.md) and any applicable physical-device checks.
