// Package store is the single-source comment repository: one implementation
// against *sql.DB plus a kernel Dialect, replacing the three per-driver
// copies. Placeholder rendering and RETURNING support come from the dialect;
// everything else (columns, scans, messages, guards) ports the SQLite
// implementation verbatim, which the moved test suite pins.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
)

func mapCommentItemToDomain(item *CommentItem) *commentdomain.Comment {
	comment := &commentdomain.Comment{
		ID:        item.ID,
		ContentID: item.ContentID,
		UserID:    item.UserID,
		Comment:   item.Comment,
		Status:    commentdomain.CommentStatus(item.Status),
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
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
	if item.ContentTitle.Valid {
		comment.ContentTitle = item.ContentTitle.String
	}
	if item.ContentSlug.Valid {
		comment.ContentSlug = item.ContentSlug.String
	}
	return comment
}

func scanCommentRows(rows *sql.Rows) ([]*commentdomain.Comment, error) {
	var items []*commentdomain.Comment
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

// Repository implements commentdomain.CommentRepository for every driver.
type Repository struct {
	db      *sql.DB
	dialect kerneldatabase.Dialect
}

// placeholders returns a positional placeholder generator: each call renders
// the next bind marker ($1, $2… on PostgreSQL; ? elsewhere), so call order
// matches argument order by construction.
func (r *Repository) placeholders() func() string {
	index := 0
	return func() string {
		marker := r.dialect.Placeholder(index)
		index++
		return marker
	}
}

// CommentItem represents a comment in the database
type CommentItem struct {
	ID           int            `json:"id"`
	ContentID    int            `json:"contentId"`
	UserID       int            `json:"userId"`
	Comment      string         `json:"comment"`
	Status       string         `json:"status"`
	Author       sql.NullString `json:"author"`
	Username     sql.NullString `json:"username"`
	Role         sql.NullString `json:"role"`
	ContentTitle sql.NullString `json:"contentTitle"`
	ContentSlug  sql.NullString `json:"contentSlug"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

func (r *Repository) Create(ctx context.Context, comment *commentdomain.Comment) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	next := r.placeholders()
	if r.dialect.SupportsReturning() {
		var createdAt, updatedAt time.Time
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO comments (content_id, user_id, comment, status)
			VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`)
			RETURNING id, created_at, updated_at
		`, comment.ContentID, comment.UserID, comment.Comment, comment.Status).Scan(&comment.ID, &createdAt, &updatedAt)
		if err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}

		comment.CreatedAt = createdAt
		comment.UpdatedAt = updatedAt

		return nil
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO comments (content_id, user_id, comment, status)
		VALUES (`+next()+`, `+next()+`, `+next()+`, `+next()+`)
	`, comment.ContentID, comment.UserID, comment.Comment, comment.Status)
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	comment.ID = int(id)

	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(ctx, `
		SELECT created_at, updated_at FROM comments WHERE id = `+next()+`
	`, comment.ID).Scan(&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to get timestamps: %w", err)
	}

	comment.CreatedAt = createdAt
	comment.UpdatedAt = updatedAt

	return nil
}

func (r *Repository) GetByContentID(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.content_id = `+next()+` AND c.status = `+next()+`
		ORDER BY c.created_at ASC
	`, contentID, commentdomain.CommentStatusApproved)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by content id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanCommentRows(rows)
}

func (r *Repository) GetByContentIDForModeration(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.content_id = `+next()+`
		ORDER BY c.created_at DESC
	`, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for moderation: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanCommentRows(rows)
}

func (r *Repository) GetByID(ctx context.Context, id int) (*commentdomain.Comment, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var item CommentItem

	next := r.placeholders()
	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.id = `+next()+`
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
		return nil, commentdomain.ErrCommentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get comment by id: %w", err)
	}

	comment := mapCommentItemToDomain(&item)
	return comment, nil
}

func (r *Repository) GetByUserID(ctx context.Context, userID int) ([]*commentdomain.Comment, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username, u.role
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.user_id = `+next()+`
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by user id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanCommentRows(rows)
}

func (r *Repository) GetByStatus(ctx context.Context, status commentdomain.CommentStatus) ([]*commentdomain.Comment, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.content_id, c.user_id, c.comment, c.status, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) AS author, u.username, u.role,
		       ci.title, ci.slug
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN content_items ci ON c.content_id = ci.id
		WHERE c.status = `+next()+`
		ORDER BY c.created_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*commentdomain.Comment
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
			&item.ContentTitle,
			&item.ContentSlug,
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

func (r *Repository) Count(ctx context.Context, userID int) (int, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return 0, fmt.Errorf("database connection lost: %w", err)
	}

	next := r.placeholders()
	var total int
	var err error
	if userID > 0 {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE user_id = `+next(), userID).Scan(&total)
	} else {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments`).Scan(&total)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to count comments: %w", err)
	}
	return total, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int, status commentdomain.CommentStatus) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	next := r.placeholders()
	result, err := r.db.ExecContext(ctx, `
		UPDATE comments
		SET status = `+next()+`, updated_at = CURRENT_TIMESTAMP
		WHERE id = `+next()+`
	`, status, id)
	if err != nil {
		return fmt.Errorf("failed to update comment status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return commentdomain.ErrCommentNotFound
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, id int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	next := r.placeholders()
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM comments WHERE id = `+next()+`
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return commentdomain.ErrCommentNotFound
	}

	return nil
}

func (r *Repository) DeleteByUserID(ctx context.Context, userID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	next := r.placeholders()
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM comments WHERE user_id = `+next()+`
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete comments by user id: %w", err)
	}

	return nil
}

// New creates a repository on db speaking dialect. Dialect must be non-nil;
// the composition root resolves it from the driver registry.
func New(db *sql.DB, dialect kerneldatabase.Dialect) *Repository {
	return &Repository{
		db:      db,
		dialect: dialect,
	}
}

var _ commentdomain.CommentRepository = (*Repository)(nil)
