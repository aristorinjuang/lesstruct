package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrFailedLoginAttemptNotFound = errors.New("failed login attempt record not found")
)

// FailedLoginAttempt represents a failed login attempt record
type FailedLoginAttempt struct {
	ID                int
	UserID            int
	Attempts          int
	LastAttemptAt     time.Time
	LockedUntil       *time.Time
	LastEmailSentAt   *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// FailedLoginAttemptRepo defines the interface for failed login attempt repository operations
type FailedLoginAttemptRepo interface {
	GetByUserID(ctx context.Context, userID int) (*FailedLoginAttempt, error)
	Create(ctx context.Context, attempt *FailedLoginAttempt) error
	IncrementAttempts(ctx context.Context, userID int) error
	LockAccount(ctx context.Context, userID int, lockoutDuration time.Duration) error
	ResetAttempts(ctx context.Context, userID int) error
	Delete(ctx context.Context, userID int) error
	IsLocked(ctx context.Context, userID int) (bool, *time.Time, error)
	UpdateLastEmailSent(ctx context.Context, userID int) error
	ShouldSendLockoutEmail(ctx context.Context, userID int, minInterval time.Duration) (bool, error)
}

// FailedLoginAttemptRepository handles failed login attempt data operations
type FailedLoginAttemptRepository struct {
	db *sql.DB
}

// GetByUserID retrieves failed login attempt record by user ID
func (r *FailedLoginAttemptRepository) GetByUserID(ctx context.Context, userID int) (*FailedLoginAttempt, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var attempt FailedLoginAttempt
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
		return nil, ErrFailedLoginAttemptNotFound
	}
	if err != nil {
		return nil, err
	}

	return &attempt, nil
}

// Create creates a new failed login attempt record
func (r *FailedLoginAttemptRepository) Create(ctx context.Context, attempt *FailedLoginAttempt) error {
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
		INSERT INTO failed_login_attempts (user_id, attempts, last_attempt_at, locked_until)
		VALUES (?, ?, ?, ?)
	`, attempt.UserID, attempt.Attempts, attempt.LastAttemptAt, attempt.LockedUntil)
	if err != nil {
		return err
	}

	// Get the last inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	attempt.ID = int(id)
	return nil
}

// IncrementAttempts increments the failed attempt counter for a user
func (r *FailedLoginAttemptRepository) IncrementAttempts(ctx context.Context, userID int) error {
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

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO failed_login_attempts (user_id, attempts, last_attempt_at)
		VALUES (?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			attempts = attempts + 1,
			last_attempt_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
	`, userID)

	return err
}

// LockAccount locks an account for a specified duration
func (r *FailedLoginAttemptRepository) LockAccount(ctx context.Context, userID int, lockoutDuration time.Duration) error {
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

	lockedUntil := time.Now().Add(lockoutDuration)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO failed_login_attempts (user_id, attempts, last_attempt_at, locked_until)
		VALUES (?, 3, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			attempts = 3,
			last_attempt_at = CURRENT_TIMESTAMP,
			locked_until = ?,
			updated_at = CURRENT_TIMESTAMP
	`, userID, lockedUntil, lockedUntil)

	return err
}

// ResetAttempts resets the failed attempt counter for a user
func (r *FailedLoginAttemptRepository) ResetAttempts(ctx context.Context, userID int) error {
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

	_, err := r.db.ExecContext(ctx, `
		DELETE FROM failed_login_attempts WHERE user_id = ?
	`, userID)

	return err
}

// Delete removes the failed login attempt record for a user
func (r *FailedLoginAttemptRepository) Delete(ctx context.Context, userID int) error {
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

	_, err := r.db.ExecContext(ctx, `
		DELETE FROM failed_login_attempts WHERE user_id = ?
	`, userID)

	return err
}

// IsLocked checks if a user account is currently locked
func (r *FailedLoginAttemptRepository) IsLocked(ctx context.Context, userID int) (bool, *time.Time, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var lockedUntil *time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT locked_until
		FROM failed_login_attempts
		WHERE user_id = ? AND locked_until > CURRENT_TIMESTAMP
	`, userID).Scan(&lockedUntil)

	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	return true, lockedUntil, nil
}

// UpdateLastEmailSent updates the timestamp when the last lockout email was sent
func (r *FailedLoginAttemptRepository) UpdateLastEmailSent(ctx context.Context, userID int) error {
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

	_, err := r.db.ExecContext(ctx, `
		UPDATE failed_login_attempts
		SET last_email_sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, userID)

	return err
}

// ShouldSendLockoutEmail checks if enough time has passed since the last email was sent
func (r *FailedLoginAttemptRepository) ShouldSendLockoutEmail(ctx context.Context, userID int, minInterval time.Duration) (bool, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var lastEmailSent *time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT last_email_sent_at
		FROM failed_login_attempts
		WHERE user_id = ?
	`, userID).Scan(&lastEmailSent)

	if err == sql.ErrNoRows {
		// No record exists, so no email has been sent
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// If no email has been sent yet, send one
	if lastEmailSent == nil {
		return true, nil
	}

	// Check if enough time has passed since the last email
	timeSinceLastEmail := time.Since(*lastEmailSent)
	return timeSinceLastEmail >= minInterval, nil
}

// NewFailedLoginAttemptRepository creates a new failed login attempt repository
func NewFailedLoginAttemptRepository(db *sql.DB) *FailedLoginAttemptRepository {
	return &FailedLoginAttemptRepository{
		db: db,
	}
}
