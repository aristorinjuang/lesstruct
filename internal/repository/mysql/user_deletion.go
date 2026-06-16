package mysql

import (
	"context"
	"database/sql"
	"fmt"
)

// UserDeletionRepository handles user deletion data operations
type UserDeletionRepository struct {
	db *sql.DB
}

// DeleteAllUserData performs a hard delete of all user data within a single transaction
func (r *UserDeletionRepository) DeleteAllUserData(
	ctx context.Context,
	userID int,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM content_items WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete user content: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM comments WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete user comments: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM media_files WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete user media: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM verification_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete verification tokens: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM email_update_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete email update tokens: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete password reset tokens: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM failed_login_attempts WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete failed login attempts: %w", err)
	}
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
func (r *UserDeletionRepository) DeleteUserAccount(
	ctx context.Context,
	userID int,
) error {
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
func (r *UserDeletionRepository) DeleteUserTokens(
	ctx context.Context,
	userID int,
) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM verification_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete verification tokens: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM email_update_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete email update tokens: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete password reset tokens: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM failed_login_attempts WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("failed to delete failed login attempts: %w", err)
	}
	return nil
}

// DeleteUserContent deletes all content created by a user
func (r *UserDeletionRepository) DeleteUserContent(
	ctx context.Context,
	userID int,
) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM content_items WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user content: %w", err)
	}
	return nil
}

// DeleteUserComments deletes all comments posted by a user
func (r *UserDeletionRepository) DeleteUserComments(
	ctx context.Context,
	userID int,
) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user comments: %w", err)
	}
	return nil
}

// DeleteUserMedia deletes all media files uploaded by a user
func (r *UserDeletionRepository) DeleteUserMedia(
	ctx context.Context,
	userID int,
) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM media_files WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user media: %w", err)
	}
	return nil
}

// CountUsersByRoleAndStatus counts users by role and status
func (r *UserDeletionRepository) CountUsersByRoleAndStatus(
	ctx context.Context,
	role string,
	status string,
) (int, error) {
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
