# Contributing

Thank you for improving `busylib-go`.

## Before you start

Open an issue before a large API or protocol change.
Do not include secrets, device tokens, or private firmware data.
Confirm that your change is compatible with the pinned firmware contract.

## Submit a change

1. Create a focused branch.
2. Add tests for observable behavior.
3. Run `go test ./...` and `go vet ./...`.
4. Update public documentation when behavior changes.
5. Add a changelog entry for user-visible changes.
6. Open a pull request with risks and verification evidence.

Keep commits small and reviewable.
Do not include unrelated formatting or generated changes.

## Commit messages

Release Please derives module versions and changelogs from Conventional Commits on `main`.

| Type | Use |
| --- | --- |
| `feat` | Backward-compatible feature |
| `fix` | Bug fix |
| `docs`, `test`, `refactor`, `perf`, `build`, `ci`, or `chore` | A change that is best described by that standard type |
| A type followed by `!`, or a `BREAKING CHANGE:` trailer | Breaking change |

GitHub Actions validates pull-request titles. The repository’s release flow expects the validated title to become the squash commit message on `main`.

## Certificates and licensing

By submitting a contribution, you confirm that you can license it to this project.
Contributions use the project license unless a file states otherwise.
