package postgresql_test

import (
	"context"
	"testing"

	postgresqlrepo "github.com/aristorinjuang/lesstruct/internal/repository/postgresql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewUserRepository(rawDB)

	// Admin user should not exist in a fresh database
	admin, err := repo.GetAdminUser(context.Background())
	require.Error(t, err)
	assert.Nil(t, admin)

	// Create a user
	user := testUser()
	err = repo.CreateUser(context.Background(), user)
	require.NoError(t, err)
	assert.NotZero(t, user.ID)

	// Retrieve by ID
	found, err := repo.GetUserByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, user.Username, found.Username)

	// Check username exists
	exists, err := repo.CheckUsernameExists(context.Background(), user.Username)
	require.NoError(t, err)
	assert.True(t, exists)

	// Check non-existent username
	exists, err = repo.CheckUsernameExists(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func testUser() *postgresqlrepo.User {
	return &postgresqlrepo.User{
		Username:     "testuser",
		PasswordHash: "$2a$12$testHash1234567890123456789012",
		Email:        "testuser@example.com",
		Role:         "Contributor",
		Status:       "pending",
	}
}
