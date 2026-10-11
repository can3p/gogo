// Command pgstore-migrations copies the migrations of gogo's sessions/pgstore
// into an application's sql-migrate migrations directory:
//
//	go run github.com/can3p/gogo/sessions/pgstore/cmd/pgstore-migrations -dir migrations
//
// With -check it changes nothing and exits non-zero when a migration is
// missing or differs.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/can3p/gogo/sessions/pgstore"
)

func main() {
	dir := flag.String("dir", "migrations", "the application's sql-migrate migrations directory")
	check := flag.Bool("check", false, "only report missing or changed migrations")
	flag.Parse()

	if err := run(*dir, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dir string, check bool) error {
	if check {
		return pgstore.CheckMigrations(dir)
	}

	written, err := pgstore.InstallMigrations(dir)
	if err != nil {
		return err
	}

	for _, name := range written {
		fmt.Println("installed", name)
	}

	return nil
}
