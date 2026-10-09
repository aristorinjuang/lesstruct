package database

import (
	"slices"
	"strconv"
	"strings"
)

// sqliteDialect is the kernel-provided SQLite dialect: "?" placeholders,
// RETURNING support, and SQLite unique-violation detection.
type sqliteDialect struct{}

func (sqliteDialect) Name() string {
	return "sqlite"
}

func (sqliteDialect) Placeholder(_ int) string {
	return "?"
}

func (sqliteDialect) SupportsReturning() bool {
	return true
}

func (sqliteDialect) IsDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (sqliteDialect) Upsert(table string, keyColumns, valueColumns []string) (string, error) {
	return conflictUpsert(func(_ int) string { return "?" }, table, keyColumns, valueColumns)
}

// postgresDialect is the kernel-provided PostgreSQL dialect: $N
// placeholders, RETURNING support, and Postgres unique-violation detection.
type postgresDialect struct{}

func (postgresDialect) Name() string {
	return "postgres"
}

func (postgresDialect) Placeholder(index int) string {
	return "$" + strconv.Itoa(index+1)
}

func (postgresDialect) SupportsReturning() bool {
	return true
}

func (postgresDialect) IsDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate key")
}

func (postgresDialect) Upsert(table string, keyColumns, valueColumns []string) (string, error) {
	return conflictUpsert(postgresPlaceholder, table, keyColumns, valueColumns)
}

func postgresPlaceholder(index int) string {
	return "$" + strconv.Itoa(index+1)
}

// mysqlDialect is the kernel-provided MySQL dialect: "?" placeholders, no
// RETURNING, and MySQL duplicate-entry detection.
type mysqlDialect struct{}

func (mysqlDialect) Name() string {
	return "mysql"
}

func (mysqlDialect) Placeholder(_ int) string {
	return "?"
}

func (mysqlDialect) SupportsReturning() bool {
	return false
}

func (mysqlDialect) IsDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") || strings.Contains(msg, "Duplicate entry")
}

func (mysqlDialect) Upsert(table string, keyColumns, valueColumns []string) (string, error) {
	return duplicateUpsert(table, keyColumns, valueColumns)
}

func conflictUpsert(
	placeholder func(int) string,
	table string,
	keyColumns []string,
	valueColumns []string,
) (string, error) {
	if len(keyColumns) == 0 {
		return "", ErrEmptyUpsertKeys
	}
	columns := append(slices.Clone(keyColumns), valueColumns...)
	var builder strings.Builder
	builder.WriteString("INSERT INTO ")
	builder.WriteString(table)
	builder.WriteString(" (")
	builder.WriteString(strings.Join(columns, ", "))
	builder.WriteString(") VALUES (")
	for i := range columns {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(placeholder(i))
	}
	builder.WriteString(") ON CONFLICT (")
	builder.WriteString(strings.Join(keyColumns, ", "))
	builder.WriteString(")")
	if len(valueColumns) == 0 {
		builder.WriteString(" DO NOTHING")
		return builder.String(), nil
	}
	builder.WriteString(" DO UPDATE SET ")
	for i, column := range valueColumns {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(column)
		builder.WriteString(" = excluded.")
		builder.WriteString(column)
	}
	return builder.String(), nil
}

func duplicateUpsert(
	table string,
	keyColumns []string,
	valueColumns []string,
) (string, error) {
	if len(keyColumns) == 0 {
		return "", ErrEmptyUpsertKeys
	}
	columns := append(slices.Clone(keyColumns), valueColumns...)
	setColumns := valueColumns
	if len(setColumns) == 0 {
		setColumns = keyColumns[:1]
	}
	var builder strings.Builder
	builder.WriteString("INSERT INTO ")
	builder.WriteString(table)
	builder.WriteString(" (")
	builder.WriteString(strings.Join(columns, ", "))
	builder.WriteString(") VALUES (")
	for i := range columns {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString("?")
	}
	builder.WriteString(") ON DUPLICATE KEY UPDATE ")
	for i, column := range setColumns {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(column)
		builder.WriteString(" = VALUES(")
		builder.WriteString(column)
		builder.WriteString(")")
	}
	return builder.String(), nil
}

// SQLiteDialect returns the SQLite dialect.
func SQLiteDialect() Dialect {
	return sqliteDialect{}
}

// PostgresDialect returns the PostgreSQL dialect.
func PostgresDialect() Dialect {
	return postgresDialect{}
}

// MySQLDialect returns the MySQL dialect.
func MySQLDialect() Dialect {
	return mysqlDialect{}
}
