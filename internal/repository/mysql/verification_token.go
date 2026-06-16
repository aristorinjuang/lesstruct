package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

type VerificationToken = repository.VerificationToken

// VerificationTokenRepository handles verification token data operations
type VerificationTokenRepository struct {
	db *sql.DB
}

// CreateToken creates a new verification token
func (r *VerificationTokenRepository) CreateToken(ctx context.Context, token *VerificationToken) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO verification_tokens (user_id, token_hash, expires_at)
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

// FindValidToken finds a valid (non-expired) verification token by hash
func (r *VerificationTokenRepository) FindValidToken(ctx context.Context, tokenHash string) (*VerificationToken, error) {
	var token VerificationToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM verification_tokens
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
func (r *VerificationTokenRepository) DeleteUserTokens(ctx context.Context, userID int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM verification_tokens WHERE user_id = ?
	`, userID)
	return err
}

// DeleteExpiredTokens deletes all expired tokens
func (r *VerificationTokenRepository) DeleteExpiredTokens(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM verification_tokens WHERE expires_at <= NOW()
	`)
	return err
}

// NewVerificationTokenRepository creates a new verification token repository
func NewVerificationTokenRepository(db *sql.DB) *VerificationTokenRepository {
	return &VerificationTokenRepository{
		db: db,
	}
}
