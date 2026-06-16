package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// UserDeletionRepo defines the interface for user deletion operations
type UserDeletionRepo interface {
	DeleteUserAccount(ctx context.Context, userID int) error
	DeleteUserTokens(ctx context.Context, userID int) error
	DeleteUserContent(ctx context.Context, userID int) error
	DeleteUserComments(ctx context.Context, userID int) error
	DeleteUserMedia(ctx context.Context, userID int) error
	CountUsersByRoleAndStatus(ctx context.Context, role, status string) (int, error)
	DeleteAllUserData(ctx context.Context, userID int) error
}

// UserDeletionRepository handles user deletion data operations
type UserDeletionRepository struct {
	db *sql.DB
}

// DeleteAllUserData performs a hard delete of all user data within a single transaction
func (r *UserDeletionRepository) DeleteAllUserData(ctx context.Context, userID int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on error
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete user content (placeholder for future functionality)
	if _, err = tx.ExecContext(ctx, `DELETE FROM content_items WHERE user_id = ?`, userID); err != nil {
		// Table may not exist yet; ignore error for placeholder
		_ = err
	}

	// Delete user comments (placeholder for future functionality)
	if _, err = tx.ExecContext(ctx, `DELETE FROM comments WHERE user_id = ?`, userID); err != nil {
		// Table may not exist yet; ignore error for placeholder
		_ = err
	}

	// Delete user media (placeholder for future functionality)
	if _, err = tx.ExecContext(ctx, `DELETE FROM media_files WHERE uploaded_by_user_id = ?`, userID); err != nil {
		// Table may not exist yet; ignore error for placeholder
		_ = err
	}

	// Delete verification tokens
	if _, err = tx.ExecContext(ctx, `DELETE FROM verification_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete verification tokens: %w", err)
	}

	// Delete email update tokens
	if _, err = tx.ExecContext(ctx, `DELETE FROM email_update_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete email update tokens: %w", err)
	}

	// Delete failed login attempts
	if _, err = tx.ExecContext(ctx, `DELETE FROM failed_login_attempts WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete failed login attempts: %w", err)
	}

	// Delete user account (hard delete)
	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user account: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found with ID %d", userID)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteUserAccount performs a hard delete of a user account
func (r *UserDeletionRepository) DeleteUserAccount(ctx context.Context, userID int) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user account: %w", err)
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

// DeleteUserTokens deletes all tokens associated with a user
func (r *UserDeletionRepository) DeleteUserTokens(ctx context.Context, userID int) error {
	// Delete verification tokens
	if _, err := r.db.ExecContext(ctx, `DELETE FROM verification_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete verification tokens: %w", err)
	}

	// Delete email update tokens
	if _, err := r.db.ExecContext(ctx, `DELETE FROM email_update_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete email update tokens: %w", err)
	}

	// Delete failed login attempts
	if _, err := r.db.ExecContext(ctx, `DELETE FROM failed_login_attempts WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete failed login attempts: %w", err)
	}

	return nil
}

// DeleteUserContent deletes all content (posts, pages) created by a user
// Note: This is a placeholder for future functionality
func (r *UserDeletionRepository) DeleteUserContent(ctx context.Context, userID int) error {
	// Placeholder for future content deletion functionality
	// This will be implemented when the content_items table is created
	return nil
}

// DeleteUserComments deletes all comments posted by a user
// Note: This is a placeholder for future functionality
func (r *UserDeletionRepository) DeleteUserComments(ctx context.Context, userID int) error {
	// Placeholder for future comment deletion functionality
	// This will be implemented when the comments table is created
	return nil
}

// DeleteUserMedia deletes all media files uploaded by a user
// Note: This is a placeholder for future functionality
func (r *UserDeletionRepository) DeleteUserMedia(ctx context.Context, userID int) error {
	// Placeholder for future media deletion functionality
	// This will be implemented when the media_files table is created
	// When implemented, this should:
	// 1. Get all media files for the user
	// 2. Delete physical files from filesystem using os.Remove()
	// 3. Delete database records
	return nil
}

// CountUsersByRoleAndStatus counts users by role and status
func (r *UserDeletionRepository) CountUsersByRoleAndStatus(ctx context.Context, role, status string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE role = ? AND status = ?
	`, role, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users by role and status: %w", err)
	}

	return count, nil
}

// NewUserDeletionRepository creates a new user deletion repository
func NewUserDeletionRepository(db *sql.DB) *UserDeletionRepository {
	return &UserDeletionRepository{
		db: db,
	}
}
