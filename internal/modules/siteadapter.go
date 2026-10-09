package modules

import (
	"context"

	contentservice "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentdomain "github.com/aristorinjuang/lesstruct/kernel/content"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
)

// projectContent maps a content entity onto the site-ssr projection: every
// field the assembler reads, nothing it does not. Identity, ownership, and
// moderation state stay behind the seam by construction.
func projectContent(item *contentdomain.Content) sitessr.ContentRef {
	return sitessr.ContentRef{
		ID:                 item.ID,
		Slug:               item.Slug,
		Title:              item.Title,
		Content:            item.Content,
		Tags:               item.Tags,
		Format:             string(item.Format),
		PostType:           item.PostType,
		Language:           item.Language,
		TranslationGroupID: item.TranslationGroupID,
		Author:             item.Author,
		Username:           item.Username,
		CreatedAt:          item.CreatedAt,
		AllowComments:      item.AllowComments,
		CustomFields:       item.CustomFields,
		MetaDescription:    item.MetaDescription,
		OGTitle:            item.OGTitle,
		OGDescription:      item.OGDescription,
	}
}

func projectContents(items []*contentdomain.Content) []sitessr.ContentRef {
	refs := make([]sitessr.ContentRef, 0, len(items))
	for _, item := range items {
		refs = append(refs, projectContent(item))
	}
	return refs
}

// siteSSRContentReader adapts the content service (plus the comments module
// service) to the site-ssr ContentReader seam. It lives in core — the only
// layer that may import both sides.
type siteSSRContentReader struct {
	content  *contentservice.Service
	comments *commentdomain.Service
}

func (r *siteSSRContentReader) GetPublished(ctx context.Context, limit, offset int) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublished(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedPages(ctx context.Context) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedPages(ctx)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedCustomPostTypes(ctx context.Context) ([]string, error) {
	return r.content.GetPublishedCustomPostTypes(ctx)
}

func (r *siteSSRContentReader) GetTranslations(ctx context.Context, groupID, excludeID int) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetTranslations(ctx, groupID, excludeID)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedByID(ctx context.Context, id int) (sitessr.ContentRef, error) {
	item, err := r.content.GetPublishedByID(ctx, id)
	if err != nil {
		return sitessr.ContentRef{}, err
	}
	return projectContent(item), nil
}

func (r *siteSSRContentReader) GetPublishedByPostType(
	ctx context.Context,
	postType string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedByPostType(ctx, postType, languages, year, month, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedTags(ctx context.Context) ([]string, error) {
	return r.content.GetPublishedTags(ctx)
}

func (r *siteSSRContentReader) GetPublishedBySlugAny(ctx context.Context, slug string) (sitessr.ContentRef, error) {
	item, err := r.content.GetPublishedBySlugAny(ctx, slug)
	if err != nil {
		return sitessr.ContentRef{}, err
	}
	return projectContent(item), nil
}

func (r *siteSSRContentReader) GetCommentsForContent(ctx context.Context, contentID int) ([]*commentdomain.Comment, error) {
	return r.comments.GetCommentsForContent(ctx, contentID)
}

func (r *siteSSRContentReader) GetRelated(ctx context.Context, id, limit int) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetRelated(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) GetPublishedByAuthorUsername(
	ctx context.Context,
	username string,
	languages []string,
	limit, offset int,
) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedByAuthorUsername(ctx, username, languages, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

func (r *siteSSRContentReader) AuthorExists(ctx context.Context, username string) (bool, error) {
	return r.content.AuthorExists(ctx, username)
}

func (r *siteSSRContentReader) GetPublishedByTag(
	ctx context.Context,
	tag string,
	languages []string,
	year, month, limit, offset int,
) ([]sitessr.ContentRef, error) {
	items, err := r.content.GetPublishedByTag(ctx, tag, languages, year, month, limit, offset)
	if err != nil {
		return nil, err
	}
	return projectContents(items), nil
}

var _ sitessr.ContentReader = (*siteSSRContentReader)(nil)

// NewSiteSSRContentReader adapts the content and comments services to the
// site-ssr ContentReader seam.
func NewSiteSSRContentReader(
	content *contentservice.Service,
	comments *commentdomain.Service,
) sitessr.ContentReader {
	return &siteSSRContentReader{
		content:  content,
		comments: comments,
	}
}
