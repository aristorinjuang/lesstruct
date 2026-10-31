package content

import (
	"fmt"
	"slices"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
)

// PostTypeAdapter wraps posttype.Service to implement content.PostTypeServiceInterface
type PostTypeAdapter struct {
	service *posttype.Service
}

// GetBySlug retrieves a post type by slug and converts it to content.PostType
func (a *PostTypeAdapter) GetBySlug(slug string) (PostType, error) {
	pt, err := a.service.GetBySlug(slug)
	if err != nil {
		return PostType{}, fmt.Errorf("post type not found: %w", err)
	}
	return PostType{
		Slug: pt.Slug,
	}, nil
}

func (a *PostTypeAdapter) GetFieldsByPostType(slug string) ([]customfield.FieldSchema, error) {
	return a.service.GetFieldsByPostType(slug)
}

func (a *PostTypeAdapter) GetSystemFieldsByPostType(slug string) ([]customfield.FieldSchema, error) {
	return a.service.GetSystemFieldsByPostType(slug)
}

// SearchableSlugs projects the registered post types into the public-search
// scope: every non-hidden slug, sorted so queries and tests are deterministic.
func (a *PostTypeAdapter) SearchableSlugs() []string {
	slugs := make([]string, 0)
	for _, pt := range a.service.GetAll() {
		if !pt.Hidden {
			slugs = append(slugs, pt.Slug)
		}
	}
	slices.Sort(slugs)
	return slugs
}

// NewPostTypeAdapter creates a new adapter for post type service
func NewPostTypeAdapter(service *posttype.Service) *PostTypeAdapter {
	return &PostTypeAdapter{
		service: service,
	}
}
