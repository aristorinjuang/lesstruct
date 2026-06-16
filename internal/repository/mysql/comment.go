package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

func mapCommentItemToDomain(item *CommentItem) *contentdomain.Comment {
	comment := &contentdomain.Comment{
		ID:        item.ID,
		ContentID: item.ContentID,
		UserID:    item.UserID,
		Comment:   item.Comment,
		Status:    contentdomain.CommentStatus(item.Status),
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
	if item.Author.Valid {
		comment.Author = item.Author.String
	}
	if item.Username.Valid {
		comment.Username = item.Username.String
	}
	if item.Role.Valid {
		comment.Role = item.Role.String
	}
	return comment
}

func scanCommentRows(rows *sql.Rows) ([]*contentdomain.Comment, error) {
	var items []*contentdomain.Comment
	for rows.Next() {
		var item CommentItem

		err := rows.Scan(
			&item.ID,
			&item.ContentID,
			&item.UserID,
			&item.Comment,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Author,
			&item.Username,
			&item.Role,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}

		items = append(items, mapCommentItemToDomain(&item))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating comment rows: %w", err)
	}
	return items, nil
}

// CommentItem represents a comment in the database
type CommentItem struct {
	ID        int            `json:"id"`
	ContentID int            `json:"contentId"`
	UserID    int            `json:"userId"`
	Comment   string         `json:"comment"`
	Status    string         `json:"status"`
	Author    sql.NullString `json:"author"`
	Username  sql.NullString `json:"username"`
	Role      sql.NullString `json:"role"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// CommentRepository handles comment data operations
type CommentRepository struct {
	db *sql.DB
}

func (r *CommentRepository) Create(ctx context.Context, comment *contentdomain.Comment) error {

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO comments (content_id, user_id, comment, status)
		VALUES (?, ?, ?, ?)
	`, comment.ContentID, comment.UserID, comment.Comment, comment.Status)
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	comment.ID = int(id)

	return nil
}

func (r *CommentRepository) GetByContentID(ctx context.Context, contentID int) ([]*contentdomain.Comment, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.content_id = ? AND c.status = ?
		ORDER BY c.created_at ASC
	`, contentID, contentdomain.CommentStatusApproved)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by content id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanCommentRows(rows)
}

func (r *CommentRepository) GetByContentIDForModeration(ctx context.Context, contentID int) ([]*contentdomain.Comment, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.content_id = ?
		ORDER BY c.created_at DESC
	`, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for moderation: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanCommentRows(rows)
}

func (r *CommentRepository) GetByID(ctx context.Context, id int) (*contentdomain.Comment, error) {

	var item CommentItem

	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.id = ?
	`, id).Scan(
		&item.ID,
		&item.ContentID,
		&item.UserID,
		&item.Comment,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Author,
		&item.Username,
		&item.Role,
	)

	if err == sql.ErrNoRows {
		return nil, contentdomain.ErrCommentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get comment by id: %w", err)
	}

	comment := mapCommentItemToDomain(&item)
	return comment, nil
}

func (r *CommentRepository) GetByUserID(ctx context.Context, userID int) ([]*contentdomain.Comment, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by user id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanCommentRows(rows)
}

func (r *CommentRepository) UpdateStatus(ctx context.Context, id int, status contentdomain.CommentStatus) error {

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE comments
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, id)
	if err != nil {
		return fmt.Errorf("failed to update comment status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return contentdomain.ErrCommentNotFound
	}

	return nil
}

func (r *CommentRepository) Delete(ctx context.Context, id int) error {

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		DELETE FROM comments WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return contentdomain.ErrCommentNotFound
	}

	return nil
}

func (r *CommentRepository) DeleteByUserID(ctx context.Context, userID int) error {

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	_, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete comments by user id: %w", err)
	}

	return nil
}

// NewCommentRepository creates a new comment repository
func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{
		db: db,
	}
}
