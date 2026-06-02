# Repository Guidelines

## Project Structure & Module Organization

This repository is a small Go module for a generic GORM-backed store package.
Core package files live at the repository root: `store.go`, `criteria.go`, and
`utils.go`. Unit tests are colocated as `*_test.go` files, including
`store_test.go`, `criteria_test.go`, and `utils_test.go`. Example integrations
belong in `_examples/`; currently `_examples/gin.go` demonstrates usage with
Gin and SQLite. CI configuration is in `.github/workflows/go.yml`, and lint
settings are in `.golangci.yml`.

## Build, Test, and Development Commands

- `go mod download`: download module dependencies.
- `go mod verify`: verify downloaded module checksums.
- `go test ./...`: run all tests.
- `go test -coverprofile=coverage.txt ./...`: run tests with the coverage file
  expected by CI/Codecov.
- `go build -v ./...`: compile all packages and examples covered by the module.
- `golangci-lint run --timeout=10m`: run the configured linter set.

Run `gofumpt -w .` and `goimports -w .` before submitting changes when local
formatting tools are available.

## Coding Style & Naming Conventions

Use standard Go formatting with tabs for indentation. Keep package code in
package `storeit`; avoid introducing new packages unless there is a clear
module boundary. Exported APIs use PascalCase and should include concise
documentation comments. Internal helpers use lowerCamelCase. Test helper names
should describe their fixture role, for example `setupTestDB`.

Prefer typed, composable GORM operations over string-heavy query assembly. Keep
new behavior consistent with the fluent API style used by `New`, `Where`,
`Columns`, `Hidden`, and related methods.

## Testing Guidelines

Tests use Go's `testing` package with `github.com/stretchr/testify/assert`.
Name tests with `Test<TypeOrFeature>_<Behavior>` where practical, matching
patterns such as `TestGormStore_Basic`. Keep database tests isolated by using
in-memory SQLite fixtures and `t.Cleanup` for teardown. Add or update tests for
new criteria operators, pagination behavior, transaction handling, and public
store methods.

## Commit & Pull Request Guidelines

Recent history uses short imperative summaries, sometimes in Chinese, such as
`add QueryInBatches method`, `Add WhereNeq`, and `优化分批次查询`. Keep commits
focused and concise; mention the changed behavior rather than implementation
noise.

Pull requests should include a brief description, linked issue when applicable,
and the commands run locally. Include screenshots only for documentation or
example output changes. Ensure CI-relevant checks pass: lint, test, and build.

## Security & Configuration Tips

Do not commit credentials, database files, coverage artifacts, or editor-local
state. Keep dependency changes minimal and explain why new modules are needed.
