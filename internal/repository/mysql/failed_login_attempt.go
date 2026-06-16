package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

// FailedLoginAttempt is a type alias for repository.FailedLoginAttempt.
type FailedLoginAttempt = repository.FailedLoginAttempt

// FailedLoginAttemptRepository handles failed login attempt data operations with MySQL syntax.
// Implements repository.FailedLoginAttemptRepo.
type FailedLoginAttemptRepository struct {
	db *sql.DB
}

// GetByUserID retrieves failed login attempt record by user ID.
func (r *FailedLoginAttemptRepository) GetByUserID(ctx context.Context, userID int) (*repository.FailedLoginAttempt, error) {

	var attempt repository.FailedLoginAttempt
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, attempts, last_attempt_at, locked_until, last_email_sent_at, created_at, updated_at
		FROM failed_login_attempts
		WHERE user_id = ?
	`, userID).Scan(
		&attempt.ID,
		&attempt.UserID,
		&attempt.Attempts,
		&attempt.LastAttemptAt,
		&attempt.LockedUntil,
		&attempt.LastEmailSentAt,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrFailedLoginAttemptNotFound
	}
	if err != nil {
		return nil, err
	}

	return &attempt, nil
}

// Create creates a new failed login attempt record.
func (r *FailedLoginAttemptRepository) Create(ctx context.Context, attempt *repository.FailedLoginAttempt) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		INSERT INTO failed_login_attempts (user_id, attempts, last_attempt_at, locked_until)
		VALUES (?, ?, ?, ?)
	`, attempt.UserID, attempt.Attempts, attempt.LastAttemptAt, attempt.LockedUntil)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	attempt.ID = int(id)

	return nil
}

// IncrementAttempts increments the failed attempt counter for a user.
func (r *FailedLoginAttemptRepository) IncrementAttempts(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	_, err := r.db.ExecContext(ctx, `
		INSERT INTO failed_login_attempts (user_id, attempts, last_attempt_at)
		VALUES (?, 1, NOW())
		ON DUPLICATE KEY UPDATE
			attempts = attempts + 1,
			last_attempt_at = NOW(),
			updated_at = NOW()
	`, userID)

	return err
}

// LockAccount locks an account for a specified duration.
func (r *FailedLoginAttemptRepository) LockAccount(ctx context.Context, userID int, lockoutDuration time.Duration) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	lockedUntil := time.Now().Add(lockoutDuration)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO failed_login_attempts (user_id, attempts, last_attempt_at, locked_until)
		VALUES (?, 3, NOW(), ?)
		ON DUPLICATE KEY UPDATE
			attempts = 3,
			last_attempt_at = NOW(),
			locked_until = ?,
			updated_at = NOW()
	`, userID, lockedUntil, lockedUntil)

	return err
}

// ResetAttempts resets the failed attempt counter for a user.
func (r *FailedLoginAttemptRepository) ResetAttempts(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	_, err := r.db.ExecContext(ctx, `
		DELETE FROM failed_login_attempts WHERE user_id = ?
	`, userID)

	return err
}

// Delete removes the failed login attempt record for a user.
func (r *FailedLoginAttemptRepository) Delete(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	_, err := r.db.ExecContext(ctx, `
		DELETE FROM failed_login_attempts WHERE user_id = ?
	`, userID)

	return err
}

// IsLocked checks if a user account is currently locked.
func (r *FailedLoginAttemptRepository) IsLocked(ctx context.Context, userID int) (bool, *time.Time, error) {

	var lockedUntil *time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT locked_until
		FROM failed_login_attempts
		WHERE user_id = ? AND locked_until > NOW()
	`, userID).Scan(&lockedUntil)

	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	return true, lockedUntil, nil
}

// UpdateLastEmailSent updates the timestamp when the last lockout email was sent.
func (r *FailedLoginAttemptRepository) UpdateLastEmailSent(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	_, err := r.db.ExecContext(ctx, `
		UPDATE failed_login_attempts
		SET last_email_sent_at = NOW(), updated_at = NOW()
		WHERE user_id = ?
	`, userID)

	return err
}

// ShouldSendLockoutEmail checks if enough time has passed since the last email was sent.
func (r *FailedLoginAttemptRepository) ShouldSendLockoutEmail(ctx context.Context, userID int, minInterval time.Duration) (bool, error) {

	var lastEmailSent *time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT last_email_sent_at
		FROM failed_login_attempts
		WHERE user_id = ?
	`, userID).Scan(&lastEmailSent)

	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	if lastEmailSent == nil {
		return true, nil
	}

	timeSinceLastEmail := time.Since(*lastEmailSent)
	return timeSinceLastEmail >= minInterval, nil
}

// NewFailedLoginAttemptRepository creates a new failed login attempt repository.
func NewFailedLoginAttemptRepository(db *sql.DB) *FailedLoginAttemptRepository {
	return &FailedLoginAttemptRepository{
		db: db,
	}
}
