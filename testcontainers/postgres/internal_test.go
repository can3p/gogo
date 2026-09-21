package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A missing migrations directory must be an error, never an empty migration
// set that leaves every test running against a schema that is quietly wrong.
func TestMigrationsDirectoryIsRequired(t *testing.T) {
	for name, dir := range map[string]string{
		"unset":         "",
		"missing":       "testdata/does-not-exist",
		"not directory": "testdata/migrations/0001-authors.sql",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newTestDB(config{
				image:           DefaultImage,
				migrationsDir:   dir,
				migrationsTable: DefaultMigrationsTable,
			})
			require.Error(t, err)
		})
	}
}
