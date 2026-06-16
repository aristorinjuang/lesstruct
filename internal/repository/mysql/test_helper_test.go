//go:build mysql

package mysql

import (
	"database/sql"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/database"
	_ "github.com/go-sql-driver/mysql"
)

// mysqlDSN returns the MySQL connection string from environment or skips the test.
func mysqlDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set, skipping MySQL integration test")
	}
	return dsn
}

// setupMySQLTestDB opens a MySQL test database and runs migrations.
func setupMySQLTestDB(t *testing.T, dsn string) (*database.Database, *sql.DB) {
	t.Helper()
	db, err := database.Open("mysql", dsn, 0)
	if err != nil {
		t.Fatalf("Failed to open MySQL test database: %v", err)
	}
	if err := db.RunMigrations("mysql"); err != nil {
		t.Fatalf("Failed to run MySQL migrations: %v", err)
	}
	return db, db.DB()
}
