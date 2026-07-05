# busylib-go

Go library for BUSY Bar devices.

This repository currently contains the phase 3 HTTP-service implementation:

- Go module: `github.com/lxdb/busylib-go`
- Root `busylib.Client` request execution for direct-device and explicit proxy modes
- Prepared requests, request IDs, session IDs, local access keys, bearer tokens,
  API semver negotiation, repeatable body handling, typed errors, and local-only
  guards
- Product-oriented typed HTTP service accessors for all synchronous OpenAPI
  operations
- Generated protobuf packages for the BUSY Bar status stream
- Local copies of the protobuf and OpenAPI contract inputs
- OpenAPI operation inventory drift test
- Reproducible protobuf and OpenAPI inventory generation scripts

Status streams, frame decoding, snapshots, USB diagnostics, and converters are
intentionally deferred to later phases.

## Typed HTTP Services

Use service accessors for normal device operations:

```go
client, err := busylib.NewClient(
	busylib.WithBaseURL("http://10.0.4.20"),
	busylib.WithLocalAccessKey("1234"),
)
if err != nil {
	// handle configuration error
}

status, err := client.System().Status(ctx)
if err != nil {
	// handle request or device error
}

err = client.Display().Draw(ctx, busylib.DisplayElements{
	ApplicationName: "my_app",
	Elements: []busylib.DisplayElement{
		busylib.TextElement{
			BaseDisplayElement: busylib.BaseDisplayElement{ID: "title"},
			Text:               "Hello",
			Font:               busylib.FontNormal,
		},
	},
})
```

The service groups are `System`, `Settings`, `Display`, `Audio`, `Assets`,
`Storage`, `Busy`, `Account`, `BLE`, `WiFi`, `Input`, `SmartHome`, `Time`, and
`Update`. The `/api/status/ws` WebSocket operation is deliberately reserved for
the stream phase.

## Request Core

The root package also exposes raw request execution for schema drift,
diagnostics, and operations that need lower-level control:

```go
client, err := busylib.NewClient(
	busylib.WithBaseURL("http://10.0.4.20"),
	busylib.WithLocalAccessKey("1234"),
)
if err != nil {
	// handle configuration error
}

resp, err := client.Do(ctx, busylib.Request{
	Method:       "GET",
	Path:         "/api/status",
	ResponseMode: busylib.ResponseModeJSON,
})
```

Important request behavior:

- Local-mode bare hosts are normalized to `http://<host>` and stored as an
  origin.
- Local requests use `/api/...`; proxy mode rewrites `/api/...` to
  `/busybar/...`.
- Local access keys are sent as `X-API-Token`.
- Proxy bearer tokens are sent as `Authorization: Bearer <token>`.
- API semver is fetched from `/api/version`, cached, and sent as
  `X-API-Sem-Ver`.
- A 405 compatibility response refreshes API semver once and retries only when
  the request body is repeatable.
- The six OpenAPI `x-local-only` operations are rejected before network I/O in
  proxy mode.
- Proxy mode requires an `https://` base URL because it sends bearer tokens.
- Caller-provided auth headers are rejected; configure auth through client
  options instead.
- Version negotiation can be disabled with `WithVersionNegotiation`.

## Development

Generate protobuf files:

```sh
scripts/generate-protobuf.sh
```

The protobuf generator verifies the pinned tool versions in
`scripts/protobuf-tools.env`, uses an already installed matching
`protoc-gen-go` or installs the pinned version into a temporary tool directory,
and checks `scripts/protobuf-packages.tsv` against all copied `.proto` files
before writing generated code. The local `protoc` binary must match the pinned
`PROTOC_VERSION`.

Check generated protobuf drift:

```sh
scripts/check-protobuf.sh
```

Regenerate the OpenAPI operation inventory:

```sh
scripts/generate-openapi-inventory.sh
```

Run tests:

```sh
go test ./...
```

## Contract Inputs

The standalone module keeps raw contract inputs under `internal/`:

- `internal/protosrc/bsb-protobuf/`
- `internal/api/testdata/busybar-f21-openapi-1.0.0-rc.yaml`

Do not edit generated files directly. Update the raw contract input, regenerate,
and run the drift checks.
