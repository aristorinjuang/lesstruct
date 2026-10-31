package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
)

// UserDataExportRepo defines the interface for user data export repository operations
type UserDataExportRepo interface {
	GetUserContent(ctx context.Context, userID int) ([]*userdomain.UserContentItem, error)
	GetUserComments(ctx context.Context, userID int) ([]*userdomain.UserCommentItem, error)
	GetUserMedia(ctx context.Context, userID int) ([]*userdomain.UserMediaItem, error)
	GetUserDataForExport(ctx context.Context, userID int) (*userdomain.UserDataExport, error)
}

// UserDataExportRepository handles user data export operations
type UserDataExportRepository struct {
	db *sql.DB
}

// getUserForExport retrieves user information for export
func (r *UserDataExportRepository) getUserForExport(ctx context.Context, userID int) (*userdomain.User, error) {
	var user userdomain.User
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, email, role, created_at, last_login_at
		FROM users
		WHERE id = ?
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.LastLoginAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found with ID %d", userID)
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserContent retrieves all content items (posts/pages) created by a user
func (r *UserDataExportRepository) GetUserContent(ctx context.Context, userID int) ([]*userdomain.UserContentItem, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, slug, content, status, created_at, updated_at
		FROM content_items
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*userdomain.UserContentItem{}, nil
		}
		return nil, fmt.Errorf("failed to get user content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var contentItems []*userdomain.UserContentItem
	for rows.Next() {
		var item userdomain.UserContentItem
		err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Slug,
			&item.Content,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan content item: %w", err)
		}
		contentItems = append(contentItems, &item)
	}

	return contentItems, nil
}

// GetUserComments retrieves all comments posted by a user
func (r *UserDataExportRepository) GetUserComments(ctx context.Context, userID int) ([]*userdomain.UserCommentItem, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, content_id, comment, created_at, updated_at
		FROM comments
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*userdomain.UserCommentItem{}, nil
		}
		return nil, fmt.Errorf("failed to get user comments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var commentItems []*userdomain.UserCommentItem
	for rows.Next() {
		var item userdomain.UserCommentItem
		err := rows.Scan(
			&item.ID,
			&item.ContentItemID,
			&item.Content,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment item: %w", err)
		}
		commentItems = append(commentItems, &item)
	}

	return commentItems, nil
}

// GetUserMedia retrieves all media files uploaded by a user
func (r *UserDataExportRepository) GetUserMedia(ctx context.Context, userID int) ([]*userdomain.UserMediaItem, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, filename, original_filename, file_path, file_size, mime_type, created_at
		FROM media_files
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*userdomain.UserMediaItem{}, nil
		}
		return nil, fmt.Errorf("failed to get user media: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var mediaItems []*userdomain.UserMediaItem
	for rows.Next() {
		var item userdomain.UserMediaItem
		err := rows.Scan(
			&item.ID,
			&item.Filename,
			&item.OriginalFilename,
			&item.FilePath,
			&item.FileSize,
			&item.MimeType,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media item: %w", err)
		}
		mediaItems = append(mediaItems, &item)
	}

	return mediaItems, nil
}

// GetUserDataForExport aggregates all user data for export
func (r *UserDataExportRepository) GetUserDataForExport(ctx context.Context, userID int) (*userdomain.UserDataExport, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	// Get user information
	user, err := r.getUserForExport(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get user content
	content, err := r.GetUserContent(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user content: %w", err)
	}

	// Get user comments
	comments, err := r.GetUserComments(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user comments: %w", err)
	}

	// Get user media
	media, err := r.GetUserMedia(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user media: %w", err)
	}

	return &userdomain.UserDataExport{
		User:       user,
		Content:    content,
		Comments:   comments,
		Media:      media,
		ExportDate: time.Now().UTC(),
	}, nil
}

// NewUserDataExportRepository creates a new user data export repository
func NewUserDataExportRepository(db *sql.DB) *UserDataExportRepository {
	return &UserDataExportRepository{
		db: db,
	}
}
