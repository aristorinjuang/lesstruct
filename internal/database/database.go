package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	iofs "github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

var (
	ErrDatabaseNotFound = errors.New("database file not found")
	ErrUnsupportedDriver = errors.New("unsupported database driver")
)

// Database wraps the SQL database connection
type Database struct {
	db *sql.DB
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// DB returns the underlying SQL database connection
func (d *Database) DB() *sql.DB {
	return d.db
}

// RunMigrations runs all pending database migrations from the embedded filesystem
// for the specified driver ("sqlite" or "postgres").
func (d *Database) RunMigrations(driver string) error {
	if driver == "sqlite" {
		if err := d.detectOldSchemaMigrations(); err != nil {
			return err
		}
	}

	var srcFS fs.FS
	var dbName string
	var dbDriver migratedb.Driver
	var err error

	var srcPath string

	switch driver {
	case "sqlite":
		srcFS = sqliteMigrationsFS
		srcPath = "migrations/sqlite"
		dbName = "sqlite"
		dbDriver, err = sqlite.WithInstance(d.db, &sqlite.Config{})
	case "postgres":
		srcFS = postgresqlMigrationsFS
		srcPath = "migrations/postgresql"
		dbName = "postgres"
		dbDriver, err = postgres.WithInstance(d.db, &postgres.Config{})
	case "mysql":
		srcFS = mysqlMigrationsFS
		srcPath = "migrations/mysql"
		dbName = "mysql"
		dbDriver, err = mysql.WithInstance(d.db, &mysql.Config{})
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedDriver, driver)
	}
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	srcDriver, err := iofs.New(srcFS, srcPath)
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		srcDriver,
		dbName,
		dbDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
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

// Open opens a database connection for the specified driver and DSN.
// For SQLite, the DSN is a file path (parent directories are created).
// For PostgreSQL, the DSN is a connection string.
// poolMaxConns is used only for PostgreSQL to set the connection pool size.
func Open(driver, dsn string, poolMaxConns int) (*Database, error) {
	var driverName string

	switch driver {
	case "sqlite":
		// Ensure parent directory exists for SQLite
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
		driverName = "sqlite"
	case "postgres":
		driverName = "pgx"
	case "mysql":
		driverName = "mysql"
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, driver)
	}

	// Open database connection
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool via database/sql (not DSN params)
	if (driver == "postgres" || driver == "mysql") && poolMaxConns > 0 {
		db.SetMaxOpenConns(poolMaxConns)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{db: db}, nil
}
