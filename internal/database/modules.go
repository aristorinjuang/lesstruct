package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	iofs "github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/aristorinjuang/lesstruct/kernel"
)

// moduleMigrationsTable renders the version-tracking table for a module.
// Names are slug-safe (lowercase, hyphens), so replacing hyphens is the only
// normalization needed.
func moduleMigrationsTable(name string) string {
	return "schema_migrations_" + strings.ReplaceAll(name, "-", "_")
}

// moduleMigrationInstance builds the migrate driver instance for one
// module's version table. The sqlite branch is tag-gated (server edition
// compiles it out); postgres and mysql are always present.
func moduleMigrationInstance(db *sql.DB, driver, table string) (string, migratedb.Driver, error) {
	switch driver {
	case "sqlite":
		return sqliteModuleInstance(db, table)
	case "postgres":
		dbDriver, err := postgres.WithInstance(db, &postgres.Config{MigrationsTable: table})
		if err != nil {
			return "", nil, fmt.Errorf("failed to create database driver: %w", err)
		}
		return "postgres", dbDriver, nil
	case "mysql":
		dbDriver, err := mysql.WithInstance(db, &mysql.Config{MigrationsTable: table})
		if err != nil {
			return "", nil, fmt.Errorf("failed to create database driver: %w", err)
		}
		return "mysql", dbDriver, nil
	default:
		return "", nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, driver)
	}
}

func migrateModule(db *sql.DB, driver, dir string, module kernel.Migrator) error {
	migrations := module.Migrations()
	if migrations == nil {
		return nil
	}
	if _, err := fs.Stat(migrations, dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to inspect migration source: %w", err)
	}

	table := moduleMigrationsTable(module.Name())
	dbName, dbDriver, err := moduleMigrationInstance(db, driver, table)
	if err != nil {
		return err
	}

	srcDriver, err := iofs.New(migrations, dir)
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

// MigrateModules runs pending migrations for every Migrator in reg, after
// the core migrations. Each module versions independently in its own
// schema_migrations_<module> table, so modules install, upgrade, and disable
// without touching the core sequence or each other. A module with no
// migration subdir for the active driver is skipped.
//
// Module migrations must be idempotent for upgrades (CREATE TABLE IF NOT
// EXISTS): pre-module installs already carry the tables from the core
// sequence they were extracted from.
func MigrateModules(db *sql.DB, driver string, reg *kernel.Registry) error {
	if reg == nil {
		return nil
	}
	drivers, err := Drivers()
	if err != nil {
		return err
	}
	drv, err := drivers.Get(driver)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupportedDriver, err)
	}
	for _, module := range kernel.Capabilities[kernel.Migrator](reg) {
		if err := migrateModule(db, driver, drv.MigrationDir(), module); err != nil {
			return fmt.Errorf("module %q migrations: %w", module.Name(), err)
		}
	}
	return nil
}
