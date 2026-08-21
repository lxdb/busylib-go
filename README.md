# busylib-go

`busylib-go` is a Go client for BUSY Bar devices.

It supports local HTTP, local status streams, USB CLI access, and caller-owned MQTT 5 transports.

> [!WARNING]
> This repository is preparing for its first public release.
> Do not publish it until the bundled protobuf license is resolved.

## Requirements

- Go 1.23 or newer.
- macOS is the supported client platform for the first release.
- BUSY Bar firmware API 25 for device operations.
- `ffmpeg` only for compressed audio conversion.

## Install

The module does not have a public tag yet.
Use a tagged version after the first release.

```sh
go get github.com/lxdb/busylib-go@v0.1.0
```

## Quick start

```go
package main

import (
	"context"
	"log"

	busylib "github.com/lxdb/busylib-go"
)

func main() {
	client, err := busylib.NewClient(
		busylib.WithBaseURL("http://10.0.4.20"),
	)
	if err != nil {
		log.Fatal(err)
	}

	status, err := client.System().Status(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("firmware: %s", status.Firmware.Version)
}
```

The client negotiates the device API version by default.
Requests return typed validation, transport, protocol, API, and version errors.
Buffered responses use a 1 MiB default limit.

## Packages

| Package | Purpose |
| --- | --- |
| `busylib` | HTTP client and typed device services |
| `convert` | Bounded image preparation for both displays |
| `convert/audio` | Bounded PCM preparation and optional `ffmpeg` conversion |
| `convert/animation` | Bounded firmware-native `.anim` generation from frames, images, or ZIPs |
| `frame` | HTTP and protobuf frame decoding |
| `remote` | MQTT 5 remote HTTP and status-stream adapter |
| `snapshot` | Best-effort snapshots and synchronized state merging |
| `stream` | Shared status-stream contracts and typed updates |
| `usb` | Bounded USB-network CLI client |

## Common operations

Service accessors group operations by device feature.

```go
status, err := client.System().Status(ctx)
err = client.Display().Draw(ctx, busylib.NewDisplayElements(
	"example",
	busylib.NewTextElement("title", "Build complete", busylib.FontNormal),
))
err = client.Audio().SetVolumeSilently(ctx, 40)
```

File helpers use repeatable request bodies.
They can participate in transport and compatibility retries.

```go
err = client.Assets().UploadFile(ctx, "example", "spinner.anim", "./spinner.anim")
err = client.Storage().WriteFile(ctx, "/ext/data.bin", "./data.bin")

var output bytes.Buffer
_, err = client.Storage().ReadTo(ctx, "/ext/data.bin", &output)
```

The service groups cover system, settings, display, audio, assets, and storage.
They also cover timers, accounts, BLE, Wi-Fi, input, Matter, time, and updates.

## Local status stream

`Client.NewStatusStream` creates a one-shot local stream.
It reuses the client's HTTP settings.

```go
statusStream, err := client.NewStatusStream(
	stream.WithStaleAfter(5*time.Second),
	stream.WithReconnectPolicy(stream.ReconnectPolicy{
		MaxAttempts: 5,
		Delay:       time.Second,
	}),
)
if err != nil {
	return err
}
if err := statusStream.Start(ctx); err != nil {
	return err
}
defer statusStream.Stop()

for message := range statusStream.Messages() {
	for _, update := range message.Updates {
		_ = update.Kind()
	}
}
```

Read `Messages`, `Statuses`, and `Errors` concurrently.
Use `RequestSnapshot` to request all current state again.
Use the `remote` package for MQTT status streams.

## Remote MQTT

The `remote` package accepts a caller-owned MQTT 5 transport.
It does not select a broker, client library, or credential policy.

```go
remoteClient, err := remote.NewClient(
	mqttTransport,
	firmwareSessionID,
	remote.WithClientID("example"),
)
if err != nil {
	return err
}
defer remoteClient.Close()

status, err := remoteClient.Device().System().Status(ctx)
```

The transport maps MQTT response topics and correlation data.
The wrapper leaves the caller's transport open.
Firmware blocks some HTTP operations through MQTT.
The client rejects these operations before publication.

## Frames and media

Use `frame.FromHTTP` after `Display.Screen`.
Use `frame.FromProto` for a status-stream frame.

```go
raw, err := client.Display().Screen(ctx, 0)
if err != nil {
	return err
}
displayFrame, err := frame.FromHTTP(0, raw)
if err != nil {
	return err
}
rgba, err := displayFrame.RGBA()
```

The `convert` package prepares static images.
The `convert/audio` package prepares raw device audio.
The `convert/animation` package creates native `bicycle0` animations from
device-ready BGR frames, equal-sized Go images, or firmware-style PNG ZIPs.
These packages enforce configurable memory limits.

## USB CLI

The `usb` package connects to the raw firmware CLI.
It uses `10.0.4.20:23` by default.

```go
cli, err := usb.NewClient()
if err != nil {
	return err
}
response, err := cli.Commands().Uptime(ctx)
```

Direct commands use a new connection.
`Open` creates one serialized persistent session.
Failed commands are not replayed.

## Documentation

- [Changelog](CHANGELOG.md)

## Stability

The project has not published a stable API release.
Review release notes before each pre-1.0 upgrade.

## Contributing and support

Read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a change.
Use [SUPPORT.md](SUPPORT.md) for support routes.
Use [SECURITY.md](SECURITY.md) for vulnerability reports.

## License

Original project code uses the MIT License.
See [LICENSE](LICENSE) for the complete terms.
See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) before distribution.

Publication remains blocked until the protobuf copyright holder grants compatible permission.
