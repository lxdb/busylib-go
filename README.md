# busylib-go

`busylib-go` provides typed services for BUSY Bar devices. Applications use the same service API through local HTTP, remote MQTT, or the optional macOS BLE transport. Supporting packages provide status streams, media conversion, frame decoding, snapshots, and access to the USB firmware CLI.

## Install

Install the main module:

```sh
go get github.com/lxdb/busylib-go@latest
```

Install the optional Eclipse Paho MQTT adapter only when the application uses it:

```sh
go get github.com/lxdb/busylib-go/pahotransport@latest
```

Install the experimental macOS BLE transport only when the application uses CoreBluetooth:

```sh
go get github.com/lxdb/busylib-go/ble@latest
```

## Read device status

`NewClient` connects to the BUSY Bar USB-network endpoint at `http://10.0.4.20` by default.

Before running the program, connect the BUSY Bar over USB and open <http://10.0.4.20> to confirm that the device is reachable. If the device UI does not load, resolve the USB-network connection before troubleshooting the Go client.

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/lxdb/busylib-go"
)

func main() {
	client, err := busylib.NewClient()
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

The request succeeds when the device is reachable and returns a compatible status response. To use another hostname or address, pass `busylib.WithBaseURL("busybar.local")` or a complete HTTP or HTTPS URL.

## Choose an entry point

| Goal | Start here |
| --- | --- |
| Connect a device and make the first request | [Getting started](docs/getting-started.md) |
| Find a client service or method | [Service reference](docs/reference/services.md) |
| Diagnose request, API, stream, MQTT, or BLE failures | [Error reference](docs/reference/errors.md) |
| Inspect status or change device settings | [Inspect and configure](docs/guides/inspect-and-configure.md) |
| Render content or play audio | [Display and media](docs/guides/display-and-media.md) |
| Upload assets or manage device files | [Assets and storage](docs/guides/assets-and-storage.md) |
| Receive live device updates | [Status streams](docs/guides/status-streams.md) |
| Connect through MQTT | [Remote MQTT](docs/integrations/remote-mqtt.md) |
| Connect through Bluetooth Low Energy | [BLE transport](ble/README.md) |
| Implement an MQTT transport | [Custom MQTT transport](docs/integrations/custom-mqtt-transport.md) |
| Use the firmware CLI | [USB CLI](docs/integrations/usb-cli.md) |
| Find a supporting package | [Package reference](docs/reference/packages.md) |

The [documentation index](docs/README.md) includes all consumer, integration, reference, and maintainer guides. Exact Go signatures, fields, constants, and examples are available on [`pkg.go.dev`](https://pkg.go.dev/github.com/lxdb/busylib-go).

## Support and licensing

Use [GitHub Issues](https://github.com/lxdb/busylib-go/issues) for reproducible bugs and focused feature requests. Follow the [security policy](SECURITY.md) for vulnerability reports.

The project uses the MIT License. See [LICENSE](LICENSE), [NOTICE](NOTICE), and [third-party notices](THIRD_PARTY_NOTICES.md).
