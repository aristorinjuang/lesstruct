package auth

import (
	"errors"
	"time"
)

var (
	// ErrFailedLoginAttemptNotFound is returned when no failed login attempt
	// record exists for a user.
	ErrFailedLoginAttemptNotFound = errors.New("failed login attempt record not found")
)

// FailedLoginAttempt represents a failed login attempt record
type FailedLoginAttempt struct {
	ID              int
	UserID          int
	Attempts        int
	LastAttemptAt   time.Time
	LockedUntil     *time.Time
	LastEmailSentAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
