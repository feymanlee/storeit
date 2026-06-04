# storeit

[![Go Reference](https://pkg.go.dev/badge/github.com/feymanlee/storeit.svg)](https://pkg.go.dev/github.com/feymanlee/storeit)
[![CI](https://github.com/feymanlee/storeit/actions/workflows/go.yml/badge.svg)](https://github.com/feymanlee/storeit/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/feymanlee/storeit)](https://goreportcard.com/report/github.com/feymanlee/storeit)
[![codecov](https://codecov.io/gh/feymanlee/storeit/graph/badge.svg)](https://codecov.io/gh/feymanlee/storeit)
[![License](https://img.shields.io/github/license/feymanlee/storeit)](./LICENSE)

`storeit` is a small generic repository helper for GORM. It wraps common CRUD,
pagination, aggregation, preload, and criteria-based query operations while
leaving the underlying `*gorm.DB` available for error handling and inspection.

## Installation

```bash
go get github.com/feymanlee/storeit
```

The module currently declares Go 1.18+ support.

## Quick Start

```go
type User struct {
	ID     int64  `gorm:"primaryKey"`
	Name   string `gorm:"column:name"`
	Status string `gorm:"column:status"`
}

store := storeit.New[User](db)

user := User{Name: "Ada", Status: "active"}
if err := store.Create(ctx, &user).Error; err != nil {
	return err
}

criteria := storeit.NewCriteria().
	Where("status = ?", "active").
	OrderDesc("id").
	Page(1).
	PerPage(20)

page, err := store.Paginate(ctx, criteria)
if err != nil {
	return err
}
fmt.Println(page.Total, len(page.Items))
```

## Criteria Tags

`ExtractCriteria` builds a `Criteria` from struct tags, which is useful for HTTP
query request structs.

```go
type SearchUsers struct {
	Keyword string `form:"keyword" criteria:"name,email:like"`
	Status  string `form:"status" criteria:"status:eq"`
	Page    int    `form:"page" criteria:"-:page"`
	PerPage int    `form:"per_page" criteria:"-:per_page"`
	Sort    string `form:"sort" criteria:"-:sort"`
}

criteria, err := storeit.ExtractCriteria(req)
```

Supported tags:

| Tag             | Value type      | SQL behavior                         |
|-----------------|-----------------|--------------------------------------|
| `field:eq`      | any             | `field = ?`                          |
| `field:neq`     | any             | `field <> ?`                         |
| `field:gt`      | any             | `field > ?`                          |
| `field:gte`     | any             | `field >= ?`                         |
| `field:lt`      | any             | `field < ?`                          |
| `field:lte`     | any             | `field <= ?`                         |
| `field:like`    | string          | `field LIKE "%value%"`               |
| `field:llike`   | string          | `field LIKE "%value"`                |
| `field:rlike`   | string          | `field LIKE "value%"`                |
| `field:in`      | slice           | `field IN (?)`                       |
| `field:notin`   | slice           | `field NOT IN (?)`                   |
| `field:isnull`  | any             | `field IS NULL`                      |
| `field:notnull` | any             | `field IS NOT NULL`                  |
| `field:between` | slice, length 2 | `field BETWEEN ? AND ?`              |
| `-:sort`        | string          | `ORDER BY`, for example `id-,name+`  |
| `-:page`        | int             | page number, defaults to 1           |
| `-:per_page`    | int             | page size, defaults to 50 in paginate |
| `-:limit`       | int             | `LIMIT ?`                            |
| `-:offset`      | int             | `OFFSET ?`                           |

Sort fields are validated as simple identifiers such as `id`, `created_at`, or
`users.created_at` before they are added to `ORDER BY`.

## Examples

See [`_examples/gin.go`](./_examples/gin.go) for a Gin and SQLite integration
example. Runnable documentation examples are also available on
[pkg.go.dev](https://pkg.go.dev/github.com/feymanlee/storeit).

## Development

```bash
go mod download
go mod verify
go test ./...
go test -race ./...
go test -coverprofile=coverage.txt ./...
golangci-lint run --timeout=10m
```

Run `gofumpt -w .` and `goimports -w .` before sending a pull request when
those tools are available locally.

## Contributing

Issues and pull requests are welcome. Please read
[`CONTRIBUTING.md`](./CONTRIBUTING.md) before proposing changes.
