//go:build nosqlite

package database

import (
	"database/sql"
	"fmt"

	migratedb "github.com/golang-migrate/migrate/v4/database"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
)

// sqliteDrivers is empty in the nosqlite build (server edition): the sqlite
// driver, its migrations, and the modernc implementation are compiled out
// for binary size. Deployments needing sqlite use the default build.
func sqliteDrivers() []kerneldatabase.Driver {
	return nil
}

// runSQLiteMigrations always fails in the nosqlite build: without the driver
// there is nothing to migrate. Open rejects sqlite DSNs first, so this is
// reachable only through direct RunMigrations calls.
func (d *Database) runSQLiteMigrations() error {
	return fmt.Errorf("%w: sqlite is not compiled into this build (postgres/mysql only)", ErrUnsupportedDriver)
}

// sqliteModuleInstance always fails in the nosqlite build.
func sqliteModuleInstance(_ *sql.DB, _ string) (string, migratedb.Driver, error) {
	return "", nil, fmt.Errorf("%w: sqlite is not compiled into this build (postgres/mysql only)", ErrUnsupportedDriver)
}
