package database_test

import (
	"database/sql"
	"errors"
	"testing"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubDialect struct {
	name string
}

func (d *stubDialect) Name() string {
	return d.name
}

func (d *stubDialect) Placeholder(index int) string {
	return "?"
}

func (d *stubDialect) SupportsReturning() bool {
	return false
}

func (d *stubDialect) Upsert(
	table string,
	keyColumns []string,
	valueColumns []string,
) (string, error) {
	return "upsert", nil
}

func (d *stubDialect) IsDuplicateError(err error) bool {
	return err != nil && err.Error() == "duplicate"
}

type stubDriver struct {
	name      string
	dialect   kerneldatabase.Dialect
	migration string
}

func (d *stubDriver) Name() string {
	return d.name
}

func (d *stubDriver) Open(_ string, _ kerneldatabase.OpenOptions) (*sql.DB, error) {
	return nil, nil
}

func (d *stubDriver) Dialect() kerneldatabase.Dialect {
	return d.dialect
}

func (d *stubDriver) MigrationDir() string {
	return d.migration
}

func newStubDriver(name string) *stubDriver {
	return &stubDriver{
		name:      name,
		dialect:   &stubDialect{name: name},
		migration: name,
	}
}

func TestDriverRegistryRegisterGet(t *testing.T) {
	registry := kerneldatabase.NewDriverRegistry()
	require.NoError(t, registry.Register(newStubDriver("sqlite")))

	driver, err := registry.Get("sqlite")
	require.NoError(t, err)
	assert.Equal(t, "sqlite", driver.Name())
	assert.Equal(t, "sqlite", driver.Dialect().Name())
	assert.Equal(t, "sqlite", driver.MigrationDir())
}

func TestDriverRegistryRegisterErrors(t *testing.T) {
	tests := []struct {
		name    string
		drivers []kerneldatabase.Driver
		wantErr bool
		errIs   error
	}{
		{
			name:    "success - distinct drivers",
			drivers: []kerneldatabase.Driver{newStubDriver("sqlite"), newStubDriver("postgres")},
			wantErr: false,
		},
		{
			name:    "error - duplicate driver",
			drivers: []kerneldatabase.Driver{newStubDriver("sqlite"), newStubDriver("sqlite")},
			wantErr: true,
			errIs:   kerneldatabase.ErrDuplicateDriver,
		},
		{
			name:    "error - nil driver",
			drivers: []kerneldatabase.Driver{nil},
			wantErr: true,
		},
		{
			name:    "error - empty name",
			drivers: []kerneldatabase.Driver{newStubDriver("")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := kerneldatabase.NewDriverRegistry()
			var err error
			for _, driver := range tt.drivers {
				err = registry.Register(driver)
				if err != nil {
					break
				}
			}
			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDriverRegistryUnknownListsInstalled(t *testing.T) {
	registry := kerneldatabase.NewDriverRegistry()
	require.NoError(t, registry.Register(newStubDriver("sqlite")))
	require.NoError(t, registry.Register(newStubDriver("postgres")))

	_, err := registry.Get("cockroach")
	require.Error(t, err)
	var unknown *kerneldatabase.UnknownDriverError
	require.True(t, errors.As(err, &unknown))
	assert.Equal(t, "cockroach", unknown.Name)
	assert.Equal(t, []string{"postgres", "sqlite"}, unknown.Installed)
	assert.Contains(t, err.Error(), `"cockroach"`)
	assert.Contains(t, err.Error(), "postgres")
	assert.Contains(t, err.Error(), "sqlite")
}

func TestDriverRegistryNames(t *testing.T) {
	registry := kerneldatabase.NewDriverRegistry()
	require.NoError(t, registry.Register(newStubDriver("sqlite")))
	require.NoError(t, registry.Register(newStubDriver("mysql")))
	require.NoError(t, registry.Register(newStubDriver("postgres")))

	assert.Equal(t, []string{"mysql", "postgres", "sqlite"}, registry.Names())
}
