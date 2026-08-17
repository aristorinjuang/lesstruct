package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// setupUserDeletionTestDB creates the tables DeleteAllUserData touches with
// just the columns its DELETE statements reference, seeded with a user owning
// content and aliases.
func setupUserDeletionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "failed to open test database")

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT
		);
		CREATE TABLE content_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL
		);
		CREATE TABLE content_aliases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_id INTEGER NOT NULL,
			alias TEXT NOT NULL UNIQUE
		);
		CREATE TABLE comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL
		);
		CREATE TABLE media_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uploaded_by_user_id INTEGER NOT NULL
		);
		CREATE TABLE verification_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL
		);
		CREATE TABLE email_update_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL
		);
		CREATE TABLE failed_login_attempts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO users (id) VALUES (1)`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO content_items (id, user_id) VALUES (10, 1), (11, 1);
		INSERT INTO content_aliases (content_id, alias) VALUES (10, 'old-a.html'), (11, 'old-b.html');
	`)
	require.NoError(t, err)

	return db
}

func TestUserDeletionRepository_DeleteAllUserDataRemovesAliases(t *testing.T) {
	db := setupUserDeletionTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repository.NewUserDeletionRepository(db)
	err := repo.DeleteAllUserData(context.Background(), 1)
	require.NoError(t, err)

	var contentCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM content_items`).Scan(&contentCount))
	assert.Zero(t, contentCount)

	var aliasCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM content_aliases`).Scan(&aliasCount))
	assert.Zero(t, aliasCount, "content_aliases must not dangle after account deletion")
}

func TestUserDeletionRepository_DeleteAllUserDataKeepsOthersAliases(t *testing.T) {
	db := setupUserDeletionTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`
		INSERT INTO users (id) VALUES (2);
		INSERT INTO content_items (id, user_id) VALUES (20, 2);
		INSERT INTO content_aliases (content_id, alias) VALUES (20, 'other.html');
	`)
	require.NoError(t, err)

	repo := repository.NewUserDeletionRepository(db)
	err = repo.DeleteAllUserData(context.Background(), 1)
	require.NoError(t, err)

	var remainingAlias string
	require.NoError(t, db.QueryRow(`SELECT alias FROM content_aliases`).Scan(&remainingAlias))
	assert.Equal(t, "other.html", remainingAlias)
}
