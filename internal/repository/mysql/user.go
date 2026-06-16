package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/repository"
)

// User is a type alias for repository.User.
type User = repository.User

// UserRepository handles user data operations with MySQL syntax.
// Implements repository.UserRepo.
type UserRepository struct {
	db *sql.DB
}

func scanUserRows(rows *sql.Rows) ([]*repository.User, error) {
	var users []*repository.User
	for rows.Next() {
		var user repository.User
		var email sql.NullString
		var name sql.NullString
		var profilePicture sql.NullString
		var customFields *string

		err := rows.Scan(
			&user.ID, &user.Username,
			&email, &name, &user.Role, &user.Status,
			&profilePicture, &user.CreatedAt, &customFields,
		)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating user rows: %w", err)
	}
	return users, nil
}

// UpdateAdminPasswordAndEmail updates the admin user's password and email.
func (r *UserRepository) UpdateAdminPasswordAndEmail(ctx context.Context, passwordHash, email, currentPasswordHash string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, email = ?, status = 'verified', updated_at = NOW()
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
		return repository.ErrSetupAlreadyCompleted
	}
	return nil
}

// GetAdminUser retrieves the admin user from the database.
func (r *UserRepository) GetAdminUser(ctx context.Context) (*repository.User, error) {
	var user repository.User
	var email sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields
		FROM users
		WHERE username = ?
	`, constants.DefaultUsername).Scan(
		&user.ID, &user.Username, &user.PasswordHash,
		&email, &name, &user.Role, &user.Status,
		&profilePicture, &user.LastLoginAt, &customFields,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrAdminNotFound
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

// CreateUser creates a new user in the database.
func (r *UserRepository) CreateUser(ctx context.Context, user *repository.User) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	email := strings.TrimSpace(user.Email)
	email = strings.ToLower(email)
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
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = int(id)
	if user.Name == "" {
		user.Name = user.Username
	}
	return nil
}

// CheckUsernameExists checks if a username already exists (case-insensitive).
func (r *UserRepository) CheckUsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER(?))
	`, username).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CheckEmailExists checks if an email already exists (case-insensitive).
func (r *UserRepository) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER(?))
	`, email).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// UpdateUserStatus updates a user's status.
func (r *UserRepository) UpdateUserStatus(ctx context.Context, userID int, status string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET status = ?, updated_at = NOW() WHERE id = ?
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

// UpdateUserStatusIfCurrentStatus updates a user's status only if current status matches.
func (r *UserRepository) UpdateUserStatusIfCurrentStatus(ctx context.Context, userID int, currentStatus string, newStatus string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET status = ?, updated_at = NOW()
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
		return fmt.Errorf("user not found with ID %d or status is not %s", userID, currentStatus)
	}
	return nil
}

// GetUserByID retrieves a user by their ID.
func (r *UserRepository) GetUserByID(ctx context.Context, userID int) (*repository.User, error) {
	var user repository.User
	var email sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Username, &user.PasswordHash,
		&email, &name, &user.Role, &user.Status,
		&profilePicture, &user.LastLoginAt, &customFields,
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
	user.CustomFields = unmarshalCustomFields(customFields)
	return &user, nil
}

// GetUserByEmail retrieves a user by their email.
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*repository.User, error) {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	var user repository.User
	var emailOut sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields
		FROM users WHERE LOWER(email) = LOWER(?)
	`, email).Scan(
		&user.ID, &user.Username, &user.PasswordHash,
		&emailOut, &name, &user.Role, &user.Status,
		&profilePicture, &user.LastLoginAt, &customFields,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if emailOut.Valid {
		user.Email = emailOut.String
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

// GetUserByUsername retrieves a user by their username.
func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*repository.User, error) {
	var user repository.User
	var email sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields
		FROM users WHERE username = ?
	`, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash,
		&email, &name, &user.Role, &user.Status,
		&profilePicture, &user.LastLoginAt, &customFields,
	)
	if err == sql.ErrNoRows {
		return nil, nil
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

// GetPendingUsers returns all users with 'pending' status.
func (r *UserRepository) GetPendingUsers(ctx context.Context, limit int, offset int) ([]*repository.User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, email, name, role, status, profile_picture, created_at, custom_fields
		FROM users WHERE status = 'pending'
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanUserRows(rows)
}

// DeleteUser deletes a user by ID.
func (r *UserRepository) DeleteUser(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
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

// SuspendUser sets a user's status to 'suspended'.
func (r *UserRepository) SuspendUser(ctx context.Context, userID int) error {
	return r.UpdateUserStatus(ctx, userID, "suspended")
}

// UnsuspendUser sets a user's status back to 'verified'.
func (r *UserRepository) UnsuspendUser(ctx context.Context, userID int) error {
	return r.UpdateUserStatus(ctx, userID, "verified")
}

// SoftDeleteUser sets a user's status to 'soft_deleted' (soft delete).
func (r *UserRepository) SoftDeleteUser(ctx context.Context, userID int) error {
	return r.UpdateUserStatus(ctx, userID, "soft_deleted")
}

// GetAllUsers returns paginated users, optionally filtered by status.
func (r *UserRepository) GetAllUsers(ctx context.Context, status string, limit int, offset int) ([]*repository.User, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, username, email, name, role, status, profile_picture, created_at, custom_fields
			FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?
		`, limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, username, email, name, role, status, profile_picture, created_at, custom_fields
			FROM users WHERE status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
		`, status, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanUserRows(rows)
}

// GetUserStatus returns the status of a user.
func (r *UserRepository) GetUserStatus(ctx context.Context, userID int) (string, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, userID).Scan(&status)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found with ID %d", userID)
	}
	if err != nil {
		return "", err
	}
	return status, nil
}

// UpdateEmail updates a user's email.
func (r *UserRepository) UpdateEmail(ctx context.Context, userID int, email string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET email = ?, updated_at = NOW() WHERE id = ?
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

// UpdatePassword updates user's password with current password verification.
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int, currentPasswordHash, newPasswordHash string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = NOW()
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

// UpdateLastLoginAt updates the last login timestamp.
func (r *UserRepository) UpdateLastLoginAt(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET last_login_at = NOW() WHERE id = ?
	`, userID)
	return err
}

// UpdatePasswordByUserID updates password without current password verification.
func (r *UserRepository) UpdatePasswordByUserID(ctx context.Context, userID int, newPasswordHash string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = NOW() WHERE id = ?
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

// UpdateProfile updates a user's profile fields.
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
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	customFieldsJSON, err := marshalCustomFields(customFields)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET name = ?, email = ?, role = ?, custom_fields = COALESCE(?, custom_fields), updated_at = NOW()
		WHERE id = ?
	`, name, email, role, customFieldsJSON, userID)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
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

// UpdateCustomFields updates only the custom_fields JSON column.
func (r *UserRepository) UpdateCustomFields(ctx context.Context, userID int, customFields map[string]any) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	customFieldsJSON, err := marshalCustomFields(customFields)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET custom_fields = COALESCE(?, custom_fields), updated_at = NOW() WHERE id = ?
	`, customFieldsJSON, userID)
	if err != nil {
		return fmt.Errorf("failed to update custom fields: %w", err)
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

// CheckEmailExistsForOtherUser checks if an email is already used by another user.
func (r *UserRepository) CheckEmailExistsForOtherUser(ctx context.Context, userID int, email string) (bool, error) {
	if err := r.db.PingContext(ctx); err != nil {
		return false, fmt.Errorf("database connection lost: %w", err)
	}
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER(?) AND id != ?)
	`, email, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// UpdateProfilePicture updates a user's profile picture.
func (r *UserRepository) UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET profile_picture = ?, updated_at = NOW() WHERE id = ?
	`, profilePicture, userID)
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

// DeleteProfilePicture clears a user's profile picture.
func (r *UserRepository) DeleteProfilePicture(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET profile_picture = NULL, updated_at = NOW() WHERE id = ?
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

// NewUserRepository creates a new user repository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}
