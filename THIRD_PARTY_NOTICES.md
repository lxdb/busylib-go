# Third-party notices

## BUSY Bar protobuf inputs

This repository contains selected inputs from `busy-app/busybar-protobuf`.
The copied inputs live under `internal/protosrc/bsb-protobuf`.
Generated Go packages under `proto` derive from those inputs.

The selected upstream snapshot did not include a license file.
The repository must not become public until the copyright holder grants compatible permission.

The recorded protobuf commit is `dba670e2ddb5cda511af997ca5fcb1254e90917f`.
The project MIT License does not override upstream rights.

## Go dependencies

Module dependency versions are recorded in `go.mod` and `go.sum`.
They are fetched as modules and are not vendored in this repository.

| Module | Version | License |
| --- | --- | --- |
| `github.com/coder/websocket` | v1.8.15 | ISC |
| `google.golang.org/protobuf` | v1.36.8 | BSD-3-Clause |
| `github.com/eclipse/paho.golang` | v0.22.0 | EPL-2.0 or EDL-1.0 |

The Paho module is used only by the separate physical-device test module.
Each module contains its complete license text in its upstream distribution.
