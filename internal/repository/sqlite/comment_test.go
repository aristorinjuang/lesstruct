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

// closeCommentTestDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeCommentTestDB(db *sql.DB) {
	_ = db.Close()
}

func setupCommentTestDB(t *testing.T) *sql.DB {
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

		CREATE TABLE comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			comment TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (content_id) REFERENCES content_items(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);

		INSERT INTO users (username, password_hash, role) VALUES ('alice', 'hash', 'user');
		INSERT INTO users (username, password_hash, role) VALUES ('bob', 'hash', 'user');
		INSERT INTO content_items (user_id, title, slug, content, status) VALUES (1, 'Post', 'post-1', 'Content', 'published');
	`)
	require.NoError(t, err, "Failed to create test tables")

	return db
}

func TestCommentRepository_Count(t *testing.T) {
	db := setupCommentTestDB(t)
	defer closeCommentTestDB(db)

	_, err := db.Exec(`
		INSERT INTO comments (content_id, user_id, comment, status) VALUES
			(1, 1, 'Alice comment', 'pending'),
			(1, 1, 'Alice second comment', 'approved'),
			(1, 2, 'Bob comment', 'pending');
	`)
	require.NoError(t, err, "Failed to insert test comments")

	repo := appsqlite.NewCommentRepository(db)
	ctx := context.Background()

	tests := []struct {
		name     string
		userID   int
		expected int
	}{
		{
			name:     "userID zero counts all comments",
			userID:   0,
			expected: 3,
		},
		{
			name:     "userID counts only that user's comments",
			userID:   1,
			expected: 2,
		},
		{
			name:     "userID with no comments returns zero",
			userID:   3,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, err := repo.Count(ctx, tt.userID)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, total, "Count() mismatch for %q", tt.name)
		})
	}
}
