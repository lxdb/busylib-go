# BUSY Bar protobuf license research

Status: **release blocked**. Evidence last rechecked on 2026-08-30 UTC.

This report records the evidence used to decide whether the BUSY Bar protobuf inputs copied into this repository, and the Go packages generated from them, have compatible public redistribution permission. It is a dated research record, not legal advice or current operational guidance.

## Finding

No compatible redistribution permission was found in the first-party public record.

The audited upstream repository is public, but GitHub reports no detected license, its license endpoint returns `404 Not Found`, its audited tree contains no license or notice file, and the schema files contain no copyright or license headers. The upstream request to add a license remains open without a maintainer response.

Repository visibility is not treated as a license grant. Licenses used by other BUSY client repositories are not applied to this separate repository. Do not publish the copied inputs, generated Go outputs, a release tag containing them, or the Go module until the condition in [Evidence required to clear the blocker](#evidence-required-to-clear-the-blocker) is met.

## Audited source

- Repository: [`busy-app/busybar-protobuf`](https://github.com/busy-app/busybar-protobuf).
- Commit: [`dba670e2ddb5cda511af997ca5fcb1254e90917f`](https://github.com/busy-app/busybar-protobuf/commit/dba670e2ddb5cda511af997ca5fcb1254e90917f), authored on 2026-07-17 and merged through pull request [#8](https://github.com/busy-app/busybar-protobuf/pull/8).
- Branch state at recheck: `dev` resolved to the audited commit.
- Repository metadata at recheck: public repository, `license: null`.
- License API result at the audited commit: `404 Not Found`.

The official [repository API](https://api.github.com/repos/busy-app/busybar-protobuf), [branch API](https://api.github.com/repos/busy-app/busybar-protobuf/branches/dev), and [license API](https://api.github.com/repos/busy-app/busybar-protobuf/license?ref=dba670e2ddb5cda511af997ca5fcb1254e90917f) reproduce these metadata checks.

## Tree and file evidence

The complete [tree at the audited commit](https://github.com/busy-app/busybar-protobuf/tree/dba670e2ddb5cda511af997ca5fcb1254e90917f) contains `README.md`, `.proto` files, nanopb `.options` files, `.gitignore`, and an empty `Changelog`. It contains no `LICENSE`, `COPYING`, `NOTICE`, SPDX or REUSE metadata, or equivalent permission file.

The commit’s [README](https://github.com/busy-app/busybar-protobuf/blob/dba670e2ddb5cda511af997ca5fcb1254e90917f/README.md) describes the protocol, transports, generator, and directory layout. It does not state a license or redistribution permission. Pull request [#8](https://github.com/busy-app/busybar-protobuf/pull/8) contains no licensing statement.

The audited `.proto` and `.options` files contain protocol declarations and generator constraints but no copyright, license, SPDX, or permission header. Representative files include [`state.proto`](https://github.com/busy-app/busybar-protobuf/blob/dba670e2ddb5cda511af997ca5fcb1254e90917f/state.proto), [`state/wifi.proto`](https://github.com/busy-app/busybar-protobuf/blob/dba670e2ddb5cda511af997ca5fcb1254e90917f/state/wifi.proto), [`frame.proto`](https://github.com/busy-app/busybar-protobuf/blob/dba670e2ddb5cda511af997ca5fcb1254e90917f/frame.proto), and [`frame.options`](https://github.com/busy-app/busybar-protobuf/blob/dba670e2ddb5cda511af997ca5fcb1254e90917f/frame.options).

## Upstream license request

Issue [#9, “No LICENSE in this repo”](https://github.com/busy-app/busybar-protobuf/issues/9) was opened on 2026-08-02 to ask the maintainers for a license that covers use by other-language clients. At the recheck date, the issue remained open. It had one follow-up comment from the requester and no maintainer response. The issue therefore records the request but supplies no permission.

## Firmware and SDK cross-checks

The first-party firmware repository identifies this protobuf repository as the `assets/proto` submodule in [`.gitmodules`](https://github.com/busy-app/busybar-firmware/blob/dev/.gitmodules). At recheck, its `dev` tree pointed that gitlink to the audited protobuf commit. This establishes provenance and first-party use; it does not establish a redistribution grant for the separate repository.

The firmware [`LICENSE.md`](https://github.com/busy-app/busybar-firmware/blob/dev/LICENSE.md) and [`REUSE.toml`](https://github.com/busy-app/busybar-firmware/blob/dev/REUSE.toml) describe licensing for material in the firmware repository. The protobuf content is stored as a gitlink to another repository. This report does not extend the superproject’s licensing rules to the submodule contents.

The official BUSY client repositories each carry their own MIT license:

- [`busylib-py/LICENSE`](https://github.com/busy-app/busylib-py/blob/main/LICENSE)
- [`busylib-ts/LICENSE`](https://github.com/busy-app/busylib-ts/blob/dev/LICENSE)
- [`busylib-kmp/LICENSE`](https://github.com/busy-app/busylib-kmp/blob/main/LICENSE)

Those licenses cover their respective repositories. None names `busybar-protobuf`, the audited commit, or the copied schema inputs.

## Impact on this repository

`internal/protosrc/bsb-protobuf` contains 32 files. A byte-for-byte comparison with the audited upstream commit, excluding the upstream README that is not copied locally, found no differences. The local directory therefore contains the complete non-README file set from that snapshot, including `.proto` and `.options` inputs, `.gitignore`, and `Changelog`.

The eight package directories under `proto` contain 15 generated files from the 15 schemas listed in `scripts/protobuf-packages.tsv`. Generated files identify their source schemas and contain schema-derived declarations and descriptors. The generator and Go protobuf runtime have their own licenses, but those licenses do not supply permission for the upstream schema inputs.

The project license cannot grant rights in third-party material that the project does not own. Attribution in [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) records provenance but does not replace redistribution permission.

## Evidence required to clear the blocker

Document one of these outcomes before publication:

1. **Upstream license grant.** The copyright holder adds a license whose scope covers the audited `.proto`, `.options`, and auxiliary files under terms compatible with the intended distribution. Update the pinned snapshot, preserve required notices, regenerate the Go packages, and rerun the protobuf drift check.
2. **Direct written permission.** The relevant rightsholder grants permission that covers copying, modifying, generating code from, and redistributing the protobuf inputs and generated Go outputs under the intended distribution terms. Preserve a durable record and reflect its conditions in the repository notices.
3. **Removal of affected material.** Remove the copied inputs and schema-derived Go outputs from the release. Review any replacement independently for provenance and rights.

An unanswered issue, public repository visibility, use as a firmware submodule, a license on a different repository, or an attribution-only notice does not meet this evidence threshold.
