package pgstore

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Migrations holds the store's sql-migrate migrations, under "migrations/",
// named in the order they apply ("0001-http-sessions.sql"). A released file is
// never edited: a schema change is a new file.
//
//go:embed migrations/*.sql
var Migrations embed.FS

// timestampLayout is the ID prefix sql-migrate's "new" command gives a
// migration, and the one installed copies get.
const timestampLayout = "20060102150405"

// installedSuffix is what follows the timestamp in the name of an installed
// copy of the migration name: "-gogo-pgstore-0001-http-sessions.sql". An
// application's directory holds a copy when a file ends with it.
func installedSuffix(name string) string {
	return "-gogo-pgstore-" + name
}

var timestampPrefix = regexp.MustCompile(`^(\d{14})-`)

// InstallMigrations copies the store's migrations that dir lacks into dir and
// returns the names it wrote. Each copy is named like a migration made with
// sql-migrate new at install time, "<timestamp>-gogo-pgstore-<name>", so it
// sorts after the application's migrations: the timestamp is now (UTC), or one
// second after the newest timestamped migration in dir if that is later. A
// copy is recognized by its name without the timestamp, so installing again
// writes nothing. A copy with other content is an error and is left as is.
func InstallMigrations(dir string) ([]string, error) {
	return installMigrations(dir, time.Now())
}

func installMigrations(dir string, now time.Time) ([]string, error) {
	missing, err := checkMigrations(dir)
	if err != nil {
		return nil, err
	}

	if len(missing) == 0 {
		return nil, nil
	}

	next, err := nextTimestamp(dir, now)
	if err != nil {
		return nil, err
	}

	written := make([]string, 0, len(missing))

	for _, name := range missing {
		data, err := Migrations.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}

		target := next.Format(timestampLayout) + installedSuffix(name)
		if err := os.WriteFile(filepath.Join(dir, target), data, 0o644); err != nil {
			return nil, fmt.Errorf("failed to install migration %s: %w", name, err)
		}

		written = append(written, target)
		next = next.Add(time.Second)
	}

	return written, nil
}

// nextTimestamp returns the timestamp of the first copy: now, or one second
// after the newest timestamped migration in dir if that is later.
func nextTimestamp(dir string, now time.Time) (time.Time, error) {
	next := now.UTC().Truncate(time.Second)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, err
	}

	for _, e := range entries {
		m := timestampPrefix.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}

		ts, err := time.Parse(timestampLayout, m[1])
		if err != nil {
			continue // fourteen digits that aren't a date: not ours to order against
		}

		if !ts.Before(next) {
			next = ts.Add(time.Second)
		}
	}

	return next, nil
}

// CheckMigrations reports an error unless dir holds a copy of every migration
// of the store, unchanged. An application calls it from a test so a gogo
// upgrade that adds a migration fails CI until InstallMigrations has run.
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

// checkMigrations returns the store's migrations dir has no copy of, in order,
// and an error if a copy has other content or a migration has two copies.
func checkMigrations(dir string) ([]string, error) {
	entries, err := fs.ReadDir(Migrations, "migrations")
	if err != nil {
		return nil, err
	}

	existing, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var missing, changed []string

	for _, e := range entries {
		var copies []string

		for _, f := range existing {
			if strings.HasSuffix(f.Name(), installedSuffix(e.Name())) {
				copies = append(copies, f.Name())
			}
		}

		switch len(copies) {
		case 0:
			missing = append(missing, e.Name())

			continue
		case 1:
		default:
			return nil, fmt.Errorf("migration %s is installed more than once in %s: %v", e.Name(), dir, copies)
		}

		want, err := Migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}

		got, err := os.ReadFile(filepath.Join(dir, copies[0]))
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(got, want) {
			changed = append(changed, copies[0])
		}
	}

	if len(changed) > 0 {
		return nil, fmt.Errorf("migrations in %s differ from gogo's sessions/pgstore: %v", dir, changed)
	}

	return missing, nil
}
