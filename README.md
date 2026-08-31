# busylib-go

`busylib-go` is a Go client library for BUSY Bar devices and remote MQTT workflows. It provides typed clients for device control, status streams, media preparation, frame decoding, snapshots, and USB CLI access.

> [!WARNING]
> Public distribution is blocked by the protobuf redistribution condition documented in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Do not publish, tag, or redistribute this module until that condition is resolved.

## What the library provides

- Local device control over HTTP, with firmware API version negotiation.
- Local status streams over WebSocket and remote status streams over MQTT.
- A transport-neutral remote client and an optional Eclipse Paho adapter module.
- Image, audio, and animation conversion for device-compatible media.
- Frame decoding, snapshot helpers, and USB CLI access.

## Requirements

The supported Go toolchains, firmware contract, platforms, generated-code policy, and default safety limits are listed in [Compatibility](docs/compatibility.md).

## Quick start

Create a local client, give each operation a bounded context, and handle the returned error.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/lxdb/busylib-go"
)

func main() {
    client, err := busylib.NewClient(busylib.WithBaseURL("http://busybar.local"))
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    status, err := client.System().Status(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("firmware version: %s", status.Firmware.Version)
}
```

The call succeeds when the device endpoint is reachable and returns a compatible status response. See [Getting started](docs/getting-started.md) for endpoint validation, API version overrides, error handling, and upload examples.

## Documentation

Start with the [documentation index](docs/README.md). It routes readers to task-specific guides for transports, media, compatibility, development, releases, and device testing.

## Contributing and support

Read [CONTRIBUTING.md](CONTRIBUTING.md) before changing code or documentation. Use [GitHub Issues](https://github.com/lxdb/busylib-go/issues) for reproducible bugs and focused feature requests. Report security issues according to [SECURITY.md](SECURITY.md).

## License

The project is licensed under the MIT License. See [LICENSE](LICENSE), [NOTICE](NOTICE), and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). The project license does not remove the separate redistribution condition described in the third-party notices.
