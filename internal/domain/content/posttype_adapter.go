package content

import (
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
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

// NewPostTypeAdapter creates a new adapter for post type service
func NewPostTypeAdapter(service *posttype.Service) *PostTypeAdapter {
	return &PostTypeAdapter{
		service: service,
	}
}
