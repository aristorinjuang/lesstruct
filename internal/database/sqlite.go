//go:build !nosqlite

package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/sqlite"

	_ "modernc.org/sqlite"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
)

var (
	//go:embed migrations/sqlite/*
	sqliteMigrationsFS embed.FS
)

func openSQLite(dsn string) (*sql.DB, error) {
	dir := filepath.Dir(dsn)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", dsn)
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
