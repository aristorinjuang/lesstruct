package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/constants"
)

var (
	ErrAdminNotFound         = errors.New("admin user not found")
	ErrSetupAlreadyCompleted = errors.New("first-login setup has already been completed")
)

// User represents a user in the system
type User struct {
	ID             int            `json:"id"`
	Username       string         `json:"username"`
	PasswordHash   string         `json:"-"`
	Email          string         `json:"email"`
	Name           string         `json:"name,omitempty"`
	Role           string         `json:"role"`
	Status         string         `json:"-"`
	ProfilePicture string         `json:"profilePicture,omitempty"`
	LastLoginAt    *string        `json:"lastLoginAt,omitempty"`
	CustomFields   map[string]any `json:"customFields,omitempty"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt,omitempty"`
}

func unmarshalCustomFields(raw *string) map[string]any {
	if raw == nil || *raw == "" {
		return nil
	}
	var fields map[string]any
	_ = json.Unmarshal([]byte(*raw), &fields)
	return fields
}

func marshalCustomFields(fields map[string]any) (any, error) {
	if fields == nil {
		return nil, nil
	}
	cfBytes, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom fields: %w", err)
	}
	return string(cfBytes), nil
}

// UserRepo defines the interface for user repository operations
type UserRepo interface {
	UpdateAdminPasswordAndEmail(ctx context.Context, passwordHash, email, currentPasswordHash string) error
	GetAdminUser(ctx context.Context) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	CheckUsernameExists(ctx context.Context, username string) (bool, error)
	CheckEmailExists(ctx context.Context, email string) (bool, error)
	UpdateUserStatus(ctx context.Context, userID int, status string) error
	UpdateUserStatusIfCurrentStatus(ctx context.Context, userID int, currentStatus string, newStatus string) error
	GetUserByID(ctx context.Context, userID int) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetPendingUsers(ctx context.Context, limit int, offset int) ([]*User, error)
	DeleteUser(ctx context.Context, userID int) error
	SuspendUser(ctx context.Context, userID int) error
	UnsuspendUser(ctx context.Context, userID int) error
	SoftDeleteUser(ctx context.Context, userID int) error
	GetAllUsers(ctx context.Context, status string, limit int, offset int) ([]*User, error)
	GetUserStatus(ctx context.Context, userID int) (string, error)
	UpdateEmail(ctx context.Context, userID int, email string) error
	UpdatePassword(ctx context.Context, userID int, currentPasswordHash, newPasswordHash string) error
	UpdateLastLoginAt(ctx context.Context, userID int) error
	UpdatePasswordByUserID(ctx context.Context, userID int, newPasswordHash string) error
	UpdateProfile(ctx context.Context, userID int, name string, email string, role string, customFields map[string]any) error
	UpdateCustomFields(ctx context.Context, userID int, customFields map[string]any) error
	CheckEmailExistsForOtherUser(ctx context.Context, userID int, email string) (bool, error)
	UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error
	DeleteProfilePicture(ctx context.Context, userID int) error
}

// UserRepository handles user data operations
type UserRepository struct {
	db *sql.DB
}

// UpdateAdminPasswordAndEmail updates the admin user's password and email.
// Uses optimistic locking via currentPasswordHash to prevent concurrent setup completion.
func (r *UserRepository) UpdateAdminPasswordAndEmail(ctx context.Context, passwordHash, email, currentPasswordHash string) error {
	// Add timeout context if not provided (#16)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Verify database connection is alive (#17)
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	// Sanitize email: trim whitespace and normalize to lowercase (defense-in-depth)
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?,
		    email = ?,
		    status = 'verified',
		    updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND password_hash = ?
	`, passwordHash, email, constants.DefaultUsername, currentPasswordHash)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrSetupAlreadyCompleted
	}

	return nil
}

// GetAdminUser retrieves the admin user from the database
func (r *UserRepository) GetAdminUser(ctx context.Context) (*User, error) {
	// Add timeout context if not provided (#16)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var user User
	var email sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields
		FROM users
		WHERE username = ?
	`, constants.DefaultUsername).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&email,
		&name,
		&user.Role,
		&user.Status,
		&profilePicture,
		&user.LastLoginAt,
		&customFields,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAdminNotFound
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if name.Valid {
		user.Name = name.String
	}
	if profilePicture.Valid {
		user.ProfilePicture = profilePicture.String
	}
	user.CustomFields = unmarshalCustomFields(customFields)

	return &user, nil
}

// CreateUser creates a new user in the database
func (r *UserRepository) CreateUser(ctx context.Context, user *User) error {
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

	// Sanitize email
	email := strings.TrimSpace(user.Email)
	email = strings.ToLower(email)

	// Set name to username if not provided (display name defaults to username)
	name := user.Name
	if name == "" {
		name = user.Username
	}

	customFieldsJSON, cErr := marshalCustomFields(user.CustomFields)
	if cErr != nil {
		return fmt.Errorf("failed to marshal custom fields: %w", cErr)
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, email, name, role, status, custom_fields)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, user.Username, user.PasswordHash, email, name, user.Role, user.Status, customFieldsJSON)
	if err != nil {
		return err
	}

	// Get the last inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	user.ID = int(id)
	// Update the Name field if it was empty
	if user.Name == "" {
		user.Name = user.Username
	}
	return nil
}

// CheckUsernameExists checks if a username already exists (case-insensitive)
func (r *UserRepository) CheckUsernameExists(ctx context.Context, username string) (bool, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE LOWER(username) = LOWER(?)
		)
	`, username).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// CheckEmailExists checks if an email already exists (case-insensitive)
func (r *UserRepository) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Sanitize email
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE LOWER(email) = LOWER(?)
		)
	`, email).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// UpdateUserStatus updates a user's status
func (r *UserRepository) UpdateUserStatus(ctx context.Context, userID int, status string) error {
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
		UPDATE users
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// UpdateUserStatusIfCurrentStatus atomically updates a user's status only if they currently have the specified status
// This prevents TOCTOU race conditions by performing the check and update in a single atomic operation
func (r *UserRepository) UpdateUserStatusIfCurrentStatus(ctx context.Context, userID int, currentStatus string, newStatus string) error {
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
		UPDATE users
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = ?
	`, newStatus, userID, currentStatus)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d or not in %s status", userID, currentStatus)
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(ctx context.Context, userID int) (*User, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var user User
	var email sql.NullString
	var name sql.NullString
	var createdAt sql.NullString
	var updatedAt sql.NullString
	var profilePicture sql.NullString
	var customFields *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, created_at, updated_at, custom_fields
		FROM users
		WHERE id = ?
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&email,
		&name,
		&user.Role,
		&user.Status,
		&profilePicture,
		&user.LastLoginAt,
		&createdAt,
		&updatedAt,
		&customFields,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found with ID %d", userID)
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if name.Valid {
		user.Name = name.String
	}
	if profilePicture.Valid {
		user.ProfilePicture = profilePicture.String
	}
	if createdAt.Valid {
		user.CreatedAt = createdAt.String
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.String
	}
	user.CustomFields = unmarshalCustomFields(customFields)

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Sanitize email
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	var user User
	var userEmail sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields
		FROM users
		WHERE LOWER(email) = ?
	`, email).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&userEmail,
		&name,
		&user.Role,
		&user.Status,
		&profilePicture,
		&user.LastLoginAt,
		&customFields,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User not found
	}
	if err != nil {
		return nil, err
	}

	if userEmail.Valid {
		user.Email = userEmail.String
	}
	if name.Valid {
		user.Name = name.String
	}
	if profilePicture.Valid {
		user.ProfilePicture = profilePicture.String
	}
	user.CustomFields = unmarshalCustomFields(customFields)

	return &user, nil
}

// GetUserByUsername retrieves a user by username (case-insensitive)
func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var user User
	var email sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields
		FROM users
		WHERE LOWER(username) = LOWER(?)
	`, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&email,
		&name,
		&user.Role,
		&user.Status,
		&profilePicture,
		&user.LastLoginAt,
		&customFields,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User not found
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if name.Valid {
		user.Name = name.String
	}
	if profilePicture.Valid {
		user.ProfilePicture = profilePicture.String
	}
	user.CustomFields = unmarshalCustomFields(customFields)

	return &user, nil
}

// GetPendingUsers retrieves all users with pending status
func (r *UserRepository) GetPendingUsers(ctx context.Context, limit int, offset int) ([]*User, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Set default limit if not specified or invalid
	if limit <= 0 {
		limit = 100 // Default limit
	}
	// Enforce maximum limit to prevent memory issues
	if limit > 1000 {
		limit = 1000
	}

	// Ensure offset is non-negative
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, email, name, profile_picture, created_at, custom_fields
		FROM users
		WHERE status = 'pending'
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		var user User
		var email sql.NullString
		var name sql.NullString
		var profilePicture sql.NullString
		var customFields *string
		err := rows.Scan(&user.ID, &user.Username, &email, &name, &profilePicture, &user.CreatedAt, &customFields)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		if email.Valid {
			user.Email = email.String
		}
		if name.Valid {
			user.Name = name.String
		}
		if profilePicture.Valid {
			user.ProfilePicture = profilePicture.String
		}
		user.CustomFields = unmarshalCustomFields(customFields)
		users = append(users, &user)
	}

	return users, nil
}

// DeleteUser deletes a user by ID
func (r *UserRepository) DeleteUser(ctx context.Context, userID int) error {
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
		DELETE FROM users WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// SuspendUser suspends a user by updating their status to 'suspended'
func (r *UserRepository) SuspendUser(ctx context.Context, userID int) error {
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

	// Update status to suspended
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET status = 'suspended', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// UnsuspendUser unsuspends a user by updating their status to 'active'
func (r *UserRepository) UnsuspendUser(ctx context.Context, userID int) error {
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

	// Update status to verified (spec: suspended → verified)
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET status = 'verified', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to unsuspend user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// SoftDeleteUser soft deletes a user by updating their status to 'soft_deleted'
func (r *UserRepository) SoftDeleteUser(ctx context.Context, userID int) error {
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

	// Update status to soft_deleted
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET status = 'soft_deleted', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// GetAllUsers retrieves all users with optional status filtering and pagination
func (r *UserRepository) GetAllUsers(ctx context.Context, status string, limit int, offset int) ([]*User, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	// Set default limit if not specified or invalid
	if limit <= 0 {
		limit = 100 // Default limit
	}
	// Enforce maximum limit to prevent memory issues
	if limit > 1000 {
		limit = 1000
	}

	// Ensure offset is non-negative
	if offset < 0 {
		offset = 0
	}

	var rows *sql.Rows
	var err error

	// Query with or without status filter
	if status != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, username, email, name, status, role, profile_picture, created_at, custom_fields
			FROM users
			WHERE status = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`, status, limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, username, email, name, status, role, profile_picture, created_at, custom_fields
			FROM users
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		var user User
		var email sql.NullString
		var name sql.NullString
		var profilePicture sql.NullString
		var customFields *string
		err := rows.Scan(&user.ID, &user.Username, &email, &name, &user.Status, &user.Role, &profilePicture, &user.CreatedAt, &customFields)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		if email.Valid {
			user.Email = email.String
		}
		if name.Valid {
			user.Name = name.String
		}
		if profilePicture.Valid {
			user.ProfilePicture = profilePicture.String
		}
		user.CustomFields = unmarshalCustomFields(customFields)
		users = append(users, &user)
	}

	return users, nil
}

// GetUserStatus retrieves the current status of a user
func (r *UserRepository) GetUserStatus(ctx context.Context, userID int) (string, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var status string
	err := r.db.QueryRowContext(ctx, `
		SELECT status FROM users WHERE id = ?
	`, userID).Scan(&status)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found with ID %d", userID)
	}
	if err != nil {
		return "", err
	}

	return status, nil
}

// UpdateEmail updates a user's email address
func (r *UserRepository) UpdateEmail(ctx context.Context, userID int, email string) error {
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

	// Sanitize email
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, email, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// UpdatePassword updates a user's password, validating the current password first
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int, currentPasswordHash, newPasswordHash string) error {
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

	// Update password only if current password matches
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND password_hash = ?
	`, newPasswordHash, userID, currentPasswordHash)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("user not found or current password incorrect")
	}

	return nil
}

// UpdateLastLoginAt updates the last login timestamp for a user
func (r *UserRepository) UpdateLastLoginAt(ctx context.Context, userID int) error {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET last_login_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// UpdatePasswordByUserID updates a user's password by user ID (no current password check)
func (r *UserRepository) UpdatePasswordByUserID(ctx context.Context, userID int, newPasswordHash string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, newPasswordHash, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// UpdateProfile updates a user's profile fields (name, email, role, custom_fields)
func (r *UserRepository) UpdateProfile(
	ctx context.Context,
	userID int,
	name string,
	email string,
	role string,
	customFields map[string]any,
) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	customFieldsJSON, err := marshalCustomFields(customFields)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET name = ?, email = ?, role = ?, custom_fields = COALESCE(?, custom_fields), updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, email, role, customFieldsJSON, userID)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// UpdateCustomFields updates only the custom_fields JSON column for a user
func (r *UserRepository) UpdateCustomFields(
	ctx context.Context,
	userID int,
	customFields map[string]any,
) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	customFieldsJSON, err := marshalCustomFields(customFields)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET custom_fields = COALESCE(?, custom_fields), updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, customFieldsJSON, userID)
	if err != nil {
		return fmt.Errorf("failed to update custom fields: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// CheckEmailExistsForOtherUser checks if an email is already in use by a different user
func (r *UserRepository) CheckEmailExistsForOtherUser(ctx context.Context, userID int, email string) (bool, error) {
	if err := r.db.PingContext(ctx); err != nil {
		return false, fmt.Errorf("database connection lost: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE LOWER(email) = ? AND id != ?
		)
	`, email, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// UpdateProfilePicture updates a user's profile picture filename
func (r *UserRepository) UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET profile_picture = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, profilePicture, userID)
	if err != nil {
		return fmt.Errorf("failed to update profile picture: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// DeleteProfilePicture clears a user's profile picture column
func (r *UserRepository) DeleteProfilePicture(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET profile_picture = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete profile picture: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	return nil
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}
