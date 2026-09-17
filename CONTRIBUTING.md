# Contributing

## Requirements

- Go 1.26 or newer
- golangci-lint v2.11 (`brew install golangci-lint`)
- goreleaser v2 for release testing (`brew install goreleaser`)

## Workflow

```bash
make build      # compile the binary
make test       # go test -race ./...
make lint       # golangci-lint run
make check      # lint + test, run this before pushing
```

CI runs the same test, lint, and snapshot-build jobs on every pull request.

## Conventions

- Commands live in `internal/cmd/<name>`; shared plumbing is in
  `internal/cmd/cmdutil`.
- API calls belong in `internal/api`, one file per resource. Commands must not
  build HTTP requests themselves.
- Every user-facing error is an `ExitError` from `internal/errors` with a
  message and an actionable suggestion. Exit codes are semantic; see the README.
- Every command supports table, `--json`, and `--quiet` output.
- Tests are table-driven and run against `httptest` stubs; see
  `internal/testutil`. No test may reach the network.
- Comments explain why, not what.

## Releasing

Tag a commit on `main`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow runs tests, builds binaries for linux, darwin, and windows
on amd64 and arm64, publishes a GitHub release, and updates the Homebrew cask in
`cloud-yyy/homebrew-tap`. It needs the `HOMEBREW_TAP_GITHUB_TOKEN` secret: a
fine-grained personal access token with read and write access to Contents on
that tap repository only.
