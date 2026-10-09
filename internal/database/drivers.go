package database

import (
	"database/sql"
	"fmt"
	"strings"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
)

const (
	// Driver names as stored in DB_DRIVER and returned by Driver.Name.
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
	DriverMySQL    = "mysql"

	// Migration subdirs as stored under internal/database/migrations and every
	// module's future migration FS. PostgreSQL differs from its driver name.
	MigrationSQLite   = "sqlite"
	MigrationPostgres = "postgresql"
	MigrationMySQL    = "mysql"
)

// openSQLite and sqliteDriver live in sqlite.go: the nosqlite build compiles
// them (and the modernc implementation) out for binary size.

func openServer(
	driverName string,
	dsn string,
	maxOpenConns int,
) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if maxOpenConns > 0 {
		db.SetMaxOpenConns(maxOpenConns)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}

// normalizeMySQLDSN ensures the MySQL DSN includes clientFoundRows=true so that
// RowsAffected() returns the number of rows matched by the WHERE clause
// rather than the number of rows whose values were actually changed.
func normalizeMySQLDSN(dsn string) string {
	if strings.Contains(dsn, "clientFoundRows=true") || strings.Contains(dsn, "clientFoundRows=1") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&clientFoundRows=true"
	}
	return dsn + "?clientFoundRows=true"
}

type postgresDriver struct{}

func (postgresDriver) Name() string {
	return DriverPostgres
}

func (postgresDriver) Open(
	dsn string,
	opts kerneldatabase.OpenOptions,
) (*sql.DB, error) {
	return openServer("pgx", dsn, opts.MaxOpenConns)
}

func (postgresDriver) Dialect() kerneldatabase.Dialect {
	return kerneldatabase.PostgresDialect()
}

func (postgresDriver) MigrationDir() string {
	return MigrationPostgres
}

type mysqlDriver struct{}

func (mysqlDriver) Name() string {
	return DriverMySQL
}

func (mysqlDriver) Open(
	dsn string,
	opts kerneldatabase.OpenOptions,
) (*sql.DB, error) {
	return openServer("mysql", normalizeMySQLDSN(dsn), opts.MaxOpenConns)
}

func (mysqlDriver) Dialect() kerneldatabase.Dialect {
	return kerneldatabase.MySQLDialect()
}

func (mysqlDriver) MigrationDir() string {
	return MigrationMySQL
}

// Drivers returns the registry of every compiled-in database driver.
// Open resolves through it on every call so driver selection stays in one
// place; modules use it to resolve Dialects without importing drivers.
func Drivers() (*kerneldatabase.DriverRegistry, error) {
	registry := kerneldatabase.NewDriverRegistry()
	drivers := []kerneldatabase.Driver{
		postgresDriver{},
		mysqlDriver{},
	}
	drivers = append(drivers, sqliteDrivers()...)
	for _, driver := range drivers {
		if err := registry.Register(driver); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
