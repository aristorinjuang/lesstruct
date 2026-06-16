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
	FindAll(ctx context.Context, limit int, offset int) ([]*Media, error)
	// ListByCursor returns the caller's media in newest-first (id DESC) order using keyset
	// pagination. beforeID <= 0 means "first page"; otherwise only rows with a strictly
	// smaller id are returned. Additive — the offset-based FindAll is untouched (the agent
	// v1 list contract is cursor-only; offset is unstable under concurrent inserts/deletes).
	ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*Media, error)
	FindAllByFilename(
		ctx context.Context,
		filename string,
		limit int,
		offset int,
	) ([]*Media, error)
	FindAllByDateRange(
		ctx context.Context,
		since time.Time,
		limit int,
		offset int,
	) ([]*Media, error)
	FindAllByFilenameAndDateRange(
		ctx context.Context,
		filename string,
		since time.Time,
		limit int,
		offset int,
	) ([]*Media, error)
	DeleteByID(ctx context.Context, id int) error
	DeleteByOwner(ctx context.Context, id int, userID int) error
}
