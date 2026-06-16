package postgresql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

type SoftDeletedContent = repository.SoftDeletedContent

// SoftDeleteRepository handles soft delete data operations
type SoftDeleteRepository struct {
	db *sql.DB
}

// SoftDeleteContent marks content as deleted
func (r *SoftDeleteRepository) SoftDeleteContent(ctx context.Context, contentType string, contentID int, userID int, deletedBy int, reason string) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO soft_deleted_content (content_type, content_id, user_id, deleted_by, reason, is_permanent)
		VALUES ($1, $2, $3, $4, $5, FALSE)
	`, contentType, contentID, userID, deletedBy, reasonPtr)
	if err != nil {
		return fmt.Errorf("failed to soft delete content: %w", err)
	}
	return nil
}

// GetSoftDeletedContentByUser retrieves all soft deleted content for a user
func (r *SoftDeleteRepository) GetSoftDeletedContentByUser(ctx context.Context, userID int) ([]*SoftDeletedContent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sdc.id, sdc.content_type, sdc.content_id, sdc.user_id, sdc.deleted_at, sdc.deleted_by, sdc.reason, sdc.is_permanent
		FROM soft_deleted_content sdc
		WHERE sdc.user_id = $1
		ORDER BY sdc.deleted_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get soft deleted content: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var contentList []*SoftDeletedContent
	for rows.Next() {
		var content SoftDeletedContent
		err := rows.Scan(
			&content.ID,
			&content.ContentType,
			&content.ContentID,
			&content.UserID,
			&content.DeletedAt,
			&content.DeletedBy,
			&content.Reason,
			&content.IsPermanent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan soft deleted content: %w", err)
		}
		contentList = append(contentList, &content)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating soft deleted content rows: %w", err)
	}
	return contentList, nil
}

// GetSoftDeletedContentByID retrieves a soft deleted content item by its ID
func (r *SoftDeleteRepository) GetSoftDeletedContentByID(ctx context.Context, id int) (*SoftDeletedContent, error) {
	var content SoftDeletedContent
	err := r.db.QueryRowContext(ctx, `
		SELECT id, content_type, content_id, user_id, deleted_at, deleted_by, reason, is_permanent
		FROM soft_deleted_content
		WHERE id = $1
	`, id).Scan(
		&content.ID,
		&content.ContentType,
		&content.ContentID,
		&content.UserID,
		&content.DeletedAt,
		&content.DeletedBy,
		&content.Reason,
		&content.IsPermanent,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("soft deleted content not found")
		}
		return nil, fmt.Errorf("failed to get soft deleted content: %w", err)
	}
	return &content, nil
}

// RestoreContent restores soft deleted content by removing it from the soft delete table
func (r *SoftDeleteRepository) RestoreContent(ctx context.Context, contentType string, contentID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM soft_deleted_content
		WHERE content_type = $1 AND content_id = $2
	`, contentType, contentID)
	if err != nil {
		return fmt.Errorf("failed to restore content: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("content not found in soft delete table")
	}
	return nil
}

// PermanentDeleteContent permanently deletes content
func (r *SoftDeleteRepository) PermanentDeleteContent(ctx context.Context, contentType string, contentID int) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM soft_deleted_content
		WHERE content_type = $1 AND content_id = $2
	`, contentType, contentID)
	if err != nil {
		return fmt.Errorf("failed to permanently delete content: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("content not found in soft delete table")
	}
	return nil
}

// NewSoftDeleteRepository creates a new soft delete repository
func NewSoftDeleteRepository(db *sql.DB) *SoftDeleteRepository {
	return &SoftDeleteRepository{
		db: db,
	}
}
