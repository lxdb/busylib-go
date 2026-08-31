# Releasing

This repository contains two independently versioned Go modules. Releases publish source and module tags; they do not produce application archives, installers, containers, or binaries.

| Module | Tag format | Publication order |
| --- | --- | --- |
| `github.com/lxdb/busylib-go` | `vX.Y.Z` | First |
| `github.com/lxdb/busylib-go/pahotransport` | `pahotransport/vX.Y.Z` | After its required root version is public |

The adapter’s `go.mod` defines its minimum compatible root version. The modules do not need matching version numbers.

## Publication gates

Do not enable release publication until all applicable gates have evidence:

- The protobuf source checkout matches the recorded license, inventory, and schema digests.
- Root and adapter module tests, race tests, vet, lint, security, contract, generation, and coverage checks pass.
- The selected Go releases use their latest security patches.
- Local HTTP, local WebSocket, and USB checks pass on a physical BUSY Bar.
- Broker-backed MQTT connection loss, reconnection, and subscription restoration pass.
- Physical media upload, read-back, and cleanup pass without leaving test assets.
- Public API changes since the previous tag have been reviewed for compatibility.
- Examples compile against the release candidate.
- Required repository security controls are active, or an unavailable control is recorded as a manual blocker.

GitHub workflows invoke the same repository harness on hosted machines. Hosted execution compiles physical-device tests but does not satisfy the local hardware gate.

## Verify a release candidate

Run every device-free release check on the candidate commit. The command requires Docker, the pinned `protoc`, vulnerability-database access, and the protobuf and firmware checkouts:

```sh
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware scripts/verify.sh all
```

Then run the physical local HTTP, WebSocket, and USB checks. Both addresses are required so no device test can pass by skipping:

```sh
BUSYBAR_BASE_URL=http://device-address \
BUSYBAR_USB_ADDRESS=device-usb-address \
BUSYBAR_FIRMWARE_DIR=/path/to/busybar-firmware \
scripts/verify.sh release
```

`release` reruns the device-free gates before touching the physical device. Record the harness command, device model, firmware version, and result without recording credentials or device tokens.

When `pahotransport/go.mod` changes its root requirement, test the declared public dependency after that root version is available:

```sh
(cd pahotransport && GOWORK=off go test -mod=readonly ./...)
```

Use the release pull request’s workflow run as supplemental hosted evidence that the same device-free harness phases passed on Linux and macOS.

## Release automation

Release Please reads Conventional Commits on `main` and maintains a separate release pull request for each module with releasable changes. Preparation can update changelogs and the version manifest, but publication runs only when the `RELEASES_ENABLED` repository variable is exactly `true`.

The release workflow validates non-merge commits after the configured bootstrap boundary before either phase. This catches direct pushes or merge histories that bypass pull-request title validation.

| Commit signal | Version effect |
| --- | --- |
| `fix:` | Patch |
| `feat:` | Minor |
| A type followed by `!`, or a `BREAKING CHANGE:` trailer | Major, or minor before `v1.0.0` |

Other valid Conventional Commit types can appear in a changelog without forcing a release. Changes confined to `pahotransport` release only the adapter because the root release configuration excludes that path.

Release Please does not update dependencies between modules. When the adapter needs a newer root API, update and test the root requirement in `pahotransport/go.mod` before Release Please prepares the adapter release.

## Repository prerequisites

Before publication is enabled:

1. Configure squash merge as the default strategy and use the pull-request title as the squash commit message.
2. Require the `Conventional Commits`, `CI`, `Contracts`, and `Security` checks on `main` when the repository plan supports branch protection. Otherwise, treat a failed check as a manual release blocker.
3. Enable GitHub Immutable Releases.
4. Install a private GitHub App with read and write access to contents, issues, and pull requests.
5. Store the App Client ID in `RELEASE_APP_CLIENT_ID` and the PEM private key in `RELEASE_APP_PRIVATE_KEY`.
6. Keep `RELEASES_ENABLED` absent or false until publication is authorized.

The workflow uses a short-lived GitHub App installation token because events created with the default `GITHUB_TOKEN` do not trigger follow-up workflows. If App authentication fails, preparation must fail instead of creating an unverified release pull request.

## Publish

1. Merge ordinary changes with validated Conventional Commit titles.
2. Let Release Please create or update the module-specific release pull requests.
3. Review the root changelog, version manifest, dependency requirements, and required checks.
4. Set `RELEASES_ENABLED` to `true` only for an authorized root release, merge the root release pull request, verify the tag and public module, and set the variable back to false.
5. Run the adapter suite with `GOWORK=off` against the published root module.
6. Review and publish the adapter release only after that workspace-disabled check passes.
7. Confirm that publication created exactly the expected tag and GitHub Release, then set `RELEASES_ENABLED` back to false.

The initial release for each module starts at `v0.1.0`. The root module must be publicly reachable before the adapter release is merged. Add `/pahotransport` as a second Dependabot `gomod` directory only after the required root version exists.

Do not edit `.release-please-manifest.json` manually after bootstrap. If the standard action fails, correct the reported repository or workflow condition and rerun it. Do not add a second publisher or move an existing tag.

## Verify a published release

Replace the example versions with the versions just published:

```sh
GOPROXY=https://proxy.golang.org go list -m github.com/lxdb/busylib-go@v0.1.0
GOPROXY=https://proxy.golang.org go list -m github.com/lxdb/busylib-go/pahotransport@v0.1.0

consumer_dir="$(mktemp -d)"
cd "$consumer_dir"
go mod init example.com/busylib-release-check
GOPROXY=https://proxy.golang.org go get github.com/lxdb/busylib-go@v0.1.0
GOPROXY=https://proxy.golang.org go get github.com/lxdb/busylib-go/pahotransport@v0.1.0
go list -m all
```

## Correct a published release

Do not move or delete a published tag. Publish a corrected patch release through the same Release Please workflow.
