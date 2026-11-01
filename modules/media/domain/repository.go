package media

import (
	"context"
	"time"
)

// Repository defines the interface for media repository operations
type Repository interface {
	Create(ctx context.Context, media *Media) error
	FindByID(ctx context.Context, id int) (*Media, error)
	FindByHash(ctx context.Context, hash string) (*Media, error)
	FindByHashPrefix(ctx context.Context, prefix string) (*Media, error)
	// ListByCursor returns the caller's media in newest-first (id DESC) order using keyset
	// pagination. beforeID <= 0 means "first page"; otherwise only rows with a strictly
	// smaller id are returned. Agent v1 list contract — cursor paging is stable under
	// concurrent inserts/deletes (unlike offset paging).
	ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*Media, error)
	// FindAllByCursor returns media in newest-first (id DESC) order using keyset
	// pagination. ownerID > 0 scopes the list to that user's library; 0 (the admin
	// scope) returns every user's media. beforeID <= 0 means "first page"; otherwise
	// only rows with a strictly smaller id are returned. Used by the browser admin
	// media library (admins manage all site media).
	FindAllByCursor(ctx context.Context, ownerID int, limit int, beforeID int) ([]*Media, error)
	// FindAllByFilenameByCursor returns media matching a filename query in
	// newest-first (id DESC) order using keyset pagination (beforeID <= 0 means
	// first page). ownerID > 0 scopes the search to that user's library; 0 (the
	// admin scope) searches every user's media.
	FindAllByFilenameByCursor(
		ctx context.Context,
		ownerID int,
		filename string,
		limit int,
		beforeID int,
	) ([]*Media, error)
	// FindAllByDateRangeByCursor returns media created since the given time in
	// newest-first (id DESC) order using keyset pagination (beforeID <= 0 means
	// first page). ownerID > 0 scopes the list to that user's library; 0 (the
	// admin scope) returns every user's media.
	FindAllByDateRangeByCursor(
		ctx context.Context,
		ownerID int,
		since time.Time,
		limit int,
		beforeID int,
	) ([]*Media, error)
	// FindAllByFilenameAndDateRangeByCursor returns media matching a filename
	// query and created since the given time in newest-first (id DESC) order
	// using keyset pagination (beforeID <= 0 means first page). ownerID > 0
	// scopes the search to that user's library; 0 (the admin scope) searches
	// every user's media.
	FindAllByFilenameAndDateRangeByCursor(
		ctx context.Context,
		ownerID int,
		filename string,
		since time.Time,
		limit int,
		beforeID int,
	) ([]*Media, error)
	// Count returns the total number of media matching the given owner scope,
	// filename query, and creation-time threshold. ownerID > 0 counts only that
	// user's media; 0 (the admin scope) counts all. since is the zero time when
	// no date filter applies. It mirrors the WHERE clauses of the FindAll*ByCursor
	// methods so a list's total always matches its rows.
	Count(ctx context.Context, ownerID int, search string, since time.Time) (int, error)
	DeleteByID(ctx context.Context, id int) error
	DeleteByOwner(ctx context.Context, id int, userID int) error
	// UpdateFileSize refreshes the stored file size after an in-place
	// rewrite (e.g. the one-time alpha-compression re-encode). Filenames,
	// URLs, and hashes are untouched — only the size column follows the
	// bytes actually on disk.
	UpdateFileSize(ctx context.Context, id int, size int64) error
}
