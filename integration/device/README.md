# Device integration tests

This directory is a separate Go module.
Ordinary repository tests do not run it.
It requires Go 1.25 or newer; the root module remains compatible with Go 1.23.

## Cloud checks

Hosted CI tests the Paho adapter with the race detector.
It also compiles and vets the files selected by the `device` build tag.

```sh
go test -race -count=1 ./...
go test -run '^$' -tags=device ./...
go vet -tags=device ./...
```

The compile command intentionally runs no tests.
Hosted CI has no physical BUSY Bar.
A passing cloud job is not device evidence.

## Local HTTP and WebSocket checks

Set `BUSYBAR_BASE_URL` to the local device URL.
Set `BUSYBAR_ACCESS_KEY` only when the device requires it.

```sh
go test -tags=device -run TestLocalDevice -v
```

This runs the local HTTP snapshot and WebSocket lifecycle checks.
The tests skip when `BUSYBAR_BASE_URL` is not set.

## Local USB checks

Set `BUSYBAR_USB_ADDRESS` to the CLI address.

```sh
go test -tags=device -run TestUSBDevice -v
```

The test skips when `BUSYBAR_USB_ADDRESS` is not set.

## Remote checks

The Paho adapter in `pahotransport` supports MQTT 5.
Caller setup must provide broker authentication and transport security.
There is no broker-backed remote test yet.
Do not treat this module as remote MQTT release evidence until that test exists.

There is also no physical media upload and read-back test yet.
Keep that release check open until the device-safe cleanup contract is defined.

Do not print access keys, passwords, tokens, or correlation data.
Record the device model, firmware version, and test command in release evidence.
