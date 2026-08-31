# Device integration tests

This directory contains opt-in tests that require a physical BUSY Bar or an external broker. The `device` build tag keeps them out of ordinary unit-test runs.

## Compile without hardware

Hosted CI can compile and vet the device-tagged files and run broker-backed adapter tests. It cannot establish that a physical device is reachable or compatible.

```sh
scripts/verify.sh integration
```

The `integration` phase compiles selected device tests without running them. A passing result is compile evidence, not device evidence.

## Test local HTTP and WebSocket behavior

Set `BUSYBAR_BASE_URL` to the local device URL. Set `BUSYBAR_ACCESS_KEY` only when the device requires it.

```sh
BUSYBAR_BASE_URL=http://device-address \
BUSYBAR_USB_ADDRESS=device-usb-address \
scripts/verify.sh device
```

The harness runs local HTTP snapshot, WebSocket lifecycle, media upload/read-back/cleanup, and USB checks. It requires both addresses so release verification cannot pass through skipped tests. Set `BUSYBAR_ACCESS_KEY` only when the device requires it.

## Test USB CLI access

Set `BUSYBAR_USB_ADDRESS` to the CLI address reported for the target device. Use the same `scripts/verify.sh device` command shown above so both physical paths are verified together.

A failure can indicate permissions or descriptor access as well as a library defect.

## Broker-backed remote checks

The Paho adapter test suite exercises MQTT 5 publication, receive delivery, forced connection loss, reconnection, and subscription restoration against its pinned Mosquitto service. Caller configuration remains responsible for broker authentication and transport security.

## Release evidence

Record the device model, firmware version, exact command, and result. Do not record access keys, passwords, tokens, or correlation data.

The media test uses a unique application name and verifies that its uploaded assets are removed.
