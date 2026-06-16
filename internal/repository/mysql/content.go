package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// escapeLike escapes SQL LIKE wildcard characters so that '%' and '_' in
// user-provided search strings match literally rather than as wildcards.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
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

	var updatedBy int
	if item.UpdatedBy.Valid {
		updatedBy = int(item.UpdatedBy.Int64)
	}

	var updatedByUsername string
	if item.UpdatedByUsername.Valid {
		updatedByUsername = item.UpdatedByUsername.String
	}

	return &contentdomain.Content{
		ID:                 item.ID,
		UserID:             item.UserID,
		Title:              item.Title,
		Slug:               item.Slug,
		Content:            item.Content.String,
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
		UpdatedBy:          updatedBy,
		UpdatedByUsername:  updatedByUsername,
		CreatedAt:          item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          item.UpdatedAt.Format(time.RFC3339),
	}
}

func scanContentRows(rows *sql.Rows) ([]*contentdomain.Content, error) {
	var items []*contentdomain.Content
	for rows.Next() {
		var item ContentItem
		var tagsJSON string
		var author sql.NullString
		var username sql.NullString

		err := rows.Scan(
			&item.ID, &item.UserID, &item.Title, &item.Slug, &item.Content,
			&tagsJSON, &item.Status, &item.PostType,
			&item.MetaDescription, &item.OGTitle, &item.OGDescription,
			&item.AllowComments, &item.CustomFields, &item.Language,
			&item.TranslationGroupID, &item.CreatedAt, &item.UpdatedAt,
			&author, &username,
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

		err := rows.Scan(
			&item.ID, &item.UserID, &item.Title, &item.Slug, &item.Content,
			&tagsJSON, &item.Status, &item.PostType,
			&item.MetaDescription, &item.OGTitle, &item.OGDescription,
			&item.AllowComments, &item.CustomFields, &item.Language,
			&item.TranslationGroupID, &item.CreatedAt, &item.UpdatedAt,
			&author, &username,
			&item.UpdatedBy, &item.UpdatedByUsername,
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

// ---------------------------------------------------------------------------
// Private model
// ---------------------------------------------------------------------------

// ContentItem represents a content item in the database
type ContentItem struct {
	ID                 int            `json:"id"`
	UserID             int            `json:"userId"`
	Title              string         `json:"title"`
	Slug               string         `json:"slug"`
	Content            sql.NullString `json:"content"`
	Status             string         `json:"status"`
	PostType           string         `json:"postType"`
	MetaDescription    *string        `json:"metaDescription"`
	OGTitle            *string        `json:"ogTitle"`
	OGDescription      *string        `json:"ogDescription"`
	AllowComments      bool           `json:"allowComments"`
	CustomFields       *string        `json:"customFields"`
	Language           string         `json:"language"`
	TranslationGroupID sql.NullInt64  `json:"translationGroupId"`
	UpdatedBy          sql.NullInt64  `json:"updatedBy"`
	UpdatedByUsername  sql.NullString `json:"updatedByUsername"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

// ---------------------------------------------------------------------------
// Public repository
// ---------------------------------------------------------------------------

// ContentRepository handles content data operations with MySQL syntax.
type ContentRepository struct {
	db *sql.DB
}

// Create creates a new content item.
func (r *ContentRepository) Create(
	ctx context.Context,
	content *contentdomain.Content,
) error {
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

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO content_items
			(user_id, title, slug, content, tags, status, post_type,
			 meta_description, og_title, og_description, allow_comments,
			 custom_fields, language, translation_group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, content.UserID, content.Title, content.Slug, content.Content,
		string(tagsJSON), content.Status, content.PostType,
		content.MetaDescription, content.OGTitle, content.OGDescription,
		content.AllowComments, customFieldsJSON, language,
		content.TranslationGroupID)
	if err != nil {
		if isMySQLDuplicateError(err) {
			return contentdomain.ErrSlugAlreadyExists
		}
		return fmt.Errorf("failed to create content: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	content.ID = int(id)

	// Fetch server-generated timestamps so the domain object is complete.
	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(ctx,
		`SELECT created_at, updated_at FROM content_items WHERE id = ?`,
		id,
	).Scan(&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to fetch created timestamps: %w", err)
	}
	content.CreatedAt = createdAt.Format(time.RFC3339)
	content.UpdatedAt = updatedAt.Format(time.RFC3339)

	return nil
}

// GetBySlug retrieves a content item by slug and language.
func (r *ContentRepository) GetBySlug(
	ctx context.Context,
	slug string,
	language string,
) (*contentdomain.Content, error) {
	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.slug = ? AND ci.language = ?
	`, slug, language).Scan(
		&item.ID, &item.UserID, &item.Title, &item.Slug, &item.Content,
		&tagsJSON, &item.Status, &item.PostType,
		&item.MetaDescription, &item.OGTitle, &item.OGDescription,
		&item.AllowComments, &item.CustomFields, &item.Language,
		&item.TranslationGroupID, &item.CreatedAt, &item.UpdatedAt,
		&author, &username,
	)
	if err == sql.ErrNoRows {
		return nil, contentdomain.ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get content by slug: %w", err)
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

// GetByUser retrieves content items by user ID with pagination.
func (r *ContentRepository) GetByUser(
	ctx context.Context,
	userID int,
	limit int,
	offset int,
) ([]*contentdomain.Content, error) {
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
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       ci.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		LEFT JOIN users u2 ON ci.updated_by = u2.id
		WHERE ci.user_id = ?
		ORDER BY ci.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get content by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

// ListByCursor retrieves the caller's content in newest-first (id DESC) order using
// keyset pagination. beforeID <= 0 requests the first page; otherwise only rows with a
// strictly smaller id are returned. The optional filters restrict which rows qualify:
// empty string / nil fields are no-ops, so a zero ContentFilters value yields the
// original unfiltered query. Tags is AND-of-tags. Author matches the joined users.name
// (with users.username as a fallback), case-insensitive. It is additive to the
// offset-based GetByUser / GetAll (the agent v1 list contract is cursor-only; offset
// is unstable under concurrent inserts/deletes). The SELECT column list + row scan are
// copied verbatim from GetByUser.
func (r *ContentRepository) ListByCursor(
	ctx context.Context,
	userID int,
	limit int,
	beforeID int,
	filters contentdomain.ContentFilters,
) ([]*contentdomain.Content, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       ci.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		LEFT JOIN users u2 ON ci.updated_by = u2.id
		WHERE ci.user_id = ?
	`)
	args := []any{userID}
	if beforeID > 0 {
		queryBuilder.WriteString(" AND ci.id < ?")
		args = append(args, beforeID)
	}
	if filters.Status != "" {
		queryBuilder.WriteString(" AND ci.status = ?")
		args = append(args, filters.Status)
	}
	if filters.PostType != "" {
		queryBuilder.WriteString(" AND ci.post_type = ?")
		args = append(args, filters.PostType)
	}
	if filters.Language != "" {
		queryBuilder.WriteString(" AND ci.language = ?")
		args = append(args, filters.Language)
	}
	if filters.Author != "" {
		queryBuilder.WriteString(" AND LOWER(COALESCE(u.name, u.username)) = LOWER(?)")
		args = append(args, filters.Author)
	}
	if filters.Search != "" {
		searchParam := "%" + escapeLike(filters.Search) + "%"
		queryBuilder.WriteString(" AND (ci.title LIKE ? OR ci.meta_description LIKE ?)")
		args = append(args, searchParam, searchParam)
	}
	for _, tag := range filters.Tags {
		tagParam := "%\"" + escapeLike(tag) + "\"%"
		queryBuilder.WriteString(" AND ci.tags LIKE ?")
		args = append(args, tagParam)
	}
	queryBuilder.WriteString(" ORDER BY ci.id DESC LIMIT ?")
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

// GetAll retrieves all content items with pagination.
func (r *ContentRepository) GetAll(
	ctx context.Context,
	limit int,
	offset int,
) ([]*contentdomain.Content, error) {
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
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       ci.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		LEFT JOIN users u2 ON ci.updated_by = u2.id
		ORDER BY ci.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

// CheckSlugUnique checks if a slug is unique for the given language.
func (r *ContentRepository) CheckSlugUnique(
	ctx context.Context,
	slug string,
	language string,
) (bool, error) {
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

// GetByID retrieves a content item by its ID.
func (r *ContentRepository) GetByID(
	ctx context.Context,
	id int,
) (*contentdomain.Content, error) {
	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       ci.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		LEFT JOIN users u2 ON ci.updated_by = u2.id
		WHERE ci.id = ?
	`, id).Scan(
		&item.ID, &item.UserID, &item.Title, &item.Slug, &item.Content,
		&tagsJSON, &item.Status, &item.PostType,
		&item.MetaDescription, &item.OGTitle, &item.OGDescription,
		&item.AllowComments, &item.CustomFields, &item.Language,
		&item.TranslationGroupID, &item.CreatedAt, &item.UpdatedAt,
		&author, &username,
		&item.UpdatedBy, &item.UpdatedByUsername,
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
	return content, nil
}

// Update updates a content item.
func (r *ContentRepository) Update(
	ctx context.Context,
	content *contentdomain.Content,
) error {
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
		SET title = ?, slug = ?, content = ?, tags = ?, status = ?, post_type = ?,
		    meta_description = ?, og_title = ?, og_description = ?,
		    allow_comments = ?, custom_fields = ?, language = ?,
		    translation_group_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ?
	`, content.Title, content.Slug, content.Content,
		string(tagsJSON), content.Status, content.PostType,
		content.MetaDescription, content.OGTitle, content.OGDescription,
		content.AllowComments, customFieldsJSON, content.Language,
		content.TranslationGroupID, content.UpdatedBy, content.ID)
	if err != nil {
		if isMySQLDuplicateError(err) {
			return contentdomain.ErrSlugAlreadyExists
		}
		return fmt.Errorf("failed to update content: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return contentdomain.ErrContentNotFound
	}

	// Fetch server-generated updated_at so the domain object is current.
	var updatedAt time.Time
	err = r.db.QueryRowContext(ctx,
		`SELECT updated_at FROM content_items WHERE id = ?`,
		content.ID,
	).Scan(&updatedAt)
	if err != nil {
		return fmt.Errorf("failed to fetch updated_at: %w", err)
	}
	content.UpdatedAt = updatedAt.Format(time.RFC3339)

	return nil
}

// GetPublished retrieves published content items with pagination.
func (r *ContentRepository) GetPublished(
	ctx context.Context,
	limit int,
	offset int,
) ([]*contentdomain.Content, error) {
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
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.status = 'published'
		ORDER BY ci.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRows(rows)
}

// GetPublishedBySlug retrieves a published content item by slug and language.
func (r *ContentRepository) GetPublishedBySlug(
	ctx context.Context,
	slug string,
	language string,
) (*contentdomain.Content, error) {
	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.slug = ? AND ci.language = ? AND ci.status = 'published'
	`, slug, language).Scan(
		&item.ID, &item.UserID, &item.Title, &item.Slug, &item.Content,
		&tagsJSON, &item.Status, &item.PostType,
		&item.MetaDescription, &item.OGTitle, &item.OGDescription,
		&item.AllowComments, &item.CustomFields, &item.Language,
		&item.TranslationGroupID, &item.CreatedAt, &item.UpdatedAt,
		&author, &username,
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

// GetPublishedBySlugAny finds published content by slug regardless of language.
func (r *ContentRepository) GetPublishedBySlugAny(
	ctx context.Context,
	slug string,
) (*contentdomain.Content, error) {
	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.slug = ? AND ci.status = 'published'
		LIMIT 1
	`, slug).Scan(
		&item.ID, &item.UserID, &item.Title, &item.Slug, &item.Content,
		&tagsJSON, &item.Status, &item.PostType,
		&item.MetaDescription, &item.OGTitle, &item.OGDescription,
		&item.AllowComments, &item.CustomFields, &item.Language,
		&item.TranslationGroupID, &item.CreatedAt, &item.UpdatedAt,
		&author, &username,
	)
	if err == sql.ErrNoRows {
		return nil, contentdomain.ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by slug any: %w", err)
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

// GetPublishedByAuthorUsername retrieves published content by author username.
func (r *ContentRepository) GetPublishedByAuthorUsername(
	ctx context.Context,
	username string,
	limit int,
	offset int,
) ([]*contentdomain.Content, error) {
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
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE u.username = ? AND ci.status = 'published'
		ORDER BY ci.created_at DESC
		LIMIT ? OFFSET ?
	`, username, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published by author username: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRows(rows)
}

// AuthorExists checks if a user with the given username exists.
func (r *ContentRepository) AuthorExists(
	ctx context.Context,
	username string,
) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE LOWER(username) = LOWER(?)
		)
	`, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check author exists: %w", err)
	}
	return exists, nil
}

// Delete removes a content item and cascades to comments within a transaction.
func (r *ContentRepository) Delete(
	ctx context.Context,
	id int,
	userID int,
) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete associated comments first.
	_, err = tx.ExecContext(ctx, `DELETE FROM comments WHERE content_id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete content comments: %w", err)
	}

	result, err := tx.ExecContext(ctx,
		`DELETE FROM content_items WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete content: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return contentdomain.ErrContentNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete: %w", err)
	}

	return nil
}

// DeleteByID removes a content item by ID within a transaction.
func (r *ContentRepository) DeleteByID(
	ctx context.Context,
	id int,
) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete associated comments first.
	_, err = tx.ExecContext(ctx, `DELETE FROM comments WHERE content_id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete content comments: %w", err)
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM content_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete content by id: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return contentdomain.ErrContentNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete: %w", err)
	}

	return nil
}

// ListByFilters retrieves content items applying the given filters.
func (r *ContentRepository) ListByFilters(
	ctx context.Context,
	userID int,
	filters contentdomain.ContentFilters,
) ([]*contentdomain.Content, error) {
	var queryBuilder strings.Builder
	var args []any

	queryBuilder.WriteString(`
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       ci.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		LEFT JOIN users u2 ON ci.updated_by = u2.id
	`)

	if userID > 0 {
		queryBuilder.WriteString(" WHERE ci.user_id = ?")
		args = append(args, userID)
	} else {
		queryBuilder.WriteString(" WHERE 1=1")
	}

	if filters.PostType != "" {
		queryBuilder.WriteString(" AND ci.post_type = ?")
		args = append(args, filters.PostType)
	}

	if filters.Status != "" {
		queryBuilder.WriteString(" AND ci.status = ?")
		args = append(args, filters.Status)
	}

	if filters.Author != "" {
		queryBuilder.WriteString(" AND LOWER(COALESCE(u.name, u.username)) = LOWER(?)")
		args = append(args, filters.Author)
	}

	if filters.Search != "" {
		searchParam := "%" + escapeLike(filters.Search) + "%"
		queryBuilder.WriteString(" AND (ci.title LIKE ? OR ci.meta_description LIKE ?)")
		args = append(args, searchParam, searchParam)
	}

	if filters.Language != "" {
		queryBuilder.WriteString(" AND ci.language = ?")
		args = append(args, filters.Language)
	}

	for _, tag := range filters.Tags {
		tagParam := "%\"" + escapeLike(tag) + "\"%"
		queryBuilder.WriteString(" AND ci.tags LIKE ?")
		args = append(args, tagParam)
	}

	// Custom field filters — filter on JSON column
	for _, cf := range filters.CustomFieldFilters {
		switch cf.Operator {
		case contentdomain.FilterOpEqual:
			queryBuilder.WriteString(" AND JSON_EXTRACT(ci.custom_fields, ?) = ?")
			args = append(args, fmt.Sprintf("$.%s", cf.Field), cf.Value)
		case contentdomain.FilterOpMin:
			queryBuilder.WriteString(" AND CAST(JSON_EXTRACT(ci.custom_fields, ?) AS DECIMAL) >= ?")
			args = append(args, fmt.Sprintf("$.%s", cf.Field), cf.Value)
		case contentdomain.FilterOpMax:
			queryBuilder.WriteString(" AND CAST(JSON_EXTRACT(ci.custom_fields, ?) AS DECIMAL) <= ?")
			args = append(args, fmt.Sprintf("$.%s", cf.Field), cf.Value)
		default:
			return nil, fmt.Errorf("unsupported filter operator: %s", cf.Operator)
		}
	}

	queryBuilder.WriteString(" ORDER BY ci.created_at DESC")

	if filters.Limit > 0 {
		queryBuilder.WriteString(" LIMIT ?")
		args = append(args, filters.Limit)
	}

	if filters.Offset > 0 {
		queryBuilder.WriteString(" OFFSET ?")
		args = append(args, filters.Offset)
	}

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by filters: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

// GetPublishedPages returns all published pages.
func (r *ContentRepository) GetPublishedPages(
	ctx context.Context,
) ([]*contentdomain.Content, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.post_type = 'page' AND ci.status = 'published'
		ORDER BY ci.title ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get published pages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRows(rows)
}

// GetPublishedCustomPostTypes returns list of distinct published post types (excluding 'post' and 'page').
func (r *ContentRepository) GetPublishedCustomPostTypes(
	ctx context.Context,
) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT post_type
		FROM content_items
		WHERE status = 'published' AND post_type NOT IN ('post', 'page')
		ORDER BY post_type ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom post types: %w", err)
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
		return nil, fmt.Errorf("failed iterating post types: %w", err)
	}
	return postTypes, nil
}

// GetPublishedByPostType returns published content for a given post type.
func (r *ContentRepository) GetPublishedByPostType(
	ctx context.Context,
	postType string,
	limit int,
	offset int,
) ([]*contentdomain.Content, error) {
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
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.post_type = ? AND ci.status = 'published'
		ORDER BY ci.created_at DESC
		LIMIT ? OFFSET ?
	`, postType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published by post type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRows(rows)
}

// GetPublishedByTag returns published content with a specific tag.
func (r *ContentRepository) GetPublishedByTag(
	ctx context.Context,
	tag string,
	limit int,
	offset int,
) ([]*contentdomain.Content, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	// MySQL JSON_CONTAINS for tag search
	tagJSON := fmt.Sprintf(`"%s"`, tag)
	rows, err := r.db.QueryContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE JSON_CONTAINS(ci.tags, ?) AND ci.status = 'published'
		ORDER BY ci.created_at DESC
		LIMIT ? OFFSET ?
	`, tagJSON, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published by tag: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRows(rows)
}

// SearchPublished searches published content by query string.
func (r *ContentRepository) SearchPublished(
	ctx context.Context,
	query string,
	limit int,
) ([]*contentdomain.Content, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	searchQuery := "%" + escapeLike(query) + "%"
	rows, err := r.db.QueryContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.status = 'published' AND ci.post_type = 'post'
		  AND (ci.title LIKE ? OR ci.meta_description LIKE ? OR ci.content LIKE ?)
		ORDER BY ci.created_at DESC
		LIMIT ?
	`, searchQuery, searchQuery, searchQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search published content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRows(rows)
}

// GetTranslations returns all content items in the same translation group, excluding the given ID.
func (r *ContentRepository) GetTranslations(
	ctx context.Context,
	translationGroupID int,
	excludeID int,
) ([]*contentdomain.Content, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ci.id, ci.user_id, ci.title, ci.slug, ci.content, ci.tags,
		       ci.status, ci.post_type, ci.meta_description, ci.og_title, ci.og_description,
		       ci.allow_comments, ci.custom_fields, ci.language, ci.translation_group_id,
		       ci.created_at, ci.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items ci
		LEFT JOIN users u ON ci.user_id = u.id
		WHERE ci.translation_group_id = ? AND ci.id != ?
		ORDER BY ci.language ASC
	`, translationGroupID, excludeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get translations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRows(rows)
}

// TranslationGroupExists checks whether a content item with the given ID exists.
func (r *ContentRepository) TranslationGroupExists(
	ctx context.Context,
	id int,
) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM content_items WHERE id = ?
		)
	`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check translation group exists: %w", err)
	}
	return exists, nil
}

// NewContentRepository creates a new content repository.
func NewContentRepository(db *sql.DB) *ContentRepository {
	return &ContentRepository{
		db: db,
	}
}
