# Getting started

This guide installs the main module, creates a local client, and verifies that the device API responds.

## Requirements

- Use a Go version supported by the root module. Read [`go.mod`](../go.mod) for the exact minimum version.
- Connect the BUSY Bar through USB networking or another reachable local-network address.
- Use a local HTTP credential if the device API requires one.

## Install the module

```sh
go get github.com/lxdb/busylib-go@latest
```

## Create a client

`NewClient()` uses `busylib.DefaultLocalBaseURL`, which is the USB-network endpoint `http://10.0.4.20`.

```go
client, err := busylib.NewClient()
if err != nil {
	return err
}
```

To use another address, pass a hostname, an IP address, or a complete HTTP or HTTPS URL:

```go
client, err := busylib.NewClient(
	busylib.WithBaseURL("busybar.local"),
)
```

A hostname or IP address without a scheme uses `http`. The client discards a path in the base URL and stores only the endpoint origin.

If the device requires credentials, configure its numeric access key or minted access token when you create the client:

```go
client, err := busylib.NewClient(
	busylib.WithBaseURL("busybar.local"),
	busylib.WithLocalAccessToken(accessToken),
)
```

`WithLocalAccessToken` sends either credential in `X-API-Token`. `WithLocalAccessKey` is a deprecated source-compatible alias and sends the same header. Do not log the credential or include it in an error message.

## Make the first request

Give each operation a deadline. The caller's deadline can end the request before the client timeout.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

status, err := client.System().Status(ctx)
if err != nil {
	return err
}

log.Printf("firmware version: %s", status.Firmware.Version)
```

Success means the call returns a `busylib.Status` value and a nil error. The exact fields are documented in [`busylib.Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#Status).

## Understand version negotiation

By default, the client requests `/api/version`, caches the returned API semantic version, and sends it in the `X-API-Sem-Ver` header. A compatible response can cause one refresh and retry when the request method and body are safe to repeat.

Disable negotiation only when the endpoint does not implement this firmware contract:

```go
client, err := busylib.NewClient(
	busylib.WithBaseURL("busybar.local"),
	busylib.WithVersionNegotiation(busylib.VersionNegotiationDisabled),
)
```

Disabling negotiation omits version discovery and the version header. It does not make an incompatible response schema safe.

## Handle failure

Check the returned error before using a result. Use `errors.Is` for sentinel and context errors. Use `errors.As` for structured library errors.

```go
var apiErr *busylib.APIError
if errors.As(err, &apiErr) {
	log.Printf(
		"device request failed: status=%d request_id=%s message=%q",
		apiErr.StatusCode,
		apiErr.RequestID,
		apiErr.DeviceError,
	)
}
```

| Failure | Check |
| --- | --- |
| `context.DeadlineExceeded` or `context.Canceled` | Confirm the caller deadline and whether the operation was canceled. |
| `*busylib.ValidationError` | Correct the request before retrying. No request was sent. |
| `*busylib.RequestError` | Check endpoint reachability and the wrapped transport error. |
| `*busylib.APIError` | Inspect the HTTP status, device message, and request ID. |
| `*busylib.ProtocolError` | Treat the response as incompatible or malformed. |
| `*busylib.VersionError` | Check firmware compatibility and version negotiation. |

Read the [error reference](reference/errors.md) before implementing retry or recovery behavior.

## Continue by task

- Use the [service reference](reference/services.md) to find every device method.
- Use [status streams](guides/status-streams.md) to receive changes over WebSocket, MQTT, or BLE.
- Use [remote MQTT](integrations/remote-mqtt.md) when the device is not locally reachable.
- Use the [BLE transport](../ble/README.md) to connect through CoreBluetooth on macOS.
- Use [display and media](guides/display-and-media.md) to prepare and render visual or audio content.
- Use [assets and storage](guides/assets-and-storage.md) for uploads and files.
