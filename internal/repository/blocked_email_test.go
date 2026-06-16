package repository_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closeBlockedEmailTestDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeBlockedEmailTestDB(db *database.Database) {
	_ = db.Close()
}

func setupBlockedEmailTestDB(t *testing.T) *database.Database {
	t.Helper()

	db, err := database.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")

	// Run migrations
	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")

	return db
}

func TestBlockedEmailRepository_BlockEmail(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Block a valid email
	err := repo.BlockEmail(ctx, "spam@example.com", "marked_as_spam")
	require.NoError(t, err, "Failed to block email")

	// Verify the email is blocked
	blocked, err := repo.IsEmailBlocked(ctx, "spam@example.com")
	require.NoError(t, err, "Failed to check if email is blocked")
	assert.True(t, blocked, "Expected email to be blocked")
}

func TestBlockedEmailRepository_BlockEmail_Duplicate(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Block email first time
	err := repo.BlockEmail(ctx, "spam@example.com", "marked_as_spam")
	require.NoError(t, err, "Failed to block email first time")

	// Try to block same email again — should fail due to UNIQUE constraint
	err = repo.BlockEmail(ctx, "spam@example.com", "marked_as_spam")
	assert.Error(t, err, "Expected error when blocking duplicate email")
}

func TestBlockedEmailRepository_BlockEmail_InvalidEmail(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Block with invalid email format
	err := repo.BlockEmail(ctx, "not-an-email", "test")
	assert.Error(t, err, "Expected error for invalid email format")
}

func TestBlockedEmailRepository_IsEmailBlocked_Blocked(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Block email first
	require.NoError(t, repo.BlockEmail(ctx, "blocked@example.com", "spam"), "Failed to block email")

	// Check if blocked
	blocked, err := repo.IsEmailBlocked(ctx, "blocked@example.com")
	require.NoError(t, err, "Failed to check if email is blocked")
	assert.True(t, blocked, "Expected email to be blocked")
}

func TestBlockedEmailRepository_IsEmailBlocked_NotBlocked(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Check unblocked email
	blocked, err := repo.IsEmailBlocked(ctx, "clean@example.com")
	require.NoError(t, err, "Failed to check if email is blocked")
	assert.False(t, blocked, "Expected email to not be blocked")
}

func TestBlockedEmailRepository_IsEmailBlocked_CaseInsensitive(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Block email in lowercase
	require.NoError(t, repo.BlockEmail(ctx, "spam@example.com", "spam"), "Failed to block email")

	// Check with different casing
	blocked, err := repo.IsEmailBlocked(ctx, "SPAM@EXAMPLE.COM")
	require.NoError(t, err, "Failed to check if email is blocked")
	assert.True(t, blocked, "Expected case-insensitive match for blocked email")
}

func TestBlockedEmailRepository_UnblockEmail(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Block then unblock
	require.NoError(t, repo.BlockEmail(ctx, "temp@example.com", "spam"), "Failed to block email")
	require.NoError(t, repo.UnblockEmail(ctx, "temp@example.com"), "Failed to unblock email")

	// Verify unblocked
	blocked, err := repo.IsEmailBlocked(ctx, "temp@example.com")
	require.NoError(t, err, "Failed to check if email is blocked")
	assert.False(t, blocked, "Expected email to be unblocked")
}

func TestBlockedEmailRepository_UnblockEmail_NotFound(t *testing.T) {
	db := setupBlockedEmailTestDB(t)
	defer closeBlockedEmailTestDB(db)

	repo := repository.NewBlockedEmailRepository(db.DB())
	ctx := context.Background()

	// Unblock non-existent email
	err := repo.UnblockEmail(ctx, "neverblocked@example.com")
	assert.Error(t, err, "Expected error when unblocking non-existent email")
}
