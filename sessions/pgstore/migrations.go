package pgstore

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Migrations holds the store's sql-migrate migrations, under "migrations/".
// Each file keeps its name in the application's migrations directory: the
// name is its sql-migrate ID, so copying it again is a no-op, and a later gogo
// version adds new files rather than changing old ones. sql-migrate applies a
// file whose ID sorts before migrations already run ("catchup"), so the dated
// names don't need to be newer than the application's own.
//
//go:embed migrations/*.sql
var Migrations embed.FS

// InstallMigrations copies the store's migrations that dir lacks into dir and
// returns the names it wrote. A file of the same name with other content is an
// error and is left as is.
func InstallMigrations(dir string) ([]string, error) {
	missing, err := checkMigrations(dir)
	if err != nil {
		return nil, err
	}

	for _, name := range missing {
		data, err := Migrations.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return nil, fmt.Errorf("failed to install migration %s: %w", name, err)
		}
	}

	return missing, nil
}

// CheckMigrations reports an error unless dir holds every migration of the
// store, unchanged. An application calls it from a test so a gogo upgrade that
// adds a migration fails CI until InstallMigrations has run.
func CheckMigrations(dir string) error {
	missing, err := checkMigrations(dir)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf("migrations missing from %s: %v; install them with InstallMigrations or go run github.com/can3p/gogo/sessions/pgstore/cmd/pgstore-migrations -dir %s", dir, missing, dir)
	}

	return nil
}

// checkMigrations returns the store's migrations dir lacks, in order, and an
// error if dir holds one of them with other content.
func checkMigrations(dir string) ([]string, error) {
	entries, err := fs.ReadDir(Migrations, "migrations")
	if err != nil {
		return nil, err
	}

	var missing, changed []string

	for _, e := range entries {
		want, err := Migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}

		got, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, e.Name())

			continue
		}

		if err != nil {
			return nil, err
		}

		if !bytes.Equal(got, want) {
			changed = append(changed, e.Name())
		}
	}

	if len(changed) > 0 {
		return nil, fmt.Errorf("migrations in %s differ from gogo's sessions/pgstore: %v", dir, changed)
	}

	slices.Sort(missing)

	return missing, nil
}
