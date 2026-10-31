// Package store is the single-source media repository: one implementation
// against *sql.DB plus a kernel Dialect, replacing the three per-driver
// copies. Placeholder rendering, RETURNING support, and duplicate detection
// come from the dialect; everything else (columns, scans, messages, guards)
// ports the SQLite implementation verbatim, which the moved test suite pins.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kerneldatabase "github.com/aristorinjuang/lesstruct/kernel/database"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
)

const (
	mediaColumns = `m.id, m.user_id, m.filename, m.original_filename, m.mime_type,
		m.file_size, m.width, m.height, m.alt_text, m.is_webp, m.file_path, m.url, m.hash,
		m.variants,
		m.created_at, m.updated_at,
		COALESCE(u.name, u.username) as uploaded_by`

	mediaFrom = `FROM media_files m LEFT JOIN users u ON m.user_id = u.id`
)

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
		return nil, fmt.Errorf("failed iterating media rows: %w", err)
	}

	return items, nil
}

// mediaListWhere builds the WHERE clause shared by the media-list queries and
// their Count mirror, so a list's total always matches its rows. ownerID > 0
// scopes every clause to that user's library; 0 (the admin scope) leaves the
// list global. The placeholder function is called once per clause, in order.
func mediaListWhere(next func() string, ownerID int, filename string, since time.Time) (string, []any) {
	var (
		clauses []string
		args    []any
	)
	if ownerID > 0 {
		clauses = append(clauses, "m.user_id = "+next())
		args = append(args, ownerID)
	}
	if filename != "" {
		clauses = append(clauses, "LOWER(m.original_filename) LIKE LOWER("+next()+`) ESCAPE '\'`)
		args = append(args, "%"+escapeLike(filename)+"%")
	}
	if !since.IsZero() {
		clauses = append(clauses, "m.created_at >= "+next())
		args = append(args, since)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// listMedia is the shared keyset-paged media list: owner scope (0 = all),
// optional filename search, optional creation-time floor, newest-first
// (id DESC), beforeID <= 0 means first page. Every exported FindAll* variant
// funnels through here so the scopes and filters stay in one place.
func (r *Repository) listMedia(
	ctx context.Context,
	ownerID int,
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

	next := r.placeholders()
	where, args := mediaListWhere(next, ownerID, filename, since)
	if beforeID > 0 {
		if where == "" {
			where = " WHERE m.id < " + next()
		} else {
			where += " AND m.id < " + next()
		}
		args = append(args, beforeID)
	}
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+where+`
			ORDER BY m.id DESC
			LIMIT `+next(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list media by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanMediaRows(rows)
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

// Repository implements mediadomain.Repository for every driver.
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

// Create stores a new media file.
func (r *Repository) Create(ctx context.Context, media *mediadomain.Media) error {
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

	next := r.placeholders()
	columns := []string{
		"user_id", "filename", "original_filename", "mime_type",
		"file_size", "width", "height", "alt_text", "is_webp", "file_path", "url", "hash", "variants",
	}
	marks := make([]string, 0, len(columns))
	for range columns {
		marks = append(marks, next())
	}
	args := []any{
		media.UserID, media.Filename, media.OriginalFilename,
		media.MimeType.String(), media.FileSize, media.Width, media.Height,
		media.AltText, media.IsWebP, media.FilePath, media.URL, media.Hash, variantsJSON,
	}

	if r.dialect.SupportsReturning() {
		var createdAt, updatedAt time.Time
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO media_files (
				user_id, filename, original_filename, mime_type,
				file_size, width, height, alt_text, is_webp, file_path, url, hash, variants
			) VALUES (`+strings.Join(marks, ", ")+`)
			RETURNING id, created_at, updated_at
		`, args...).Scan(&media.ID, &createdAt, &updatedAt)
		if err != nil {
			if r.dialect.IsDuplicateError(err) {
				return mediadomain.ErrDuplicateMedia
			}
			return fmt.Errorf("failed to create media: %w", err)
		}
		media.CreatedAt = createdAt
		media.UpdatedAt = updatedAt
		return nil
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO media_files (
			user_id, filename, original_filename, mime_type,
			file_size, width, height, alt_text, is_webp, file_path, url, hash, variants
		) VALUES (`+strings.Join(marks, ", ")+`)
	`, args...)
	if err != nil {
		if r.dialect.IsDuplicateError(err) {
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
	next = r.placeholders()
	err = r.db.QueryRowContext(ctx, `
		SELECT created_at, updated_at FROM media_files WHERE id = `+next()+`
	`, media.ID).Scan(&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to get timestamps: %w", err)
	}

	media.CreatedAt = createdAt
	media.UpdatedAt = updatedAt

	return nil
}

// FindByID retrieves a media file by ID with uploader name
func (r *Repository) FindByID(ctx context.Context, id int) (*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.id = `+next()+`
	`, id)
	return scanMediaRow(row)
}

// FindByHash retrieves a media file by its full hash.
func (r *Repository) FindByHash(ctx context.Context, hash string) (*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.hash = `+next()+`
	`, hash)
	return scanMediaRow(row)
}

// FindByHashPrefix retrieves a media file whose hash starts with the given
// prefix. The prefix is LIKE-escaped with an explicit ESCAPE clause (unlike
// the legacy SQLite copy, which interpolated it raw): hex prefixes behave
// identically, while wildcard input matches literally instead of globbing.
// The clause matters on every driver — SQLite has no default escape
// character, so without it the backslash escapes would be inert there.
func (r *Repository) FindByHashPrefix(ctx context.Context, prefix string) (*mediadomain.Media, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	row := r.db.QueryRowContext(ctx, `
		SELECT `+mediaColumns+`
		`+mediaFrom+`
		WHERE m.hash LIKE `+next()+` ESCAPE '\'
		LIMIT 1
	`, escapeLike(prefix)+"%")
	return scanMediaRow(row)
}

// FindAllByCursor returns media in newest-first (id DESC) order using keyset
// pagination (beforeID <= 0 means first page; otherwise only rows with
// id < beforeID). ownerID > 0 scopes the list to that user's library; 0 (the
// admin scope) returns every user's media. Used by the browser admin media
// library. The SELECT column list + row scan are reused via
// mediaColumns/mediaFrom/scanMediaRows.
func (r *Repository) FindAllByCursor(ctx context.Context, ownerID int, limit int, beforeID int) ([]*mediadomain.Media, error) {
	return r.listMedia(ctx, ownerID, "", time.Time{}, limit, beforeID)
}

// ListByCursor returns the caller's media in newest-first (id DESC) order using keyset
// pagination (beforeID <= 0 means first page; otherwise only rows with id < beforeID). The
// SELECT column list + row scan are reused via mediaColumns/mediaFrom/scanMediaRows.
func (r *Repository) ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*mediadomain.Media, error) {
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

	next := r.placeholders()
	var rows *sql.Rows
	var err error
	if beforeID > 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.user_id = `+next()+` AND m.id < `+next()+`
			ORDER BY m.id DESC
			LIMIT `+next()+`
		`, userID, beforeID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+mediaColumns+`
			`+mediaFrom+`
			WHERE m.user_id = `+next()+`
			ORDER BY m.id DESC
			LIMIT `+next()+`
		`, userID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list media by cursor: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanMediaRows(rows)
}

// FindAllByFilenameByCursor returns media matching a filename query in
// newest-first (id DESC) order using keyset pagination (beforeID <= 0 means
// first page). ownerID > 0 scopes the search to that user's library; 0 (the
// admin scope) searches every user's media.
func (r *Repository) FindAllByFilenameByCursor(
	ctx context.Context,
	ownerID int,
	filename string,
	limit int,
	beforeID int,
) ([]*mediadomain.Media, error) {
	return r.listMedia(ctx, ownerID, filename, time.Time{}, limit, beforeID)
}

// FindAllByDateRangeByCursor returns media created since the given time in
// newest-first (id DESC) order using keyset pagination (beforeID <= 0 means
// first page). ownerID > 0 scopes the list to that user's library; 0 (the
// admin scope) returns every user's media.
func (r *Repository) FindAllByDateRangeByCursor(
	ctx context.Context,
	ownerID int,
	since time.Time,
	limit int,
	beforeID int,
) ([]*mediadomain.Media, error) {
	return r.listMedia(ctx, ownerID, "", since, limit, beforeID)
}

// FindAllByFilenameAndDateRangeByCursor returns media matching a filename
// query and created since the given time in newest-first (id DESC) order using
// keyset pagination (beforeID <= 0 means first page). ownerID > 0 scopes the
// search to that user's library; 0 (the admin scope) searches every user's.
func (r *Repository) FindAllByFilenameAndDateRangeByCursor(
	ctx context.Context,
	ownerID int,
	filename string,
	since time.Time,
	limit int,
	beforeID int,
) ([]*mediadomain.Media, error) {
	return r.listMedia(ctx, ownerID, filename, since, limit, beforeID)
}

// Count returns the total number of media matching the given owner scope,
// filename query, and creation-time threshold (zero time means no date
// filter). ownerID > 0 counts only that user's media; 0 (the admin scope)
// counts all. It mirrors the WHERE clauses of the FindAll*ByCursor methods so
// a list's total always matches its rows.
func (r *Repository) Count(ctx context.Context, ownerID int, search string, since time.Time) (int, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	if err := r.db.PingContext(ctx); err != nil {
		return 0, fmt.Errorf("database connection lost: %w", err)
	}

	next := r.placeholders()
	where, args := mediaListWhere(next, ownerID, strings.TrimSpace(search), since)

	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_files m`+where, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count media: %w", err)
	}
	return total, nil
}

// DeleteByID removes a media file by ID (admin path)
func (r *Repository) DeleteByID(ctx context.Context, id int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	result, err := r.db.ExecContext(ctx, `DELETE FROM media_files WHERE id = `+next(), id)
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
func (r *Repository) DeleteByOwner(ctx context.Context, id int, userID int) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	next := r.placeholders()
	result, err := r.db.ExecContext(ctx, `DELETE FROM media_files WHERE id = `+next()+` AND user_id = `+next(), id, userID)
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

// New creates a repository on db speaking dialect. Dialect must be non-nil;
// the composition root resolves it from the driver registry.
func New(db *sql.DB, dialect kerneldatabase.Dialect) *Repository {
	return &Repository{
		db:      db,
		dialect: dialect,
	}
}

var _ mediadomain.Repository = (*Repository)(nil)
