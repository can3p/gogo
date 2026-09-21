// Package postgres gives an integration test a real, migrated, empty
// PostgreSQL database of its own.
//
// # Design
//
// One Postgres container is started per test binary (per image) with
// testcontainers-go and shared by every test in it. Migrations are applied
// once, into a template database; each test then gets its own database made
// with CREATE DATABASE ... TEMPLATE .... Copying a template is a file copy
// inside Postgres rather than a replay of the migration history, so the cost
// of setting up a test stays constant as the number of migrations grows.
//
// Every database gets a unique name, so tests may call t.Parallel freely.
// The database is dropped WITH (FORCE) when the test ends, even if the test
// leaked connections.
//
// # Usage
//
//	func TestMain(m *testing.M) {
//		code := m.Run()
//		_ = postgres.Cleanup() // optional: stop the container right away
//		os.Exit(code)
//	}
//
//	func TestSomething(t *testing.T) {
//		t.Parallel()
//
//		db := postgres.New(t, postgres.WithMigrationsDir("../../migrations"))
//
//		// db.DB is a *sqlx.DB and db.SQL a *sql.DB on an empty, fully
//		// migrated database. db.URL is its connection string, for handing
//		// to a subprocess or to a different driver.
//		var n int
//		if err := db.DB.Get(&n, "SELECT count(*) FROM users"); err != nil {
//			t.Fatal(err)
//		}
//	}
//
// The migrations directory is required. A relative path is resolved against
// the working directory of the test binary, which `go test` sets to the
// directory of the package under test.
//
// # Migration bookkeeping table
//
// sql-migrate records applied migrations in a table. Its Go API defaults to
// "gorp_migrations", while its CLI reads the table name from dbconfig.yml. If
// the two disagree, the tests and the CLI keep separate bookkeeping and
// disagree about what has been applied. New therefore defaults to
// "migrations", the name most dbconfig.yml files use; set
// WithMigrationsTable to whatever the `table:` key of your dbconfig.yml says.
// The table is configured per migration run, so sql-migrate's global
// migrate.SetTable setting is neither needed nor modified.
//
// # Driver
//
// The package registers and uses github.com/lib/pq (driver name "postgres").
// URL is a standard postgres:// URL, so code that prefers pgx can open it
// with sql.Open("pgx", db.URL) after importing github.com/jackc/pgx/v5/stdlib.
package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq" // database/sql driver, registered as "postgres"
	migrate "github.com/rubenv/sql-migrate"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	// DefaultImage is the Postgres image used unless WithImage says otherwise.
	DefaultImage = "postgres:16-alpine"

	// DefaultMigrationsTable is the sql-migrate bookkeeping table New uses
	// unless WithMigrationsTable says otherwise.
	DefaultMigrationsTable = "migrations"

	driverName = "postgres"

	// Kept identical to the credentials of earlier versions of this package,
	// which exposed them through ConnectionInfo.
	user     = "testuser"
	password = "secret"

	maintenanceDB = "postgres"
)

var (
	mu        sync.Mutex
	instances = map[string]*instance{}

	dbCounter       atomic.Uint64
	templateCounter atomic.Uint64
)

// Option configures New.
type Option func(*config)

type config struct {
	image           string
	migrationsDir   string
	migrationsTable string
	// legacy selects the behaviour of the deprecated NewTestDB: an empty
	// migrationsDir means "no migrations", and an empty migrationsTable means
	// sql-migrate's global default.
	legacy bool
}

// WithMigrationsDir sets the directory of sql-migrate migrations to apply.
// It is required: a library cannot guess where its consumer keeps
// migrations. A relative path is resolved against the working directory.
func WithMigrationsDir(dir string) Option {
	return func(c *config) { c.migrationsDir = dir }
}

// WithMigrationsTable sets the table sql-migrate records applied migrations
// in. It defaults to DefaultMigrationsTable ("migrations") and should match
// the `table:` key of your dbconfig.yml.
func WithMigrationsTable(table string) Option {
	return func(c *config) { c.migrationsTable = table }
}

// WithImage sets the Postgres image to run, DefaultImage by default. Tests
// asking for different images get different containers. The image must be
// Postgres 13 or newer, which DROP DATABASE ... WITH (FORCE) requires.
func WithImage(image string) Option {
	return func(c *config) { c.image = image }
}

// TestDB is one test's own database.
type TestDB struct {
	// DB is connected to the test's database.
	DB *sqlx.DB

	// SQL is the *sql.DB underlying DB, for code that does not use sqlx.
	SQL *sql.DB

	// URL is the connection string of the database, for handing it to a
	// subprocess or opening it with a different driver.
	URL string

	// ConnInfo holds the same connection details as URL, split into fields.
	//
	// Deprecated: use URL.
	ConnInfo ConnectionInfo

	name  string
	admin *sql.DB
}

// New returns an empty database with every migration applied, for the
// duration of the test, and registers t.Cleanup to drop it. It fails the
// test rather than returning an error: a test cannot carry on without its
// database.
func New(t testing.TB, opts ...Option) *TestDB {
	t.Helper()

	cfg := config{
		image:           DefaultImage,
		migrationsTable: DefaultMigrationsTable,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	tdb, err := newTestDB(cfg)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}

	t.Cleanup(func() {
		if err := tdb.Close(); err != nil {
			t.Errorf("postgres: %v", err)
		}
	})

	return tdb
}

// Close disconnects from the database and drops it. New registers it with
// t.Cleanup, so a test does not normally call it. Calling it again is a no-op.
func (t *TestDB) Close() error {
	if t.DB != nil {
		if err := t.DB.Close(); err != nil {
			return fmt.Errorf("closing connection to %s: %w", t.name, err)
		}
		t.DB = nil
		t.SQL = nil
	}

	if t.admin == nil || t.name == "" {
		return nil
	}

	if _, err := t.admin.Exec("DROP DATABASE IF EXISTS " + pq.QuoteIdentifier(t.name) + " WITH (FORCE)"); err != nil {
		return fmt.Errorf("dropping database %s: %w", t.name, err)
	}
	t.name = ""

	return nil
}

// Cleanup terminates the shared containers. Call it from TestMain to remove
// them as soon as the tests finish; otherwise testcontainers' reaper removes
// them after the test binary exits. A later New starts a fresh container.
func Cleanup() error {
	mu.Lock()
	defer mu.Unlock()

	var errs []error
	for image, inst := range instances {
		if err := inst.terminate(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", image, err))
		}
		delete(instances, image)
	}

	return errors.Join(errs...)
}

type templateKey struct {
	dir   string
	table string
}

type templateResult struct {
	name string
	err  error
}

type instance struct {
	container *tcpostgres.PostgresContainer
	host      string
	port      string

	// admin is connected to the maintenance database and is used to create
	// and drop the per-test databases.
	admin *sql.DB

	mu        sync.Mutex
	templates map[templateKey]templateResult
}

func (i *instance) url(dbName string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(i.host, i.port),
		Path:     "/" + dbName,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func (i *instance) terminate() error {
	var errs []error
	if err := i.admin.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing the admin connection: %w", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := i.container.Terminate(ctx); err != nil {
		errs = append(errs, fmt.Errorf("terminating the container: %w", err))
	}

	return errors.Join(errs...)
}

func newTestDB(cfg config) (*TestDB, error) {
	var key templateKey
	withMigrations := !cfg.legacy || cfg.migrationsDir != ""

	if withMigrations {
		dir, err := checkMigrationsDir(cfg.migrationsDir)
		if err != nil {
			return nil, err
		}
		key = templateKey{dir: dir, table: cfg.migrationsTable}
	}

	inst, err := getOrStart(cfg.image)
	if err != nil {
		return nil, err
	}

	stmt := "CREATE DATABASE %s"
	if withMigrations {
		template, err := inst.template(key)
		if err != nil {
			return nil, err
		}
		// TEMPLATE copies the already migrated schema instead of replaying
		// every migration.
		stmt += " TEMPLATE " + pq.QuoteIdentifier(template)
	}

	suffix, err := randomSuffix()
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("test_%s_%d", suffix, dbCounter.Add(1))

	if _, err := inst.admin.Exec(fmt.Sprintf(stmt, pq.QuoteIdentifier(name))); err != nil {
		return nil, fmt.Errorf("creating database %s: %w", name, err)
	}

	dbURL := inst.url(name)
	db, err := sqlx.Open(driverName, dbURL)
	if err != nil {
		_, _ = inst.admin.Exec("DROP DATABASE IF EXISTS " + pq.QuoteIdentifier(name))
		return nil, fmt.Errorf("connecting to database %s: %w", name, err)
	}

	return &TestDB{
		DB:  db,
		SQL: db.DB,
		URL: dbURL,
		ConnInfo: ConnectionInfo{
			Host:     inst.host,
			Port:     inst.port,
			User:     user,
			Password: password,
			DBName:   name,
			SSLMode:  "disable",
		},
		name:  name,
		admin: inst.admin,
	}, nil
}

// checkMigrationsDir returns the absolute path of dir, or an error when dir is
// unset, missing or not a directory. A missing directory is not an empty
// migration set: sql-migrate would apply nothing, and every test would run
// against a schema that is quietly wrong.
func checkMigrationsDir(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("the migrations directory is required, pass WithMigrationsDir")
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving migrations directory %s: %w", dir, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("migrations directory %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("migrations directory %s is not a directory", abs)
	}

	return abs, nil
}

func getOrStart(image string) (*instance, error) {
	mu.Lock()
	defer mu.Unlock()

	if inst, ok := instances[image]; ok {
		return inst, nil
	}

	inst, err := start(image)
	if err != nil {
		return nil, err
	}
	instances[image] = inst

	return inst, nil
}

func start(image string) (*instance, error) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase(maintenanceDB),
		tcpostgres.WithUsername(user),
		tcpostgres.WithPassword(password),
		// Postgres accepts connections during initdb and then restarts. This
		// waits for the readiness line twice and then for the port to be
		// served on the host, which keeps startup from being flaky where
		// Docker runs behind a proxy, as on macOS.
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		if container != nil {
			_ = container.Terminate(ctx)
		}
		return nil, fmt.Errorf("starting the %s container: %w", image, err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("resolving the container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("resolving the mapped port: %w", err)
	}

	inst := &instance{
		container: container,
		host:      host,
		port:      port.Port(),
		templates: map[templateKey]templateResult{},
	}

	inst.admin, err = sql.Open(driverName, inst.url(maintenanceDB))
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("connecting to the maintenance database: %w", err)
	}

	log.Printf("postgres: started %s at %s", image, net.JoinHostPort(inst.host, inst.port))

	return inst, nil
}

// template returns the name of the template database holding the migrations
// in key.dir, building it on first use. The outcome, error included, is
// remembered so every test in the binary sees the same result.
func (i *instance) template(key templateKey) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if res, ok := i.templates[key]; ok {
		return res.name, res.err
	}

	name := fmt.Sprintf("template_%d", templateCounter.Add(1))
	err := i.buildTemplate(name, key)
	if err != nil {
		_, _ = i.admin.Exec("DROP DATABASE IF EXISTS " + pq.QuoteIdentifier(name) + " WITH (FORCE)")
	}
	i.templates[key] = templateResult{name: name, err: err}

	return name, err
}

func (i *instance) buildTemplate(name string, key templateKey) error {
	if _, err := i.admin.Exec("CREATE DATABASE " + pq.QuoteIdentifier(name)); err != nil {
		return fmt.Errorf("creating template database: %w", err)
	}

	db, err := sql.Open(driverName, i.url(name))
	if err != nil {
		return fmt.Errorf("connecting to template database: %w", err)
	}

	source := &migrate.FileMigrationSource{Dir: key.dir}
	var n int
	if key.table == "" {
		// Deprecated NewTestDB without a table: keep its historical behaviour
		// of honouring whatever migrate.SetTable the caller configured.
		n, err = migrate.Exec(db, "postgres", source, migrate.Up)
	} else {
		n, err = migrate.MigrationSet{TableName: key.table}.Exec(db, "postgres", source, migrate.Up)
	}
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("applying migrations from %s: %w", key.dir, err)
	}

	// Postgres refuses to copy a template that has an open connection, and
	// every test copies this one.
	if err := db.Close(); err != nil {
		return fmt.Errorf("closing template database: %w", err)
	}

	log.Printf("postgres: applied %d migrations from %s to template %s", n, key.dir, name)

	return nil
}

func randomSuffix() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating a database name: %w", err)
	}

	return hex.EncodeToString(b), nil
}
