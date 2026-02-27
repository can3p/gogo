# Agent Context for Gogo Library

## Overview

Gogo is a Go library providing common utilities for web applications built with the Gin framework.

## Packages

### `forms/`
Form handling with htmx integration. Provides validation, error handling, and SPA-like behavior.

### `testcontainers/postgres/`
PostgreSQL test container for integration testing.

**Key types:**
- `TestDB` - Wraps a test database instance with `*sqlx.DB` and connection info
- `Options` - Configuration for `NewTestDB()`, includes `MigrationsDir`
- `ConnectionInfo` - Database connection details (Host, Port, User, Password, DBName, SSLMode)

**Key functions:**
- `NewTestDB(opts Options)` - Creates isolated test database with migrations applied
- `Cleanup()` - Purges the shared container (call in `TestMain`)

**Usage pattern:**
```go
func TestMain(m *testing.M) {
    code := m.Run()
    _ = postgres.Cleanup()
    if code != 0 {
        os.Exit(code)
    }
}

func TestExample(t *testing.T) {
    testDB, err := postgres.NewTestDB(postgres.Options{
        MigrationsDir: "path/to/migrations",
    })
    require.NoError(t, err)
    defer testDB.Close()
    
    // Use testDB.DB for queries
    // Use testDB.ConnInfo for connection details
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
- `github.com/ory/dockertest/v3` - Docker test containers
- `github.com/rubenv/sql-migrate` - Database migrations
