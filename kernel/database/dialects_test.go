package database_test

import (
	"errors"
	"testing"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/stretchr/testify/assert"
)

func TestKernelDialects(t *testing.T) {
	tests := []struct {
		name              string
		dialect           kerneldatabase.Dialect
		dialectName       string
		placeholder0      string
		placeholder1      string
		supportsReturning bool
		duplicateErr      string
		otherErr          string
	}{
		{
			name:              "sqlite",
			dialect:           kerneldatabase.SQLiteDialect(),
			dialectName:       "sqlite",
			placeholder0:      "?",
			placeholder1:      "?",
			supportsReturning: true,
			duplicateErr:      "UNIQUE constraint failed: media_files.hash",
			otherErr:          "no such table",
		},
		{
			name:              "postgres",
			dialect:           kerneldatabase.PostgresDialect(),
			dialectName:       "postgres",
			placeholder0:      "$1",
			placeholder1:      "$2",
			supportsReturning: true,
			duplicateErr:      `duplicate key value violates unique constraint "x"`,
			otherErr:          "relation does not exist",
		},
		{
			name:              "mysql",
			dialect:           kerneldatabase.MySQLDialect(),
			dialectName:       "mysql",
			placeholder0:      "?",
			placeholder1:      "?",
			supportsReturning: false,
			duplicateErr:      "Error 1062: Duplicate entry 'x' for key 'y'",
			otherErr:          "Error 1146: Table doesn't exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.dialectName, tt.dialect.Name())
			assert.Equal(t, tt.placeholder0, tt.dialect.Placeholder(0))
			assert.Equal(t, tt.placeholder1, tt.dialect.Placeholder(1))
			assert.Equal(t, tt.supportsReturning, tt.dialect.SupportsReturning())
			assert.True(t, tt.dialect.IsDuplicateError(errors.New(tt.duplicateErr)))
			assert.False(t, tt.dialect.IsDuplicateError(errors.New(tt.otherErr)))
			assert.False(t, tt.dialect.IsDuplicateError(nil))
		})
	}
}

func TestKernelDialectUpsert(t *testing.T) {
	sqlite := kerneldatabase.SQLiteDialect()
	result, err := sqlite.Upsert("users", []string{"username"}, []string{"role"})
	assert.NoError(t, err)
	assert.Equal(t,
		"INSERT INTO users (username, role) VALUES (?, ?) ON CONFLICT (username) DO UPDATE SET role = excluded.role",
		result,
	)

	postgres := kerneldatabase.PostgresDialect()
	result, err = postgres.Upsert("users", []string{"username"}, []string{"role"})
	assert.NoError(t, err)
	assert.Equal(t,
		"INSERT INTO users (username, role) VALUES ($1, $2) ON CONFLICT (username) DO UPDATE SET role = excluded.role",
		result,
	)

	mysql := kerneldatabase.MySQLDialect()
	result, err = mysql.Upsert("users", []string{"username"}, nil)
	assert.NoError(t, err)
	assert.Equal(t,
		"INSERT INTO users (username) VALUES (?) ON DUPLICATE KEY UPDATE username = VALUES(username)",
		result,
	)

	_, err = sqlite.Upsert("users", nil, nil)
	assert.ErrorIs(t, err, kerneldatabase.ErrEmptyUpsertKeys)
}
