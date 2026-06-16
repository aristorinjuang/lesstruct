package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EmailUpdateToken represents an email update token in the system
type EmailUpdateToken struct {
	ID        int
	Token     string
	UserID    int
	NewEmail  string
	ExpiresAt string
	CreatedAt string
}

// EmailUpdateTokenRepo defines the interface for email update token repository operations
type EmailUpdateTokenRepo interface {
	CreateToken(ctx context.Context, token string, userID int, newEmail string) error
	GetToken(ctx context.Context, token string) (*EmailUpdateToken, error)
	DeleteToken(ctx context.Context, tokenID int) error
	CleanUpExpiredTokens(ctx context.Context) error
}

// EmailUpdateTokenRepository handles email update token data operations
type EmailUpdateTokenRepository struct {
	db *sql.DB
}

// CreateToken creates a new email update token
func (r *EmailUpdateTokenRepository) CreateToken(ctx context.Context, token string, userID int, newEmail string) error {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_update_tokens (token, user_id, new_email, expires_at)
		VALUES (?, ?, ?, datetime('now', '+24 hours'))
	`, token, userID, newEmail)
	if err != nil {
		return fmt.Errorf("failed to create email update token: %w", err)
	}

	return nil
}

// GetToken retrieves an email update token by token value
func (r *EmailUpdateTokenRepository) GetToken(ctx context.Context, token string) (*EmailUpdateToken, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var emailUpdateToken EmailUpdateToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, token, user_id, new_email, expires_at, created_at
		FROM email_update_tokens
		WHERE token = ?
		  AND expires_at > datetime('now')
	`, token).Scan(
		&emailUpdateToken.ID,
		&emailUpdateToken.Token,
		&emailUpdateToken.UserID,
		&emailUpdateToken.NewEmail,
		&emailUpdateToken.ExpiresAt,
		&emailUpdateToken.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("token not found or expired")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email update token: %w", err)
	}

	return &emailUpdateToken, nil
}

// DeleteToken deletes an email update token by ID
func (r *EmailUpdateTokenRepository) DeleteToken(ctx context.Context, tokenID int) error {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := r.db.ExecContext(ctx, `
		DELETE FROM email_update_tokens WHERE id = ?
	`, tokenID)
	if err != nil {
		return fmt.Errorf("failed to delete email update token: %w", err)
	}

	return nil
}

// CleanUpExpiredTokens deletes all expired email update tokens
func (r *EmailUpdateTokenRepository) CleanUpExpiredTokens(ctx context.Context) error {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	_, err := r.db.ExecContext(ctx, `
		DELETE FROM email_update_tokens WHERE expires_at < datetime('now')
	`)
	if err != nil {
		return fmt.Errorf("failed to clean up expired tokens: %w", err)
	}

	return nil
}

// NewEmailUpdateTokenRepository creates a new email update token repository
func NewEmailUpdateTokenRepository(db *sql.DB) *EmailUpdateTokenRepository {
	return &EmailUpdateTokenRepository{
		db: db,
	}
}
