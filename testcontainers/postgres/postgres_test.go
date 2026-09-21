package postgres_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/can3p/gogo/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const migrationsDir = "testdata/migrations"

// The tables created by testdata/migrations.
var migratedTables = []string{"authors", "books"}

func TestMain(m *testing.M) {
	code := m.Run()

	if err := postgres.Cleanup(); err != nil {
		// The reaper removes the container when the binary exits either way,
		// so this is worth reporting but not worth failing a green run over.
		fmt.Fprintln(os.Stderr, "postgres cleanup:", err)
	}

	os.Exit(code)
}

func newDB(t *testing.T, opts ...postgres.Option) *postgres.TestDB {
	t.Helper()
	return postgres.New(t, append([]postgres.Option{postgres.WithMigrationsDir(migrationsDir)}, opts...)...)
}

func tableExists(t *testing.T, db *sqlx.DB, table string) bool {
	t.Helper()

	var exists bool
	require.NoError(t, db.Get(&exists,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
		table))

	return exists
}

func databaseExists(t *testing.T, db *sqlx.DB, name string) bool {
	t.Helper()

	var exists bool
	require.NoError(t, db.Get(&exists, "SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1)", name))

	return exists
}

func TestNewGivesAWorkingDatabase(t *testing.T) {
	db := newDB(t)

	var one int
	require.NoError(t, db.DB.Get(&one, "SELECT 1"))
	assert.Equal(t, 1, one)

	require.NoError(t, db.SQL.QueryRow("SELECT 1").Scan(&one))
	assert.Equal(t, 1, one)
}

// sql-migrate's Go API defaults to "gorp_migrations", while its CLI reads the
// table from dbconfig.yml. The harness must record migrations in the table
// the CLI looks at, or the two disagree about what is applied.
func TestMigrationsAreRecordedInTheConfiguredTable(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		db := newDB(t)

		var applied int
		require.NoError(t, db.DB.Get(&applied, "SELECT count(*) FROM "+postgres.DefaultMigrationsTable))
		assert.Equal(t, 2, applied)
		assert.False(t, tableExists(t, db.DB, "gorp_migrations"))
	})

	t.Run("custom", func(t *testing.T) {
		db := newDB(t, postgres.WithMigrationsTable("schema_history"))

		var applied int
		require.NoError(t, db.DB.Get(&applied, "SELECT count(*) FROM schema_history"))
		assert.Equal(t, 2, applied)
		assert.False(t, tableExists(t, db.DB, postgres.DefaultMigrationsTable))
		assert.False(t, tableExists(t, db.DB, "gorp_migrations"))
	})
}

// Each test must get the migrated schema, and nothing written by another test.
func TestDatabaseStartsMigratedAndEmpty(t *testing.T) {
	for i := range 2 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			db := newDB(t)

			for _, table := range migratedTables {
				var count int
				require.NoError(t, db.DB.Get(&count, "SELECT count(*) FROM "+table),
					"table %s should exist in a migrated database", table)
				require.Zero(t, count, "table %s should start empty", table)
			}

			// Leave rows behind for the next iteration to not see.
			_, err := db.DB.Exec("INSERT INTO authors (name) VALUES ('Anonymous')")
			require.NoError(t, err)
		})
	}
}

// The property the package exists for: one test's writes are invisible to
// another's, including when they run at the same time.
func TestDatabasesAreIsolated(t *testing.T) {
	for _, name := range []string{"first", "second", "third", "fourth"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db := newDB(t)

			_, err := db.DB.Exec("CREATE TABLE scratch (note text)")
			require.NoError(t, err, "each test should get a database with no scratch table in it")

			_, err = db.DB.Exec("INSERT INTO authors (name) VALUES ($1)", name)
			require.NoError(t, err)

			var names []string
			require.NoError(t, db.DB.Select(&names, "SELECT name FROM authors"))
			require.Equal(t, []string{name}, names, "only this test's row should be visible")
		})
	}
}

func TestCleanupDropsTheDatabase(t *testing.T) {
	var name string
	t.Run("inner", func(t *testing.T) {
		name = newDB(t).ConnInfo.DBName
	})

	observer := newDB(t)
	require.NotEmpty(t, name)
	assert.False(t, databaseExists(t, observer.DB, name), "database %s should be dropped when its test ends", name)
}

func TestCloseDropsTheDatabaseEvenWithOpenConnections(t *testing.T) {
	db := newDB(t)
	observer := newDB(t)
	name := db.ConnInfo.DBName

	// A connection the test forgot to close.
	leaked, err := sql.Open("postgres", db.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = leaked.Close() })
	require.NoError(t, leaked.Ping())

	require.NoError(t, db.Close())
	assert.False(t, databaseExists(t, observer.DB, name))

	require.NoError(t, db.Close(), "Close should be idempotent")
}

// URL is a plain postgres:// URL, so any driver can open it, for example a
// subprocess or code using pgx instead of lib/pq.
func TestURLOpensWithEitherDriver(t *testing.T) {
	db := newDB(t)

	for _, driver := range []string{"postgres", "pgx"} {
		t.Run(driver, func(t *testing.T) {
			conn, err := sqlx.Open(driver, db.URL)
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()

			var count int
			require.NoError(t, conn.Get(&count, "SELECT count(*) FROM authors WHERE name = $1", "nobody"))
			assert.Zero(t, count)
		})
	}
}

func TestDeprecatedAPI(t *testing.T) {
	t.Run("without migrations", func(t *testing.T) {
		testDB, err := postgres.NewTestDB(postgres.Options{})
		require.NoError(t, err)
		defer func() { _ = testDB.Close() }()

		var one int
		require.NoError(t, testDB.DB.Get(&one, "SELECT 1"))
		assert.Equal(t, 1, one)
		assert.False(t, tableExists(t, testDB.DB, "authors"))
	})

	t.Run("with migrations", func(t *testing.T) {
		testDB, err := postgres.NewTestDB(postgres.Options{MigrationsDir: migrationsDir})
		require.NoError(t, err)
		defer func() { _ = testDB.Close() }()

		for _, table := range migratedTables {
			assert.True(t, tableExists(t, testDB.DB, table), "table %s", table)
		}
		// Without MigrationsTable, NewTestDB keeps using sql-migrate's own
		// default, as it always did.
		assert.True(t, tableExists(t, testDB.DB, "gorp_migrations"))
	})

	t.Run("with migrations table", func(t *testing.T) {
		testDB, err := postgres.NewTestDB(postgres.Options{MigrationsDir: migrationsDir, MigrationsTable: "migrations"})
		require.NoError(t, err)
		defer func() { _ = testDB.Close() }()

		assert.True(t, tableExists(t, testDB.DB, "migrations"))
	})

	t.Run("missing migrations directory", func(t *testing.T) {
		_, err := postgres.NewTestDB(postgres.Options{MigrationsDir: "testdata/does-not-exist"})
		require.Error(t, err)
	})

	t.Run("connection info", func(t *testing.T) {
		testDB, err := postgres.NewTestDB(postgres.Options{})
		require.NoError(t, err)

		info := testDB.ConnInfo
		assert.NotEmpty(t, info.Host)
		assert.NotEmpty(t, info.Port)
		assert.Equal(t, "testuser", info.User)
		assert.Equal(t, "secret", info.Password)
		assert.NotEmpty(t, info.DBName)
		assert.Equal(t, "disable", info.SSLMode)
		assert.Equal(t, testDB.URL, info.ConnectionString())

		conn, err := sqlx.Open("postgres", info.ConnectionString())
		require.NoError(t, err)
		require.NoError(t, conn.Ping())
		require.NoError(t, conn.Close())

		observer := newDB(t)
		require.NoError(t, testDB.Close())
		assert.False(t, databaseExists(t, observer.DB, info.DBName), "Close should drop the database")
	})
}
