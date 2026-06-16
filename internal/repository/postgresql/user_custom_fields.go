package postgresql

import (
	"context"
	"fmt"
	"strings"
)

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


	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	customFieldsJSON, err := marshalCustomFields(customFields)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET name = $1, email = $2, role = $3, custom_fields = COALESCE($4, custom_fields), updated_at = NOW()
		WHERE id = $5
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


	customFieldsJSON, err := marshalCustomFields(customFields)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET custom_fields = COALESCE($1, custom_fields), updated_at = NOW()
		WHERE id = $2
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


	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE LOWER(email) = $1 AND id != $2
		)
	`, email, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
