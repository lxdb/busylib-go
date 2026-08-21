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

## Certificates and licensing

By submitting a contribution, you confirm that you can license it to this project.
Contributions use the project license unless a file states otherwise.
