package postgresql_test

import (
	"context"
	"testing"
	"time"

	postgresqlrepo "github.com/aristorinjuang/lesstruct/internal/repository/postgresql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationTokenRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewVerificationTokenRepository(rawDB)
	ctx := context.Background()

	token := &postgresqlrepo.VerificationToken{
		UserID:    1,
		TokenHash: "test-token-hash-123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := repo.CreateToken(ctx, token)
	require.NoError(t, err)
	assert.NotZero(t, token.ID)

	found, err := repo.FindValidToken(ctx, "test-token-hash-123")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, 1, found.UserID)

	// Cleanup
	err = repo.DeleteUserTokens(ctx, 1)
	require.NoError(t, err)

	// Delete expired tokens
	err = repo.DeleteExpiredTokens(ctx)
	require.NoError(t, err)
}

func TestPasswordResetTokenRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewPasswordResetTokenRepository(rawDB)
	ctx := context.Background()

	token := &postgresqlrepo.PasswordResetToken{
		UserID:    1,
		TokenHash: "reset-token-hash-123",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	err := repo.CreateToken(ctx, token)
	require.NoError(t, err)
	assert.NotZero(t, token.ID)

	found, err := repo.FindValidToken(ctx, "reset-token-hash-123")
	require.NoError(t, err)
	require.NotNil(t, found)

	err = repo.DeleteUserTokens(ctx, 1)
	require.NoError(t, err)
}

func TestSoftDeleteRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewSoftDeleteRepository(rawDB)
	ctx := context.Background()

	err := repo.SoftDeleteContent(ctx, "content_items", 1, 1, 1, "test deletion")
	require.NoError(t, err)

	items, err := repo.GetSoftDeletedContentByUser(ctx, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestUserDeletionRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewUserDeletionRepository(rawDB)
	ctx := context.Background()

	err := repo.DeleteUserTokens(ctx, 999)
	_ = err
}

func TestUserDataExportRepo_PostgreSQL(t *testing.T) {
	dsn := postgresDSN(t)
	db, rawDB := setupPostgresTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	repo := postgresqlrepo.NewUserDataExportRepository(rawDB)
	ctx := context.Background()

	content, err := repo.GetUserContent(ctx, 1)
	require.NoError(t, err)
	assert.NotNil(t, content)

	comments, err := repo.GetUserComments(ctx, 1)
	require.NoError(t, err)
	assert.NotNil(t, comments)

	media, err := repo.GetUserMedia(ctx, 1)
	require.NoError(t, err)
	assert.NotNil(t, media)
}
