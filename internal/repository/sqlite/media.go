package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
)

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

// escapeLike escapes SQL LIKE wildcard characters in a search string
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
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
		CreatedAt:        file.CreatedAt,
		UpdatedAt:        file.UpdatedAt,
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
			CreatedAt:        file.CreatedAt,
			UpdatedAt:        file.UpdatedAt,
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

const mediaColumns = `m.id, m.user_id, m.filename, m.original_filename, m.mime_type,
		m.file_size, m.width, m.height, m.alt_text, m.is_webp, m.file_path, m.url, m.hash,
		m.variants,
		m.created_at, m.updated_at,
		COALESCE(u.name, u.username) as uploaded_by`

const mediaFrom = `FROM media_files m LEFT JOIN users u ON m.user_id = u.id`

// Create stores a new media file in the database
func (r *MediaRepository) Create(ctx context.Context, media *mediadomain.Media) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

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

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO media_files (
			user_id, filename, original_filename, mime_type,
			file_size, width, height, alt_text, is_webp, file_path, url, hash, variants
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, media.UserID, media.Filename, media.OriginalFilename,
		media.MimeType.String(), media.FileSize, media.Width, media.Height,
		media.AltText, media.IsWebP, media.FilePath, media.URL, media.Hash, variantsJSON)
	if err != nil {
		if isUniqueConstraintError(err) {
			return mediadomain.ErrDuplicateMedia
		}
		return fmt.Errorf("failed to create media: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	media.ID = int(id)

	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(ctx, `
		SELECT created_at, updated_at FROM media_files WHERE id = ?
	`, media.ID).Scan(&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to get timestamps: %w", err)
	}

	media.CreatedAt = createdAt
	media.UpdatedAt = updatedAt

	return nil
}

// FindByID retrieves a media file by ID with uploader name
func (r *MediaRepository) FindByID(ctx context.Context, id int) (*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.id = ?
	`, id)

	return scanMediaRow(row)
}

// FindByHash retrieves a media file by its hash
func (r *MediaRepository) FindByHash(ctx context.Context, hash string) (*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.hash = ?
	`, hash)

	return scanMediaRow(row)
}

// FindByHashPrefix retrieves a media file whose hash starts with the given prefix
func (r *MediaRepository) FindByHashPrefix(ctx context.Context, prefix string) (*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.hash LIKE ?
	`, prefix+"%")

	return scanMediaRow(row)
}

// FindAllByCursor returns ALL media (not user-scoped) in newest-first (id DESC) order using
// keyset pagination (beforeID <= 0 means first page; otherwise only rows with id < beforeID).
// Global counterpart of ListByCursor, used by the browser admin media library. The SELECT
// column list + row scan are reused via mediaColumns/mediaFrom/scanMediaRows.
func (r *MediaRepository) FindAllByCursor(ctx context.Context, limit int, beforeID int) ([]*mediadomain.Media, error) {
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

	var rows *sql.Rows
	var err error
	if beforeID > 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.id < ?
			ORDER BY m.id DESC
			LIMIT ?
		`, beforeID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			ORDER BY m.id DESC
			LIMIT ?
		`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get all media by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// ListByCursor returns the caller's media in newest-first (id DESC) order using keyset
// pagination (beforeID <= 0 means first page; otherwise only rows with id < beforeID). The
// SELECT column list + row scan are reused via mediaColumns/mediaFrom/scanMediaRows.
func (r *MediaRepository) ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*mediadomain.Media, error) {
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

	var rows *sql.Rows
	var err error
	if beforeID > 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.user_id = ? AND m.id < ?
			ORDER BY m.id DESC
			LIMIT ?
		`, userID, beforeID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.user_id = ?
			ORDER BY m.id DESC
			LIMIT ?
		`, userID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list media by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// FindAllByFilenameByCursor returns ALL media matching a filename query in newest-first
// (id DESC) order using keyset pagination (beforeID <= 0 means first page).
func (r *MediaRepository) FindAllByFilenameByCursor(
	ctx context.Context,
	filename string,
	limit int,
	beforeID int,
) ([]*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

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
			WHERE LOWER(m.original_filename) LIKE LOWER(?) ESCAPE '\' AND m.id < ?
			ORDER BY m.id DESC
			LIMIT ?
		`, "%"+escapeLike(filename)+"%", beforeID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE LOWER(m.original_filename) LIKE LOWER(?) ESCAPE '\'
			ORDER BY m.id DESC
			LIMIT ?
		`, "%"+escapeLike(filename)+"%", limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to search media by filename by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// FindAllByDateRangeByCursor returns ALL media created since the given time in newest-first
// (id DESC) order using keyset pagination (beforeID <= 0 means first page).
func (r *MediaRepository) FindAllByDateRangeByCursor(
	ctx context.Context,
	since time.Time,
	limit int,
	beforeID int,
) ([]*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

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
			WHERE m.created_at >= ? AND m.id < ?
			ORDER BY m.id DESC
			LIMIT ?
		`, since, beforeID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.created_at >= ?
			ORDER BY m.id DESC
			LIMIT ?
		`, since, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get media by date range by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// FindAllByFilenameAndDateRangeByCursor returns ALL media matching a filename query and
// created since the given time in newest-first (id DESC) order using keyset pagination
// (beforeID <= 0 means first page).
func (r *MediaRepository) FindAllByFilenameAndDateRangeByCursor(
	ctx context.Context,
	filename string,
	since time.Time,
	limit int,
	beforeID int,
) ([]*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection lost: %w", err)
	}

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
			WHERE LOWER(m.original_filename) LIKE LOWER(?) ESCAPE '\' AND m.created_at >= ? AND m.id < ?
			ORDER BY m.id DESC
			LIMIT ?
		`, "%"+escapeLike(filename)+"%", since, beforeID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE LOWER(m.original_filename) LIKE LOWER(?) ESCAPE '\' AND m.created_at >= ?
			ORDER BY m.id DESC
			LIMIT ?
		`, "%"+escapeLike(filename)+"%", since, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to search media by filename and date range by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// Count returns the total number of ALL media (not user-scoped) matching the given
// filename query and creation-time threshold (zero time means no date filter). It mirrors
// the WHERE clauses of the FindAll*ByCursor methods so a list's total always matches its
// rows.
func (r *MediaRepository) Count(ctx context.Context, search string, since time.Time) (int, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return 0, fmt.Errorf("database connection lost: %w", err)
	}

	var (
		where string
		args  []any
	)
	if search != "" {
		where = ` WHERE LOWER(m.original_filename) LIKE LOWER(?) ESCAPE '\'`
		args = append(args, "%"+escapeLike(search)+"%")
	}
	if !since.IsZero() {
		if where != "" {
			where += ` AND m.created_at >= ?`
		} else {
			where = ` WHERE m.created_at >= ?`
		}
		args = append(args, since)
	}

	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_files m`+where, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count media: %w", err)
	}
	return total, nil
}

// DeleteByID removes a media file by ID (admin path)
func (r *MediaRepository) DeleteByID(ctx context.Context, id int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx, `
		DELETE FROM media_files WHERE id = ?
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
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	result, err := r.db.ExecContext(ctx, `
		DELETE FROM media_files WHERE id = ? AND user_id = ?
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