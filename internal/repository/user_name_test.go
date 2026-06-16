package repository_test

import (
	"context"
	"database/sql"
	"testing"

	apprepository "github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestUserNameField(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := apprepository.NewUserRepository(db.DB())

	// Test creating a user with a custom name
	userWithName := &apprepository.User{
		Username:     "testuser",
		PasswordHash: "$2a$12$testHash",
		Email:        "test@example.com",
		Role:         "Contributor",
		Status:       "pending",
	}
	err := repo.CreateUser(context.Background(), userWithName)
	require.NoError(t, err, "CreateUser unexpected error")

	// Verify the name field exists and is defaulted to username
	var name sql.NullString
	err = db.DB().QueryRow(`
		SELECT name FROM users WHERE id = ?
	`, userWithName.ID).Scan(&name)
	require.NoError(t, err, "Failed to query name field")
	assert.True(t, name.Valid, "Name should be valid (not NULL)")
	assert.Equal(t, "testuser", name.String, "Name should default to username")
}

func TestUserNameField_CustomName(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	// Insert a user with a custom display name
	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, name)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "customuser", "$2a$12$testHash", "custom@example.com", "Contributor", "pending", "Custom Display Name")
	require.NoError(t, err, "Failed to insert user with custom name")

	id, err := result.LastInsertId()
	require.NoError(t, err, "Failed to get last insert ID")

	// Verify the custom name is stored
	var name string
	err = db.DB().QueryRow(`
		SELECT name FROM users WHERE id = ?
	`, id).Scan(&name)
	require.NoError(t, err, "Failed to query name field")
	assert.Equal(t, "Custom Display Name", name, "Name should match custom display name")
}

func TestUserNameField_Nullable(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	// Insert a user with NULL name
	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, name)
		VALUES (?, ?, ?, ?, ?, NULL)
	`, "nullnameuser", "$2a$12$testHash", "nullname@example.com", "Contributor", "pending")
	require.NoError(t, err, "Failed to insert user with NULL name")

	id, err := result.LastInsertId()
	require.NoError(t, err, "Failed to get last insert ID")

	// Verify the name can be NULL
	var name sql.NullString
	err = db.DB().QueryRow(`
		SELECT name FROM users WHERE id = ?
	`, id).Scan(&name)
	require.NoError(t, err, "Failed to query name field")
	assert.False(t, name.Valid, "Name should be NULL")
}

func TestGetUserByID_WithName(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := apprepository.NewUserRepository(db.DB())

	// Insert a user with a custom name
	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, name)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "nameduser", "$2a$12$testHash", "named@example.com", "Contributor", "pending", "Jane Doe")
	require.NoError(t, err, "Failed to insert user with custom name")

	id, err := result.LastInsertId()
	require.NoError(t, err, "Failed to get last insert ID")

	// Get the user by ID
	user, err := repo.GetUserByID(context.Background(), int(id))
	require.NoError(t, err, "GetUserByID unexpected error")
	assert.Equal(t, "nameduser", user.Username, "Expected username 'nameduser'")
	assert.Equal(t, "Jane Doe", user.Name, "Expected name 'Jane Doe'")
}

func TestGetUserByUsername_WithName(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := apprepository.NewUserRepository(db.DB())

	// Insert a user with a custom name
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status, name)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "byusernameuser", "$2a$12$testHash", "byusername@example.com", "Contributor", "pending", "John Smith")
	require.NoError(t, err, "Failed to insert user with custom name")

	// Get the user by username
	user, err := repo.GetUserByUsername(context.Background(), "byusernameuser")
	require.NoError(t, err, "GetUserByUsername unexpected error")
	assert.NotNil(t, user, "Expected user to be returned")
	assert.Equal(t, "byusernameuser", user.Username, "Expected username 'byusernameuser'")
	assert.Equal(t, "John Smith", user.Name, "Expected name 'John Smith'")
}

func TestGetAdminUser_WithName(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := apprepository.NewUserRepository(db.DB())

	// Create admin user with custom name
	defaultPasswordHash := "$2a$12$testHash"
	_, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, name)
		VALUES (?, ?, ?, ?, ?)
	`, "admin", defaultPasswordHash, "admin@example.com", "Admin", "Administrator")
	require.NoError(t, err, "Failed to create admin user")

	// Get admin user
	user, err := repo.GetAdminUser(context.Background())
	require.NoError(t, err, "GetAdminUser unexpected error")
	assert.Equal(t, "admin", user.Username, "Expected username 'admin'")
	assert.Equal(t, "Admin", user.Role, "Expected role 'Admin'")
	assert.Equal(t, "Administrator", user.Name, "Expected name 'Administrator'")
}
