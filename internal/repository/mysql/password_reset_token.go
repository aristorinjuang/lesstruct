package mysql

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
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)
	`, token.UserID, token.TokenHash, token.ExpiresAt)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	token.ID = int(id)
	return nil
}

// FindValidToken finds a valid (non-expired) password reset token by hash
func (r *PasswordResetTokenRepository) FindValidToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	var token PasswordResetToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = ? AND expires_at > NOW()
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
		DELETE FROM password_reset_tokens WHERE user_id = ?
	`, userID)
	return err
}

// NewPasswordResetTokenRepository creates a new password reset token repository
func NewPasswordResetTokenRepository(db *sql.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{
		db: db,
	}
}
