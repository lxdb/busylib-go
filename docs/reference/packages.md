# Package reference

Use the smallest package that owns the task. The root module does not require a specific MQTT implementation or an external audio tool unless the application selects those features.

| Package | Responsibility | Start with |
| --- | --- | --- |
| [`busylib`](https://pkg.go.dev/github.com/lxdb/busylib-go) | Local HTTP client, typed device services, request bodies, and local status streams. | `NewClient`, [service reference](services.md), [client reference](client.md) |
| [`remote`](https://pkg.go.dev/github.com/lxdb/busylib-go/remote) | Adapt a caller-owned MQTT 5 transport to the firmware remote HTTP and status-stream protocols. | `NewClient`, [Remote MQTT](../integrations/remote-mqtt.md) |
| [`stream`](https://pkg.go.dev/github.com/lxdb/busylib-go/stream) | Shared one-shot status-stream interface, lifecycle status, decoded messages, and typed updates. | `Stream`, [Status streams](../guides/status-streams.md) |
| [`convert`](https://pkg.go.dev/github.com/lxdb/busylib-go/convert) | Convert PNG, JPEG, or single-frame GIF input to a device-ready image. | `Image`, `ImageFile` |
| [`convert/audio`](https://pkg.go.dev/github.com/lxdb/busylib-go/convert/audio) | Validate device-ready PCM or invoke `ffmpeg` for supported encoded input. | `Convert`, `ConvertFile` |
| [`convert/animation`](https://pkg.go.dev/github.com/lxdb/busylib-go/convert/animation) | Encode image sequences or supported archives as firmware-native animation data. | `EncodeImages`, `EncodeRGB888`, `ConvertZIP` |
| [`display`](https://pkg.go.dev/github.com/lxdb/busylib-go/display) | Define front and back physical display targets shared by the client and frame decoder. | `Front`, `Back` |
| [`frame`](https://pkg.go.dev/github.com/lxdb/busylib-go/frame) | Validate and decode HTTP or protobuf display frames as pixels or `image.RGBA`. | `FromHTTP`, `FromProto` |
| [`snapshot`](https://pkg.go.dev/github.com/lxdb/busylib-go/snapshot) | Collect best-effort device state and merge typed stream updates without owning transport lifecycle. | `Collect`, `NewStore` |
| [`usb`](https://pkg.go.dev/github.com/lxdb/busylib-go/usb) | Access the raw firmware CLI through the USB network interface. | `NewClient`, `Client.Commands`, [USB CLI](../integrations/usb-cli.md) |
| [`pahotransport`](https://pkg.go.dev/github.com/lxdb/busylib-go/pahotransport) | Optional, independently versioned Eclipse Paho MQTT 5 implementation of `remote.Transport`. | `Dial`, [Remote MQTT](../integrations/remote-mqtt.md) |
| [`proto/*`](https://pkg.go.dev/github.com/lxdb/busylib-go/proto/statepb) | Generated protocol values retained by stream and frame types. | Use through typed `stream.Update` or `frame.Frame` values unless implementing protocol-level tooling. |

## Module boundary

The root module is `github.com/lxdb/busylib-go`. The Paho adapter is the separate module `github.com/lxdb/busylib-go/pahotransport`. Each module declares its own minimum Go version and dependencies.

Install the Paho module only when the application selects Eclipse Paho:

```sh
go get github.com/lxdb/busylib-go/pahotransport@latest
```

The remote client accepts any implementation of `remote.Transport`. Read [Custom MQTT transport](../integrations/custom-mqtt-transport.md) before implementing one.

## Optional runtime tools

The audio converter can invoke `ffmpeg`. It is a runtime executable, not a Go dependency. Device-ready PCM input does not require `ffmpeg`.

The USB package requires host routing to the device's USB-network address and a compatible firmware CLI. Importing the package does not discover or configure a USB device.
