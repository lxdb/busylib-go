# busylib-go documentation

Start with the task you need to complete. The service reference lists every typed device operation.

## Start

| I want to... | Read... |
| --- | --- |
| Install the module and make a first request | [Getting started](getting-started.md) |
| Find a service or method | [Service reference](reference/services.md) |
| Understand client options, retries, and request bodies | [Client reference](reference/client.md) |
| Handle a returned error | [Error reference](reference/errors.md) |

## Build with a device

| I want to... | Read... |
| --- | --- |
| Inspect status, change the device name, or set the clock | [Inspect and configure](guides/inspect-and-configure.md) |
| Draw content, play audio, send input, or decode a display frame | [Display and media](guides/display-and-media.md) |
| Upload application assets or manage device storage | [Assets and storage](guides/assets-and-storage.md) |
| Read or replace busy snapshots and profiles | [Busy state](guides/busy-state.md) |
| Configure Wi-Fi, BLE, an account, or smart-home pairing | [Connectivity and pairing](guides/connectivity-and-pairing.md) |
| Check or install firmware updates | [Firmware updates](guides/firmware-updates.md) |
| Receive status changes and maintain a live snapshot | [Status streams](guides/status-streams.md) |

## Integrate another transport

| I want to... | Read... |
| --- | --- |
| Use the device through an existing MQTT connection | [Remote MQTT](integrations/remote-mqtt.md) |
| Implement `remote.Transport` | [Custom MQTT transport](integrations/custom-mqtt-transport.md) |
| Use the raw firmware CLI over the USB network interface | [USB CLI](integrations/usb-cli.md) |

## Reference

- [Services](reference/services.md) lists all root client services and methods.
- [Client](reference/client.md) covers construction, options, requests, bodies, retries, and version negotiation.
- [Errors](reference/errors.md) maps error types to caller actions.
- [Packages](reference/packages.md) maps each public package to its responsibility and entry points.
- [Compatibility](reference/compatibility.md) records toolchain, firmware, platform, generated-code, and resource-limit contracts.

## Maintain the repository

- [Development](maintainers/development.md) provides the normal local development loop.
- [Testing](maintainers/testing.md) separates device-free, broker-backed, and physical-device evidence.
- [Releasing](maintainers/releasing.md) defines module order, publication gates, and recovery.
- [Device integration tests](../integration/device/README.md) documents the hardware test entry points.
- [Contributing](../CONTRIBUTING.md) defines change and documentation requirements.

The dated files under [`research/`](research/) preserve evidence for specific decisions. They are not current operating instructions. The ignored `okf/` tree is a separate research corpus and is not part of the published consumer documentation contract.
