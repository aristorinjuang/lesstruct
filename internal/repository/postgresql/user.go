package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/constants"
	"github.com/aristorinjuang/lesstruct/internal/repository"
)

func unmarshalCustomFields(raw *string) map[string]any {
	if raw == nil || *raw == "" {
		return nil
	}
	var fields map[string]any
	_ = json.Unmarshal([]byte(*raw), &fields)
	return fields
}

// User is a type alias for repository.User.
type User = repository.User

// UserRepository handles user data operations with PostgreSQL syntax.
// Implements repository.UserRepo.
type UserRepository struct {
	db *sql.DB
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
		SET password_hash = $1,
		    email = $2,
		    status = 'verified',
		    updated_at = NOW()
		WHERE username = $3 AND password_hash = $4
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
		WHERE username = $1
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

	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (username, password_hash, email, name, role, status, custom_fields)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, user.Username, user.PasswordHash, email, name, user.Role, user.Status, customFieldsJSON).Scan(&user.ID)
	if err != nil {
		return err
	}

	if user.Name == "" {
		user.Name = user.Username
	}
	return nil
}

// CheckUsernameExists checks if a username already exists (case-insensitive).
func (r *UserRepository) CheckUsernameExists(ctx context.Context, username string) (bool, error) {

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE LOWER(username) = LOWER($1)
		)
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
		SELECT EXISTS(
			SELECT 1 FROM users WHERE LOWER(email) = LOWER($1)
		)
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
		UPDATE users
		SET status = $1, updated_at = NOW()
		WHERE id = $2
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

// UpdateUserStatusIfCurrentStatus updates a user's status only if it matches the expected current status.
func (r *UserRepository) UpdateUserStatusIfCurrentStatus(ctx context.Context, userID int, currentStatus string, newStatus string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status = $3
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
		SELECT id, username, email, name, role, status, profile_picture, last_login_at, custom_fields, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&email,
		&name,
		&user.Role,
		&user.Status,
		&profilePicture,
		&user.LastLoginAt,
		&customFields,
		&user.CreatedAt,
		&user.UpdatedAt,
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

// GetUserByEmail retrieves a user by their email (case-insensitive).
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*repository.User, error) {

	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	var user repository.User
	var dbEmail sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, email, name, role, status, profile_picture, last_login_at, custom_fields, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`, email).Scan(
		&user.ID,
		&user.Username,
		&dbEmail,
		&name,
		&user.Role,
		&user.Status,
		&profilePicture,
		&user.LastLoginAt,
		&customFields,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if dbEmail.Valid {
		user.Email = dbEmail.String
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

// GetUserByUsername retrieves a user by their username (case-insensitive).
func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*repository.User, error) {

	var user repository.User
	var email sql.NullString
	var name sql.NullString
	var profilePicture sql.NullString
	var customFields *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, email, name, role, status, profile_picture, last_login_at, custom_fields, created_at, updated_at
		FROM users
		WHERE LOWER(username) = LOWER($1)
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
		&user.CreatedAt,
		&user.UpdatedAt,
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

// GetPendingUsers retrieves pending users with pagination.
func (r *UserRepository) GetPendingUsers(ctx context.Context, limit int, offset int) ([]*repository.User, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, email, name, role, status, profile_picture, created_at, custom_fields
		FROM users
		WHERE status = 'pending'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*repository.User
	for rows.Next() {
		var user repository.User
		var email sql.NullString
		var name sql.NullString
		var profilePicture sql.NullString
		var customFields *string
		err := rows.Scan(&user.ID, &user.Username, &email, &name, &user.Role, &user.Status, &profilePicture, &user.CreatedAt, &customFields)
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

// DeleteUser deletes a user by ID.
func (r *UserRepository) DeleteUser(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		DELETE FROM users WHERE id = $1
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

// SuspendUser suspends a user.
func (r *UserRepository) SuspendUser(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET status = 'suspended', updated_at = NOW()
		WHERE id = $1
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

// UnsuspendUser unsuspends a user.
func (r *UserRepository) UnsuspendUser(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET status = 'active', updated_at = NOW()
		WHERE id = $1
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

// SoftDeleteUser soft-deletes a user.
func (r *UserRepository) SoftDeleteUser(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET status = 'soft_deleted', updated_at = NOW()
		WHERE id = $1
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

// GetAllUsers retrieves all users with optional status filtering and pagination.
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

	if status != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, username, email, name, status, role, profile_picture, created_at, custom_fields
			FROM users
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`, status, limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, username, email, name, status, role, profile_picture, created_at, custom_fields
			FROM users
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*repository.User
	for rows.Next() {
		var user repository.User
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

// GetUserStatus retrieves the current status of a user.
func (r *UserRepository) GetUserStatus(ctx context.Context, userID int) (string, error) {

	var status string
	err := r.db.QueryRowContext(ctx, `
		SELECT status FROM users WHERE id = $1
	`, userID).Scan(&status)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found with ID %d", userID)
	}
	if err != nil {
		return "", err
	}

	return status, nil
}

// UpdateEmail updates a user's email address.
func (r *UserRepository) UpdateEmail(ctx context.Context, userID int, email string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email = $1, updated_at = NOW()
		WHERE id = $2
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

// UpdatePassword updates a user's password, validating the current password first.
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int, currentPasswordHash, newPasswordHash string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2 AND password_hash = $3
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

// UpdateLastLoginAt updates the last login timestamp for a user.
func (r *UserRepository) UpdateLastLoginAt(ctx context.Context, userID int) error {

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET last_login_at = NOW()
		WHERE id = $1
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

// UpdatePasswordByUserID updates a user's password by user ID (no current password check).
func (r *UserRepository) UpdatePasswordByUserID(ctx context.Context, userID int, newPasswordHash string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2
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

// UpdateProfilePicture updates a user's profile picture filename.
func (r *UserRepository) UpdateProfilePicture(ctx context.Context, userID int, profilePicture string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET profile_picture = $1, updated_at = NOW()
		WHERE id = $2
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

// DeleteProfilePicture clears a user's profile picture column.
func (r *UserRepository) DeleteProfilePicture(ctx context.Context, userID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}


	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET profile_picture = NULL, updated_at = NOW()
		WHERE id = $1
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

// NewUserRepository creates a new user repository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}
