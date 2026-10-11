# Agent Context for Gogo Library

## Overview

Gogo is a Go library providing common utilities for web applications built with the Gin framework.

## Packages

### `forms/`
Form handling with htmx integration. Provides validation, error handling, and SPA-like behavior.
`Validate(c)` and `Save(ctx)` take no executor, and `DefaultHandler(c, logger, form)` opens no transaction: a form
holds the services it calls.

### `testcontainers/postgres/`
PostgreSQL databases for integration tests, on testcontainers-go.

**Key API:**
- `New(t testing.TB, opts ...Option) *TestDB` - migrated, empty database per test; dropped via `t.Cleanup`
- Options: `WithMigrationsDir` (required), `WithMigrationsTable` (default `"migrations"`), `WithImage` (default `postgres:16-alpine`), `WithLogger` (default logs to `t.Log`)
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

### `settings/`
go-flags helpers: `Parse(p, args)` treats an empty required setting as missing and names flag and env var;
`Secret` redacts itself when printed.

### `apperr/`
Errors services return and transports map: `ErrNotFound`, `ErrForbidden`, `ErrNeedsLogin`, `ErrConflict`,
`ValidationError`/`Invalid`. Imports nothing.

### `sender/`
`Sender.Send(ctx, mail)`; console (`NewSender(logger)`, logs the mail) and Mailjet senders. Mailjet's `config.Config.BaseURL` points it at a mock
(tommy). No database: queueing mail is the application's job.

### `sessions/pgstore/`
gin sessions (gin-contrib/sessions on gorilla/sessions) stored in PostgreSQL. Reads and writes the rows and
cookies of `antonlindstrom/pgstore`, so an app can switch without logging anyone out.

**Key API:**
- `New(db *sql.DB, keyPairs ...[]byte) *Store` - path `/`, `DefaultMaxAge` (30 days); a `gorilla` and a
  `gin-contrib` `sessions.Store`; `Options`, `MaxAge`, `MaxLength`
- `Schema` - the `http_sessions` DDL (pgstore's, with `IF NOT EXISTS`): the app puts it into its own
  migration; the store never creates its table
- `Regenerate(ctx, ginsessions.Session)` - deletes the old row and clears the session, so the next `Save` uses a
  fresh ID and cookie (login and logout, against session fixation)
- `DeleteExpired(ctx)`, `Cleanup(ctx, interval, logger)` - the app starts `Cleanup` itself (blocking, until
  ctx is done)

A bad or tampered cookie and a missing or expired row give a new empty session, not an error. A row expires
MaxAge seconds after its last save; with MaxAge 0 (a browser-session cookie), after `DefaultMaxAge`.

### `util/`
`ginhelpers` (`Status`, `HTML`, `HTMLError`, `API`, configured per router with `Configure(Options)`),
`ginhelpers/csrf`, `transact`, `lorem`, `logging`. No globals: configuration is passed in, never read from
the environment.

## Logging

Gogo logs with `*zap.Logger` (`go.uber.org/zap`), always passed in by the caller: no `fmt.Print*`, `log`,
or `slog`, and no global logger.

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
- `github.com/jessevdk/go-flags` - settings
- `github.com/testcontainers/testcontainers-go` - Docker test containers
- `github.com/rubenv/sql-migrate` - Database migrations
- `go.uber.org/zap` - Logging
- `github.com/gorilla/sessions`, `github.com/gorilla/securecookie`, `github.com/gin-contrib/sessions` - session store
