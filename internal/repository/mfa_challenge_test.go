package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/database"
	"github.com/aristorinjuang/lesstruct/internal/repository"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func setupMFATestDB(t *testing.T) (*database.Database, int) {
	t.Helper()

	db, err := database.Open("sqlite", ":memory:", 0)
	require.NoError(t, err, "Failed to open test database")
	t.Cleanup(func() {
		_ = db.Close()
	})

	require.NoError(t, db.RunMigrations("sqlite"), "Failed to run migrations")

	result, err := db.DB().Exec(`
		INSERT INTO users (username, password_hash, email, role, status)
		VALUES ('mfauser', 'hash', 'mfa@example.com', 'Commentator', 'verified')
	`)
	require.NoError(t, err, "Failed to seed user")

	userID, err := result.LastInsertId()
	require.NoError(t, err, "Failed to read seeded user ID")

	return db, int(userID)
}

func TestMFAChallengeRepository(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, repo *repository.MFAChallengeRepository, userID int)
	}{
		{
			name: "success - create and find by token hash",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, userID int) {
				createdAt := time.Now().Add(-time.Minute).Truncate(time.Second)
				expiresAt := time.Now().Add(5 * time.Minute).Truncate(time.Second)
				challenge := &authdomain.MFAChallenge{
					UserID:    userID,
					TokenHash: "token-hash",
					CodeHash:  "code-hash",
					CreatedAt: createdAt,
					ExpiresAt: expiresAt,
				}

				require.NoError(t, repo.Create(context.Background(), challenge))
				assert.NotZero(t, challenge.ID, "assigned ID")

				found, err := repo.FindByTokenHash(context.Background(), "token-hash")
				require.NoError(t, err)
				require.NotNil(t, found)
				assert.Equal(t, challenge.ID, found.ID, "ID")
				assert.Equal(t, userID, found.UserID, "UserID")
				assert.Equal(t, "code-hash", found.CodeHash, "CodeHash")
				assert.Equal(t, 0, found.Attempts, "Attempts")
				assert.WithinDuration(t, createdAt, found.CreatedAt, time.Second, "CreatedAt")
				assert.WithinDuration(t, expiresAt, found.ExpiresAt, time.Second, "ExpiresAt")
			},
		},
		{
			name: "success - zero created at defaults to now",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, userID int) {
				challenge := &authdomain.MFAChallenge{
					UserID:    userID,
					TokenHash: "token-hash",
					CodeHash:  "code-hash",
					ExpiresAt: time.Now().Add(time.Minute),
				}

				require.NoError(t, repo.Create(context.Background(), challenge))
				assert.WithinDuration(t, time.Now(), challenge.CreatedAt, time.Second, "assigned CreatedAt")
			},
		},
		{
			name: "success - unknown token returns nil",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, _ int) {
				found, err := repo.FindByTokenHash(context.Background(), "missing")
				require.NoError(t, err)
				assert.Nil(t, found)
			},
		},
		{
			name: "success - increment attempts stops at the cap",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, userID int) {
				challenge := &authdomain.MFAChallenge{
					UserID:    userID,
					TokenHash: "token-hash",
					CodeHash:  "code-hash",
					ExpiresAt: time.Now().Add(time.Minute),
				}
				require.NoError(t, repo.Create(context.Background(), challenge))

				spent, err := repo.IncrementAttempts(context.Background(), challenge.ID, 2)
				require.NoError(t, err)
				assert.True(t, spent, "first attempt")

				spent, err = repo.IncrementAttempts(context.Background(), challenge.ID, 2)
				require.NoError(t, err)
				assert.True(t, spent, "second attempt")

				spent, err = repo.IncrementAttempts(context.Background(), challenge.ID, 2)
				require.NoError(t, err)
				assert.False(t, spent, "attempt above the cap")

				found, err := repo.FindByTokenHash(context.Background(), "token-hash")
				require.NoError(t, err)
				require.NotNil(t, found)
				assert.Equal(t, 2, found.Attempts, "Attempts")
			},
		},
		{
			name: "success - consume deletes the challenge only once",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, userID int) {
				challenge := &authdomain.MFAChallenge{
					UserID:    userID,
					TokenHash: "token-hash",
					CodeHash:  "code-hash",
					ExpiresAt: time.Now().Add(time.Minute),
				}
				require.NoError(t, repo.Create(context.Background(), challenge))

				consumed, err := repo.Consume(context.Background(), challenge.ID)
				require.NoError(t, err)
				assert.True(t, consumed, "first consume")

				consumed, err = repo.Consume(context.Background(), challenge.ID)
				require.NoError(t, err)
				assert.False(t, consumed, "second consume")

				found, err := repo.FindByTokenHash(context.Background(), "token-hash")
				require.NoError(t, err)
				assert.Nil(t, found)
			},
		},
		{
			name: "success - delete one challenge",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, userID int) {
				challenge := &authdomain.MFAChallenge{
					UserID:    userID,
					TokenHash: "token-hash",
					CodeHash:  "code-hash",
					ExpiresAt: time.Now().Add(time.Minute),
				}
				require.NoError(t, repo.Create(context.Background(), challenge))

				require.NoError(t, repo.Delete(context.Background(), challenge.ID))

				found, err := repo.FindByTokenHash(context.Background(), "token-hash")
				require.NoError(t, err)
				assert.Nil(t, found)
			},
		},
		{
			name: "success - delete every challenge for a user",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, userID int) {
				for _, tokenHash := range []string{"first", "second"} {
					require.NoError(t, repo.Create(context.Background(), &authdomain.MFAChallenge{
						UserID:    userID,
						TokenHash: tokenHash,
						CodeHash:  "code-hash",
						ExpiresAt: time.Now().Add(time.Minute),
					}))
				}

				require.NoError(t, repo.DeleteUserChallenges(context.Background(), userID))

				for _, tokenHash := range []string{"first", "second"} {
					found, err := repo.FindByTokenHash(context.Background(), tokenHash)
					require.NoError(t, err)
					assert.Nil(t, found, tokenHash)
				}
			},
		},
		{
			name: "success - delete expired keeps live challenges",
			run: func(t *testing.T, repo *repository.MFAChallengeRepository, userID int) {
				require.NoError(t, repo.Create(context.Background(), &authdomain.MFAChallenge{
					UserID:    userID,
					TokenHash: "expired",
					CodeHash:  "code-hash",
					ExpiresAt: time.Now().Add(-time.Hour),
				}))
				require.NoError(t, repo.Create(context.Background(), &authdomain.MFAChallenge{
					UserID:    userID,
					TokenHash: "live",
					CodeHash:  "code-hash",
					ExpiresAt: time.Now().Add(time.Hour),
				}))

				require.NoError(t, repo.DeleteExpired(context.Background()))

				expired, err := repo.FindByTokenHash(context.Background(), "expired")
				require.NoError(t, err)
				assert.Nil(t, expired, "expired challenge")

				live, err := repo.FindByTokenHash(context.Background(), "live")
				require.NoError(t, err)
				assert.NotNil(t, live, "live challenge")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, userID := setupMFATestDB(t)
			repo := repository.NewMFAChallengeRepository(db.DB())

			tt.run(t, repo, userID)
		})
	}
}
