# Device integration tests

This directory contains opt-in tests that require a physical BUSY Bar or an external broker. The `device` build tag keeps them out of ordinary unit-test runs.

## Compile without hardware

Hosted CI can compile and vet the device-tagged files and run broker-backed adapter tests. It cannot establish that a physical device is reachable or compatible.

```sh
workspace="$(mktemp -d)/go.work"
repository="$(pwd)"
GOWORK="$workspace" go work init "$repository/pahotransport"
GOWORK="$workspace" go work edit -replace github.com/lxdb/busylib-go="$repository"
(cd pahotransport && GOWORK="$workspace" go test -race -count=1 ./... && GOWORK="$workspace" go vet ./...)
GOWORK=off go test -run '^$' -tags=device ./integration/device
GOWORK=off go vet -tags=device ./integration/device
```

The root `go test` command uses `-run '^$'` so it compiles selected tests without running them. A passing result is compile evidence, not device evidence.

## Test local HTTP and WebSocket behavior

Set `BUSYBAR_BASE_URL` to the local device URL. Set `BUSYBAR_ACCESS_KEY` only when the device requires it.

```sh
go test -tags=device -run TestLocalDevice -v ./integration/device
```

This command runs the local HTTP snapshot and WebSocket lifecycle checks. The tests skip when `BUSYBAR_BASE_URL` is not set.

## Test USB CLI access

Set `BUSYBAR_USB_ADDRESS` to the CLI address reported for the target device.

```sh
go test -tags=device -run TestUSBDevice -v ./integration/device
```

The test skips when `BUSYBAR_USB_ADDRESS` is not set. A failure can indicate permissions or descriptor access as well as a library defect.

## Broker-backed remote checks

The Paho adapter test suite exercises MQTT 5 publication, receive delivery, forced connection loss, reconnection, and subscription restoration against its pinned Mosquitto service. Caller configuration remains responsible for broker authentication and transport security.

## Release evidence

Record the device model, firmware version, exact command, and result. Do not record access keys, passwords, tokens, or correlation data.

There is no physical media upload-and-read-back test. Keep that release gate open until an executable test defines safe cleanup behavior.
