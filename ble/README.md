# BUSY Bar BLE transport

`ble` is an optional Go module that connects the existing `busylib-go` device API and status-stream API to a BUSY Bar over Bluetooth Low Energy. The separate module keeps CoreBluetooth, CGO, and native dependencies out of the root library.

## Install

```sh
go get github.com/lxdb/busylib-go/ble@latest
```

Read [`go.mod`](go.mod) for the minimum Go version and root-module dependency. Exact exported types and options are available in the [`ble` package documentation](https://pkg.go.dev/github.com/lxdb/busylib-go/ble).

## Support boundary

| Build or connection | Behavior |
| --- | --- |
| macOS with `CGO_ENABLED=1` | Uses CoreBluetooth to scan and connect. |
| macOS with `CGO_ENABLED=0` | The package compiles; `Scan` and `Connect` return `ble.ErrUnsupported`. |
| Other operating systems | The package compiles; `Scan` and `Connect` return `ble.ErrUnsupported`. |
| Fresh pairing with BUSY Bar firmware 1.2.3 | The documented qualification covers NUS HTTP and FFE1 state delivery. |
| Saved-bond reconnect on macOS with BUSY Bar firmware 1.2.3 | This combination is outside the supported boundary because [busybar-firmware#1014](https://github.com/busy-app/busybar-firmware/issues/1014) prevents a usable GATT session. The client does not delete pairing as a workaround. |

The BLE module is experimental. The direct `cbgo` dependency [documents Objective-C ownership and memory-leak limitations](https://github.com/tinygo-org/cbgo/blob/v0.0.4/README.md#issues) that affect long-running processes.

## Requirements

- Build on macOS with `CGO_ENABLED=1` and Apple Command Line Tools.
- Enable Bluetooth for the consuming executable. A packaged application must provide an appropriate `NSBluetoothAlwaysUsageDescription` in its `Info.plist`.
- Enable BLE on the BUSY Bar before scanning. Use an existing LAN or USB client when the device requires configuration.
- Accept the Bluetooth access and pairing prompts that macOS presents.

The BLE module does not accept a BUSY Bar HTTP API token. It does not enable device BLE, request pair mode, remove the BUSY Bar pairing, or alter the macOS association.

## Scan, connect, and make a request

`Scan` reports opaque CoreBluetooth identifiers. `Connect` retrieves the selected known peripheral without scanning again.

```go
ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
defer cancel()

peripherals, err := ble.Scan(ctx, 10*time.Second)
if err != nil {
	return err
}

client, err := ble.Connect(ctx, peripherals[0].Identifier)
if err != nil {
	return err
}

status, requestErr := client.Device().System().Status(ctx)
closeErr := client.Close()
if err := errors.Join(requestErr, closeErr); err != nil {
	return err
}

log.Printf("firmware version: %s", status.Firmware.Version)
```

`Scan` returns `ble.ErrNotFound` instead of an empty successful result. Persist `Peripheral.Identifier` when the application must reconnect to the same BUSY Bar. The identifier is a CoreBluetooth UUID, not a hardware address. Do not log it unless a user explicitly enables diagnostic output.

## Reuse the root device API

`Client.Device()` returns the existing `*busylib.Client`. The BLE module does not duplicate `SystemService`, `BusyService`, `DisplayService`, `AssetsService`, `StorageService`, or their request and response types.

```text
Application
    |
    | client.Device().Busy().SetSnapshot(...)
    v
busylib.Client and existing service types
    |
    | create net/http.Request and call http.Client.Do
    v
ble HTTP RoundTripper
    |
    | serialize HTTP/1.1 and fragment NUS RX writes
    v
CoreBluetooth -> NUS RX -> BUSY Bar
                            |
                            | HTTP response notifications
                            v
CoreBluetooth <- NUS TX <- BUSY Bar
    |
    | assemble and parse http.Response
    v
Existing busylib response decoding
```

The root client uses `http://busybar.ble.invalid` as a synthetic origin. The BLE `RoundTripper` intercepts each request before network resolution. Device API calls do not use DNS, TCP, Wi-Fi, or an external HTTP server.

| Operation | Reused implementation | Physical transport |
| --- | --- | --- |
| `Device().System().Status()` | `busylib.SystemService` | HTTP over NUS |
| `Device().Busy().SetSnapshot()` | `busylib.BusyService` | HTTP over NUS |
| `Device().Display().Screen()` | `busylib.DisplayService` | HTTP over NUS |
| `Device().Assets().Upload()` | `busylib.AssetsService` | HTTP over NUS |
| `Device().Storage().Read()` | `busylib.StorageService` | HTTP over NUS |
| `ble.Scan()` | BLE-specific discovery | CoreBluetooth service `0x308A` |
| `ble.Connect()` | BLE-specific connection and GATT discovery | CoreBluetooth |
| `Client.NewStatusStream()` | Shared `stream.Stream` contract with a BLE implementation | FFE1 notifications |
| `Client.Close()` | BLE-specific cleanup | CoreBluetooth notification and connection shutdown |

The same model applies to every method in the [root service reference](../docs/reference/services.md) that satisfies the transport contract below. An operation remains a normal HTTP request; the BLE module does not translate each endpoint into a separate command code.

## HTTP transport contract

The BLE transport:

- supports `GET`, `POST`, `PUT`, and `DELETE`;
- requires responses with an exact `Content-Length`;
- rejects authentication headers;
- limits each NUS write to the smaller of the CoreBluetooth maximum and the firmware limit of 237 bytes;
- buffers requests and responses within `WithMaxMessageBytes`;
- disables root API-version negotiation and omits `X-API-Sem-Ver`; and
- configures the root client for one attempt, so it does not retry a request automatically.

`StorageService.ReadTo` still crosses the BLE response buffer before it writes to the destination. The default buffer limit is `ble.DefaultMaxMessageBytes`.

If a request starts writing but no complete response establishes its result, the error matches `ble.ErrOutcomeUnknown`. The HTTP session then rejects further requests because it cannot safely associate late NUS data with another request. Close the BLE client, connect again, and read the applicable device state before repeating a mutation.

When the request context also ends, the returned `*busylib.RequestError` can match both the context error and `ble.ErrOutcomeUnknown`. Check both identities with `errors.Is`.

## Consume the status stream

`Client.NewStatusStream` receives device state from FFE1. It does not send an HTTP request through NUS. The implementation uses the shared status decoder and implements the same one-shot `stream.Stream` contract as the local WebSocket and remote MQTT streams.

```go
statusStream, err := client.NewStatusStream()
if err != nil {
	return err
}
if err := statusStream.Start(ctx); err != nil {
	return err
}

messages := statusStream.Messages()
for {
	select {
	case message, ok := <-messages:
		if !ok {
			return statusStream.Wait()
		}
		if message.DecodeError != nil {
			log.Printf("discard malformed state message: %v", message.DecodeError)
			continue
		}
		log.Printf("received %d state updates", len(message.Updates))

	case <-ctx.Done():
		return errors.Join(ctx.Err(), statusStream.Stop())
	}
}
```

Only one status stream can be active per BLE client. `RequestSnapshot` returns `stream.ErrSnapshotUnsupported`. After a physical disconnect, the stream applies its bounded reconnect policy and restores both NUS and FFE1 subscriptions. It does not remove pairing or retry without a limit.

Stream reconnection does not make an HTTP session reusable after `ble.ErrOutcomeUnknown`. Replace the BLE client in that case.

Read [Status streams](../docs/guides/status-streams.md) for the complete lifecycle, status, message, and snapshot contracts.

The caller must close the BLE client after the stream finishes and handle the returned cleanup error.

## Develop and qualify the module

The repository verification script creates a disposable workspace that replaces the declared root dependency with the repository checkout. Do not commit a `go.work` file or local `replace` directive.

Run the device-free BLE checks with:

```sh
scripts/verify.sh current-ble
scripts/verify.sh minimum-ble
```

These checks do not prove Bluetooth permission, pairing, device compatibility, saved-bond reconnect, or long-running behavior. Physical qualification requires macOS and a BUSY Bar. The release gate is defined in [Releasing](../docs/maintainers/releasing.md).
