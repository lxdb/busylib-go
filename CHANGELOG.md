# Changelog

## 0.1.0 (2026-08-31)


### ⚠ BREAKING CHANGES

* **stream:** stream.Stream replaces Errors with Wait and standardizes one-shot Start, Stop, cleanup, and terminal-error behavior.
* **remote:** SubscriptionRequest requires MaxPayloadBytes, and transports must reject larger payloads before retaining them for a subscriber.
* **client:** PreparedRequest fields are private; callers must use Method, Path, URL, Header, ResponseMode, and RequestID accessors.
* **api:** Wi-Fi request and response types were renamed, and implementation-only response, display discriminator, and USB protocol constants are no longer exported.
* **display:** DisplayService.Screen and frame.FromHTTP now accept named display targets instead of numeric selectors.

### Features

* **display:** replace numeric selectors with named targets ([dee7db4](https://github.com/lxdb/busylib-go/commit/dee7db4afd3a4b6af45550af82ce046ab8ea6d42))
* **pahotransport:** add the optional Paho transport module ([057499d](https://github.com/lxdb/busylib-go/commit/057499d81f8369835376e23a5967d49e5d00f889))
* **remote:** enforce subscription payload limits ([4c14f03](https://github.com/lxdb/busylib-go/commit/4c14f03c857e1c15ff59f1504d75e2872cbdb97b))
* **stream:** expose stable stream completion ([ad8961a](https://github.com/lxdb/busylib-go/commit/ad8961a5d402483151d02428dbe3e72b5f8ff1d3))


### Bug Fixes

* **ci:** defer CodeQL to default setup ([87e7283](https://github.com/lxdb/busylib-go/commit/87e72838e4fdc625a7669d090f8e7da7dd9aeb6c))
* **ci:** remove deprecated action interfaces ([22eb202](https://github.com/lxdb/busylib-go/commit/22eb2025d71b10bfccdf5b28acd9cbdadea1c15d))
* **client:** isolate prepared request state ([00e000b](https://github.com/lxdb/busylib-go/commit/00e000bead2d605442dd30322b97cd7fc450b42f))
* **client:** preserve shared version refreshes ([332b607](https://github.com/lxdb/busylib-go/commit/332b6077f1d23c7caa7ed8c6fd9e94717c8bb8f2))
* **client:** retry only safe repeatable requests ([572cf8d](https://github.com/lxdb/busylib-go/commit/572cf8ddea4b1c7b73901c610fdb977d58048315))


### Performance Improvements

* **animation:** bound frame conversion memory ([8115edd](https://github.com/lxdb/busylib-go/commit/8115edd2c5809f5b17556c4fcc604dc572a6fb6d))


### Code Refactoring

* **api:** narrow the initial public surface ([95875a9](https://github.com/lxdb/busylib-go/commit/95875a9e57928b3354483343568ce3e331c3bba3))

## Changelog

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
