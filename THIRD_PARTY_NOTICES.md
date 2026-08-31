# Third-party notices

This file records third-party material distributed with the root module. Dependency versions remain owned by `go.mod` and `go.sum`.

## BUSY Bar protobuf inputs

This repository contains selected inputs copied from [`busy-app/busybar-protobuf`](https://github.com/busy-app/busybar-protobuf) under `internal/protosrc/bsb-protobuf`. The generated Go packages under `proto` derive from those inputs.

The recorded upstream commit is `dba670e2ddb5cda511af997ca5fcb1254e90917f`. The selected snapshot does not include a license file or other verified redistribution permission. The project license does not grant rights in third-party material that the project does not own.

**Distribution blocker:** Do not make this repository public, create a release tag containing the copied inputs or generated outputs, or publish the Go module until compatible permission is documented or the affected material is removed.

The dated [protobuf license research](docs/research/protobuf-license.md) records the evidence and the conditions for clearing this blocker.

## Go dependencies

The root module fetches its dependencies as Go modules and does not vendor them.

| Module | License |
| --- | --- |
| `github.com/coder/websocket` | ISC |
| `google.golang.org/protobuf` | BSD-3-Clause |

Consult each dependency’s upstream distribution for its complete license text and `go.mod` or `go.sum` for the selected version.

The separate `github.com/lxdb/busylib-go/pahotransport` module has its own third-party notices.
