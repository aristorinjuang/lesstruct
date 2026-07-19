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
	// GetPublishedByAuthorUsername returns published content by the given author
	// username. When language is non-empty, results are additionally restricted
	// to that language at the database level (used by the paginated public
	// author page so the fetch-limit+1 HasMore probe stays accurate). An empty
	// language returns content in every language.
	GetPublishedByAuthorUsername(ctx context.Context, username string, language string, limit int, offset int) ([]*Content, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	Delete(ctx context.Context, id int, userID int) error
	DeleteByID(ctx context.Context, id int) error
	ListByFilters(ctx context.Context, userID int, filters ContentFilters) ([]*Content, error)
	GetPublishedPages(ctx context.Context) ([]*Content, error)
	GetPublishedCustomPostTypes(ctx context.Context) ([]string, error)
	// GetPublishedByPostType returns published content of the given post type.
	// When language is non-empty, results are additionally restricted to that
	// language at the database level. An empty language returns every language.
	// When year and month are both non-zero, results are restricted to that
	// calendar month.
	GetPublishedByPostType(ctx context.Context, postType string, language string, year int, month int, limit int, offset int) ([]*Content, error)
	// GetPublishedByTag returns published content carrying the given tag. When
	// language is non-empty, results are additionally restricted to that
	// language at the database level. An empty language returns every language.
	// When year and month are both non-zero, results are restricted to that
	// calendar month.
	GetPublishedByTag(ctx context.Context, tag string, language string, year int, month int, limit int, offset int) ([]*Content, error)
	// GetPublishedTags returns the distinct set of tags used by any published
	// content item, ordered for stable display. An empty (non-nil) slice is
	// returned when no published content carries tags.
	GetPublishedTags(ctx context.Context) ([]string, error)
	// GetPublishedAuthors returns the users who have at least one published
	// content item, aggregated with their published-content count and the
	// distinct post types they publish under. Results are ordered by content
	// count (desc) then username (asc). Only safe display fields are populated
	// (username, display name, profile picture filename) — never email, role,
	// or custom fields. The handler resolves the profile-picture filename into
	// a public URL.
	GetPublishedAuthors(ctx context.Context, limit int, offset int) ([]*PublishedAuthor, error)
	// GetPublishedArchive returns published-content counts grouped by year and
	// month, ordered newest-first. When postType is non-empty, results are
	// restricted to that post type; when language is non-empty, to that language.
	// An empty postType aggregates across all types.
	GetPublishedArchive(ctx context.Context, postType string, language string) ([]*ArchiveMonth, error)
	SearchPublished(ctx context.Context, query string, limit int) ([]*Content, error)
	// GetTranslations returns all content items in the same translation group, excluding the given content ID.
	GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*Content, error)
	// TranslationGroupExists checks whether a content item with the given ID exists.
	TranslationGroupExists(ctx context.Context, id int) (bool, error)
	// GetPublishedBySlugAny finds published content by slug regardless of language.
	GetPublishedBySlugAny(ctx context.Context, slug string) (*Content, error)
	// GetRelatedByTags returns published content of the same post type and language
	// that shares at least one tag with the given tags, excluding excludeID. Results
	// are ranked by the number of shared tags (descending) then by created_at
	// (descending). An empty tags slice yields no rows.
	GetRelatedByTags(ctx context.Context, excludeID int, tags []string, postType string, language string, limit int) ([]*Content, error)
	// GetLatestByPostType returns the most recently created published content of the
	// given post type and language, excluding excludeID, ordered by created_at desc.
	GetLatestByPostType(ctx context.Context, excludeID int, postType string, language string, limit int) ([]*Content, error)
}
