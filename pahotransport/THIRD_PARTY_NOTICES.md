# Third-party notices

This file records third-party material used by the optional Paho adapter module. Dependency versions remain owned by `go.mod` and `go.sum`.

The module is distributed under the MIT License in `LICENSE`. It depends on the root `github.com/lxdb/busylib-go` module and must not be published before the root module’s protobuf redistribution blocker is resolved.

| Module | License |
| --- | --- |
| `github.com/lxdb/busylib-go` | MIT, subject to its third-party notices |
| `github.com/eclipse/paho.golang` | EPL-2.0 or EDL-1.0 |
| `github.com/coder/websocket` | ISC |
| `github.com/gorilla/websocket` | BSD-2-Clause |
| `golang.org/x/net` | BSD-3-Clause |
| `google.golang.org/protobuf` | BSD-3-Clause |

The dependencies are fetched as Go modules and are not vendored here. Consult each upstream distribution for its complete license text and this module’s `go.mod` or `go.sum` for the selected version.
