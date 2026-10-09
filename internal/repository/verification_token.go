package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
)

// VerificationTokenRepository handles verification token data operations
type VerificationTokenRepository struct {
	db *sql.DB
}

// CreateToken creates a new verification token
func (r *VerificationTokenRepository) CreateToken(ctx context.Context, token *authdomain.VerificationToken) error {
	// Verify database connection is alive
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO verification_tokens (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)
	`, token.UserID, token.TokenHash, token.ExpiresAt)
	if err != nil {
		return err
	}

	// Get the last inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	token.ID = int(id)
	return nil
}

// FindValidToken finds a valid (non-expired) verification token by hash
func (r *VerificationTokenRepository) FindValidToken(ctx context.Context, tokenHash string) (*authdomain.VerificationToken, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var token authdomain.VerificationToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM verification_tokens
		WHERE token_hash = ? AND expires_at > datetime('now')
	`, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Token not found or expired
	}
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// DeleteUserTokens deletes all tokens for a user
func (r *VerificationTokenRepository) DeleteUserTokens(ctx context.Context, userID int) error {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := r.db.ExecContext(ctx, `
		DELETE FROM verification_tokens WHERE user_id = ?
	`, userID)
	return err
}

// DeleteExpiredTokens deletes all expired tokens
func (r *VerificationTokenRepository) DeleteExpiredTokens(ctx context.Context) error {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := r.db.ExecContext(ctx, `
		DELETE FROM verification_tokens WHERE expires_at <= datetime('now')
	`)
	return err
}

// PasswordResetTokenRepository handles password reset token data operations
type PasswordResetTokenRepository struct {
	db *sql.DB
}

// CreateToken creates a new password reset token
func (r *PasswordResetTokenRepository) CreateToken(ctx context.Context, token *authdomain.PasswordResetToken) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
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
func (r *PasswordResetTokenRepository) FindValidToken(ctx context.Context, tokenHash string) (*authdomain.PasswordResetToken, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var token authdomain.PasswordResetToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = ? AND expires_at > datetime('now')
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

// DeleteUserTokens deletes all password reset tokens for a user
func (r *PasswordResetTokenRepository) DeleteUserTokens(ctx context.Context, userID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

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

// NewVerificationTokenRepository creates a new verification token repository
func NewVerificationTokenRepository(db *sql.DB) *VerificationTokenRepository {
	return &VerificationTokenRepository{
		db: db,
	}
}
