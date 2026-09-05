# Changelog

## [0.3.1](https://github.com/lxdb/busylib-go/compare/v0.3.0...v0.3.1) (2026-09-05)


### Bug Fixes

* retract superseded module versions ([6e0e37b](https://github.com/lxdb/busylib-go/commit/6e0e37bc578270666f7f5d38790e3beefb8fdcd1))
* retract superseded module versions ([#2](https://github.com/lxdb/busylib-go/issues/2)) ([f3ca57e](https://github.com/lxdb/busylib-go/commit/f3ca57e3e2f7f51b06ad94e958e480a0e5155025))

## 0.3.0 (2026-09-05)

### Maintenance

- Establish the release baseline for the recreated repository.
- Preserve the root module API and behavior from v0.2.1.

## [0.2.1](https://github.com/lxdb/busylib-go/compare/v0.2.0...v0.2.1) (2026-09-05)


### Bug Fixes

* **firmware:** audit release tag instead of checkout head ([d9f9009](https://github.com/lxdb/busylib-go/commit/d9f900960f10a9cc5b8c544274850fa98fce4ef6))

## [0.2.0](https://github.com/lxdb/busylib-go/compare/v0.1.0...v0.2.0) (2026-09-04)


### Features

* **auth:** support minted local access tokens ([92b614b](https://github.com/lxdb/busylib-go/commit/92b614be01f0184cf2bcb8a2b710e63b63bee3ea))
* **media:** add layered display and asset contracts ([b08dd85](https://github.com/lxdb/busylib-go/commit/b08dd856ca3d0adc29a1e73c593adc5d05559683))
* **storage:** support append writes ([e9729aa](https://github.com/lxdb/busylib-go/commit/e9729aab8538e10140c07fe03c71895405b66fd4))


### Bug Fixes

* **docscheck:** remove ineffective scanner break ([cec1398](https://github.com/lxdb/busylib-go/commit/cec1398fbed9b6f9f4bd181d070cac325c0b70ca))
* **errors:** prefer firmware error codes ([a4c4515](https://github.com/lxdb/busylib-go/commit/a4c45159477ac3fcec7c04fc312443d218af2434))
* update .gitignore to include new directories ([4d08854](https://github.com/lxdb/busylib-go/commit/4d08854f91d250529f7abee2551efde4b59bfe9c))

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
