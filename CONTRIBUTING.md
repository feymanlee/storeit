# Contributing

Thanks for helping improve `storeit`. Keep changes focused and easy to review.

## Development Setup

```bash
go mod download
go mod verify
go test ./...
go test -race ./...
golangci-lint run --timeout=10m
```

Format Go files with `gofumpt -w .` and `goimports -w .` when those tools are
available. Generated coverage files such as `coverage.txt` should stay local.

## Pull Requests

- Describe the behavior change and why it is needed.
- Link related issues when applicable.
- Add or update tests for public behavior, criteria parsing, pagination,
  transaction handling, and batch queries.
- Include the local commands you ran.
- Avoid unrelated refactors in feature or bug-fix pull requests.

## Compatibility

The module declares Go 1.18+ support. Do not raise the minimum Go version or add
new dependencies unless the compatibility impact is intentional and documented.
