package postgres

import "fmt"

// ConnectionInfo contains database connection details.
//
// Deprecated: use TestDB.URL, which carries the same details.
type ConnectionInfo struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// ConnectionString returns a PostgreSQL connection string.
func (c ConnectionInfo) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode)
}

// Options configures NewTestDB.
//
// Deprecated: use New with WithMigrationsDir and WithMigrationsTable.
type Options struct {
	// MigrationsDir is the path to the migrations directory. Unlike New,
	// NewTestDB treats an empty value as "apply no migrations".
	MigrationsDir string

	// MigrationsTable is the sql-migrate bookkeeping table. Empty keeps the
	// historical behaviour of using sql-migrate's global setting
	// ("gorp_migrations" unless migrate.SetTable was called).
	MigrationsTable string
}

// NewTestDB creates a new isolated test database with migrations applied.
// The caller must Close it.
//
// It shares the container and the template databases with New, so it gets
// the same speed-up, but it returns an error instead of failing a test and
// does not register any cleanup.
//
// Deprecated: use New, which fails the test on error and drops the database
// when the test ends.
func NewTestDB(opts Options) (*TestDB, error) {
	return newTestDB(config{
		image:           DefaultImage,
		migrationsDir:   opts.MigrationsDir,
		migrationsTable: opts.MigrationsTable,
		legacy:          true,
	})
}
