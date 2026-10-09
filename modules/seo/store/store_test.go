package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/database"
	"github.com/aristorinjuang/lesstruct/modules/seo/alias"
	"github.com/aristorinjuang/lesstruct/modules/seo/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// closeAliasTestDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeAliasTestDB(db *sql.DB) {
	_ = db.Close()
}

func setupAliasTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := "file:" + filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err, "Failed to open test database")

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL
		);

		CREATE TABLE content_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			content TEXT NOT NULL,
			status TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(slug)
		);

		CREATE TABLE content_aliases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_id INTEGER NOT NULL,
			alias TEXT NOT NULL UNIQUE,
			FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE
		);

		INSERT INTO users (username, password_hash, role) VALUES ('alice', 'hash', 'user');
		INSERT INTO content_items (user_id, title, slug, content, status) VALUES (1, 'Post', 'post-1', 'Content', 'published');
		INSERT INTO content_items (user_id, title, slug, content, status) VALUES (1, 'Post Two', 'post-2', 'Content', 'published');
	`)
	require.NoError(t, err, "Failed to create test tables")

	return db
}

func TestRepository_Create(t *testing.T) {
	tests := []struct {
		name    string
		seed    bool
		alias   string
		wantErr error
	}{
		{
			name:    "success - assigns the new id",
			seed:    false,
			alias:   "old-post",
			wantErr: nil,
		},
		{
			name:    "error - duplicate alias",
			seed:    true,
			alias:   "old-post",
			wantErr: alias.ErrAliasAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupAliasTestDB(t)
			defer closeAliasTestDB(db)

			repo := store.New(db, database.SQLiteDialect())
			ctx := context.Background()

			if tt.seed {
				require.NoError(t, repo.Create(ctx, &alias.Alias{ContentID: 1, Alias: tt.alias}))
			}

			a := &alias.Alias{ContentID: 1, Alias: tt.alias}
			err := repo.Create(ctx, a)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.NotZero(t, a.ID, "Create() did not set ID")
		})
	}
}

func TestRepository_Find(t *testing.T) {
	tests := []struct {
		name      string
		lookup    string
		wantID    int
		wantCount int
		wantErr   error
	}{
		{
			name:      "success - finds by alias",
			lookup:    "old-post",
			wantID:    1,
			wantCount: -1,
			wantErr:   nil,
		},
		{
			name:      "error - missing alias",
			lookup:    "no-such-alias",
			wantCount: -1,
			wantErr:   alias.ErrAliasNotFound,
		},
		{
			name:      "success - lists content aliases and all aliases",
			lookup:    "",
			wantCount: 2,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupAliasTestDB(t)
			defer closeAliasTestDB(db)

			repo := store.New(db, database.SQLiteDialect())
			ctx := context.Background()

			require.NoError(t, repo.Create(ctx, &alias.Alias{ContentID: 1, Alias: "old-post"}))
			require.NoError(t, repo.Create(ctx, &alias.Alias{ContentID: 2, Alias: "older-post"}))

			if tt.wantCount >= 0 {
				byContent, err := repo.FindByContentID(ctx, 1)
				require.NoError(t, err)
				assert.Len(t, byContent, 1, "FindByContentID() mismatch")

				all, err := repo.FindAll(ctx)
				require.NoError(t, err)
				assert.Len(t, all, tt.wantCount, "FindAll() mismatch")
				return
			}

			found, err := repo.FindByAlias(ctx, tt.lookup)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, found.ID, "FindByAlias() ID mismatch")
			assert.Equal(t, 1, found.ContentID, "FindByAlias() ContentID mismatch")
		})
	}
}

func TestRepository_Mutate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  string
		wantErr error
	}{
		{
			name:    "success - repoints to the new content",
			mutate:  "repoint",
			wantErr: nil,
		},
		{
			name:    "error - repoint misses the guarded content id",
			mutate:  "repoint-miss",
			wantErr: alias.ErrAliasNotFound,
		},
		{
			name:    "success - deletes by content id",
			mutate:  "delete",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupAliasTestDB(t)
			defer closeAliasTestDB(db)

			repo := store.New(db, database.SQLiteDialect())
			ctx := context.Background()

			require.NoError(t, repo.Create(ctx, &alias.Alias{ContentID: 1, Alias: "old-post"}))

			switch tt.mutate {
			case "repoint":
				require.NoError(t, repo.Repoint(ctx, "old-post", 1, 2))
				found, err := repo.FindByAlias(ctx, "old-post")
				require.NoError(t, err)
				assert.Equal(t, 2, found.ContentID, "Repoint() did not move the alias")
			case "repoint-miss":
				err := repo.Repoint(ctx, "old-post", 999, 2)
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			case "delete":
				require.NoError(t, repo.DeleteByContentID(ctx, 1))
				remaining, err := repo.FindByContentID(ctx, 1)
				require.NoError(t, err)
				assert.Empty(t, remaining, "DeleteByContentID() left rows behind")
			}
		})
	}
}
