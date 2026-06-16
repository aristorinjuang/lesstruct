package content

import (
	"context"
)

// Repository defines the interface for content repository operations
type Repository interface {
	Create(ctx context.Context, content *Content) error
	GetBySlug(ctx context.Context, slug string, language string) (*Content, error)
	GetByUser(ctx context.Context, userID int, limit int, offset int) ([]*Content, error)
	GetAll(ctx context.Context, limit int, offset int) ([]*Content, error)
	// ListByCursor returns the caller's content in newest-first (id DESC) order using
	// keyset pagination. beforeID <= 0 means "first page"; otherwise only rows with a
	// strictly smaller id are returned. The optional filters restrict which rows
	// qualify: empty string / nil fields are no-ops, so a zero ContentFilters value
	// behaves exactly as the old unfiltered call. Tags is AND-of-tags. Author matches
	// the joined users.name (with users.username as a fallback), case-insensitive.
	// It is additive — the offset-based GetAll / GetByUser / ListByFilters methods are
	// untouched (the agent v1 list contract is cursor-only; offset is unstable under
	// concurrent inserts/deletes).
	ListByCursor(ctx context.Context, userID int, limit int, beforeID int, filters ContentFilters) ([]*Content, error)
	CheckSlugUnique(ctx context.Context, slug string, language string) (bool, error)
	GetByID(ctx context.Context, id int) (*Content, error)
	Update(ctx context.Context, content *Content) error
	GetPublished(ctx context.Context, limit int, offset int) ([]*Content, error)
	GetPublishedBySlug(ctx context.Context, slug string, language string) (*Content, error)
	GetPublishedByAuthorUsername(ctx context.Context, username string, limit int, offset int) ([]*Content, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	Delete(ctx context.Context, id int, userID int) error
	DeleteByID(ctx context.Context, id int) error
	ListByFilters(ctx context.Context, userID int, filters ContentFilters) ([]*Content, error)
	GetPublishedPages(ctx context.Context) ([]*Content, error)
	GetPublishedCustomPostTypes(ctx context.Context) ([]string, error)
	GetPublishedByPostType(ctx context.Context, postType string, limit int, offset int) ([]*Content, error)
	GetPublishedByTag(ctx context.Context, tag string, limit int, offset int) ([]*Content, error)
	SearchPublished(ctx context.Context, query string, limit int) ([]*Content, error)
	// GetTranslations returns all content items in the same translation group, excluding the given content ID.
	GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*Content, error)
	// TranslationGroupExists checks whether a content item with the given ID exists.
	TranslationGroupExists(ctx context.Context, id int) (bool, error)
	// GetPublishedBySlugAny finds published content by slug regardless of language.
	GetPublishedBySlugAny(ctx context.Context, slug string) (*Content, error)
}
