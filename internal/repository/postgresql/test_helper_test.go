package postgresql_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/database"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// postgresDSN returns the PostgreSQL connection string from environment or skips the test.
func postgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skipping PostgreSQL integration test")
	}
	return dsn
}

// setupPostgresTestDB opens a PostgreSQL test database and runs migrations.
func setupPostgresTestDB(t *testing.T, dsn string) (*database.Database, *sql.DB) {
	t.Helper()
	db, err := database.Open("postgres", dsn, 0)
	if err != nil {
		t.Fatalf("Failed to open PostgreSQL test database: %v", err)
	}
	if err := db.RunMigrations("postgres"); err != nil {
		t.Fatalf("Failed to run PostgreSQL migrations: %v", err)
	}
	return db, db.DB()
}
