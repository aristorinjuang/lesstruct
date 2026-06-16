package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDeleteTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "failed to open test database")

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			email TEXT,
			name TEXT,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS content_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			content TEXT,
			tags TEXT,
			status TEXT DEFAULT 'draft',
			post_type TEXT DEFAULT 'post',
			meta_description TEXT,
			og_title TEXT,
			og_description TEXT,
			allow_comments INTEGER DEFAULT 1,
			custom_fields TEXT,
			updated_by INTEGER REFERENCES users(id),
			language TEXT NOT NULL DEFAULT 'en',
			translation_group_id INTEGER DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(slug, language)
		);

		CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			comment TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (content_id) REFERENCES content_items(id)
		);

		INSERT INTO users (id, username, password_hash, email, name, role, status) VALUES
			(1, 'user1', 'hash', 'user1@example.com', 'User One', 'user', 'active'),
			(2, 'user2', 'hash', 'user2@example.com', 'User Two', 'user', 'active');
	`)
	require.NoError(t, err, "failed to create test tables")

	return db
}

func TestContentRepository_Delete(t *testing.T) {
	db := setupDeleteTestDB(t)
	defer func() { assert.NoError(t, db.Close()) }()

	repo := sqlite.NewContentRepository(db)

	_, err := db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type)
		VALUES (1, 'Test Content', 'test-content', 'Test body', '["test"]', 'draft', 'post')
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO comments (content_id, user_id, comment, status)
		VALUES (1, 1, 'A comment', 'approved')
	`)
	require.NoError(t, err)

	t.Run("successful deletion with cascade cleanup", func(t *testing.T) {
		err := repo.Delete(context.Background(), 1, 1)
		require.NoError(t, err)

		var count int
		err = db.QueryRow(`SELECT COUNT(*) FROM content_items WHERE id = 1`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "content should be deleted")

		err = db.QueryRow(`SELECT COUNT(*) FROM comments WHERE content_id = 1`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "comments should be cascade deleted")
	})

	t.Run("content not found", func(t *testing.T) {
		err := repo.Delete(context.Background(), 999, 1)
		require.Error(t, err)
		assert.ErrorIs(t, err, content.ErrContentNotFound)
	})

	t.Run("wrong owner returns not found", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type)
			VALUES (1, 'Owned Content', 'owned-content', 'Body', '[]', 'draft', 'post')
		`)
		require.NoError(t, err)

		err = repo.Delete(context.Background(), 2, 2)
		require.Error(t, err)
		assert.ErrorIs(t, err, content.ErrContentNotFound)
	})
}
