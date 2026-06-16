package postgresql_test

import (
	"context"
	"testing"

	postgresqlrepo "github.com/aristorinjuang/lesstruct/internal/repository/postgresql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailedLoginAttemptRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewFailedLoginAttemptRepository(rawDB)

	// Should not exist initially
	_, err := repo.GetByUserID(context.Background(), 1)
	assert.Error(t, err)

	// Increment attempts
	err = repo.IncrementAttempts(context.Background(), 1)
	require.NoError(t, err)

	// Should now exist
	attempt, err := repo.GetByUserID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, attempt.Attempts)
	assert.NotNil(t, attempt.LastAttemptAt)

	// Reset
	err = repo.ResetAttempts(context.Background(), 1)
	require.NoError(t, err)

	// Should be deleted
	_, err = repo.GetByUserID(context.Background(), 1)
	assert.Error(t, err)
}

func TestBlockedEmailRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewBlockedEmailRepository(rawDB)
	ctx := context.Background()

	// Check non-blocked email
	blocked, err := repo.IsEmailBlocked(ctx, "test@example.com")
	require.NoError(t, err)
	assert.False(t, blocked)

	// Block email (use a valid email format to pass auth.ValidateEmail)
	err = repo.BlockEmail(ctx, "blocked@example.com", "test reason")
	require.NoError(t, err)

	// Verify blocked
	blocked, err = repo.IsEmailBlocked(ctx, "blocked@example.com")
	require.NoError(t, err)
	assert.True(t, blocked)

	// Unblock
	err = repo.UnblockEmail(ctx, "blocked@example.com")
	require.NoError(t, err)

	// Verify unblocked
	blocked, err = repo.IsEmailBlocked(ctx, "blocked@example.com")
	require.NoError(t, err)
	assert.False(t, blocked)

	// Block invalid email should fail
	err = repo.BlockEmail(ctx, "invalid-email", "reason")
	assert.Error(t, err)
	// Invalid email format should be caught by auth.ValidateEmail
	if err != nil {
		assert.Contains(t, err.Error(), "email")
	}
}
