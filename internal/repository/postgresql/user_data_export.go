package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/repository"
)

type UserContentItem = repository.UserContentItem
type UserCommentItem = repository.UserCommentItem
type UserMediaItem = repository.UserMediaItem
type UserDataExport = repository.UserDataExport

// UserDataExportRepository handles user data export operations
type UserDataExportRepository struct {
	db *sql.DB
}

// GetUserContent retrieves all content items created by a user
func (r *UserDataExportRepository) GetUserContent(ctx context.Context, userID int) ([]*UserContentItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, slug, content, status, created_at, updated_at
		FROM content_items
		WHERE user_id = $1
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating content rows: %w", err)
	}
	return contentItems, nil
}

// GetUserComments retrieves all comments posted by a user
func (r *UserDataExportRepository) GetUserComments(ctx context.Context, userID int) ([]*UserCommentItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, content_id, comment, created_at, updated_at
		FROM comments
		WHERE user_id = $1
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating comment rows: %w", err)
	}
	return commentItems, nil
}

// GetUserMedia retrieves all media files uploaded by a user
func (r *UserDataExportRepository) GetUserMedia(ctx context.Context, userID int) ([]*UserMediaItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, filename, original_filename, file_path, file_size, mime_type, created_at
		FROM media_files
		WHERE user_id = $1
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating media rows: %w", err)
	}
	return mediaItems, nil
}

// GetUserDataForExport aggregates all user data for export
func (r *UserDataExportRepository) GetUserDataForExport(ctx context.Context, userID int) (*UserDataExport, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	var usr repository.User
	var email, name, profilePicture sql.NullString
	var customFieldsRaw *string
	var lastLoginAt *string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, email, name, role, status, profile_picture, last_login_at, custom_fields, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&usr.ID,
		&usr.Username,
		&email,
		&name,
		&usr.Role,
		&usr.Status,
		&profilePicture,
		&lastLoginAt,
		&customFieldsRaw,
		&usr.CreatedAt,
		&usr.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user for export: %w", err)
	}
	if email.Valid {
		usr.Email = email.String
	}
	if name.Valid {
		usr.Name = name.String
	}
	if profilePicture.Valid {
		usr.ProfilePicture = profilePicture.String
	}
	usr.LastLoginAt = lastLoginAt
	usr.CustomFields = unmarshalCustomFields(customFieldsRaw)
	content, err := r.GetUserContent(ctx, userID)
	if err != nil {
		return nil, err
	}
	comments, err := r.GetUserComments(ctx, userID)
	if err != nil {
		return nil, err
	}
	media, err := r.GetUserMedia(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &UserDataExport{
		ExportDate: time.Now().Format(time.RFC3339),
		User:       &usr,
		Content:    content,
		Comments:   comments,
		Media:      media,
	}, nil
}

// NewUserDataExportRepository creates a new user data export repository
func NewUserDataExportRepository(db *sql.DB) *UserDataExportRepository {
	return &UserDataExportRepository{
		db: db,
	}
}
