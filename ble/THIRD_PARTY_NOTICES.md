# Third-party notices

This file records third-party material used by the optional BLE module. Dependency versions remain owned by `go.mod` and `go.sum`.

The module is distributed under the MIT License in `LICENSE`. It depends on the root `github.com/lxdb/busylib-go` module and is subject to that module's third-party notices.

| Module | License |
| --- | --- |
| `github.com/lxdb/busylib-go` | MIT, subject to its third-party notices |
| `github.com/tinygo-org/cbgo` | [Apache-2.0](LICENSES/cbgo-Apache-2.0.txt) |
| `github.com/sirupsen/logrus` | [MIT](LICENSES/logrus-MIT.txt) |
| `golang.org/x/sys` | [BSD-3-Clause](LICENSES/x-sys-BSD-3-Clause.txt) |
| `google.golang.org/protobuf` | [BSD-3-Clause](LICENSES/protobuf-BSD-3-Clause.txt) |
| `github.com/coder/websocket` | [ISC](LICENSES/coder-websocket-ISC.txt) |

The dependencies are fetched as Go modules and are not vendored. The `LICENSES` directory preserves the complete license text from each production dependency distribution. Consult this module's `go.mod` or `go.sum` for the selected version.
