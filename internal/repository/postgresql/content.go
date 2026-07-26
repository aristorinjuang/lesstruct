package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/repository"
)

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

	format := contentdomain.Format(item.Format)
	if format == "" {
		format = contentdomain.FormatTiptap
	}

	return &contentdomain.Content{
		ID:                 item.ID,
		UserID:             item.UserID,
		Title:              item.Title,
		Slug:               item.Slug,
		Content:            item.Content,
		Tags:               tags,
		Status:             contentdomain.Status(item.Status),
		Format:             format,
		PostType:           item.PostType,
		MetaDescription:    metaDescription,
		OGTitle:            ogTitle,
		OGDescription:      ogDescription,
		AllowComments:      item.AllowComments,
		CustomFields:       customFields,
		Language:           item.Language,
		TranslationGroupID: translationGroupID,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func pgUniqueError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "SQLSTATE 23505")
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
			&item.Format,
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
			&item.Format,
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

// ContentItem represents a content item in the database
type ContentItem struct {
	ID                 int           `json:"id"`
	UserID             int           `json:"userId"`
	Title              string        `json:"title"`
	Slug               string        `json:"slug"`
	Content            string        `json:"content"`
	Tags               string        `json:"tags"`
	Status             string        `json:"status"`
	Format             string        `json:"format"`
	PostType           string        `json:"postType"`
	MetaDescription    *string       `json:"metaDescription"`
	OGTitle            *string       `json:"ogTitle"`
	OGDescription      *string       `json:"ogDescription"`
	AllowComments      bool          `json:"allowComments"`
	CustomFields       *string       `json:"customFields"`
	Language           string        `json:"language"`
	TranslationGroupID sql.NullInt64 `json:"translationGroupId"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
}

// ContentRepository handles content data operations
type ContentRepository struct {
	db *sql.DB
}

func (r *ContentRepository) Create(ctx context.Context, content *contentdomain.Content) error {

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tagsJSON, err := json.Marshal(content.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	customFieldsJSON, err := repository.MarshalCustomFields(content.CustomFields)
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

	var id int
	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO content_items (user_id, title, slug, content, tags, status, format, post_type, meta_description, og_title, og_description, allow_comments, custom_fields, language, translation_group_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at
	`, content.UserID, content.Title, content.Slug, content.Content, string(tagsJSON), content.Status, content.Format, content.PostType, content.MetaDescription, content.OGTitle, content.OGDescription, content.AllowComments, customFieldsJSON, language, translationGroupID).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		if pgUniqueError(err) {
			return contentdomain.ErrSlugAlreadyExists
		}
		return fmt.Errorf("failed to create content: %w", err)
	}

	content.ID = id
	content.CreatedAt = createdAt
	content.UpdatedAt = updatedAt

	return nil
}

func (r *ContentRepository) GetBySlug(ctx context.Context, slug string, language string) (*contentdomain.Content, error) {

	var item ContentItem
	var tagsJSON string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, slug, content, tags, status, format, post_type, meta_description, og_title, og_description, allow_comments, custom_fields, language, translation_group_id, created_at, updated_at
		FROM content_items
		WHERE slug = $1 AND language = $2
	`, slug, language).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.Format,
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
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		WHERE c.user_id = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get content by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

func (r *ContentRepository) ListByCursor(ctx context.Context, userID int, limit int, beforeID int, filters contentdomain.ContentFilters) ([]*contentdomain.Content, error) {
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
	// appends one clause. Tags is AND-of-tags (each tag must be present in the JSONB
	// array — matched via LIKE '%"<tag>"%'). Author matches the joined users.name (with
	// users.username as fallback) case-insensitively. A zero ContentFilters value
	// yields the original unfiltered query.
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		WHERE c.user_id = $1
	`)
	args := []any{userID}
	argN := 1
	if beforeID > 0 {
		argN++
		fmt.Fprintf(&queryBuilder, " AND c.id < $%d", argN)
		args = append(args, beforeID)
	}
	if filters.Status != "" {
		argN++
		fmt.Fprintf(&queryBuilder, " AND c.status = $%d", argN)
		args = append(args, filters.Status)
	}
	if filters.PostType != "" {
		argN++
		fmt.Fprintf(&queryBuilder, " AND c.post_type = $%d", argN)
		args = append(args, filters.PostType)
	}
	if filters.Language != "" {
		argN++
		fmt.Fprintf(&queryBuilder, " AND c.language = $%d", argN)
		args = append(args, filters.Language)
	}
	if filters.Author != "" {
		argN++
		fmt.Fprintf(&queryBuilder, " AND LOWER(COALESCE(u.name, u.username)) = LOWER($%d)", argN)
		args = append(args, filters.Author)
	}
	if filters.Search != "" {
		escapedQuery := strings.ReplaceAll(filters.Search, "%", `\%`)
		escapedQuery = strings.ReplaceAll(escapedQuery, "_", `\_`)
		likePattern := "%" + escapedQuery + "%"
		argN++
		argN++
		fmt.Fprintf(&queryBuilder, " AND (LOWER(c.title) LIKE LOWER($%d) ESCAPE '\\' OR LOWER(c.meta_description) LIKE LOWER($%d) ESCAPE '\\')", argN-1, argN)
		args = append(args, likePattern, likePattern)
	}
	for _, tag := range filters.Tags {
		escapedTag := strings.ReplaceAll(tag, "%", `\%`)
		escapedTag = strings.ReplaceAll(escapedTag, "_", `\_`)
		likePattern := `%"` + escapedTag + `"%`
		argN++
		fmt.Fprintf(&queryBuilder, " AND c.tags LIKE $%d ESCAPE '\\'", argN)
		args = append(args, likePattern)
	}
	argN++
	fmt.Fprintf(&queryBuilder, " ORDER BY c.id DESC LIMIT $%d", argN)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

func (r *ContentRepository) GetAll(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error) {

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
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		ORDER BY c.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

func (r *ContentRepository) CheckSlugUnique(ctx context.Context, slug string, language string) (bool, error) {

	slug = strings.TrimSpace(slug)

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM content_items WHERE slug = $1 AND language = $2
		)
	`, slug, language).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check slug uniqueness: %w", err)
	}

	return !exists, nil
}

func (r *ContentRepository) GetByID(ctx context.Context, id int) (*contentdomain.Content, error) {

	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString
	var updatedBy sql.NullInt64
	var updatedByUsername sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username,
		       c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN users u2 ON c.updated_by = u2.id
		WHERE c.id = $1
	`, id).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.Format,
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

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		DELETE FROM content_items WHERE id = $1 AND user_id = $2
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
		DELETE FROM comments WHERE content_id = $1
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

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		DELETE FROM content_items WHERE id = $1
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
		DELETE FROM comments WHERE content_id = $1
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

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	tagsJSON, err := json.Marshal(content.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	customFieldsJSON, err := repository.MarshalCustomFields(content.CustomFields)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE content_items
		SET title = $1, slug = $2, content = $3, tags = $4, status = $5, format = $6, post_type = $7, meta_description = $8, og_title = $9, og_description = $10, allow_comments = $11, custom_fields = $12, updated_at = NOW(), updated_by = $13
		WHERE id = $14
	`, content.Title, content.Slug, content.Content, string(tagsJSON), content.Status, content.Format, content.PostType, content.MetaDescription, content.OGTitle, content.OGDescription, content.AllowComments, customFieldsJSON, content.UpdatedBy, content.ID)
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
		SELECT updated_at FROM content_items WHERE id = $1
	`, content.ID).Scan(&updatedAt)
	if err != nil {
		return fmt.Errorf("failed to get updated timestamp after update: %w", err)
	}

	content.UpdatedAt = updatedAt

	return nil
}

func (r *ContentRepository) GetPublished(ctx context.Context, limit int, offset int) ([]*contentdomain.Content, error) {

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
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3
	`, contentdomain.StatusPublished, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetPublishedBySlug(ctx context.Context, slug string, language string) (*contentdomain.Content, error) {

	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.slug = $1 AND c.language = $2 AND c.status = $3
	`, slug, language, contentdomain.StatusPublished).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.Format,
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

	var item ContentItem
	var tagsJSON string
	var author sql.NullString
	var username sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.slug = $1 AND c.status = $2
	`, slug, contentdomain.StatusPublished).Scan(
		&item.ID,
		&item.UserID,
		&item.Title,
		&item.Slug,
		&item.Content,
		&tagsJSON,
		&item.Status,
		&item.Format,
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

func (r *ContentRepository) GetPublishedByAuthorUsername(ctx context.Context, username string, language string, limit int, offset int) ([]*contentdomain.Content, error) {

	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var rows *sql.Rows
	var err error
	if language != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
			       COALESCE(u.name, u.username) as author, u.username
			FROM content_items c
			LEFT JOIN users u ON c.user_id = u.id
			WHERE u.username = $1 AND c.status = $2 AND c.language = $3
			ORDER BY c.created_at DESC
			LIMIT $4 OFFSET $5
		`, username, contentdomain.StatusPublished, language, limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
			       COALESCE(u.name, u.username) as author, u.username
			FROM content_items c
			LEFT JOIN users u ON c.user_id = u.id
			WHERE u.username = $1 AND c.status = $2
			ORDER BY c.created_at DESC
			LIMIT $3 OFFSET $4
		`, username, contentdomain.StatusPublished, limit, offset)
	}
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

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER($1))
	`, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check author existence: %w", err)
	}
	return exists, nil
}

func (r *ContentRepository) GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*contentdomain.Content, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.translation_group_id = $1 AND c.id != $2
		ORDER BY c.language ASC
	`, translationGroupID, excludeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get translations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) TranslationGroupExists(ctx context.Context, id int) (bool, error) {

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM content_items WHERE id = $1)
	`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check translation group existence: %w", err)
	}
	return exists, nil
}

func (r *ContentRepository) GetPublishedByTag(ctx context.Context, tag string, language string, year int, month int, limit int, offset int) ([]*contentdomain.Content, error) {

	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = $1 AND c.tags IS NOT NULL AND jsonb_array_length(c.tags) > 0 AND EXISTS (
			SELECT 1 FROM jsonb_array_elements_text(c.tags) AS elem WHERE LOWER(elem) = LOWER($2)
		)
	`
	args := []any{contentdomain.StatusPublished, tag}
	argN := 2
	if language != "" {
		argN++
		query += fmt.Sprintf(` AND c.language = $%d`, argN)
		args = append(args, language)
	}
	if year > 0 && month > 0 {
		argN++
		query += fmt.Sprintf(` AND EXTRACT(YEAR FROM c.created_at)::int = $%d`, argN)
		args = append(args, year)
		argN++
		query += fmt.Sprintf(` AND EXTRACT(MONTH FROM c.created_at)::int = $%d`, argN)
		args = append(args, month)
	}
	argN++
	query += fmt.Sprintf(` ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d`, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*contentdomain.Content{}, nil
		}
		return nil, fmt.Errorf("failed to get published content by tag: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetPublishedTags(ctx context.Context) ([]string, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT elem AS tag
		FROM content_items c, jsonb_array_elements_text(c.tags) AS elem
		WHERE c.status = $1
		  AND c.tags IS NOT NULL
		  AND jsonb_array_length(c.tags) > 0
		ORDER BY tag ASC
	`, contentdomain.StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("failed to get published tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating tag rows: %w", err)
	}

	return tags, nil
}

func (r *ContentRepository) GetPublishedAuthors(ctx context.Context, filters contentdomain.PublishedAuthorFilters) ([]*contentdomain.PublishedAuthor, error) {

	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT u.username,
		       COALESCE(u.name, u.username) AS display_name,
		       COALESCE(u.profile_picture, '') AS profile_picture,
		       COUNT(c.id) AS content_count,
		       STRING_AGG(DISTINCT c.post_type, ',') AS post_types,
		       u.custom_fields
		FROM content_items c
		JOIN users u ON c.user_id = u.id
		WHERE c.status = $1
	`)

	args := []any{contentdomain.StatusPublished}
	argN := 1

	for _, f := range filters.CustomFieldFilters {
		switch f.Operator {
		case contentdomain.FilterOpEqual:
			argN++
			argN++
			fmt.Fprintf(&queryBuilder, ` AND u.custom_fields::jsonb->>$%d = $%d`, argN-1, argN)
			args = append(args, f.Field, f.Value)
		case contentdomain.FilterOpMin:
			argN++
			argN++
			fmt.Fprintf(&queryBuilder, ` AND (u.custom_fields::jsonb->>$%d)::numeric >= $%d`, argN-1, argN)
			args = append(args, f.Field, f.Value)
		case contentdomain.FilterOpMax:
			argN++
			argN++
			fmt.Fprintf(&queryBuilder, ` AND (u.custom_fields::jsonb->>$%d)::numeric <= $%d`, argN-1, argN)
			args = append(args, f.Field, f.Value)
		default:
			return nil, fmt.Errorf("unsupported filter operator: %s", f.Operator)
		}
	}

	queryBuilder.WriteString(`
		GROUP BY u.id, u.username, u.name, u.profile_picture, u.custom_fields
	`)

	if filters.SortBy != "" {
		direction := repository.SortDirectionSQL(filters.SortOrder)
		argN++
		argN++
		fmt.Fprintf(&queryBuilder,
			` ORDER BY CASE WHEN (u.custom_fields::jsonb->>$%d) ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (u.custom_fields::jsonb->>$%d)::numeric ELSE 0 END %s, u.username ASC`,
			argN-1, argN, direction)
		args = append(args, filters.SortBy, filters.SortBy)
	} else {
		queryBuilder.WriteString(` ORDER BY content_count DESC, u.username ASC`)
	}

	argN++
	argN++
	fmt.Fprintf(&queryBuilder, ` LIMIT $%d OFFSET $%d`, argN-1, argN)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get published authors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	authors := make([]*contentdomain.PublishedAuthor, 0)
	for rows.Next() {
		var a contentdomain.PublishedAuthor
		var postTypes string
		var customFieldsJSON *string
		if err := rows.Scan(&a.Username, &a.DisplayName, &a.ProfilePicture, &a.ContentCount, &postTypes, &customFieldsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan published author: %w", err)
		}
		a.PostTypes = repository.SplitCSV(postTypes)
		if customFieldsJSON != nil {
			var cf map[string]any
			_ = json.Unmarshal([]byte(*customFieldsJSON), &cf)
			a.CustomFields = cf
		}
		authors = append(authors, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating published author rows: %w", err)
	}

	return authors, nil
}

func (r *ContentRepository) GetPublishedAuthor(ctx context.Context, username string) (*contentdomain.PublishedAuthor, error) {
	query := `
		SELECT u.username,
		       COALESCE(u.name, u.username) AS display_name,
		       COALESCE(u.profile_picture, '') AS profile_picture,
		       COUNT(c.id) AS content_count,
		       STRING_AGG(DISTINCT c.post_type, ',') AS post_types,
		       u.custom_fields
		FROM content_items c
		JOIN users u ON c.user_id = u.id
		WHERE c.status = $1
		  AND LOWER(u.username) = LOWER($2)
		GROUP BY u.id, u.username, u.name, u.profile_picture, u.custom_fields
	`

	var a contentdomain.PublishedAuthor
	var postTypes string
	var customFieldsJSON *string
	err := r.db.QueryRowContext(ctx, query, contentdomain.StatusPublished, username).Scan(
		&a.Username, &a.DisplayName, &a.ProfilePicture, &a.ContentCount, &postTypes, &customFieldsJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get published author: %w", err)
	}

	a.PostTypes = repository.SplitCSV(postTypes)
	if customFieldsJSON != nil {
		var cf map[string]any
		_ = json.Unmarshal([]byte(*customFieldsJSON), &cf)
		a.CustomFields = cf
	}

	return &a, nil
}

func (r *ContentRepository) GetPublishedArchive(ctx context.Context, postType string, language string) ([]*contentdomain.ArchiveMonth, error) {
	query := `
		SELECT EXTRACT(YEAR FROM c.created_at)::int AS year,
		       EXTRACT(MONTH FROM c.created_at)::int AS month,
		       COUNT(*) AS count
		FROM content_items c
		WHERE c.status = $1
	`
	args := []any{contentdomain.StatusPublished}
	argN := 1
	if postType != "" {
		argN++
		query += fmt.Sprintf(` AND c.post_type = $%d`, argN)
		args = append(args, postType)
	}
	if language != "" {
		argN++
		query += fmt.Sprintf(` AND c.language = $%d`, argN)
		args = append(args, language)
	}
	query += ` GROUP BY year, month ORDER BY year DESC, month DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get published archive: %w", err)
	}
	defer func() { _ = rows.Close() }()

	archive := make([]*contentdomain.ArchiveMonth, 0)
	for rows.Next() {
		var m contentdomain.ArchiveMonth
		if err := rows.Scan(&m.Year, &m.Month, &m.Count); err != nil {
			return nil, fmt.Errorf("failed to scan archive month: %w", err)
		}
		archive = append(archive, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating archive rows: %w", err)
	}

	return archive, nil
}

func (r *ContentRepository) ListByFilters(ctx context.Context, userID int, filters contentdomain.ContentFilters) ([]*contentdomain.Content, error) {

	var queryBuilder strings.Builder
	queryBuilder.WriteString(`SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at, COALESCE(u.name, u.username) as author, u.username, c.updated_by, COALESCE(u2.name, u2.username) as updated_by_username FROM content_items c LEFT JOIN users u ON c.user_id = u.id LEFT JOIN users u2 ON c.updated_by = u2.id`)

	var args []any
	argN := 0

	if userID > 0 {
		argN++
		fmt.Fprintf(&queryBuilder, ` WHERE c.user_id = $%d`, argN)
		args = append(args, userID)
	} else {
		queryBuilder.WriteString(` WHERE 1=1`)
	}

	if filters.Language != "" {
		argN++
		fmt.Fprintf(&queryBuilder, ` AND c.language = $%d`, argN)
		args = append(args, filters.Language)
	}

	if filters.PostType != "" {
		argN++
		fmt.Fprintf(&queryBuilder, ` AND c.post_type = $%d`, argN)
		args = append(args, filters.PostType)
	}

	if filters.Status != "" {
		argN++
		fmt.Fprintf(&queryBuilder, ` AND c.status = $%d`, argN)
		args = append(args, filters.Status)
	}

	if filters.Author != "" {
		argN++
		fmt.Fprintf(&queryBuilder, ` AND LOWER(COALESCE(u.name, u.username)) = LOWER($%d)`, argN)
		args = append(args, filters.Author)
	}

	if filters.Search != "" {
		escapedQuery := strings.ReplaceAll(filters.Search, "%", `\%`)
		escapedQuery = strings.ReplaceAll(escapedQuery, "_", `\_`)
		likePattern := "%" + escapedQuery + "%"
		argN++
		argN++
		fmt.Fprintf(&queryBuilder, ` AND (LOWER(c.title) LIKE LOWER($%d) ESCAPE '\' OR LOWER(c.meta_description) LIKE LOWER($%d) ESCAPE '\')`, argN-1, argN)
		args = append(args, likePattern, likePattern)
	}

	for _, tag := range filters.Tags {
		escapedTag := strings.ReplaceAll(tag, "%", `\%`)
		escapedTag = strings.ReplaceAll(escapedTag, "_", `\_`)
		likePattern := `%"` + escapedTag + `"%`
		argN++
		fmt.Fprintf(&queryBuilder, ` AND c.tags LIKE $%d ESCAPE '\'`, argN)
		args = append(args, likePattern)
	}

	for _, f := range filters.CustomFieldFilters {
		switch f.Operator {
		case contentdomain.FilterOpEqual:
			argN++
			argN++
			fmt.Fprintf(&queryBuilder, ` AND c.custom_fields::jsonb->>$%d = $%d`, argN-1, argN)
			args = append(args, f.Field, f.Value)
		case contentdomain.FilterOpMin:
			argN++
			argN++
			fmt.Fprintf(&queryBuilder, ` AND (c.custom_fields::jsonb->>$%d)::numeric >= $%d`, argN-1, argN)
			args = append(args, f.Field, f.Value)
		case contentdomain.FilterOpMax:
			argN++
			argN++
			fmt.Fprintf(&queryBuilder, ` AND (c.custom_fields::jsonb->>$%d)::numeric <= $%d`, argN-1, argN)
			args = append(args, f.Field, f.Value)
		default:
			return nil, fmt.Errorf("unsupported filter operator: %s", f.Operator)
		}
	}

	if filters.SortBy != "" {
		direction := repository.SortDirectionSQL(filters.SortOrder)
		argN++
		argN++
		fmt.Fprintf(&queryBuilder,
			` ORDER BY CASE WHEN (c.custom_fields::jsonb->>$%d) ~ '^-?[0-9]+(\.[0-9]+)?$' THEN (c.custom_fields::jsonb->>$%d)::numeric ELSE 0 END %s, c.created_at DESC`,
			argN-1, argN, direction)
		args = append(args, filters.SortBy, filters.SortBy)
	} else {
		queryBuilder.WriteString(` ORDER BY c.created_at DESC`)
	}

	argN++
	argN++
	fmt.Fprintf(&queryBuilder, ` LIMIT $%d OFFSET $%d`, argN-1, argN)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by filters: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuditInfo(rows)
}

func (r *ContentRepository) GetPublishedPages(ctx context.Context) ([]*contentdomain.Content, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.post_type = 'page' AND c.status = $1
		ORDER BY c.title ASC
	`, contentdomain.StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("failed to get published pages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetPublishedCustomPostTypes(ctx context.Context) ([]string, error) {

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT c.post_type
		FROM content_items c
		WHERE c.status = $1 AND c.post_type NOT IN ('post', 'page')
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

func (r *ContentRepository) GetPublishedByPostType(ctx context.Context, postType string, language string, year int, month int, limit int, offset int) ([]*contentdomain.Content, error) {

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.post_type = $1 AND c.status = $2
	`
	args := []any{postType, contentdomain.StatusPublished}
	argN := 2
	if language != "" {
		argN++
		query += fmt.Sprintf(` AND c.language = $%d`, argN)
		args = append(args, language)
	}
	if year > 0 && month > 0 {
		argN++
		query += fmt.Sprintf(` AND EXTRACT(YEAR FROM c.created_at)::int = $%d`, argN)
		args = append(args, year)
		argN++
		query += fmt.Sprintf(` AND EXTRACT(MONTH FROM c.created_at)::int = $%d`, argN)
		args = append(args, month)
	}
	argN++
	query += fmt.Sprintf(` ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d`, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by post type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) SearchPublished(ctx context.Context, query string, limit int) ([]*contentdomain.Content, error) {

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
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = $1 AND c.post_type = 'post'
		  AND (LOWER(c.title) LIKE LOWER($2) ESCAPE '\' OR LOWER(c.meta_description) LIKE LOWER($3) ESCAPE '\')
		ORDER BY c.created_at DESC
		LIMIT $4
	`, contentdomain.StatusPublished, likePattern, likePattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search published content: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetRelatedByTags(ctx context.Context, excludeID int, tags []string, postType string, language string, limit int) ([]*contentdomain.Content, error) {
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
		placeholders[i] = fmt.Sprintf("$%d", 5+i)
	}
	inList := "(" + strings.Join(placeholders, ",") + ")"
	limitArg := 5 + len(lowerTags)

	args := []any{contentdomain.StatusPublished, postType, language, excludeID}
	for _, t := range lowerTags {
		args = append(args, t)
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = $1 AND c.post_type = $2 AND c.language = $3 AND c.id <> $4
		  AND c.tags IS NOT NULL AND jsonb_array_length(c.tags) > 0
		  AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(c.tags) AS elem WHERE LOWER(elem) IN %s)
		ORDER BY (SELECT COUNT(*) FROM jsonb_array_elements_text(c.tags) AS elem WHERE LOWER(elem) IN %s) DESC,
		         c.created_at DESC
		LIMIT $%d
	`, inList, inList, limitArg)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get related content by tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

func (r *ContentRepository) GetLatestByPostType(ctx context.Context, excludeID int, postType string, language string, limit int) ([]*contentdomain.Content, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.title, c.slug, c.content, c.tags, c.status, c.format, c.post_type, c.meta_description, c.og_title, c.og_description, c.allow_comments, c.custom_fields, c.language, c.translation_group_id, c.created_at, c.updated_at,
		       COALESCE(u.name, u.username) as author, u.username
		FROM content_items c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.status = $1 AND c.post_type = $2 AND c.language = $3 AND c.id <> $4
		ORDER BY c.created_at DESC
		LIMIT $5
	`, contentdomain.StatusPublished, postType, language, excludeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest content by post type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanContentRowsWithAuthorAndUsername(rows)
}

// NewContentRepository creates a new content repository
func NewContentRepository(db *sql.DB) *ContentRepository {
	return &ContentRepository{
		db: db,
	}
}
