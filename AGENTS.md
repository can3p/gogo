# Agent Context for Gogo Library

## Overview

Gogo is a Go library providing common utilities for web applications built with the Gin framework.

## Packages

### `forms/`
Form handling with htmx integration. Provides validation, error handling, and SPA-like behavior.

### `testcontainers/postgres/`
PostgreSQL databases for integration tests, on testcontainers-go.

**Key API:**
- `New(t testing.TB, opts ...Option) *TestDB` - migrated, empty database per test; dropped via `t.Cleanup`
- Options: `WithMigrationsDir` (required), `WithMigrationsTable` (default `"migrations"`), `WithImage` (default `postgres:16-alpine`)
- `TestDB` - `DB *sqlx.DB`, `SQL *sql.DB`, `URL string`
- `Cleanup()` - terminates the shared container (call in `TestMain`)
- Deprecated: `NewTestDB(Options)`, `Options`, `ConnectionInfo`, `TestDB.ConnInfo`

**Design:** one container per test binary; migrations applied once into a template database; each test gets `CREATE DATABASE ... TEMPLATE`.

**Usage pattern:**
```go
func TestMain(m *testing.M) {
    code := m.Run()
    _ = postgres.Cleanup()
    os.Exit(code)
}

func TestExample(t *testing.T) {
    db := postgres.New(t, postgres.WithMigrationsDir("path/to/migrations"))
    // Use db.DB for queries, db.URL for external tools
}
```

### `sender/`
Email sending utilities.

### `util/`
General utilities.

### `links/`
URL/link generation helpers.

### `markdown/`
Markdown processing utilities.

## Development

### Development Workflow
Before making code changes, run `make fix` to ensure code is properly formatted and dependencies are up to date:
```bash
make fix
```

This runs `go fix` and `go mod tidy` to prevent old code patterns from slipping in.

### Verification
Always run checks before committing:
```bash
make check
```

This runs build, test, and lint in sequence.

### Individual Commands
```bash
make test   # Run tests with race detection and coverage
make lint   # Run golangci-lint
make build  # Build all packages
make fix    # Run go fix and go mod tidy
```

### CI Requirements
- All PRs must pass `make test` and `make lint`
- Tests require Docker (for testcontainers)

## Dependencies

- `github.com/gin-gonic/gin` - Web framework
- `github.com/jmoiron/sqlx` - SQL extensions
- `github.com/volatiletech/sqlboiler/v4` - ORM
- `github.com/testcontainers/testcontainers-go` - Docker test containers
- `github.com/rubenv/sql-migrate` - Database migrations
