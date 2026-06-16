package content

import (
	"errors"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
)

// PostTypeServiceInterface defines the interface for post type service
// This allows the content service to validate post types without tight coupling
type PostTypeServiceInterface interface {
	GetBySlug(slug string) (PostType, error)
	GetFieldsByPostType(slug string) ([]customfield.FieldSchema, error)
	GetSystemFieldsByPostType(slug string) ([]customfield.FieldSchema, error)
}

// PostType represents a minimal post type for validation purposes
type PostType struct {
	Slug string
}

// ErrPostTypeNotFound is returned when a post type is not found
var ErrPostTypeNotFound = errors.New("post type not found")
