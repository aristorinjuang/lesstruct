package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	appsqlite "github.com/aristorinjuang/lesstruct/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// closeDashboardTestDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeDashboardTestDB(db *sql.DB) {
	_ = db.Close()
}

func setupDashboardTestDB(t *testing.T) *sql.DB {
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
			role TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending'
		);

		CREATE TABLE content_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			status TEXT NOT NULL,
			post_type TEXT DEFAULT 'post',
			meta_description TEXT,
			og_title TEXT,
			og_description TEXT,
			language TEXT NOT NULL DEFAULT 'en',
			translation_group_id INTEGER DEFAULT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(slug, language)
		);

		CREATE TABLE media_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			content_id INTEGER,
			filename TEXT NOT NULL,
			original_filename TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			width INTEGER,
			height INTEGER,
			alt_text TEXT,
			is_webp BOOLEAN DEFAULT TRUE,
			file_path TEXT NOT NULL,
			url TEXT NOT NULL,
			hash TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE SET NULL
		);

		CREATE INDEX idx_content_items_user_id ON content_items(user_id);
		CREATE INDEX idx_content_items_created_at ON content_items(created_at);
		CREATE INDEX idx_media_files_user_id ON media_files(user_id);

		INSERT INTO users (username, password_hash, role, status) VALUES ('testuser', 'hash', 'user', 'verified');
		INSERT INTO users (username, password_hash, role, status) VALUES ('testuser2', 'hash', 'user', 'verified');
		INSERT INTO users (username, password_hash, role, status) VALUES ('admin', 'hash', 'admin', 'verified');
	`)
	require.NoError(t, err, "Failed to create test tables")

	return db
}

func TestDashboardRepository_GetStats_Empty(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer closeDashboardTestDB(db)

	repo := appsqlite.NewDashboardRepository(db)
	ctx := context.Background()

	stats, err := repo.GetStats(ctx, 1)
	require.NoError(t, err, "GetStats() failed")

	assert.Equal(t, 0, stats.PublishedPosts, "GetStats() PublishedPosts mismatch")
	assert.Equal(t, 0, stats.DraftPosts, "GetStats() DraftPosts mismatch")
	assert.Equal(t, 0, stats.MediaItems, "GetStats() MediaItems mismatch")
	assert.Equal(t, 3, stats.RegisteredUsers, "GetStats() RegisteredUsers mismatch")
	assert.Len(t, stats.RecentContent, 0, "GetStats() RecentContent should be empty")
}

func TestDashboardRepository_GetStats_WithData(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer closeDashboardTestDB(db)

	// Insert test data
	_, err := db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, status) VALUES
			(1, 'Published Post 1', 'published-post-1', 'Content 1', 'published'),
			(1, 'Published Post 2', 'published-post-2', 'Content 2', 'published'),
			(1, 'Draft Post 1', 'draft-post-1', 'Content 3', 'draft'),
			(1, 'Draft Post 2', 'draft-post-2', 'Content 4', 'draft'),
			(1, 'Draft Post 3', 'draft-post-3', 'Content 5', 'draft'),
			(2, 'Other User Post', 'other-user-post', 'Content 6', 'published');

		INSERT INTO media_files (user_id, filename, original_filename, mime_type, file_size, file_path, url, hash) VALUES
			(1, 'image1.webp', 'image1.jpg', 'image/webp', 1000, '/uploads/image1.webp', 'http://localhost:8080/uploads/image1.webp', 'hash1'),
			(1, 'image2.webp', 'image2.jpg', 'image/webp', 2000, '/uploads/image2.webp', 'http://localhost:8080/uploads/image2.webp', 'hash2'),
			(2, 'image3.webp', 'image3.jpg', 'image/webp', 3000, '/uploads/image3.webp', 'http://localhost:8080/uploads/image3.webp', 'hash3');
	`)
	require.NoError(t, err, "Failed to insert test data")

	repo := appsqlite.NewDashboardRepository(db)
	ctx := context.Background()

	stats, err := repo.GetStats(ctx, 1)
	require.NoError(t, err, "GetStats() failed")

	assert.Equal(t, 2, stats.PublishedPosts, "GetStats() PublishedPosts mismatch")
	assert.Equal(t, 3, stats.DraftPosts, "GetStats() DraftPosts mismatch")
	assert.Equal(t, 2, stats.MediaItems, "GetStats() MediaItems mismatch")
	assert.Equal(t, 3, stats.RegisteredUsers, "GetStats() RegisteredUsers mismatch")
	assert.Len(t, stats.RecentContent, 5, "GetStats() RecentContent should have 5 items")
}

func TestDashboardRepository_GetStats_RecentContentOrder(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer closeDashboardTestDB(db)

	// Insert content in specific order
	_, err := db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, status, created_at) VALUES
			(1, 'Oldest Post', 'oldest', 'Content', 'published', datetime('now', '-3 days')),
			(1, 'Middle Post', 'middle', 'Content', 'published', datetime('now', '-1 days')),
			(1, 'Newest Post', 'newest', 'Content', 'published', datetime('now'));
	`)
	require.NoError(t, err, "Failed to insert test data")

	repo := appsqlite.NewDashboardRepository(db)
	ctx := context.Background()

	stats, err := repo.GetStats(ctx, 1)
	require.NoError(t, err, "GetStats() failed")

	assert.Len(t, stats.RecentContent, 3, "GetStats() RecentContent should have 3 items")
	// Verify order is by created_at DESC (newest first)
	assert.Equal(t, "Newest Post", stats.RecentContent[0].Title, "First item should be newest")
	assert.Equal(t, "Middle Post", stats.RecentContent[1].Title, "Second item should be middle")
	assert.Equal(t, "Oldest Post", stats.RecentContent[2].Title, "Third item should be oldest")
}

func TestDashboardRepository_GetStats_UserFiltering(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer closeDashboardTestDB(db)

	// Insert test data for different users
	_, err := db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, status) VALUES
			(1, 'User 1 Post', 'user-1-post', 'Content', 'published'),
			(2, 'User 2 Post', 'user-2-post', 'Content', 'published');

		INSERT INTO media_files (user_id, filename, original_filename, mime_type, file_size, file_path, url, hash) VALUES
			(1, 'user1.webp', 'user1.jpg', 'image/webp', 1000, '/uploads/user1.webp', 'http://localhost:8080/uploads/user1.webp', 'hash-user1'),
			(2, 'user2.webp', 'user2.jpg', 'image/webp', 2000, '/uploads/user2.webp', 'http://localhost:8080/uploads/user2.webp', 'hash-user2');
	`)
	require.NoError(t, err, "Failed to insert test data")

	repo := appsqlite.NewDashboardRepository(db)
	ctx := context.Background()

	// Check stats for user 1
	stats1, err := repo.GetStats(ctx, 1)
	require.NoError(t, err, "GetStats() failed for user 1")
	assert.Equal(t, 1, stats1.PublishedPosts, "User 1 should have 1 content")
	assert.Equal(t, 1, stats1.MediaItems, "User 1 should have 1 media")

	// Check stats for user 2
	stats2, err := repo.GetStats(ctx, 2)
	require.NoError(t, err, "GetStats() failed for user 2")
	assert.Equal(t, 1, stats2.PublishedPosts, "User 2 should have 1 content")
	assert.Equal(t, 1, stats2.MediaItems, "User 2 should have 1 media")

	// Both users should see the same total users count
	assert.Equal(t, stats1.RegisteredUsers, stats2.RegisteredUsers, "RegisteredUsers should be the same for all users")
}

func TestDashboardRepository_GetStats_LimitRecentContent(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer closeDashboardTestDB(db)

	// Insert more than 5 content items
	_, err := db.Exec(`
		INSERT INTO content_items (user_id, title, slug, content, status) VALUES
			(1, 'Post 1', 'post-1', 'Content 1', 'published'),
			(1, 'Post 2', 'post-2', 'Content 2', 'published'),
			(1, 'Post 3', 'post-3', 'Content 3', 'published'),
			(1, 'Post 4', 'post-4', 'Content 4', 'published'),
			(1, 'Post 5', 'post-5', 'Content 5', 'published'),
			(1, 'Post 6', 'post-6', 'Content 6', 'published'),
			(1, 'Post 7', 'post-7', 'Content 7', 'published');
	`)
	require.NoError(t, err, "Failed to insert test data")

	repo := appsqlite.NewDashboardRepository(db)
	ctx := context.Background()

	stats, err := repo.GetStats(ctx, 1)
	require.NoError(t, err, "GetStats() failed")

	assert.Equal(t, 7, stats.PublishedPosts, "GetStats() PublishedPosts mismatch")
	assert.Len(t, stats.RecentContent, 5, "GetStats() RecentContent should be limited to 5 items")
}
