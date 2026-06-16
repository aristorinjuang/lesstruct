package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/auth"
)

// BlockedEmail represents a blocked email entry
type BlockedEmail struct {
	ID        int
	Email     string
	CreatedAt string
	Reason    string
}

// BlockedEmailRepository handles blocked email data operations
type BlockedEmailRepository struct {
	db *sql.DB
}

// BlockEmail adds an email to the blocked list
func (r *BlockedEmailRepository) BlockEmail(ctx context.Context, email string, reason string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	if err := auth.ValidateEmail(email); err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO blocked_emails (email, reason)
		VALUES ($1, $2)
	`, email, reason)
	if err != nil {
		return fmt.Errorf("failed to block email: %w", err)
	}
	return nil
}

// IsEmailBlocked checks if an email is in the blocked list
func (r *BlockedEmailRepository) IsEmailBlocked(ctx context.Context, email string) (bool, error) {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	if err := auth.ValidateEmail(email); err != nil {
		return false, fmt.Errorf("invalid email format: %w", err)
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM blocked_emails WHERE LOWER(email) = LOWER($1)
		)
	`, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if email is blocked: %w", err)
	}
	return exists, nil
}

// UnblockEmail removes an email from the blocked list
func (r *BlockedEmailRepository) UnblockEmail(ctx context.Context, email string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	if err := auth.ValidateEmail(email); err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM blocked_emails WHERE LOWER(email) = LOWER($1)
	`, email)
	if err != nil {
		return fmt.Errorf("failed to unblock email: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("email not found in blocked list")
	}
	return nil
}

// NewBlockedEmailRepository creates a new blocked email repository
func NewBlockedEmailRepository(db *sql.DB) *BlockedEmailRepository {
	return &BlockedEmailRepository{
		db: db,
	}
}
