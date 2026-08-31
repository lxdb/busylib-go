# Changelog

This file records user-visible changes to `busylib-go`.

Published release history is generated from Conventional Commits and recorded in GitHub Releases.
The `Unreleased` section below documents the initial release preparation only; contributors do not update this file for each change.

## Unreleased

### Added

- An API 25 client for local HTTP operations and status streams.
- Optional remote MQTT, USB CLI, snapshot, frame, image, and audio packages.
- Public examples, compatibility guidance, and release verification checks.

### Changed

- Original project code now uses the MIT License.
- The main module requires Go 1.23, the optional Paho module requires Go 1.24, and physical-device tests share the root module.
- The optional Paho adapter moved to the separate `github.com/lxdb/busylib-go/pahotransport` module and now owns connection, reconnection, subscription, and shutdown lifecycle.
- Display screen capture and HTTP frame decoding now use `DisplayTarget` instead of numeric display selectors.
- Wi-Fi scan and connection types are now `WiFiNetwork`, `WiFiNetworkList`, and `WiFiConnectRequest`.
- Internal success responses, display discriminators, and USB prompt constants are no longer exported.

No public version has been tagged.
