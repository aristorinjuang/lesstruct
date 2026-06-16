package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserDataExportRepo defines the interface for user data export repository operations
type UserDataExportRepo interface {
	GetUserContent(ctx context.Context, userID int) ([]*UserContentItem, error)
	GetUserComments(ctx context.Context, userID int) ([]*UserCommentItem, error)
	GetUserMedia(ctx context.Context, userID int) ([]*UserMediaItem, error)
	GetUserDataForExport(ctx context.Context, userID int) (*UserDataExport, error)
}

// UserContentItem represents a content item (post/page) created by a user
type UserContentItem struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// UserCommentItem represents a comment posted by a user
type UserCommentItem struct {
	ID            int    `json:"id"`
	ContentItemID int    `json:"contentItemId"`
	Content       string `json:"content"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// UserMediaItem represents a media file uploaded by a user
type UserMediaItem struct {
	ID               int    `json:"id"`
	Filename         string `json:"filename"`
	OriginalFilename string `json:"originalFilename"`
	FilePath         string `json:"filePath"`
	FileSize         int    `json:"fileSize"`
	MimeType         string `json:"mimeType"`
	CreatedAt        string `json:"createdAt"`
}

// UserDataExport represents all user data for export
type UserDataExport struct {
	ExportDate string             `json:"exportDate"`
	User       *User              `json:"user"`
	Content    []*UserContentItem `json:"content"`
	Comments   []*UserCommentItem `json:"comments"`
	Media      []*UserMediaItem   `json:"media"`
}

// UserDataExportRepository handles user data export operations
type UserDataExportRepository struct {
	db *sql.DB
}

// GetUserContent retrieves all content items (posts/pages) created by a user
func (r *UserDataExportRepository) GetUserContent(ctx context.Context, userID int) ([]*UserContentItem, error) {
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
			return []*UserContentItem{}, nil
		}
		return nil, fmt.Errorf("failed to get user content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var contentItems []*UserContentItem
	for rows.Next() {
		var item UserContentItem
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
func (r *UserDataExportRepository) GetUserComments(ctx context.Context, userID int) ([]*UserCommentItem, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, content_item_id, content, created_at, updated_at
		FROM comments
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*UserCommentItem{}, nil
		}
		return nil, fmt.Errorf("failed to get user comments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var commentItems []*UserCommentItem
	for rows.Next() {
		var item UserCommentItem
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
func (r *UserDataExportRepository) GetUserMedia(ctx context.Context, userID int) ([]*UserMediaItem, error) {
	// Add timeout context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, filename, original_filename, file_path, file_size, mime_type, created_at
		FROM media_files
		WHERE uploaded_by_user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*UserMediaItem{}, nil
		}
		return nil, fmt.Errorf("failed to get user media: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var mediaItems []*UserMediaItem
	for rows.Next() {
		var item UserMediaItem
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
func (r *UserDataExportRepository) GetUserDataForExport(ctx context.Context, userID int) (*UserDataExport, error) {
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

	return &UserDataExport{
		User:       user,
		Content:    content,
		Comments:   comments,
		Media:      media,
		ExportDate: time.Now().Format(time.RFC3339),
	}, nil
}

// getUserForExport retrieves user information for export
func (r *UserDataExportRepository) getUserForExport(ctx context.Context, userID int) (*User, error) {
	var user User
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

// NewUserDataExportRepository creates a new user data export repository
func NewUserDataExportRepository(db *sql.DB) *UserDataExportRepository {
	return &UserDataExportRepository{
		db: db,
	}
}
