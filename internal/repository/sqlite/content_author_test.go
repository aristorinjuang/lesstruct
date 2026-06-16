package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupContentTestDBWithName(t *testing.T) *sql.DB {
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

		CREATE INDEX IF NOT EXISTS idx_content_items_slug ON content_items(slug);
		CREATE INDEX IF NOT EXISTS idx_content_items_user_id ON content_items(user_id);
		CREATE INDEX IF NOT EXISTS idx_content_items_status ON content_items(status);
		CREATE INDEX IF NOT EXISTS idx_content_items_post_type ON content_items(post_type);
		CREATE INDEX IF NOT EXISTS idx_content_items_language ON content_items(language);
		CREATE INDEX IF NOT EXISTS idx_content_items_translation_group ON content_items(translation_group_id);
	`)
	require.NoError(t, err, "failed to create test tables")

	// Insert test users with different name scenarios
	_, err = db.Exec(`
		INSERT INTO users (id, username, password_hash, email, name, role, status) VALUES
		(1, 'janedoe', 'hash1', 'jane@example.com', 'Jane Doe', 'Author', 'verified'),
		(2, 'bobsmith', 'hash2', 'bob@example.com', NULL, 'Author', 'verified'),
		(3, 'alice', 'hash3', 'alice@example.com', 'Alice Johnson', 'Author', 'verified')
	`)
	require.NoError(t, err, "failed to insert test users")

	// Insert test content
	_, err = db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type) VALUES
		(1, 'Post by Jane', 'post-by-jane', 'Content by Jane', '["test"]', 'published', 'post'),
		(2, 'Post by Bob', 'post-by-bob', 'Content by Bob', '["test"]', 'published', 'post'),
		(3, 'Post by Alice', 'post-by-alice', 'Content by Alice', '["test"]', 'published', 'post')
	`)
	require.NoError(t, err, "failed to insert test content")

	return db
}

func TestContentRepository_GetPublishedBySlug_WithAuthor(t *testing.T) {
	db := setupContentTestDBWithName(t)
	defer func() { _ = db.Close() }()

	repo := sqlite.NewContentRepository(db)

	content, err := repo.GetPublishedBySlug(context.Background(), "post-by-jane", "en")
	require.NoError(t, err, "GetPublishedBySlug unexpected error")
	assert.NotNil(t, content, "Content should not be nil")
	assert.Equal(t, "Jane Doe", content.Author, "Expected author name 'Jane Doe'")
}

func TestContentRepository_GetPublishedBySlug_AuthorFallbackToUsername(t *testing.T) {
	db := setupContentTestDBWithName(t)
	defer func() { _ = db.Close() }()

	repo := sqlite.NewContentRepository(db)

	content, err := repo.GetPublishedBySlug(context.Background(), "post-by-bob", "en")
	require.NoError(t, err, "GetPublishedBySlug unexpected error")
	assert.NotNil(t, content, "Content should not be nil")
	// Bob has NULL name, should fallback to username
	assert.Equal(t, "bobsmith", content.Author, "Expected author name to fallback to username 'bobsmith'")
}

func TestContentRepository_GetPublished_WithAuthor(t *testing.T) {
	db := setupContentTestDBWithName(t)
	defer func() { _ = db.Close() }()

	repo := sqlite.NewContentRepository(db)

	contents, err := repo.GetPublished(context.Background(), 10, 0)
	require.NoError(t, err, "GetPublished unexpected error")
	assert.Len(t, contents, 3, "Expected 3 published contents")

	// Check that all contents have author names set
	for _, content := range contents {
		assert.NotEmpty(t, content.Author, "Author should not be empty for content: "+content.Slug)
	}

	// Verify specific authors
	authors := make(map[string]string)
	for _, content := range contents {
		authors[content.Slug] = content.Author
	}

	assert.Equal(t, "Jane Doe", authors["post-by-jane"], "Expected Jane Doe as author")
	assert.Equal(t, "bobsmith", authors["post-by-bob"], "Expected bobsmith as author (fallback)")
	assert.Equal(t, "Alice Johnson", authors["post-by-alice"], "Expected Alice Johnson as author")
}
