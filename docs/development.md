# Development

Run repository checks from the root of an authorized checkout. The root library and the Paho adapter are separate Go modules and must be verified separately.

## Root module

Install the Go version declared in `go.mod`, using the latest available security patch for that release. Then run:

```sh
go mod download
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
```

Run targeted tests while developing. Run the full suite and race detector before requesting review for a shared or concurrent behavior change.

```sh
go test -race ./...
```

Tests should assert observable behavior. Use exact counts when the contract defines an exact set, synchronization barriers for concurrent behavior, and deadlines only as failure bounds. A sleep is not evidence that concurrent work completed correctly.

## Paho adapter module

The adapter depends on an unreleased root module version. Use a temporary workspace to test the two checkouts together without changing either `go.mod` file.

```sh
workspace="$(mktemp -d)/go.work"
repository="$(pwd)"
GOWORK="$workspace" go work init "$repository/pahotransport"
GOWORK="$workspace" go work edit -replace github.com/lxdb/busylib-go="$repository"
(cd pahotransport && GOWORK="$workspace" go test ./... && GOWORK="$workspace" go vet ./...)
```

Do not commit `go.work` or a local `replace` directive. The temporary replacement tests the adapter against the current checkout; it does not verify that the declared public dependency can be downloaded.

## Generated contracts

Use the repository scripts to check generated protobuf code and the pinned firmware API contract.

```sh
scripts/check-protobuf.sh
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware scripts/check-firmware-contract.sh
```

The firmware checkout must match `internal/api/testdata/firmware-contract.json`. The protobuf check must leave no diff. Do not edit generated `.pb.go` files directly.

## Physical-device tests

Device tests use the `device` build tag, so ordinary unit tests do not run them. Hosted CI compiles and vets them but cannot supply physical-device evidence. Follow [Device integration tests](../integration/device/README.md) for local HTTP, WebSocket, USB, and broker-backed checks.

## Documentation changes

Keep repository-wide guidance in `docs/`, package contracts in Go comments, and executable usage in examples. Follow the documentation rules in [CONTRIBUTING.md](../CONTRIBUTING.md). Update a copied value only when that page is the declared owner of the value.
