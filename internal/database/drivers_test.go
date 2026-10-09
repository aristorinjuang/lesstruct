package database_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenSQLiteMemory(t *testing.T) {
	db, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err)
	defer closeDB(db)
	require.NoError(t, db.DB().Ping())
}

func TestOpenUnknownDriver(t *testing.T) {
	_, err := appdatabase.Open("cockroach", "whatever", 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, appdatabase.ErrUnsupportedDriver)

	var unknown *kerneldatabase.UnknownDriverError
	require.True(t, errors.As(err, &unknown))
	assert.Equal(t, "cockroach", unknown.Name)
	assert.Equal(t, []string{"mysql", "postgres", "sqlite"}, unknown.Installed)
}

func TestOpenUnreachableServer(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dsn    string
	}{
		{
			name:   "error - postgres connection refused",
			driver: "postgres",
			dsn:    "postgres://127.0.0.1:1/lesstruct?sslmode=disable",
		},
		{
			name:   "error - mysql connection refused",
			driver: "mysql",
			dsn:    "user:pass@tcp(127.0.0.1:1)/lesstruct",
		},
		{
			name:   "error - sqlite directory not creatable",
			driver: "sqlite",
			dsn:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.dsn
			if tt.driver == "sqlite" {
				blocker, err := os.CreateTemp(t.TempDir(), "blocker-*")
				require.NoError(t, err)
				require.NoError(t, blocker.Close())
				dsn = filepath.Join(blocker.Name(), "db.sqlite")
			}
			_, err := appdatabase.Open(tt.driver, dsn, 1)
			require.Error(t, err)
		})
	}
}

func TestDriversRegistry(t *testing.T) {
	registry, err := appdatabase.Drivers()
	require.NoError(t, err)
	assert.Equal(t, []string{"mysql", "postgres", "sqlite"}, registry.Names())
}

func TestDriverDialects(t *testing.T) {
	tests := []struct {
		name              string
		driver            string
		placeholder0      string
		placeholder1      string
		supportsReturning bool
		migrationDir      string
		duplicateErr      string
		otherErr          string
		wantErr           bool
	}{
		{
			name:              "success - sqlite",
			driver:            "sqlite",
			placeholder0:      "?",
			placeholder1:      "?",
			supportsReturning: true,
			migrationDir:      "sqlite",
			duplicateErr:      "UNIQUE constraint failed: media_files.hash",
			otherErr:          "no such table: media_files",
			wantErr:           false,
		},
		{
			name:              "success - postgres",
			driver:            "postgres",
			placeholder0:      "$1",
			placeholder1:      "$2",
			supportsReturning: true,
			migrationDir:      "postgresql",
			duplicateErr:      `duplicate key value violates unique constraint "media_files_hash_key"`,
			otherErr:          `relation "media_files" does not exist`,
			wantErr:           false,
		},
		{
			name:              "success - mysql",
			driver:            "mysql",
			placeholder0:      "?",
			placeholder1:      "?",
			supportsReturning: false,
			migrationDir:      "mysql",
			duplicateErr:      "Error 1062: Duplicate entry 'abc' for key 'hash'",
			otherErr:          "Error 1146: Table 'lesstruct.media_files' doesn't exist",
			wantErr:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := appdatabase.Drivers()
			require.NoError(t, err)
			driver, err := registry.Get(tt.driver)
			require.NoError(t, err)
			assert.Equal(t, tt.driver, driver.Name())
			assert.Equal(t, tt.placeholder0, driver.Dialect().Placeholder(0))
			assert.Equal(t, tt.placeholder1, driver.Dialect().Placeholder(1))
			assert.Equal(t, tt.supportsReturning, driver.Dialect().SupportsReturning())
			assert.Equal(t, tt.migrationDir, driver.MigrationDir())
			assert.True(t, driver.Dialect().IsDuplicateError(errors.New(tt.duplicateErr)))
			assert.False(t, driver.Dialect().IsDuplicateError(errors.New(tt.otherErr)))
			assert.False(t, driver.Dialect().IsDuplicateError(nil))
		})
	}
}

func TestDialectUpsert(t *testing.T) {
	tests := []struct {
		name     string
		driver   string
		table    string
		keys     []string
		values   []string
		expected string
		wantErr  bool
	}{
		{
			name:     "success - sqlite conflict update",
			driver:   "sqlite",
			table:    "users",
			keys:     []string{"username"},
			values:   []string{"password_hash", "role"},
			expected: "INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?) ON CONFLICT (username) DO UPDATE SET password_hash = excluded.password_hash, role = excluded.role",
			wantErr:  false,
		},
		{
			name:     "success - postgres conflict update",
			driver:   "postgres",
			table:    "users",
			keys:     []string{"username"},
			values:   []string{"password_hash", "role"},
			expected: "INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) ON CONFLICT (username) DO UPDATE SET password_hash = excluded.password_hash, role = excluded.role",
			wantErr:  false,
		},
		{
			name:     "success - mysql duplicate update",
			driver:   "mysql",
			table:    "users",
			keys:     []string{"username"},
			values:   []string{"password_hash", "role"},
			expected: "INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), role = VALUES(role)",
			wantErr:  false,
		},
		{
			name:     "success - sqlite conflict do nothing",
			driver:   "sqlite",
			table:    "users",
			keys:     []string{"username"},
			values:   nil,
			expected: "INSERT INTO users (username) VALUES (?) ON CONFLICT (username) DO NOTHING",
			wantErr:  false,
		},
		{
			name:     "success - mysql duplicate self assign",
			driver:   "mysql",
			table:    "users",
			keys:     []string{"username"},
			values:   nil,
			expected: "INSERT INTO users (username) VALUES (?) ON DUPLICATE KEY UPDATE username = VALUES(username)",
			wantErr:  false,
		},
		{
			name:    "error - sqlite empty keys",
			driver:  "sqlite",
			table:   "users",
			keys:    nil,
			wantErr: true,
		},
		{
			name:    "error - postgres empty keys",
			driver:  "postgres",
			table:   "users",
			keys:    []string{},
			wantErr: true,
		},
		{
			name:    "error - mysql empty keys",
			driver:  "mysql",
			table:   "users",
			keys:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := appdatabase.Drivers()
			require.NoError(t, err)
			driver, err := registry.Get(tt.driver)
			require.NoError(t, err)
			result, err := driver.Dialect().Upsert(tt.table, tt.keys, tt.values)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, kerneldatabase.ErrEmptyUpsertKeys)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
