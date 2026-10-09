package postgresql_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/kernel"
	commentmodule "github.com/aristorinjuang/lesstruct/modules/comments"
	mediamodule "github.com/aristorinjuang/lesstruct/modules/media"
	seomodule "github.com/aristorinjuang/lesstruct/modules/seo"
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
// It mirrors the production boot order (core sequence, then module
// migrations): the media, comments, and alias tables live in their modules
// since Phases 3.2/3.3/3.4, so the module migrations must run before any test
// touching media_files, comments, or content_aliases.
func setupPostgresTestDB(t *testing.T, dsn string) (*database.Database, *sql.DB) {
	t.Helper()
	db, err := database.Open("postgres", dsn, 0)
	if err != nil {
		t.Fatalf("Failed to open PostgreSQL test database: %v", err)
	}
	if err := db.RunMigrations("postgres"); err != nil {
		t.Fatalf("Failed to run PostgreSQL migrations: %v", err)
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
	if err := database.MigrateModules(db.DB(), "postgres", registry); err != nil {
		t.Fatalf("Failed to run PostgreSQL module migrations: %v", err)
	}
	return db, db.DB()
}
