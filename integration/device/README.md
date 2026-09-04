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
BUSYBAR_EXPECTED_FIRMWARE_VERSION=1.2.3 \
BUSYBAR_EXPECTED_API_VERSION=27.5.0 \
scripts/verify.sh device
```

The harness runs local HTTP snapshot, WebSocket lifecycle, access-token authorization and self-revocation, nested media upload/read-back/cleanup, append, XPM layering, selective-clear, and USB checks. It requires both addresses and both expected versions so release verification cannot pass through skipped or mismatched device tests. Set `BUSYBAR_ACCESS_KEY` only when the device requires a credential; the harness sends it as an API token.

The harness runs selective clear in a dedicated final process after the other local and USB checks. Firmware 1.2.3 can behave incorrectly or restart because its internal element-ID pointer list is unterminated. The test verifies the requested layer change and asserts that the device boot time did not change.

## Test USB CLI access

Set `BUSYBAR_USB_ADDRESS` to the CLI address reported for the target device. Use the same `scripts/verify.sh device` command shown above so both physical paths are verified together.

A failure can indicate permissions or descriptor access as well as a library defect.

## Broker-backed remote checks

The Paho adapter test suite exercises MQTT 5 publication, receive delivery, forced connection loss, reconnection, and subscription restoration against its pinned Mosquitto service. Caller configuration remains responsible for broker authentication and transport security.

## Release evidence

Record the device model, firmware version, exact command, and result. Do not record access keys, passwords, tokens, or correlation data.

The media test uses a unique application name and verifies that its uploaded assets are removed.
