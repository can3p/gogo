package pgstore_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/can3p/gogo/sessions/pgstore"
	"github.com/can3p/gogo/testcontainers/postgres"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/stretchr/testify/require"
)

// installAt is when the tests install, in sql-migrate's timestamp layout.
var installAt = time.Date(2027, 3, 4, 5, 6, 7, 0, time.UTC)

const installAtID = "20270304050607"

func migrationNames(t *testing.T) []string {
	t.Helper()

	entries, err := fs.ReadDir(pgstore.Migrations, "migrations")
	require.NoError(t, err)
	require.Len(t, entries, 1, "the tests below name the one migration there is; extend them with the next")

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	return names
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestInstallMigrationsNamesCopiesByInstallTime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "20200101000000-app-users.sql"), "-- app\n")
	writeFile(t, filepath.Join(dir, "README"), "not a migration\n")
	require.Error(t, pgstore.CheckMigrations(dir), "a directory without the copies fails the check")

	written, err := pgstore.InstallMigrationsAt(dir, installAt.In(time.FixedZone("UTC+3", 3*3600)))
	require.NoError(t, err)
	require.Equal(t, []string{installAtID + "-gogo-pgstore-0001-http-sessions.sql"}, written,
		"named by the install time in UTC, after the app's older migrations")

	got, err := os.ReadFile(filepath.Join(dir, written[0]))
	require.NoError(t, err)
	want, err := pgstore.Migrations.ReadFile("migrations/" + migrationNames(t)[0])
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))

	require.NoError(t, pgstore.CheckMigrations(dir))

	again, err := pgstore.InstallMigrationsAt(dir, installAt.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, again, "an installed copy is recognized whatever its timestamp")
}

func TestInstallMigrationsSortsAfterFutureDatedMigrations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "29990101000000-app-things.sql"), "-- app\n")

	written, err := pgstore.InstallMigrationsAt(dir, installAt)
	require.NoError(t, err)
	require.Equal(t, []string{"29990101000001-gogo-pgstore-0001-http-sessions.sql"}, written)
}

func TestInstallMigrationsRefusesAChangedCopy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "20200101000000-gogo-pgstore-0001-http-sessions.sql")
	writeFile(t, path, "-- edited\n")

	_, err := pgstore.InstallMigrationsAt(dir, installAt)
	require.ErrorContains(t, err, "differ")
	require.ErrorContains(t, pgstore.CheckMigrations(dir), "20200101000000-gogo-pgstore-0001-http-sessions.sql")

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "-- edited\n", string(got), "a changed copy is left as is")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "nothing else is written")
}

func TestInstallMigrationsRefusesTwoCopies(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want, err := pgstore.Migrations.ReadFile("migrations/" + migrationNames(t)[0])
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "20200101000000-gogo-pgstore-0001-http-sessions.sql"), string(want))
	writeFile(t, filepath.Join(dir, "20210101000000-gogo-pgstore-0001-http-sessions.sql"), string(want))

	require.ErrorContains(t, pgstore.CheckMigrations(dir), "more than once")
}

func TestCheckMigrationsNamesTheMissingOnes(t *testing.T) {
	t.Parallel()

	err := pgstore.CheckMigrations(t.TempDir())
	require.Error(t, err)

	for _, name := range migrationNames(t) {
		require.ErrorContains(t, err, name)
	}
}

// The installed copy runs as the newest migration of an application that has
// already run its own, as with the sql-migrate command.
func TestInstalledMigrationsRunAfterTheAppsMigrations(t *testing.T) {
	t.Parallel()

	db := postgres.New(t, postgres.WithMigrationsDir("testdata/app")).SQL

	dir := t.TempDir()
	app, err := os.ReadFile("testdata/app/29990101000000-app-things.sql")
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "29990101000000-app-things.sql"), string(app))

	_, err = pgstore.InstallMigrations(dir)
	require.NoError(t, err)

	set := migrate.MigrationSet{TableName: "migrations"}
	n, err := set.Exec(db, "postgres", &migrate.FileMigrationSource{Dir: dir}, migrate.Up)
	require.NoError(t, err)
	require.Equal(t, 1, n, "only the copy is new")

	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM http_sessions").Scan(&count))

	records, err := set.GetMigrationRecords(db, "postgres")
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "29990101000000-app-things.sql", records[0].Id)
	require.Equal(t, "29990101000001-gogo-pgstore-0001-http-sessions.sql", records[1].Id,
		"the copy is ordered after the app's newest migration")
}
