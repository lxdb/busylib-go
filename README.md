# busylib-go

Go library for BUSY Bar devices.

This repository currently contains the completed Phase 3 and Phase 4 local
HTTP contract implementation, plus Phase 5 helpers in progress:

- Go module: `github.com/lxdb/busylib-go`
- Root `busylib.Client` request execution for direct-device and explicit proxy modes
- Prepared requests, request IDs, session IDs, local access keys, bearer tokens,
  API semver negotiation, repeatable body handling, typed errors, and local-only
  guards
- Product-oriented typed HTTP service accessors for all 67 synchronous
  operations audited from BUSY Bar firmware API `24.4.0`
- Firmware-aligned request/response models and validation, including Wi-Fi,
  Matter partial updates, display/path rules, and Busy timer structures
- Display, asset, storage, and audio helpers for common app workflows
- Generated protobuf packages for the BUSY Bar status stream
- Local copies of the firmware-selected protobuf inputs
- A pinned firmware contract audit receipt with source provenance
- Reproducible protobuf generation and an optional firmware contract checker

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

Phase 5 also exposes constructors for common display and audio payloads:

```go
err = client.Display().Draw(ctx, busylib.NewDisplayElements(
	"my_app",
	busylib.NewTextElement("title", "Hello", busylib.FontNormal),
	busylib.NewAssetAnimationElement("spinner", "spinner.anim"),
))

err = client.Audio().PlayStock(ctx, "my_app", "shared/tone.snd")
err = client.Audio().SetVolumeSilently(ctx, 40)
```

File-backed helpers use repeatable bodies, so they can participate in transport
and API-version compatibility retries:

```go
err = client.Assets().UploadFile(ctx, "my_app", "spinner.anim", "./spinner.anim")
err = client.Storage().WriteFile(ctx, "/ext/data.bin", "./data.bin")

var out bytes.Buffer
n, err := client.Storage().ReadTo(ctx, "/ext/data.bin", &out)
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
- Six sensitive direct-device operations are conservatively rejected before
  network I/O in proxy mode. This is a library privacy policy, not firmware
  metadata.
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

Verify the pinned firmware contract audit against a maintainer checkout:

```sh
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware scripts/check-firmware-contract.sh
```

Normal builds, tests, and library use do not require the firmware checkout.

Run tests:

```sh
go test ./...
```

## Contract Authority And Inputs

The BUSY Bar firmware source is the sole canonical source for device behavior.
Other busylib implementations and the historical OpenAPI snapshot are research
inputs only and cannot override firmware handlers, validation, serialization,
or constants.

The Phase 3/4 audit used
`https://github.com/busy-app/busybar-firmware.git` at commit
`1add7be4f1fd31cbd0763c4c20add1ff6382232e` (branch `dev`, API `24.4.0`).
Firmware selects protobuf commit
`07223321a4ab39a13c5167dbf85c87c418325634`.

The standalone module keeps independently maintained contract artifacts under
`internal/`:

- `internal/protosrc/bsb-protobuf/`
- `internal/api/testdata/firmware-contract.json`

The firmware source is GPL-2.0-or-later. No firmware implementation code is
copied into this module; the JSON receipt records protocol facts and source
provenance. Do not edit generated protobuf files directly. Audit a new firmware
revision, refresh the receipt or protobuf input as needed, regenerate, and run
the checks.
