package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
)

const mediaColumns = `m.id, m.user_id, m.filename, m.original_filename, m.mime_type,
		m.file_size, m.width, m.height, m.alt_text, m.is_webp, m.file_path, m.url, m.hash,
		m.variants,
		m.created_at, m.updated_at,
		COALESCE(u.name, u.username) as uploaded_by`

const mediaFrom = `FROM media_files m LEFT JOIN users u ON m.user_id = u.id`

// escapeLike escapes SQL LIKE wildcard characters in a search string
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// isUniqueConstraintError checks if an error is a PostgreSQL unique constraint violation
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key")
}

// scanMediaRow scans a single media row with uploaded_by from a JOIN
func scanMediaRow(row *sql.Row) (*mediadomain.Media, error) {
	var file MediaFile
	var uploadedBy sql.NullString

	err := row.Scan(
		&file.ID,
		&file.UserID,
		&file.Filename,
		&file.OriginalFilename,
		&file.MimeType,
		&file.FileSize,
		&file.Width,
		&file.Height,
		&file.AltText,
		&file.IsWebP,
		&file.FilePath,
		&file.URL,
		&file.Hash,
		&file.Variants,
		&file.CreatedAt,
		&file.UpdatedAt,
		&uploadedBy,
	)
	if err == sql.ErrNoRows {
		return nil, mediadomain.ErrMediaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan media: %w", err)
	}

	media := &mediadomain.Media{
		ID:               file.ID,
		UserID:           file.UserID,
		Filename:         file.Filename,
		OriginalFilename: file.OriginalFilename,
		MimeType:         mediadomain.MimeType(file.MimeType),
		FileSize:         file.FileSize,
		Width:            file.Width,
		Height:           file.Height,
		AltText:          file.AltText,
		IsWebP:           file.IsWebP,
		FilePath:         file.FilePath,
		URL:              file.URL,
		Hash:             file.Hash,
		UploadedBy:       uploadedBy.String,
		Variants:         make(map[string]mediadomain.MediaVariant),
		CreatedAt:        file.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        file.UpdatedAt.Format(time.RFC3339),
	}

	if file.Variants.Valid && file.Variants.String != "" {
		_ = json.Unmarshal([]byte(file.Variants.String), &media.Variants)
	}

	return media, nil
}

// scanMediaRows scans multiple rows from a media query with uploaded_by
func scanMediaRows(rows *sql.Rows) ([]*mediadomain.Media, error) {
	var items []*mediadomain.Media
	for rows.Next() {
		var file MediaFile
		var uploadedBy sql.NullString

		err := rows.Scan(
			&file.ID,
			&file.UserID,
			&file.Filename,
			&file.OriginalFilename,
			&file.MimeType,
			&file.FileSize,
			&file.Width,
			&file.Height,
			&file.AltText,
			&file.IsWebP,
			&file.FilePath,
			&file.URL,
			&file.Hash,
			&file.Variants,
			&file.CreatedAt,
			&file.UpdatedAt,
			&uploadedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media: %w", err)
		}

		media := &mediadomain.Media{
			ID:               file.ID,
			UserID:           file.UserID,
			Filename:         file.Filename,
			OriginalFilename: file.OriginalFilename,
			MimeType:         mediadomain.MimeType(file.MimeType),
			FileSize:         file.FileSize,
			Width:            file.Width,
			Height:           file.Height,
			AltText:          file.AltText,
			IsWebP:           file.IsWebP,
			FilePath:         file.FilePath,
			URL:              file.URL,
			Hash:             file.Hash,
			UploadedBy:       uploadedBy.String,
			Variants:         make(map[string]mediadomain.MediaVariant),
			CreatedAt:        file.CreatedAt.Format(time.RFC3339),
			UpdatedAt:        file.UpdatedAt.Format(time.RFC3339),
		}

		if file.Variants.Valid && file.Variants.String != "" {
			_ = json.Unmarshal([]byte(file.Variants.String), &media.Variants)
		}

		items = append(items, media)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate media rows: %w", err)
	}

	return items, nil
}

// MediaFile represents a media file in the database
type MediaFile struct {
	ID               int            `json:"id"`
	UserID           int            `json:"userId"`
	Filename         string         `json:"filename"`
	OriginalFilename string         `json:"originalFilename"`
	MimeType         string         `json:"mimeType"`
	FileSize         int64          `json:"fileSize"`
	Width            int            `json:"width"`
	Height           int            `json:"height"`
	AltText          string         `json:"altText"`
	IsWebP           bool           `json:"isWebp"`
	FilePath         string         `json:"filePath"`
	URL              string         `json:"url"`
	Hash             string         `json:"hash"`
	Variants         sql.NullString `json:"variants"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// MediaRepository handles media data operations
type MediaRepository struct {
	db *sql.DB
}

// Create stores a new media file in the database
func (r *MediaRepository) Create(ctx context.Context, media *mediadomain.Media) error {

	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection lost: %w", err)
	}

	var variantsJSON any
	if media.Variants != nil {
		vBytes, err := json.Marshal(media.Variants)
		if err != nil {
			return fmt.Errorf("failed to marshal variants: %w", err)
		}
		variantsJSON = string(vBytes)
	}

	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO media_files (
			user_id, filename, original_filename, mime_type,
			file_size, width, height, alt_text, is_webp, file_path, url, hash, variants
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`, media.UserID, media.Filename, media.OriginalFilename,
		media.MimeType.String(), media.FileSize, media.Width, media.Height,
		media.AltText, media.IsWebP, media.FilePath, media.URL, media.Hash, variantsJSON).Scan(&media.ID, &createdAt, &updatedAt)
	if err != nil {
		if isUniqueConstraintError(err) {
			return mediadomain.ErrDuplicateMedia
		}
		return fmt.Errorf("failed to create media: %w", err)
	}

	media.CreatedAt = createdAt.Format(time.RFC3339)
	media.UpdatedAt = updatedAt.Format(time.RFC3339)

	return nil
}

// FindByID retrieves a media file by ID with uploader name
func (r *MediaRepository) FindByID(ctx context.Context, id int) (*mediadomain.Media, error) {

	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.id = $1
	`, id)

	return scanMediaRow(row)
}

// FindByHash retrieves a media file by its hash
func (r *MediaRepository) FindByHash(ctx context.Context, hash string) (*mediadomain.Media, error) {

	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.hash = $1
	`, hash)

	return scanMediaRow(row)
}

// FindByHashPrefix retrieves a media file whose hash starts with the given prefix
func (r *MediaRepository) FindByHashPrefix(ctx context.Context, prefix string) (*mediadomain.Media, error) {

	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.hash LIKE $1
	`, prefix+"%")

	return scanMediaRow(row)
}

// FindAll retrieves all media files
func (r *MediaRepository) FindAll(ctx context.Context, limit int, offset int) ([]*mediadomain.Media, error) {

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
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		ORDER BY m.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all media: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// ListByCursor returns the caller's media in newest-first (id DESC) order using keyset
// pagination (beforeID <= 0 means first page; otherwise only rows with id < beforeID). It
// is additive to the offset-based FindAll (the agent v1 list contract is cursor-only). The
// SELECT column list + row scan are reused from FindAll via mediaColumns/mediaFrom/scanMediaRows.
func (r *MediaRepository) ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*mediadomain.Media, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var rows *sql.Rows
	var err error
	if beforeID > 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.user_id = $1 AND m.id < $2
			ORDER BY m.id DESC
			LIMIT $3
		`, userID, beforeID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.user_id = $1
			ORDER BY m.id DESC
			LIMIT $2
		`, userID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list media by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// FindAllByFilename retrieves media files matching a filename query
func (r *MediaRepository) FindAllByFilename(
	ctx context.Context,
	filename string,
	limit int,
	offset int,
) ([]*mediadomain.Media, error) {

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
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
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE LOWER(m.original_filename) LIKE LOWER($1) ESCAPE '\'
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3
	`, "%"+escapeLike(filename)+"%", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search media by filename: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// FindAllByDateRange retrieves media files uploaded since a given date
func (r *MediaRepository) FindAllByDateRange(
	ctx context.Context,
	since time.Time,
	limit int,
	offset int,
) ([]*mediadomain.Media, error) {

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
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
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.created_at >= $1
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3
	`, since, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get media by date range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// FindAllByFilenameAndDateRange retrieves media files matching filename and date range
func (r *MediaRepository) FindAllByFilenameAndDateRange(
	ctx context.Context,
	filename string,
	since time.Time,
	limit int,
	offset int,
) ([]*mediadomain.Media, error) {

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
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
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE LOWER(m.original_filename) LIKE LOWER($1) ESCAPE '\' AND m.created_at >= $2
		ORDER BY m.created_at DESC
		LIMIT $3 OFFSET $4
	`, "%"+escapeLike(filename)+"%", since, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search media by filename and date range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// DeleteByID removes a media file by ID (admin path)
func (r *MediaRepository) DeleteByID(ctx context.Context, id int) error {

	result, err := r.db.ExecContext(ctx, `
		DELETE FROM media_files WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return mediadomain.ErrMediaNotFound
	}

	return nil
}

// DeleteByOwner removes a media file owned by a specific user
func (r *MediaRepository) DeleteByOwner(ctx context.Context, id int, userID int) error {

	result, err := r.db.ExecContext(ctx, `
		DELETE FROM media_files WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return mediadomain.ErrMediaNotFound
	}

	return nil
}

// NewMediaRepository creates a new media repository
func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{
		db: db,
	}
}
