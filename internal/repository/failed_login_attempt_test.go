package repository_test

import (
	"context"
	"testing"
	"time"

	appdatabase "github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFailedLoginAttemptRepository_MigratedSchemaSupportsLockout proves the
// migrated SQLite schema exposes every column the failed-login repository reads.
// Schema drift here caused a real outage: bad passwords for existing users
// answered 500, attempts were never recorded, and the lockout never engaged.
func TestFailedLoginAttemptRepository_MigratedSchemaSupportsLockout(t *testing.T) {
	core, err := appdatabase.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "failed to open test database")
	t.Cleanup(func() { _ = core.Close() })
	require.NoError(t, core.RunMigrations("sqlite"), "failed to run migrations")

	repo := repository.NewFailedLoginAttemptRepository(core.DB())
	ctx := context.Background()

	attempt, err := repo.GetByUserID(ctx, 1)
	require.ErrorIs(t, err, authdomain.ErrFailedLoginAttemptNotFound, "fresh user must have no failed-attempt record")
	assert.Nil(t, attempt)

	require.NoError(t, repo.Create(ctx, &authdomain.FailedLoginAttempt{
		UserID:        1,
		Attempts:      1,
		LastAttemptAt: time.Now(),
	}))

	attempt, err = repo.GetByUserID(ctx, 1)
	require.NoError(t, err, "migrated schema must expose every column the repository reads")
	require.NotNil(t, attempt)
	assert.Equal(t, 1, attempt.Attempts)

	require.NoError(t, repo.UpdateLastEmailSent(ctx, 1), "lockout-email bookkeeping must persist")
	require.NoError(t, repo.LockAccount(ctx, 1, 15*time.Minute), "lockout must persist")

	isLocked, lockedUntil, err := repo.IsLocked(ctx, 1)
	require.NoError(t, err)
	assert.True(t, isLocked)
	assert.NotNil(t, lockedUntil)
}
