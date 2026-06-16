package postgresql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

type EmailUpdateToken = repository.EmailUpdateToken

// EmailUpdateTokenRepository handles email update token data operations
type EmailUpdateTokenRepository struct {
	db *sql.DB
}

// CreateToken creates a new email update token
func (r *EmailUpdateTokenRepository) CreateToken(ctx context.Context, token string, userID int, newEmail string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_update_tokens (token, user_id, new_email, expires_at)
		VALUES ($1, $2, $3, NOW() + INTERVAL '24 hours')
	`, token, userID, newEmail)
	if err != nil {
		return fmt.Errorf("failed to create email update token: %w", err)
	}
	return nil
}

// GetToken retrieves an email update token by token value
func (r *EmailUpdateTokenRepository) GetToken(ctx context.Context, token string) (*EmailUpdateToken, error) {
	var emailUpdateToken EmailUpdateToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, token, user_id, new_email, expires_at, created_at
		FROM email_update_tokens
		WHERE token = $1
		  AND expires_at > NOW()
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
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM email_update_tokens WHERE id = $1
	`, tokenID)
	if err != nil {
		return fmt.Errorf("failed to delete email update token: %w", err)
	}
	return nil
}

// CleanUpExpiredTokens deletes all expired email update tokens
func (r *EmailUpdateTokenRepository) CleanUpExpiredTokens(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM email_update_tokens WHERE expires_at < NOW()
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
