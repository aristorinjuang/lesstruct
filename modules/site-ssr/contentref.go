// Package sitessr is the official public-site SSR engine module: contentpage
// data assembly, template resolution, and the HTTP surfaces that render the
// content site (plus its auth pages).
package sitessr

import (
	"context"
	"time"

	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
)

// FormatHTML marks content stored as raw HTML with inline styles. It mirrors
// the content domain's format constant so the assembler branches identically
// without importing the content domain.
const FormatHTML = "html"

// ContentRef is the sitemap- and page-relevant projection of a published
// content item: everything the assembler reads, nothing it does not. Core
// projects content items into it so this package never imports the content
// domain.
type ContentRef struct {
	ID                 int
	Slug               string
	Title              string
	Content            string
	Tags               []string
	Format             string
	PostType           string
	Language           string
	TranslationGroupID *int
	Author             string
	Username           string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	AllowComments      bool
	CustomFields       map[string]any
	MetaDescription    string
	OGTitle            string
	OGDescription      string
	FeaturedImage      string
}

// ContentReader is the published-content source the assembler renders from.
// Core satisfies it with an adapter over the content service (which cannot
// be imported here); the comments reader reuses the comments module's own
// domain. Tests stub it with hand doubles.
type ContentReader interface {
	GetPublished(ctx context.Context, limit int, offset int) ([]ContentRef, error)
	GetPublishedPages(ctx context.Context) ([]ContentRef, error)
	GetPublishedCustomPostTypes(ctx context.Context) ([]string, error)
	GetTranslations(ctx context.Context, groupID, excludeID int) ([]ContentRef, error)
	GetPublishedByID(ctx context.Context, id int) (ContentRef, error)
	GetPublishedByPostType(
		ctx context.Context,
		postType string,
		languages []string,
		year, month, limit, offset int,
	) ([]ContentRef, error)
	GetPublishedTags(ctx context.Context) ([]string, error)
	GetPublishedBySlugAny(ctx context.Context, slug string) (ContentRef, error)
	GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error)
	GetRelated(ctx context.Context, id, limit int) ([]ContentRef, error)
	GetPublishedByAuthorUsername(
		ctx context.Context,
		username string,
		languages []string,
		limit, offset int,
	) ([]ContentRef, error)
	AuthorExists(ctx context.Context, username string) (bool, error)
	GetPublishedByTag(
		ctx context.Context,
		tag string,
		languages []string,
		year, month, limit, offset int,
	) ([]ContentRef, error)
}
