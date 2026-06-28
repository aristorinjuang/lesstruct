package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

func marshalCustomFields(fields map[string]any) (any, error) {
	if fields == nil {
		return nil, nil
	}
	cfBytes, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom fields: %w", err)
	}
	return string(cfBytes), nil
}

func mapContentItemToDomain(item *ContentItem, tagsJSON string) *contentdomain.Content {
	var tags []string
	if tagsJSON != "" && tagsJSON != "null" {
		_ = json.Unmarshal([]byte(tagsJSON), &tags)
	}
	if tags == nil {
		tags = []string{}
	}

	var metaDescription string
	if item.MetaDescription != nil {
		metaDescription = *item.MetaDescription
	}

	var ogTitle string
	if item.OGTitle != nil {
		ogTitle = *item.OGTitle
	}

	var ogDescription string
	if item.OGDescription != nil {
		ogDescription = *item.OGDescription
	}

	var customFields map[string]any
	if item.CustomFields != nil && *item.CustomFields != "" {
		_ = json.Unmarshal([]byte(*item.CustomFields), &customFields)
	}

	var translationGroupID *int
	if item.TranslationGroupID.Valid {
		v := int(item.TranslationGroupID.Int64)
		translationGroupID = &v
	}

	return &contentdomain.Content{
		ID:                 item.ID,
		UserID:             item.UserID,
		Title:              item.Title,
		Slug:               item.Slug,
		Content:            item.Content,
		Tags:               tags,
		Status:             contentdomain.Status(item.Status),
		PostType:           item.PostType,
		MetaDescription:    metaDescription,
		OGTitle:            ogTitle,
		OGDescription:      ogDescription,
		AllowComments:      item.AllowComments,
		CustomFields:       customFields,
		Language:           item.Language,
		TranslationGroupID: translationGroupID,
		CreatedAt:          item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          item.UpdatedAt.Format(time.RFC3339),
	}
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed")
}

// ContentItem represents a content item in the database
type ContentItem struct {
	ID                 int            `json:"id"`
	UserID             int            `json:"userId"`
	Title              string         `json:"title"`
	Slug               string         `json:"slug"`
	Content            string         `json:"content"`
	Tags               string         `json:"tags"`
	Status             string         `json:"status"`
	PostType           string         `json:"postType"`
	MetaDescription    *string        `json:"metaDescription"`
	OGTitle            *string        `json:"ogTitle"`
	OGDescription      *string        `json:"ogDescription"`
	AllowComments      bool           `json:"allowComments"`
	CustomFields       *string        `json:"customFields"`
	Language           string         `json:"language"`
	TranslationGroupID sql.NullInt64  `json:"translationGroupId"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

// ContentRepository handles content data operations
type ContentRepository struct {
	db *sql.DB
}

func (r *ContentRepository) Create(ctx context.Context, content *contentdomain.Content) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tagsJSON, err := json.Marshal(content.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	customFieldsJSON, err := marshalCustomFields(content.CustomFields)
	if err != nil {
		return err
	}

	language := content.Language
	if language == "" {
		language = "en"
	}

	var translationGroupID any
	if content.TranslationGroupID != nil {
		translationGroupID = *content.TranslationGroupID
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO content_items (user_id, title, slug, content, tags, status, post_type, meta_description, og_title, og_description, allow_comments, custom_fields, language, translation_group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, content.UserID, content.Title, content.Slug, content.Content, string(tagsJSON), content.Status, content.PostType, content.MetaDescription, content.OGTitle, content.OGDescription, content.AllowComments, customFieldsJSON, language, translationGroupID)
	if err != nil {
		if isUniqueConstraintError(err) {
			return contentdomain.ErrSlugAlreadyExists
		}
		return fmt.Errorf("failed to create content: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	content.ID = int(id)

	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(ctx, `
		SELECT created_at, updated_at FROM content_items WHERE id = ?
	`, content.ID).Scan(&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to get timestamps: %w", err)
	}

	content.CreatedAt = createdAt.Format(time.RFC3339)
	content.UpdatedAt = updatedAt.Format(time.RFC3339)

	return nil
}

func (r *ContentRepository) GetBySlug(ctx context.Context, slug string, language string) (*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var item ContentItem
	var tagsJSON string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, slug, content, tags, status, post_type, meta_description, og_title, og_description, allow_comments, custom_fields, language, translation_group_id, created_at, updated_at
		FROM content_items
		WHERE slug = ? AND language = ?
	`, slug, language).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.PostType,
		&item.MetaDescription,
		&item.OGTitle,
		&item.OGDescription,
		&item.AllowComments,
		&item.CustomFields,
		&item.Language,
		&item.TranslationGroupID,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, contentdomain.ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get content by slug: %w", err)
	}

	return mapContentItemToDomain(&item, tagsJSON), nil
}

func (r *ContentRepository) GetByUser(ctx context.Context, userID int, limit int, offset int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get content by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

func (r *ContentRepository) ListByCursor(ctx context.Context, userID int, limit int, beforeID int, filters contentdomain.ContentFilters) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Keyset (cursor) pagination keyed by the monotonic PK `id`. beforeID <= 0 requests
	// the first page; otherwise only rows older than beforeID are returned. ORDER BY id
	// DESC is deterministic and unaffected by concurrent inserts/deletes (unlike offset).
	// The SELECT column list + row scan are copied verbatim from GetByUser so the
	// *content.Content hydration (tags + custom fields + audit info) matches exactly.
	//
	// Optional filters are AND-ed onto the base WHERE clause: each non-empty value
	// appends one clause. Tags is AND-of-tags (each tag must be present in the JSON
	// array — matched via LIKE '%"<tag>"%' with the same ESCAPE pattern as the
	// search clause). Author matches the joined users.name (with users.username as
	// fallback) case-insensitively. A zero ContentFilters value yields the original
	// unfiltered query.
	query := `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		WHERE c.user_id = ?
	`
	args := []any{userID}
	if beforeID > 0 {
		query += ` AND c.id < ?`
		args = append(args, beforeID)
	}
	if filters.Status != "" {
		query += ` AND c.status = ?`
		args = append(args, filters.Status)
	}
	if filters.PostType != "" {
		query += ` AND c.post_type = ?`
		args = append(args, filters.PostType)
	}
	if filters.Language != "" {
		query += ` AND c.language = ?`
		args = append(args, filters.Language)
	}
	if filters.Author != "" {
		query += ` AND LOWER(COALESCE(u.name, u.username)) = LOWER(?)`
		args = append(args, filters.Author)
	}
	if filters.Search != "" {
		escapedQuery := strings.ReplaceAll(filters.Search, "%", `\%`)
		escapedQuery = strings.ReplaceAll(escapedQuery, "_", `\_`)
		likePattern := "%" + escapedQuery + "%"
		query += ` AND (LOWER(c.title) LIKE LOWER(?) ESCAPE '\' OR LOWER(c.meta_description) LIKE LOWER(?) ESCAPE '\')`
		args = append(args, likePattern, likePattern)
	}
	for _, tag := range filters.Tags {
		escapedTag := strings.ReplaceAll(tag, "%", `\%`)
		escapedTag = strings.ReplaceAll(escapedTag, "_", `\_`)
		likePattern := `%"` + escapedTag + `"%`
		query += ` AND c.tags LIKE ? ESCAPE '\'`
		args = append(args, likePattern)
	}
	query += ` ORDER BY c.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

func (r *ContentRepository) GetAll(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

func (r *ContentRepository) CheckSlugUnique(ctx context.Context, slug string, language string) (bool, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	slug = strings.TrimSpace(slug)

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM content_items WHERE slug = ? AND language = ?
		)
	`, slug, language).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check slug uniqueness: %w", err)
	}

	return !exists, nil
}

func (r *ContentRepository) GetByID(ctx context.Context, id int) (*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString
	var updatedBy sql.NullInt64
	var updatedByUsername sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		WHERE c.id = ?
	`, id).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.PostType,
		&item.MetaDescription,
		&item.OGTitle,
		&item.OGDescription,
		&item.AllowComments,
		&item.CustomFields,
		&item.Language,
		&item.TranslationGroupID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&author,
		&username,
		&updatedBy,
		&updatedByUsername,
	)

	if err == sql.ErrNoRows {
		return nil, contentdomain.ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get content by id: %w", err)
	}

	content := mapContentItemToDomain(&item, tagsJSON)
	if author.Valid {
		content.Author = author.String
	}
	if username.Valid {
		content.Username = username.String
	}
	if updatedBy.Valid {
		content.UpdatedBy = int(updatedBy.Int64)
	}
	if updatedByUsername.Valid {
		content.UpdatedByUsername = updatedByUsername.String
	}
	return content, nil
}

func (r *ContentRepository) Delete(ctx context.Context, id int, userID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		DELETE FROM content_items WHERE id = ? AND user_id = ?
	`, id, userID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to delete content: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		_ = tx.Rollback()
		return contentdomain.ErrContentNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM comments WHERE content_id = ?
	`, id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to delete comments: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *ContentRepository) DeleteByID(ctx context.Context, id int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		DELETE FROM content_items WHERE id = ?
	`, id)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to delete content: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		_ = tx.Rollback()
		return contentdomain.ErrContentNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM comments WHERE content_id = ?
	`, id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to delete comments: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *ContentRepository) Update(ctx context.Context, content *contentdomain.Content) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tagsJSON, err := json.Marshal(content.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	customFieldsJSON, err := marshalCustomFields(content.CustomFields)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE content_items
		SET title = ?, slug = ?, content = ?, tags = ?, status = ?, post_type = ?, meta_description = ?, og_title = ?, og_description = ?, allow_comments = ?, custom_fields = ?, updated_at = CURRENT_TIMESTAMP, updated_by = ?
		WHERE id = ?
	`, content.Title, content.Slug, content.Content, string(tagsJSON), content.Status, content.PostType, content.MetaDescription, content.OGTitle, content.OGDescription, content.AllowComments, customFieldsJSON, content.UpdatedBy, content.ID)
	if err != nil {
		return fmt.Errorf("failed to update content: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return contentdomain.ErrContentNotFound
	}

	var updatedAt time.Time
	err = r.db.QueryRowContext(ctx, `
		SELECT updated_at FROM content_items WHERE id = ?
	`, content.ID).Scan(&updatedAt)
	if err != nil {
		return fmt.Errorf("failed to get updated timestamp after update: %w", err)
	}

	content.UpdatedAt = updatedAt.Format(time.RFC3339)

	return nil
}

func (r *ContentRepository) GetPublished(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = ?
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?
	`, contentdomain.StatusPublished, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetPublishedBySlug(ctx context.Context, slug string, language string) (*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.slug = ? AND c.language = ? AND c.status = ?
	`, slug, language, contentdomain.StatusPublished).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.PostType,
		&item.MetaDescription,
		&item.OGTitle,
		&item.OGDescription,
		&item.AllowComments,
		&item.CustomFields,
		&item.Language,
		&item.TranslationGroupID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&author,
		&username,
	)

	if err == sql.ErrNoRows {
		return nil, contentdomain.ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by slug: %w", err)
	}

	content := mapContentItemToDomain(&item, tagsJSON)
	if author.Valid {
		content.Author = author.String
	}
	if username.Valid {
		content.Username = username.String
	}
	return content, nil
}

func (r *ContentRepository) GetPublishedBySlugAny(ctx context.Context, slug string) (*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.slug = ? AND c.status = ?
	`, slug, contentdomain.StatusPublished).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.PostType,
		&item.MetaDescription,
		&item.OGTitle,
		&item.OGDescription,
		&item.AllowComments,
		&item.CustomFields,
		&item.Language,
		&item.TranslationGroupID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&author,
		&username,
	)

	if err == sql.ErrNoRows {
		return nil, contentdomain.ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by slug: %w", err)
	}

	content := mapContentItemToDomain(&item, tagsJSON)
	if author.Valid {
		content.Author = author.String
	}
	if username.Valid {
		content.Username = username.String
	}
	return content, nil
}

func (r *ContentRepository) GetPublishedByAuthorUsername(ctx context.Context, username string, limit int, offset int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE u.username = ? AND c.status = ?
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?
	`, username, contentdomain.StatusPublished, limit, offset)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*contentdomain.Content{}, nil
		}
		return nil, fmt.Errorf("failed to get published content by author: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) AuthorExists(ctx context.Context, username string) (bool, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER(?))
	`, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check author existence: %w", err)
	}
	return exists, nil
}

func (r *ContentRepository) GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.translation_group_id = ? AND c.id != ?
		ORDER BY c.language ASC
	`, translationGroupID, excludeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get translations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) TranslationGroupExists(ctx context.Context, id int) (bool, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM content_items WHERE id = ?)
	`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check translation group existence: %w", err)
	}
	return exists, nil
}

func (r *ContentRepository) GetPublishedByTag(ctx context.Context, tag string, limit int, offset int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = ? AND c.tags IS NOT NULL AND c.tags != '' AND EXISTS (
			SELECT 1 FROM json_each(c.tags) WHERE LOWER(json_each.value) = LOWER(?)
		)
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?
	`, contentdomain.StatusPublished, tag, limit, offset)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*contentdomain.Content{}, nil
		}
		return nil, fmt.Errorf("failed to get published content by tag: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func scanContentRowsWithAuthorAndUsername(rows *sql.Rows) ([]*contentdomain.Content, error) {
	var items []*contentdomain.Content
	for rows.Next() {
		var item ContentItem
		var tagsJSON string
		var author sql.NullString
		var username sql.NullString

		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Title,
			&item.Slug,
			&item.Content,
			&tagsJSON,
			&item.Status,
			&item.PostType,
			&item.MetaDescription,
			&item.OGTitle,
			&item.OGDescription,
			&item.AllowComments,
			&item.CustomFields,
			&item.Language,
			&item.TranslationGroupID,
			&item.CreatedAt,
			&item.UpdatedAt,
			&author,
			&username,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan content: %w", err)
		}

		content := mapContentItemToDomain(&item, tagsJSON)
		if author.Valid {
			content.Author = author.String
		}
		if username.Valid {
			content.Username = username.String
		}
		items = append(items, content)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating content rows: %w", err)
	}
	return items, nil
}

func scanContentRowsWithAuditInfo(rows *sql.Rows) ([]*contentdomain.Content, error) {
	var items []*contentdomain.Content
	for rows.Next() {
		var item ContentItem
		var tagsJSON string
		var author sql.NullString
		var username sql.NullString
		var updatedBy sql.NullInt64
		var updatedByUsername sql.NullString

		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Title,
			&item.Slug,
			&item.Content,
			&tagsJSON,
			&item.Status,
			&item.PostType,
			&item.MetaDescription,
			&item.OGTitle,
			&item.OGDescription,
			&item.AllowComments,
			&item.CustomFields,
			&item.Language,
			&item.TranslationGroupID,
			&item.CreatedAt,
			&item.UpdatedAt,
			&author,
			&username,
			&updatedBy,
			&updatedByUsername,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan content: %w", err)
		}

		content := mapContentItemToDomain(&item, tagsJSON)
		if author.Valid {
			content.Author = author.String
		}
		if username.Valid {
			content.Username = username.String
		}
		if updatedBy.Valid {
			content.UpdatedBy = int(updatedBy.Int64)
		}
		if updatedByUsername.Valid {
			content.UpdatedByUsername = updatedByUsername.String
		}
		items = append(items, content)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating content rows: %w", err)
	}
	return items, nil
}

func (r *ContentRepository) ListByFilters(ctx context.Context, userID int, filters contentdomain.ContentFilters) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(`SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at, COALESCE(u.name, u.username) as author, u.username, c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username FROM content_items c LEFT JOIN users u ON c.user_id = u.id LEFT JOIN users u2 ON c.updated_by = u2.id`)

	var args []any

	if userID > 0 {
		queryBuilder.WriteString(` WHERE c.user_id = ?`)
		args = append(args, userID)
	} else {
		queryBuilder.WriteString(` WHERE 1=1`)
	}

	if filters.Language != "" {
		queryBuilder.WriteString(` AND c.language = ?`)
		args = append(args, filters.Language)
	}

	if filters.PostType != "" {
		queryBuilder.WriteString(` AND c.post_type = ?`)
		args = append(args, filters.PostType)
	}

	if filters.Status != "" {
		queryBuilder.WriteString(` AND c.status = ?`)
		args = append(args, filters.Status)
	}

	if filters.Author != "" {
		queryBuilder.WriteString(` AND LOWER(COALESCE(u.name, u.username)) = LOWER(?)`)
		args = append(args, filters.Author)
	}

	if filters.Search != "" {
		escapedQuery := strings.ReplaceAll(filters.Search, "%", `\%`)
		escapedQuery = strings.ReplaceAll(escapedQuery, "_", `\_`)
		likePattern := "%" + escapedQuery + "%"
		queryBuilder.WriteString(` AND (LOWER(c.title) LIKE LOWER(?) ESCAPE '\' OR LOWER(c.meta_description) LIKE LOWER(?) ESCAPE '\')`)
		args = append(args, likePattern, likePattern)
	}

	for _, tag := range filters.Tags {
		escapedTag := strings.ReplaceAll(tag, "%", `\%`)
		escapedTag = strings.ReplaceAll(escapedTag, "_", `\_`)
		likePattern := `%"` + escapedTag + `"%`
		queryBuilder.WriteString(` AND c.tags LIKE ? ESCAPE '\'`)
		args = append(args, likePattern)
	}

	for _, f := range filters.CustomFieldFilters {
		jsonPath := `$.` + f.Field
		switch f.Operator {
		case contentdomain.FilterOpEqual:
			queryBuilder.WriteString(` AND json_extract(c.custom_fields, ?) = ?`)
			args = append(args, jsonPath, f.Value)
		case contentdomain.FilterOpMin:
			queryBuilder.WriteString(` AND CAST(json_extract(c.custom_fields, ?) AS REAL) >= ?`)
			args = append(args, jsonPath, f.Value)
		case contentdomain.FilterOpMax:
			queryBuilder.WriteString(` AND CAST(json_extract(c.custom_fields, ?) AS REAL) <= ?`)
			args = append(args, jsonPath, f.Value)
		default:
			return nil, fmt.Errorf("unsupported filter operator: %s", f.Operator)
		}
	}

	queryBuilder.WriteString(` ORDER BY c.created_at DESC LIMIT ? OFFSET ?`)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by filters: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

// NewContentRepository creates a new content repository
func NewContentRepository(db *sql.DB) *ContentRepository {
	return &ContentRepository{
		db: db,
	}
}

func (r *ContentRepository) GetPublishedPages(ctx context.Context) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.post_type = 'page' AND c.status = ?
		ORDER BY c.title ASC
	`, contentdomain.StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("failed to get published pages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetPublishedCustomPostTypes(ctx context.Context) ([]string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT c.post_type
		FROM content_items c
		WHERE c.status = ? AND c.post_type NOT IN ('post', 'page')
	`, contentdomain.StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("failed to get published custom post types: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var postTypes []string
	for rows.Next() {
		var pt string
		if err := rows.Scan(&pt); err != nil {
			return nil, fmt.Errorf("failed to scan post type: %w", err)
		}
		postTypes = append(postTypes, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating post type rows: %w", err)
	}

	return postTypes, nil
}

func (r *ContentRepository) GetPublishedByPostType(ctx context.Context, postType string, limit int, offset int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.post_type = ? AND c.status = ?
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?
	`, postType, contentdomain.StatusPublished, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by post type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) SearchPublished(ctx context.Context, query string, limit int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	escapedQuery := strings.ReplaceAll(query, "%", `\%`)
	escapedQuery = strings.ReplaceAll(escapedQuery, "_", `\_`)
	likePattern := "%" + escapedQuery + "%"

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = ? AND c.post_type = 'post'
		  AND (LOWER(c.title) LIKE LOWER(?) ESCAPE '\' OR LOWER(c.meta_description) LIKE LOWER(?) ESCAPE '\')
		ORDER BY c.created_at DESC
		LIMIT ?
	`, contentdomain.StatusPublished, likePattern, likePattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search published content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetRelatedByTags(ctx context.Context, excludeID int, tags []string, postType string, language string, limit int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if len(tags) == 0 {
		return []*contentdomain.Content{}, nil
	}

	if limit <= 0 {
		limit = 5
	}
	if limit > 100 {
		limit = 100
	}

	if len(tags) > 10 {
		tags = tags[:10]
	}

	lowerTags := make([]string, len(tags))
	for i, t := range tags {
		lowerTags[i] = strings.ToLower(t)
	}

	placeholders := make([]string, len(lowerTags))
	for i := range lowerTags {
		placeholders[i] = "?"
	}
	inList := "(" + strings.Join(placeholders, ",") + ")"

	args := []any{contentdomain.StatusPublished, postType, language, excludeID}
	for _, t := range lowerTags {
		args = append(args, t)
	}
	for _, t := range lowerTags {
		args = append(args, t)
	}
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = ? AND c.post_type = ? AND c.language = ? AND c.id != ?
		  AND c.tags IS NOT NULL AND c.tags != '' AND c.tags != 'null'
		  AND EXISTS (SELECT 1 FROM json_each(c.tags) WHERE LOWER(json_each.value) IN `+inList+`)
		ORDER BY (SELECT COUNT(*) FROM json_each(c.tags) WHERE LOWER(json_each.value) IN `+inList+`) DESC,
		         c.created_at DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get related content by tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetLatestByPostType(ctx context.Context, excludeID int, postType string, language string, limit int) ([]*contentdomain.Content, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if limit <= 0 {
		limit = 5
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = ? AND c.post_type = ? AND c.language = ? AND c.id != ?
		ORDER BY c.created_at DESC
		LIMIT ?
	`, contentdomain.StatusPublished, postType, language, excludeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest content by post type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}
