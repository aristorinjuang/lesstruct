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
	// FindAllByCursor returns ALL media (not user-scoped) in newest-first (id DESC) order
	// using keyset pagination. beforeID <= 0 means "first page"; otherwise only rows with a
	// strictly smaller id are returned. Global counterpart of ListByCursor, used by the
	// browser admin media library (admins manage all site media).
	FindAllByCursor(ctx context.Context, limit int, beforeID int) ([]*Media, error)
	// FindAllByFilenameByCursor returns ALL media matching a filename query in newest-first
	// (id DESC) order using keyset pagination (beforeID <= 0 means first page).
	FindAllByFilenameByCursor(
		ctx context.Context,
		filename string,
		limit int,
		beforeID int,
	) ([]*Media, error)
	// FindAllByDateRangeByCursor returns ALL media created since the given time in
	// newest-first (id DESC) order using keyset pagination (beforeID <= 0 means first page).
	FindAllByDateRangeByCursor(
		ctx context.Context,
		since time.Time,
		limit int,
		beforeID int,
	) ([]*Media, error)
	// FindAllByFilenameAndDateRangeByCursor returns ALL media matching a filename query and
	// created since the given time in newest-first (id DESC) order using keyset pagination
	// (beforeID <= 0 means first page).
	FindAllByFilenameAndDateRangeByCursor(
		ctx context.Context,
		filename string,
		since time.Time,
		limit int,
		beforeID int,
	) ([]*Media, error)
	// Count returns the total number of ALL media (not user-scoped) matching the given
	// filename query and creation-time threshold. since is the zero time when no date
	// filter applies. It mirrors the WHERE clauses of the FindAll*ByCursor methods so a
	// list's total always matches its rows.
	Count(ctx context.Context, search string, since time.Time) (int, error)
	DeleteByID(ctx context.Context, id int) error
	DeleteByOwner(ctx context.Context, id int, userID int) error
}
