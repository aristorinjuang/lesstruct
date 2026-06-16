package postgresql_test

import (
	"context"
	"testing"

	postgresqlrepo "github.com/aristorinjuang/lesstruct/internal/repository/postgresql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserCustomFieldsRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	userRepo := postgresqlrepo.NewUserRepository(rawDB)
	userID := seedPostgresUser(t, rawDB, userRepo, "customfielduser", "Author")

	customFields := map[string]any{
		"bio":    "Test bio",
		"website": "https://example.com",
	}

	// Update custom fields
	err := userRepo.UpdateCustomFields(context.Background(), userID, customFields)
	require.NoError(t, err)

	// Verify
	user, err := userRepo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "Test bio", user.CustomFields["bio"])

	// Update profile with custom fields
	err = userRepo.UpdateProfile(context.Background(), userID, "Custom Name", "custom@example.com", "Author", map[string]any{"job": "Engineer"})
	require.NoError(t, err)

	user, err = userRepo.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, "Custom Name", user.Name)
	assert.Equal(t, "Engineer", user.CustomFields["job"])

	// Check email for other user
	exists, err := userRepo.CheckEmailExistsForOtherUser(context.Background(), userID, "other@example.com")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestUserNameRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	userRepo := postgresqlrepo.NewUserRepository(rawDB)

	// Create user with username as default name
	user := &postgresqlrepo.User{
		Username:     "nameuser",
		PasswordHash: "$2a$12$testHash1234567890123456789012",
		Email:        "nameuser@example.com",
		Role:         "Contributor",
		Status:       "pending",
	}
	err := userRepo.CreateUser(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, "nameuser", user.Name) // name defaults to username

	// Get user and verify name
	found, err := userRepo.GetUserByUsername(context.Background(), "nameuser")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.NotEmpty(t, found.Name)
}
