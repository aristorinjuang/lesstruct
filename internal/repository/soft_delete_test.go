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

// closeSoftDeleteTestDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeSoftDeleteTestDB(db *database.Database) {
	_ = db.Close()
}

func setupSoftDeleteTestDB(t *testing.T) *database.Database {
	t.Helper()

	db, err := database.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")

	// Run migrations
	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")

	return db
}

func TestSoftDeleteContent(t *testing.T) {
	db := setupSoftDeleteTestDB(t)
	defer closeSoftDeleteTestDB(db)

	repo := repository.NewSoftDeleteRepository(db.DB())
	userRepo := repository.NewUserRepository(db.DB())

	// Create a test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Soft delete some content
	err = repo.SoftDeleteContent(context.Background(), "post", 123, user.ID, 1, "Test deletion")
	require.NoError(t, err, "SoftDeleteContent failed")

	// Verify content was soft deleted
	contentList, err := repo.GetSoftDeletedContentByUser(context.Background(), user.ID)
	require.NoError(t, err, "GetSoftDeletedContentByUser failed")
	assert.Len(t, contentList, 1, "Expected 1 soft deleted content")
	assert.Equal(t, "post", contentList[0].ContentType, "Expected content type 'post'")
	assert.Equal(t, 123, contentList[0].ContentID, "Expected content ID 123")
}

func TestGetSoftDeletedContentByUser(t *testing.T) {
	db := setupSoftDeleteTestDB(t)
	defer closeSoftDeleteTestDB(db)

	repo := repository.NewSoftDeleteRepository(db.DB())
	userRepo := repository.NewUserRepository(db.DB())

	// Create a test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Soft delete multiple content items
	contentItems := []struct {
		contentType string
		contentID   int
		reason      string
	}{
		{"post", 123, "Spam"},
		{"post", 124, "Inappropriate"},
		{"comment", 456, "Violation"},
	}

	for _, item := range contentItems {
		err = repo.SoftDeleteContent(context.Background(), item.contentType, item.contentID, user.ID, 1, item.reason)
		require.NoError(t, err, "Failed to soft delete content")
	}

	// Get all soft deleted content for user
	contentList, err := repo.GetSoftDeletedContentByUser(context.Background(), user.ID)
	require.NoError(t, err, "GetSoftDeletedContentByUser failed")
	assert.Len(t, contentList, 3, "Expected 3 soft deleted content items")

	// Verify all content types are present
	contentTypes := make(map[string]bool)
	for _, content := range contentList {
		contentTypes[content.ContentType] = true
	}

	assert.True(t, contentTypes["post"], "Expected post content type")
	assert.True(t, contentTypes["comment"], "Expected comment content type")
}

func TestRestoreContent(t *testing.T) {
	db := setupSoftDeleteTestDB(t)
	defer closeSoftDeleteTestDB(db)

	repo := repository.NewSoftDeleteRepository(db.DB())
	userRepo := repository.NewUserRepository(db.DB())

	// Create a test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Soft delete content
	err = repo.SoftDeleteContent(context.Background(), "post", 123, user.ID, 1, "Test deletion")
	require.NoError(t, err, "Failed to soft delete content")

	// Verify content is soft deleted
	contentList, err := repo.GetSoftDeletedContentByUser(context.Background(), user.ID)
	require.NoError(t, err, "Failed to get soft deleted content")
	require.Len(t, contentList, 1, "Expected 1 soft deleted content")

	// Restore the content
	err = repo.RestoreContent(context.Background(), "post", 123)
	require.NoError(t, err, "RestoreContent failed")

	// Verify content is restored
	contentList, err = repo.GetSoftDeletedContentByUser(context.Background(), user.ID)
	require.NoError(t, err, "Failed to get soft deleted content")
	assert.Len(t, contentList, 0, "Expected 0 soft deleted content after restore")
}

func TestRestoreNonExistentContent(t *testing.T) {
	db := setupSoftDeleteTestDB(t)
	defer closeSoftDeleteTestDB(db)

	repo := repository.NewSoftDeleteRepository(db.DB())

	// Try to restore non-existent content
	err := repo.RestoreContent(context.Background(), "post", 99999)
	assert.Error(t, err, "Expected error when restoring non-existent content")
}

func TestPermanentDeleteContent(t *testing.T) {
	db := setupSoftDeleteTestDB(t)
	defer closeSoftDeleteTestDB(db)

	repo := repository.NewSoftDeleteRepository(db.DB())
	userRepo := repository.NewUserRepository(db.DB())

	// Create a test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Soft delete content
	err = repo.SoftDeleteContent(context.Background(), "post", 123, user.ID, 1, "Test deletion")
	require.NoError(t, err, "Failed to soft delete content")

	// Verify content is soft deleted
	contentList, err := repo.GetSoftDeletedContentByUser(context.Background(), user.ID)
	require.NoError(t, err, "Failed to get soft deleted content")
	require.Len(t, contentList, 1, "Expected 1 soft deleted content")

	// Permanently delete the content (placeholder for Story 1.8)
	err = repo.PermanentDeleteContent(context.Background(), "post", 123)
	require.NoError(t, err, "PermanentDeleteContent failed")

	// Verify content is removed from soft delete table
	contentList, err = repo.GetSoftDeletedContentByUser(context.Background(), user.ID)
	require.NoError(t, err, "Failed to get soft deleted content")
	assert.Len(t, contentList, 0, "Expected 0 soft deleted content after permanent delete")
}
