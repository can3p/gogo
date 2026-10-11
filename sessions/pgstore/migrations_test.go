package pgstore_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/can3p/gogo/sessions/pgstore"
	"github.com/can3p/gogo/testcontainers/postgres"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/stretchr/testify/require"
)

func migrationNames(t *testing.T) []string {
	t.Helper()

	entries, err := fs.ReadDir(pgstore.Migrations, "migrations")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	return names
}

func TestInstallMigrationsCopiesThemOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.Error(t, pgstore.CheckMigrations(dir), "an empty directory lacks the migrations")

	written, err := pgstore.InstallMigrations(dir)
	require.NoError(t, err)
	require.Equal(t, migrationNames(t), written)

	for _, name := range written {
		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		want, err := pgstore.Migrations.ReadFile("migrations/" + name)
		require.NoError(t, err)
		require.Equal(t, string(want), string(got))
	}

	require.NoError(t, pgstore.CheckMigrations(dir))

	again, err := pgstore.InstallMigrations(dir)
	require.NoError(t, err)
	require.Empty(t, again, "installed migrations are not written again")
}

func TestInstallMigrationsRefusesAChangedCopy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := migrationNames(t)[0]
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("-- edited\n"), 0o644))

	_, err := pgstore.InstallMigrations(dir)
	require.ErrorContains(t, err, name)
	require.ErrorContains(t, pgstore.CheckMigrations(dir), "differ")

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "-- edited\n", string(got), "a changed copy is left as is")
}

func TestCheckMigrationsNamesTheMissingOnes(t *testing.T) {
	t.Parallel()

	err := pgstore.CheckMigrations(t.TempDir())
	require.Error(t, err)

	for _, name := range migrationNames(t) {
		require.ErrorContains(t, err, name)
	}
}

// An application that already ran migrations newer than gogo's dated ones
// still gets them: sql-migrate applies unapplied older IDs ("catchup").
func TestInstalledMigrationsRunAfterNewerAppMigrations(t *testing.T) {
	t.Parallel()

	db := postgres.New(t, postgres.WithMigrationsDir("testdata/app")).SQL

	dir := t.TempDir()
	app, err := os.ReadFile("testdata/app/29990101000000-app-things.sql")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "29990101000000-app-things.sql"), app, 0o644))
	_, err = pgstore.InstallMigrations(dir)
	require.NoError(t, err)

	n, err := migrate.MigrationSet{TableName: "migrations"}.Exec(db, "postgres", &migrate.FileMigrationSource{Dir: dir}, migrate.Up)
	require.NoError(t, err)
	require.Equal(t, len(migrationNames(t)), n, "only gogo's migrations are new")

	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM http_sessions").Scan(&count))
}
