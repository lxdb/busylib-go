# Release busylib-go

This repository publishes source and Go module tags. It does not publish application archives, installers, containers, or binaries.

## Release order

| Module | Tag format | Order |
| --- | --- | --- |
| `github.com/lxdb/busylib-go` | `vX.Y.Z` | Publish first |
| `github.com/lxdb/busylib-go/pahotransport` | `pahotransport/vX.Y.Z` | Publish after its required root version is public |
| `github.com/lxdb/busylib-go/ble` | `ble/vX.Y.Z` | Publish after its required root version is public and BLE qualification passes |

The modules are independently versioned and do not need matching version numbers. Each optional module's `go.mod` defines its minimum root version.

Release Please tracks BLE as an independent component. Do not merge or publish its release pull request until the physical BLE gate passes and the module points to a published root version that satisfies its declared API requirements.

## Satisfy publication gates

Do not enable publication until all applicable gates have evidence:

- The protobuf source matches the recorded license, inventory, and schema digests.
- Root and optional-module tests, race tests, vet, lint, security, contracts, generation, and coverage pass.
- The selected Go releases use their latest security patches.
- Local HTTP, local WebSocket, USB, and physical media checks pass on a BUSY Bar.
- Broker-backed connection loss, reconnection, and subscription restoration pass.
- BLE fresh pairing, HTTP, FFE1, cleanup, and at least 30 saved-bond reconnect cycles pass on the supported macOS and firmware combination.
- Public API changes since the previous tag have a compatibility review.
- Examples compile against the release candidate.
- Required repository security controls are active, or an unavailable control is recorded as a blocker.

Hosted workflows compile physical-device tests but do not satisfy the hardware gate.

## Verify a release candidate

Run every device-free gate on the candidate commit:

```sh
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware scripts/verify.sh all
```

For a root or Paho candidate, run the complete release command with both physical addresses:

```sh
BUSYBAR_BASE_URL=http://device-address \
BUSYBAR_USB_ADDRESS=device-usb-address \
BUSYBAR_EXPECTED_FIRMWARE_VERSION=1.2.3 \
BUSYBAR_EXPECTED_API_VERSION=27.5.0 \
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware \
scripts/verify.sh release
```

For a BLE candidate, run its separate gate on macOS. This command requires the write/read-back cleanup check and runs 30 saved-bond reconnect cycles:

```sh
BUSYBAR_BLE_IDENTIFIER='<corebluetooth-uuid>' \
BUSYBAR_BLE_WRITE_TEST=1 \
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware \
scripts/verify.sh ble-release
```

Record the harness command, device model, firmware version, and result. Do not record credentials or device tokens. Use the release pull request workflow as supplemental Linux and macOS evidence.

## Verify the declared module dependency

When an optional module changes its root requirement, wait until that root version is public and test without a workspace:

```sh
(cd pahotransport && GOWORK=off go test -mod=readonly ./...)
(cd ble && GOWORK=off go test -mod=readonly ./...)
```

This check proves that each optional module consumes its declared public dependency instead of the local checkout.

## Understand release automation

Release Please reads Conventional Commits on `main` and maintains a separate release pull request for each module with releasable changes. Preparation can update changelogs and the version manifest. Publication runs only when the `RELEASES_ENABLED` repository variable is exactly `true`.

| Commit signal | Version effect |
| --- | --- |
| `fix:` | Patch |
| `feat:` | Minor |
| Type followed by `!`, or a `BREAKING CHANGE:` trailer | Major, or minor before `v1.0.0` |

Other valid Conventional Commit types can appear in a changelog without forcing a release. Changes confined to `pahotransport/` or `ble/` release only that module because the root release configuration excludes both paths. Release Please does not update dependencies between modules.

The release workflow validates non-merge commits after the configured bootstrap boundary. This detects direct pushes or merge histories that bypass pull-request title validation.

## Configure repository prerequisites

Before publication is enabled:

1. Configure squash merge as the default and use the pull-request title as the squash commit message.
2. Require the `Conventional Commits`, `CI`, `Contracts`, and `Security` checks on `main` when the repository plan supports branch protection. Otherwise, treat a failed check as a release blocker.
3. Enable GitHub Immutable Releases.
4. Install a private GitHub App with read and write access to contents, issues, and pull requests.
5. Store the App Client ID in `RELEASE_APP_CLIENT_ID` and its PEM private key in `RELEASE_APP_PRIVATE_KEY`.
6. Keep `RELEASES_ENABLED` absent or false until publication is authorized.

The workflow uses a short-lived GitHub App installation token because events created with the default `GITHUB_TOKEN` do not trigger follow-up workflows. If App authentication fails, preparation must fail instead of creating an unverified release pull request.

## Publish

1. Merge ordinary changes with validated Conventional Commit titles.
2. Let Release Please create or update the module-specific release pull requests.
3. Review the root changelog, version manifest, dependency requirements, and required checks.
4. Set `RELEASES_ENABLED` to `true` only for an authorized root release.
5. Merge the root release pull request, verify the tag and public module, and set the variable back to false.
6. Run the workspace-disabled suite for each affected optional module against the published root module.
7. Review and publish an optional-module release only after its check passes.
8. Confirm that publication created exactly the expected tag and GitHub Release, then set `RELEASES_ENABLED` back to false.

The root module must be public before an optional-module release that depends on it. Keep each optional `gomod` Dependabot directory aligned with a public root requirement.

Do not edit `.release-please-manifest.json` manually after bootstrap. If the standard action fails, correct the reported condition and rerun it. Do not add a second publisher or move an existing tag.

## Verify a published release

Replace the example versions with the versions just published:

```sh
GOPROXY=https://proxy.golang.org go list -m github.com/lxdb/busylib-go@v0.1.0
GOPROXY=https://proxy.golang.org go list -m github.com/lxdb/busylib-go/pahotransport@v0.1.0
GOPROXY=https://proxy.golang.org go list -m github.com/lxdb/busylib-go/ble@v0.1.0

consumer_dir="$(mktemp -d)"
cd "$consumer_dir"
go mod init example.com/busylib-release-check
GOPROXY=https://proxy.golang.org go get github.com/lxdb/busylib-go@v0.1.0
GOPROXY=https://proxy.golang.org go get github.com/lxdb/busylib-go/pahotransport@v0.1.0
GOPROXY=https://proxy.golang.org go get github.com/lxdb/busylib-go/ble@v0.1.0
go list -m all
```

Published tags are immutable. If a release is wrong, publish a corrected patch release through the same workflow. Do not move or delete the tag.
