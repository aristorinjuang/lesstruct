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

// closeUserTestDB is a helper function that explicitly ignores the error from Close()
// to satisfy errcheck linter for defer statements.
func closeUserTestDB(db *database.Database) {
	_ = db.Close()
}

func setupUserTestDB(t *testing.T) *database.Database {
	t.Helper()

	db, err := database.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")

	// Run migrations
	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")

	return db
}

func TestNewUserRepository(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())
	assert.NotNil(t, repo, "NewUserRepository returned nil")
}

func TestUpdateAdminPasswordAndEmail(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// First, create the default admin user
	defaultPasswordHash := "$2a$12$testHash"
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, role)
		VALUES (?, ?, ?)
	`, "admin", defaultPasswordHash, "Admin")
	require.NoError(t, err, "Failed to create admin user")

	// Update admin user
	newPasswordHash := "$2a$12$newHash"
	newEmail := "admin@example.com"
	err = repo.UpdateAdminPasswordAndEmail(context.Background(), newPasswordHash, newEmail, defaultPasswordHash)
	require.NoError(t, err, "UpdateAdminPasswordAndEmail unexpected error")

	// Verify update
	var passwordHash, email string
	err = db.DB().QueryRow(`
		SELECT password_hash, email FROM users WHERE username = ?
	`, "admin").Scan(&passwordHash, &email)
	require.NoError(t, err, "Failed to query admin user")

	assert.Equal(t, newPasswordHash, passwordHash, "Expected password_hash to match")
	assert.Equal(t, newEmail, email, "Expected email to match")
}

func TestUpdateAdminPasswordAndEmail_AdminNotFound(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Don't create admin user - it doesn't exist
	err := repo.UpdateAdminPasswordAndEmail(context.Background(), "$2a$12$newHash", "admin@example.com", "$2a$12$defaultHash")
	assert.Error(t, err, "UpdateAdminPasswordAndEmail expected error when admin not found")
	assert.ErrorIs(t, err, repository.ErrSetupAlreadyCompleted, "Expected ErrSetupAlreadyCompleted")
}

func TestGetAdminUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create admin user
	defaultPasswordHash := "$2a$12$testHash"
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role)
		VALUES (?, ?, ?, ?)
	`, "admin", defaultPasswordHash, "admin@example.com", "Admin")
	require.NoError(t, err, "Failed to create admin user")

	// Get admin user
	user, err := repo.GetAdminUser(context.Background())
	require.NoError(t, err, "GetAdminUser unexpected error")

	assert.Equal(t, "admin", user.Username, "Expected username 'admin'")
	assert.Equal(t, "Admin", user.Role, "Expected role 'Admin'")
}

func TestGetAdminUser_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Don't create admin user
	_, err := repo.GetAdminUser(context.Background())
	assert.Error(t, err, "GetAdminUser expected error when admin not found")
	assert.ErrorIs(t, err, repository.ErrAdminNotFound, "Expected ErrAdminNotFound")
}

func TestCreateUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create a new user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "$2a$12$testHash",
		Email:        "test@example.com",
		Role:         "Contributor",
		Status:       "pending",
	}

	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err, "CreateUser unexpected error")

	// Verify user was created
	assert.NotZero(t, user.ID, "Expected user ID to be set after creation")

	// Verify user in database
	var username, email, role, status string
	err = db.DB().QueryRow(`
		SELECT username, email, role, status FROM users WHERE id = ?
	`, user.ID).Scan(&username, &email, &role, &status)
	require.NoError(t, err, "Failed to query created user")

	assert.Equal(t, "testuser", username, "Expected username 'testuser'")
	assert.Equal(t, "test@example.com", email, "Expected email 'test@example.com'")
	assert.Equal(t, "Contributor", role, "Expected role 'Contributor'")
	assert.Equal(t, "pending", status, "Expected status 'pending'")
}

func TestCheckUsernameExists(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create a user
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status)
		VALUES (?, ?, ?, ?, ?)
	`, "existinguser", "$2a$12$testHash", "existing@example.com", "Contributor", "pending")
	require.NoError(t, err, "Failed to create user")

	// Test existing username (case-insensitive)
	exists, err := repo.CheckUsernameExists(context.Background(), "ExistingUser")
	require.NoError(t, err, "CheckUsernameExists unexpected error")
	assert.True(t, exists, "Expected username to exist")

	// Test non-existing username
	exists, err = repo.CheckUsernameExists(context.Background(), "nonexistinguser")
	require.NoError(t, err, "CheckUsernameExists unexpected error")
	assert.False(t, exists, "Expected username to not exist")
}

func TestCheckEmailExists(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create a user
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status)
		VALUES (?, ?, ?, ?, ?)
	`, "testuser", "$2a$12$testHash", "existing@example.com", "Contributor", "pending")
	require.NoError(t, err, "Failed to create user")

	// Test existing email (case-insensitive)
	exists, err := repo.CheckEmailExists(context.Background(), "Existing@Example.com")
	require.NoError(t, err, "CheckEmailExists unexpected error")
	assert.True(t, exists, "Expected email to exist")

	// Test non-existing email
	exists, err = repo.CheckEmailExists(context.Background(), "nonexisting@example.com")
	require.NoError(t, err, "CheckEmailExists unexpected error")
	assert.False(t, exists, "Expected email to not exist")
}

func TestSuspendUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create a test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Suspend the user
	err = repo.SuspendUser(context.Background(), user.ID)
	require.NoError(t, err, "SuspendUser failed")

	// Verify user is suspended
	status, err := repo.GetUserStatus(context.Background(), user.ID)
	require.NoError(t, err, "GetUserStatus failed")
	assert.Equal(t, "suspended", status, "Expected status 'suspended'")
}

func TestUnsuspendUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create a suspended user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "suspended",
	}
	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Unsuspend the user
	err = repo.UnsuspendUser(context.Background(), user.ID)
	require.NoError(t, err, "UnsuspendUser failed")

	// Verify user is verified
	status, err := repo.GetUserStatus(context.Background(), user.ID)
	require.NoError(t, err, "GetUserStatus failed")
	assert.Equal(t, "verified", status, "Expected status 'verified'")
}

func TestSoftDeleteUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create a test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Soft delete the user
	err = repo.SoftDeleteUser(context.Background(), user.ID)
	require.NoError(t, err, "SoftDeleteUser failed")

	// Verify user is soft deleted
	status, err := repo.GetUserStatus(context.Background(), user.ID)
	require.NoError(t, err, "GetUserStatus failed")
	assert.Equal(t, "soft_deleted", status, "Expected status 'soft_deleted'")
}

func TestGetAllUsers(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create test users with different statuses
	users := []*repository.User{
		{Username: "user1", PasswordHash: "hash", Email: "user1@example.com", Role: "Author", Status: "verified"},
		{Username: "user2", PasswordHash: "hash", Email: "user2@example.com", Role: "Author", Status: "suspended"},
		{Username: "user3", PasswordHash: "hash", Email: "user3@example.com", Role: "Author", Status: "verified"},
		{Username: "user4", PasswordHash: "hash", Email: "user4@example.com", Role: "Author", Status: "soft_deleted"},
	}

	for _, user := range users {
		err := repo.CreateUser(context.Background(), user)
		require.NoError(t, err, "Failed to create user")
	}

	// Test getting all users
	allUsers, err := repo.GetAllUsers(context.Background(), "", 100, 0)
	require.NoError(t, err, "GetAllUsers failed")
	assert.GreaterOrEqual(t, len(allUsers), 4, "Expected at least 4 users")

	// Test filtering by status
	verifiedUsers, err := repo.GetAllUsers(context.Background(), "verified", 100, 0)
	require.NoError(t, err, "GetAllUsers with status filter failed")
	assert.GreaterOrEqual(t, len(verifiedUsers), 2, "Expected at least 2 verified users")

	// Test pagination
	paginatedUsers, err := repo.GetAllUsers(context.Background(), "", 2, 0)
	require.NoError(t, err, "GetAllUsers with pagination failed")
	assert.Len(t, paginatedUsers, 2, "Expected 2 users with limit=2")
}

func TestGetUserStatus(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	// Create a test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	// Get user status
	status, err := repo.GetUserStatus(context.Background(), user.ID)
	require.NoError(t, err, "GetUserStatus failed")
	assert.Equal(t, "verified", status, "Expected status 'verified'")

	// Test non-existing user
	_, err = repo.GetUserStatus(context.Background(), 99999)
	assert.Error(t, err, "Expected error for non-existing user")
}

func TestUpdatePasswordByUserID(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "$2a$12$oldHash",
		Email:        "test@example.com",
		Role:         "Author",
		Status:       "verified",
	}
	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err, "Failed to create user")

	newHash := "$2a$12$newHash"
	err = repo.UpdatePasswordByUserID(context.Background(), user.ID, newHash)
	require.NoError(t, err, "UpdatePasswordByUserID failed")

	var passwordHash string
	err = db.DB().QueryRow(`
		SELECT password_hash FROM users WHERE id = ?
	`, user.ID).Scan(&passwordHash)
	require.NoError(t, err, "Failed to query password")
	assert.Equal(t, newHash, passwordHash, "Expected password to be updated")
}

func TestUpdatePasswordByUserID_UserNotFound(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())

	err := repo.UpdatePasswordByUserID(context.Background(), 99999, "$2a$12$newHash")
	assert.Error(t, err, "Expected error for non-existing user")
}
