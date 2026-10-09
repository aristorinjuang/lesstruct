package modules

import (
	"context"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	seoapi "github.com/aristorinjuang/lesstruct/modules/seo/api"
)

// NewSEOPublishedPages adapts the content service to the seo module's
// sitemap seam: published items in, sitemap-relevant projections out. It
// lives in core — the only layer that may import both sides — until a later
// phase moves the seam. Lookup failures propagate for the handler's 500 path.
func NewSEOPublishedPages(content *contentdomain.Service) seoapi.PublishedPages {
	return func(ctx context.Context, limit, offset int) ([]seoapi.PageRef, error) {
		items, err := content.GetPublished(ctx, limit, offset)
		if err != nil {
			return nil, err
		}
		pages := make([]seoapi.PageRef, 0, len(items))
		for _, item := range items {
			pages = append(pages, seoapi.PageRef{
				ID:                 item.ID,
				Slug:               item.Slug,
				PostType:           item.PostType,
				Language:           item.Language,
				TranslationGroupID: item.TranslationGroupID,
				UpdatedAt:          item.UpdatedAt,
			})
		}
		return pages, nil
	}
}
