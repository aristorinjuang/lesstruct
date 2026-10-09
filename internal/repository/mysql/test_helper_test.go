//go:build mysql

package mysql

import (
	"database/sql"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/kernel"
	commentmodule "github.com/aristorinjuang/lesstruct/modules/comments"
	mediamodule "github.com/aristorinjuang/lesstruct/modules/media"
	seomodule "github.com/aristorinjuang/lesstruct/modules/seo"
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
// It mirrors the production boot order (core sequence, then module
// migrations): the media, comments, and alias tables live in their modules
// since Phases 3.2/3.3/3.4, so the module migrations must run before any test
// touching media_files, comments, or content_aliases.
func setupMySQLTestDB(t *testing.T, dsn string) (*database.Database, *sql.DB) {
	t.Helper()
	db, err := database.Open("mysql", dsn, 0)
	if err != nil {
		t.Fatalf("Failed to open MySQL test database: %v", err)
	}
	if err := db.RunMigrations("mysql"); err != nil {
		t.Fatalf("Failed to run MySQL migrations: %v", err)
	}
	registry := kernel.NewRegistry()
	if err := registry.Register(mediamodule.New()); err != nil {
		t.Fatalf("Failed to register media module: %v", err)
	}
	if err := registry.Register(commentmodule.New()); err != nil {
		t.Fatalf("Failed to register comments module: %v", err)
	}
	if err := registry.Register(seomodule.New()); err != nil {
		t.Fatalf("Failed to register seo module: %v", err)
	}
	if err := database.MigrateModules(db.DB(), "mysql", registry); err != nil {
		t.Fatalf("Failed to run MySQL module migrations: %v", err)
	}
	return db, db.DB()
}
