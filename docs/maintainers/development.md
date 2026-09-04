# Develop busylib-go

Use this page for repository changes. Consumer documentation starts at [Getting started](../getting-started.md).

## Repository layout

The repository contains two independently versioned Go modules:

| Module | Directory | Purpose |
| --- | --- | --- |
| `github.com/lxdb/busylib-go` | repository root | Local client, remote protocol, USB CLI, media helpers, and shared types |
| `github.com/lxdb/busylib-go/pahotransport` | `pahotransport/` | Optional Eclipse Paho implementation of `remote.Transport` |

Generated protobuf packages are committed under `proto/`. Device tests are under `integration/device/` and require the `device` build tag.

The root package groups HTTP service code by firmware capability. Keep each service, its request and response types, and its validation in the owning capability file, such as `display.go` or `storage.go`. Shared client and HTTP execution logic belongs in `client.go`, `request.go`, and `response.go`.

## Get fast feedback

Run the standard device-free test and vet loop:

```sh
scripts/verify.sh quick
```

Run the documentation contracts when changing Markdown, examples, public service methods, or remote protocol metadata:

```sh
scripts/verify.sh docs
```

The harness reads pinned versions from `scripts/verify-tools.env`. It creates any temporary Paho workspace outside the repository and does not add a local replacement to a committed module file.

## Work across both modules

The adapter depends on a published root module in `pahotransport/go.mod`. During development, the verification harness creates a disposable workspace that replaces that dependency with the current root checkout.

Do not commit a `go.work` file or a local `replace` directive. Before releasing an adapter change, run the workspace-disabled consumer check in [Releasing](releasing.md#verify-the-declared-module-dependency) against the root version declared in `pahotransport/go.mod`.

## Change generated protobuf code

Do not edit files under `proto/` by hand. The source checkout and tool versions are defined by `scripts/protobuf-tools.env` and the generation scripts.

Run:

```sh
scripts/verify.sh generated
```

The default source is the sibling `../busybar-protobuf` checkout. Set `BUSYLIB_GO_PROTO_SRC` to an authorized alternate checkout.

The generated files are redistributed only under the recorded permission and inventory in [Protobuf license research](../research/protobuf-license.md). A schema or source change must update the applicable digest and license evidence.

## Keep documentation aligned

Keep task guidance in `docs/`, API contracts in Go comments, and short compilable workflows in Go examples. The checked [service reference](../reference/services.md) must list every exported service method exactly once and must match the remote-blocked firmware metadata.

Write direct technical English. State prerequisites before actions, use stable API names, and distinguish device-free checks from physical-device evidence. Do not claim formal ASD-STE100 compliance.

## Before opening a change

Run the checks proportional to the change. At minimum, run `scripts/verify.sh quick`, `scripts/verify.sh docs`, and `scripts/verify.sh repository` for a documentation or public API change. Use [Testing](testing.md) to select additional gates.
