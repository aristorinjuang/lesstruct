package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	iofs "github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
)

var (
	ErrDatabaseNotFound  = errors.New("database file not found")
	ErrUnsupportedDriver = errors.New("unsupported database driver")
)

// Database wraps the SQL database connection
type Database struct {
	db *sql.DB
}

// detectOldSchemaMigrations lives in sqlite.go: the pragma probe only runs
// against sqlite databases.

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
// for the specified driver ("sqlite", "postgres", or "mysql").
func (d *Database) RunMigrations(driver string) error {
	if driver == "sqlite" {
		return d.runSQLiteMigrations()
	}

	var srcFS fs.FS
	var dbName string
	var dbDriver migratedb.Driver
	var err error

	var srcPath string

	switch driver {
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

	return d.runWithInstance(srcFS, srcPath, dbName, dbDriver)
}

// runWithInstance runs the migration source against a database driver
// instance. The per-driver RunMigrations branches converge here so the
// migrate boilerplate lives in one place.
func (d *Database) runWithInstance(srcFS fs.FS, srcPath, dbName string, dbDriver migratedb.Driver) error {
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

// Open opens a database connection for the specified driver and DSN.
// For SQLite, the DSN is a file path (parent directories are created).
// For PostgreSQL, the DSN is a connection string.
// poolMaxConns is used only for PostgreSQL to set the connection pool size.
//
// Driver selection resolves through the kernel driver registry: an unknown
// driver fails with both the legacy ErrUnsupportedDriver (back-compat) and a
// kernel UnknownDriverError listing installed drivers.
func Open(driver, dsn string, poolMaxConns int) (*Database, error) {
	registry, err := Drivers()
	if err != nil {
		return nil, err
	}
	drv, err := registry.Get(driver)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupportedDriver, err)
	}
	db, err := drv.Open(dsn, kerneldatabase.OpenOptions{MaxOpenConns: poolMaxConns})
	if err != nil {
		return nil, err
	}
	return &Database{db: db}, nil
}
