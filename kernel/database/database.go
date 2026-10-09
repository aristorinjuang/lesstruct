package database

import (
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

// OpenOptions tunes Driver.Open. MaxOpenConns configures the pool for server
// databases; embedded drivers ignore it.
type OpenOptions struct {
	MaxOpenConns int
}

// Dialect abstracts the SQL differences between drivers so module
// repositories stay single-source: one implementation against *sql.DB plus
// this shim, with per-dialect escapes only where SQL truly diverges.
//
// Table and column names are trusted identifiers (schema-defined, never user
// input): Upsert interpolates them verbatim.
type Dialect interface {
	// Name is the migration subdir and registry key ("sqlite", ...).
	Name() string
	// Placeholder renders the bind placeholder for the 0-based parameter
	// index ("?" on SQLite/MySQL, "$N" with N=index+1 on PostgreSQL).
	Placeholder(index int) string
	// SupportsReturning reports whether INSERT/UPDATE ... RETURNING works.
	SupportsReturning() bool
	// Upsert renders an insert-or-update statement for the given table,
	// conflict key columns, and value columns. Empty keyColumns fails with
	// ErrEmptyUpsertKeys — every dialect needs a conflict target.
	Upsert(table string, keyColumns, valueColumns []string) (string, error)
	// IsDuplicateError reports whether err is a unique-constraint violation
	// (duplicate key entry) on this driver. Stores map it to domain
	// duplicate sentinels instead of generic persistence errors.
	IsDuplicateError(err error) bool
}

// Driver opens a database and describes its SQL dialect. One implementation
// exists per supported database (sqlite, postgresql, mysql).
type Driver interface {
	// Name matches DB_DRIVER values ("sqlite", "postgres", "mysql").
	Name() string
	Open(dsn string, opts OpenOptions) (*sql.DB, error)
	Dialect() Dialect
	// MigrationDir is the subdir inside every module's migration FS
	// holding this driver's files.
	MigrationDir() string
}

var (
	ErrDuplicateDriver = errors.New("kernel: duplicate database driver")
	ErrEmptyUpsertKeys = errors.New("kernel: upsert requires at least one key column")
)

// UnknownDriverError is returned when DB_DRIVER names a driver that was not
// compiled in. It lists installed drivers so the fix is obvious ("did you
// forget to import the driver module?").
type UnknownDriverError struct {
	Name      string
	Installed []string
}

func (e *UnknownDriverError) Error() string {
	return fmt.Sprintf(
		"kernel: unknown database driver %q (installed: %s)",
		e.Name,
		strings.Join(e.Installed, ", "),
	)
}

// DriverRegistry maps driver names to implementations. DB_DRIVER selects an
// entry; an unknown name fails with UnknownDriverError.
type DriverRegistry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

// Register adds a driver under its Name. Duplicates are rejected.
func (r *DriverRegistry) Register(driver Driver) error {
	if driver == nil {
		return errors.New("kernel: database driver cannot be nil")
	}
	name := driver.Name()
	if name == "" {
		return errors.New("kernel: database driver name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.drivers[name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateDriver, name)
	}
	r.drivers[name] = driver
	return nil
}

// Get returns the driver for name, or UnknownDriverError listing installed
// drivers when the name was not compiled in.
func (r *DriverRegistry) Get(name string) (Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if driver, ok := r.drivers[name]; ok {
		return driver, nil
	}
	return nil, &UnknownDriverError{
		Name:      name,
		Installed: r.sortedNamesLocked(),
	}
}

// Names returns every registered driver name in sorted order.
func (r *DriverRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sortedNamesLocked()
}

func (r *DriverRegistry) sortedNamesLocked() []string {
	return slices.Sorted(maps.Keys(r.drivers))
}

// NewDriverRegistry creates an empty driver registry.
func NewDriverRegistry() *DriverRegistry {
	return &DriverRegistry{
		drivers: make(map[string]Driver),
	}
}
