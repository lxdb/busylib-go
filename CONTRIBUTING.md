# Contributing

Contributions should be focused, testable, and compatible with the repository’s pinned contracts.

## Before you start

Open an issue before a large API or protocol change. Do not include secrets, device tokens, broker credentials, or private firmware data. Check [Compatibility](docs/compatibility.md) before changing a toolchain, generated artifact, firmware contract, platform assumption, or safety limit.

## Submit a change

1. Create a focused branch.
2. Add tests for observable behavior when the risk justifies them.
3. Run the relevant targeted checks, then `go test ./...` and `go vet ./...`.
4. Update public documentation when the user-visible contract changes.
5. Use a Conventional Commit pull-request title.
6. Describe risks, verification evidence, and any check that was not run.

Keep unrelated formatting, generated output, and cleanup out of the change. Follow [Development](docs/development.md) for module-specific, generated-code, and device-test commands.

## Commit messages

Release Please derives module versions and changelogs from Conventional Commits on `main`.

| Type | Use |
| --- | --- |
| `feat` | Backward-compatible feature |
| `fix` | Bug fix |
| `docs`, `test`, `refactor`, `perf`, `build`, `ci`, or `chore` | A change that is best described by that standard type |
| A type followed by `!`, or a `BREAKING CHANGE:` trailer | Breaking change |

GitHub Actions validates pull-request titles. The repository’s release flow expects the validated title to become the squash commit message on `main`.

## Documentation style

Write for the reader’s task. Start with purpose or outcome, then prerequisites, procedure, expected result, and failure guidance when those sections apply.

- Use direct, matter-of-fact sentences and established technical terms.
- Prefer active voice and explicit ownership. Name the caller, library, workflow, or maintainer that performs an action.
- Give one instruction per step. State conditions before the action they control.
- Keep values in their source-owned file and link to that source when copying the value would create drift.
- Keep repository-wide guidance in `docs/`, package contracts in Go comments, and executable usage in examples.
- Mark dated investigations as historical evidence. Do not present them as current operating instructions.
- Do not hard-wrap Markdown paragraphs, list items, headings, or table rows. Keep one logical item on one source line.
- Avoid slogans, marketing claims, conversational filler, and unsupported assurances.

These rules are inspired by controlled technical writing. They are not a claim of formal ASD-STE100 compliance.

## Certificates and licensing

By submitting a contribution, you confirm that you can license it to this project. Contributions use the project license unless a file states otherwise.
