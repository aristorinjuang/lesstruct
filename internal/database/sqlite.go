//go:build !nosqlite

package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/sqlite"

	_ "modernc.org/sqlite"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
)

var (
	//go:embed migrations/sqlite/*
	sqliteMigrationsFS embed.FS
)

// sqlitePragmas are the connection pragmas every SQLite connection opens
// with. busy_timeout makes concurrent writers wait for the database file
// instead of failing instantly with SQLITE_BUSY (the default is zero, which
// is what turned parallel checkouts on one cart into 500s); journal_mode=WAL
// lets readers proceed while a writer commits. They ride the DSN so every
// pooled connection gets them.
const sqlitePragmas = "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"

// sqliteDSN appends the connection pragmas to a SQLite DSN unless the
// operator already supplies pragmas of their own.
func sqliteDSN(dsn string) string {
	if strings.Contains(dsn, "_pragma=") {
		return dsn
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + sqlitePragmas
}

// sqliteFilePath reduces a DSN to its on-disk path so the parent directory
// can be created. ":memory:" and shared-cache memory DSNs have no file.
func sqliteFilePath(dsn string) (string, bool) {
	trimmed := dsn
	if after, ok := strings.CutPrefix(trimmed, "file:"); ok {
		trimmed = after
	}
	if before, _, ok := strings.Cut(trimmed, "?"); ok {
		trimmed = before
	}
	if trimmed == ":memory:" || strings.HasPrefix(trimmed, "mode=memory") {
		return "", false
	}
	return trimmed, true
}

func openSQLite(dsn string) (*sql.DB, error) {
	if path, ok := sqliteFilePath(dsn); ok {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", sqliteDSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}

type sqliteDriver struct{}

func (sqliteDriver) Name() string {
	return DriverSQLite
}

func (sqliteDriver) Open(
	dsn string,
	_ kerneldatabase.OpenOptions,
) (*sql.DB, error) {
	return openSQLite(dsn)
}

func (sqliteDriver) Dialect() kerneldatabase.Dialect {
	return kerneldatabase.SQLiteDialect()
}

func (sqliteDriver) MigrationDir() string {
	return MigrationSQLite
}

// sqliteDrivers returns the sqlite driver for the default build. The
// nosqlite build (server edition) compiles it out for binary size.
func sqliteDrivers() []kerneldatabase.Driver {
	return []kerneldatabase.Driver{sqliteDriver{}}
}

// detectOldSchemaMigrations checks for the old-style schema_migrations table
// (with applied_at column) and returns an error if found, directing the user
// to delete the database file.
func (d *Database) detectOldSchemaMigrations() error {
	var hasAppliedAt bool
	err := d.db.QueryRow(`
		SELECT COUNT(*) > 0 FROM pragma_table_info('schema_migrations')
		WHERE name = 'applied_at'
	`).Scan(&hasAppliedAt)
	if err == nil && hasAppliedAt {
		return fmt.Errorf("old schema_migrations table detected: delete the database file and restart")
	}
	return nil
}

// runSQLiteMigrations runs the sqlite migration sequence.
func (d *Database) runSQLiteMigrations() error {
	if err := d.detectOldSchemaMigrations(); err != nil {
		return err
	}
	dbDriver, err := sqlite.WithInstance(d.db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}
	return d.runWithInstance(sqliteMigrationsFS, "migrations/sqlite", "sqlite", dbDriver)
}

// sqliteModuleInstance builds the migrate driver instance for one module's
// version table on sqlite.
func sqliteModuleInstance(db *sql.DB, table string) (string, migratedb.Driver, error) {
	dbDriver, err := sqlite.WithInstance(db, &sqlite.Config{MigrationsTable: table})
	if err != nil {
		return "", nil, fmt.Errorf("failed to create database driver: %w", err)
	}
	return "sqlite", dbDriver, nil
}
