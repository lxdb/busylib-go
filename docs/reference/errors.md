# Error reference

Every operation returns an error that the caller must inspect. Match structured errors with `errors.As` and wrapped causes with `errors.Is`.

## Root client errors

| Error | Meaning | Caller action |
| --- | --- | --- |
| [`*busylib.ValidationError`](https://pkg.go.dev/github.com/lxdb/busylib-go#ValidationError) | Caller input failed validation before transport use. | Correct the input. Do not retry the same request unchanged. |
| [`*busylib.RequestError`](https://pkg.go.dev/github.com/lxdb/busylib-go#RequestError) | The HTTP transport failed after all permitted attempts. | Inspect the wrapped error, request context, and attempt count before deciding whether retry is safe. |
| [`*busylib.APIError`](https://pkg.go.dev/github.com/lxdb/busylib-go#APIError) | The device returned a non-success HTTP status. | Inspect `StatusCode`, `DeviceCode`, `DeviceError`, and `RequestID`. |
| [`*busylib.ProtocolError`](https://pkg.go.dev/github.com/lxdb/busylib-go#ProtocolError) | A response did not match its required format. | Treat the response as incompatible or malformed. Preserve the error for diagnostics. |
| [`*busylib.VersionError`](https://pkg.go.dev/github.com/lxdb/busylib-go#VersionError) | API version discovery or compatibility retry failed. | Check firmware compatibility and the wrapped discovery error. |
| [`busylib.ErrResponseTooLarge`](https://pkg.go.dev/github.com/lxdb/busylib-go#ErrResponseTooLarge) | A buffered HTTP response exceeded the client limit. | Use a streaming method when available or raise the limit only for a measured requirement. |

`*busylib.RequestError` can wrap more than one cause. If a context ends while a transport reports a more specific failure, the same error can match both causes through `errors.Is`. Check each identity that affects recovery.

`APIError.DeviceCode` contains the firmware `error_code` value when present. For older payloads, it uses `code` instead. The client represents either value as a string.

```go
var apiErr *busylib.APIError
switch {
case errors.Is(err, context.DeadlineExceeded):
	return fmt.Errorf("device request timed out: %w", err)
case errors.As(err, &apiErr):
	return fmt.Errorf(
		"device rejected request %s with status %d: %w",
		apiErr.RequestID,
		apiErr.StatusCode,
		err,
	)
case err != nil:
	return err
}
```

## Status stream errors

| Error | Meaning | Caller action |
| --- | --- | --- |
| [`stream.ErrNotStarted`](https://pkg.go.dev/github.com/lxdb/busylib-go/stream#ErrNotStarted) | `Wait` was called before `Start` or `Stop`. | Start or stop the one-shot stream before waiting. |
| [`stream.ErrAlreadyStarted`](https://pkg.go.dev/github.com/lxdb/busylib-go/stream#ErrAlreadyStarted) | `Start` was called more than once. | Create a new stream. A finished stream cannot restart. |
| [`stream.ErrSnapshotUnsupported`](https://pkg.go.dev/github.com/lxdb/busylib-go/stream#ErrSnapshotUnsupported) | The transport cannot request an immediate stream snapshot. | Use `snapshot.Collect` with the device client. Remote MQTT streams have this behavior. |
| [`*stream.Error`](https://pkg.go.dev/github.com/lxdb/busylib-go/stream#Error) | A stream transport, lifecycle, protocol, or firmware failure occurred. | Inspect `Operation`, `Attempt`, `Terminal`, status or close code, and the wrapped cause. |

`Wait` returns the stable terminal or cleanup result. `Stop` requests shutdown, waits for owned resources to close, and returns the same completion result. Do not discard either error.

## Remote MQTT errors

| Error | Meaning | Caller action |
| --- | --- | --- |
| [`remote.ErrClosed`](https://pkg.go.dev/github.com/lxdb/busylib-go/remote#ErrClosed) | The remote client is closed. | Create another client if more work is required. |
| [`remote.ErrMessageTooLarge`](https://pkg.go.dev/github.com/lxdb/busylib-go/remote#ErrMessageTooLarge) | A payload exceeded `WithMaxMessageBytes` or a subscription limit. | Reject the payload and keep the configured bound unless the application has a measured need. |
| [`*remote.Error`](https://pkg.go.dev/github.com/lxdb/busylib-go/remote#Error) | MQTT publication, subscription, response, or lifecycle handling failed. | Inspect `Operation`, `Route`, `Attempt`, `Terminal`, and the wrapped transport error. |

The remote client does not close its caller-owned MQTT transport. Join or report errors from closing remote clients before closing the transport.

## BLE errors

| Error | Meaning | Caller action |
| --- | --- | --- |
| [`ble.ErrUnsupported`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#ErrUnsupported) | The current platform or build does not provide a BLE backend. | Use macOS with CGO or choose another transport. |
| [`ble.ErrClosed`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#ErrClosed) | The BLE client or its HTTP transport is closed. | Create another BLE client if more work is required. |
| [`ble.ErrNotFound`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#ErrNotFound) | A scan found no BUSY Bar, or CoreBluetooth no longer knows the selected identifier. | Verify that BLE is enabled and the device is advertising; do not automatically delete pairing. |
| [`ble.ErrDisconnected`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#ErrDisconnected) | No usable physical BLE link exists. | Let an active stream apply its bounded reconnect policy or reconnect explicitly. |
| [`ble.ErrOutcomeUnknown`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#ErrOutcomeUnknown) | A request began writing but no complete response established its outcome. The current BLE HTTP session is no longer reusable. | Close the BLE client, connect again, and use the new client to read current device state before repeating a mutation. |
| [`ble.ErrMessageTooLarge`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#ErrMessageTooLarge) | A buffered request, response, or FFE1 message exceeded its limit. | Keep the configured bound unless a measured operation requires more memory. |
| [`ble.ErrProtocol`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#ErrProtocol) | An HTTP request, HTTP response, GATT map, or FFE1 message does not satisfy the BLE protocol contract. | Treat the data or device as incompatible and preserve the wrapped error for diagnostics. |
| [`*ble.Error`](https://pkg.go.dev/github.com/lxdb/busylib-go/ble#Error) | A BLE setup, GATT, protocol, or native CoreBluetooth operation failed. | Inspect `Operation`, `NativeCode`, and the wrapped cause without logging device identifiers or payloads. |

The BLE client owns its CoreBluetooth connection. Close it after stopping or waiting for its status stream.

## USB CLI errors

| Error | Meaning | Caller action |
| --- | --- | --- |
| [`usb.ErrClosed`](https://pkg.go.dev/github.com/lxdb/busylib-go/usb#ErrClosed) | A persistent session is closed or failed. | Open a new session. |
| [`usb.ErrInvalidCommand`](https://pkg.go.dev/github.com/lxdb/busylib-go/usb#ErrInvalidCommand) | The command is empty or contains a disallowed control character. | Correct the command or argument. |
| [`usb.ErrResponseTooLarge`](https://pkg.go.dev/github.com/lxdb/busylib-go/usb#ErrResponseTooLarge) | Prompt-framed output exceeded its configured limit. | Use a bounded streaming command when applicable or review the limit. |
| [`usb.ErrPromptNotFound`](https://pkg.go.dev/github.com/lxdb/busylib-go/usb#ErrPromptNotFound) | Input ended before the firmware prompt arrived. | Check firmware CLI compatibility and connection state. |
| [`*usb.Error`](https://pkg.go.dev/github.com/lxdb/busylib-go/usb#Error) | A CLI connection, command, protocol, or lifecycle operation failed. | Inspect the operation, address, safe command text, and wrapped cause. |

## Conversion and frame errors

Image, audio, animation, and frame packages expose sentinel errors for unsupported input, malformed data, invalid configuration, and configured size limits. Their structured errors preserve operation context and support `errors.Is` through `Unwrap`.

| Package | Structured error | Sentinel errors |
| --- | --- | --- |
| `convert` | [`*convert.ConversionError`](https://pkg.go.dev/github.com/lxdb/busylib-go/convert#ConversionError) | [Image conversion errors](https://pkg.go.dev/github.com/lxdb/busylib-go/convert#pkg-variables) |
| `convert/audio` | [`*audio.ConversionError`](https://pkg.go.dev/github.com/lxdb/busylib-go/convert/audio#ConversionError) | [Audio conversion errors](https://pkg.go.dev/github.com/lxdb/busylib-go/convert/audio#pkg-variables) |
| `convert/animation` | [`*animation.ConversionError`](https://pkg.go.dev/github.com/lxdb/busylib-go/convert/animation#ConversionError) | [Animation conversion errors](https://pkg.go.dev/github.com/lxdb/busylib-go/convert/animation#pkg-variables) |
| `frame` | [`*frame.Error`](https://pkg.go.dev/github.com/lxdb/busylib-go/frame#Error) | [Frame validation and decoding errors](https://pkg.go.dev/github.com/lxdb/busylib-go/frame#pkg-variables) |

Do not include source media, MQTT payloads, device tokens, or private response bodies in an application error message.
