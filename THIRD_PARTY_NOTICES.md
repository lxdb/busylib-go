# Third-party notices

This file records third-party material distributed with the root module. Dependency versions remain owned by `go.mod` and `go.sum`.

## BUSY Bar protobuf

The generated Go packages under `proto` derive from [`busy-app/busybar-protobuf`](https://github.com/busy-app/busybar-protobuf), copyright 2026 BUSY App and distributed under the upstream [MIT License](LICENSES/busybar-protobuf-MIT.txt). The source revision and schema digests are recorded under `scripts/`.

## Go dependencies

The root module fetches its dependencies as Go modules and does not vendor them.

| Module | License |
| --- | --- |
| `github.com/coder/websocket` | ISC |
| `google.golang.org/protobuf` | BSD-3-Clause |

Consult each dependency’s upstream distribution for its complete license text and `go.mod` or `go.sum` for the selected version.

The separate `github.com/lxdb/busylib-go/pahotransport` module has its own third-party notices.
