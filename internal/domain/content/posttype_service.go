package content

import (
	"errors"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
)

// PostTypeServiceInterface defines the interface for post type service
// This allows the content service to validate post types without tight coupling
type PostTypeServiceInterface interface {
	GetBySlug(slug string) (PostType, error)
	GetFieldsByPostType(slug string) ([]customfield.FieldSchema, error)
	GetSystemFieldsByPostType(slug string) ([]customfield.FieldSchema, error)
	// SearchableSlugs returns the slugs of all registered post types that are
	// not hidden — the scope public search runs over. Hidden types stay out of
	// every presentation surface, search included.
	SearchableSlugs() []string
}

// PostType represents a minimal post type for validation purposes
type PostType struct {
	Slug string
}

// ErrPostTypeNotFound is returned when a post type is not found
var ErrPostTypeNotFound = errors.New("post type not found")
