package postgresql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

type PasswordResetToken = repository.PasswordResetToken

// PasswordResetTokenRepository handles password reset token data operations
type PasswordResetTokenRepository struct {
	db *sql.DB
}

// CreateToken creates a new password reset token
func (r *PasswordResetTokenRepository) CreateToken(ctx context.Context, token *PasswordResetToken) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id
	`, token.UserID, token.TokenHash, token.ExpiresAt).Scan(&token.ID)
	if err != nil {
		return err
	}
	return nil
}

// FindValidToken finds a valid (non-expired) password reset token by hash
func (r *PasswordResetTokenRepository) FindValidToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	var token PasswordResetToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = $1 AND expires_at > NOW()
	`, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// DeleteUserTokens deletes all tokens for a user
func (r *PasswordResetTokenRepository) DeleteUserTokens(ctx context.Context, userID int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM password_reset_tokens WHERE user_id = $1
	`, userID)
	return err
}

// NewPasswordResetTokenRepository creates a new password reset token repository
func NewPasswordResetTokenRepository(db *sql.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{
		db: db,
	}
}
