package repository_test

import (
	"context"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserRepository_TokenVersionRoundTrip proves every user lookup used by the
// auth middleware returns the stored session generation. Without it the
// middleware would compare against a zero value and never detect a revoked
// token after a password change.
func TestUserRepository_TokenVersionRoundTrip(t *testing.T) {
	db := setupUserTestDB(t)
	defer closeUserTestDB(db)

	repo := repository.NewUserRepository(db.DB())
	ctx := context.Background()

	user := &userdomain.User{
		Username:     "tokenuser",
		PasswordHash: "$2a$12$oldHash",
		Email:        "tokenuser@example.com",
		Role:         "Contributor",
		Status:       "verified",
	}
	require.NoError(t, repo.CreateUser(ctx, user))
	require.Equal(t, 0, user.TokenVersion, "new accounts start at version zero")

	_, err := db.DB().Exec(`UPDATE users SET token_version = 7 WHERE id = ?`, user.ID)
	require.NoError(t, err)

	byID, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, byID)
	assert.Equal(t, 7, byID.TokenVersion)

	byUsername, err := repo.GetUserByUsername(ctx, "tokenuser")
	require.NoError(t, err)
	require.NotNil(t, byUsername)
	assert.Equal(t, 7, byUsername.TokenVersion)

	byEmail, err := repo.GetUserByEmail(ctx, "tokenuser@example.com")
	require.NoError(t, err)
	require.NotNil(t, byEmail)
	assert.Equal(t, 7, byEmail.TokenVersion)
}
