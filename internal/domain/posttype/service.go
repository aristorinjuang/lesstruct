package posttype

import (
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
)

type config struct {
	PostTypes  []PostType `toml:"post_type"`
	UserFields UserFields `toml:"user_fields"`
}

// Service manages post type registration and lookup
type Service struct {
	registry     map[string]PostType
	mu           sync.RWMutex
	defaultSlugs map[string]bool // Track default post type slugs to prevent overriding
	userFields   UserFields
}

// Register registers a new post type
func (s *Service) Register(pt PostType) error {
	if err := Validate(pt); err != nil {
		return err
	}

	// Prevent custom post types from overriding default post types
	s.mu.RLock()
	_, isDefault := s.defaultSlugs[pt.Slug]
	s.mu.RUnlock()

	if isDefault {
		return ErrDuplicatePostType
	}

	return s.registerUnsafe(pt)
}

// registerUnsafe registers a post type without validation (internal use)
func (s *Service) registerUnsafe(pt PostType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.registry[pt.Slug]; exists {
		return ErrDuplicatePostType
	}

	s.registry[pt.Slug] = pt
	return nil
}

// GetBySlug retrieves a post type by its slug
func (s *Service) GetBySlug(slug string) (PostType, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pt, exists := s.registry[slug]
	if !exists {
		return PostType{}, ErrPostTypeNotFound
	}

	return pt, nil
}

// GetAll returns all registered post types
func (s *Service) GetAll() []PostType {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PostType, 0, len(s.registry))
	for _, pt := range s.registry {
		result = append(result, pt)
	}

	return result
}

// IsSupported checks if a post type supports a specific feature
func (s *Service) IsSupported(pt PostType, feature string) bool {
	return slices.Contains(pt.Supports, feature)
}

// GetAllBySupports returns all post types that support the given feature
func (s *Service) GetAllBySupports(feature string) []PostType {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []PostType
	for _, pt := range s.registry {
		if s.IsSupported(pt, feature) {
			result = append(result, pt)
		}
	}

	return result
}

// GetFieldsByPostType returns the custom field schemas for a given post type slug
func (s *Service) GetFieldsByPostType(slug string) ([]customfield.FieldSchema, error) {
	pt, err := s.GetBySlug(slug)
	if err != nil {
		return nil, err
	}

	result := make([]customfield.FieldSchema, len(pt.Fields))
	copy(result, pt.Fields)
	return result, nil
}

// GetSystemFieldsByPostType returns the system field schemas for a given post type slug
func (s *Service) GetSystemFieldsByPostType(slug string) ([]customfield.FieldSchema, error) {
	pt, err := s.GetBySlug(slug)
	if err != nil {
		return nil, err
	}

	result := make([]customfield.FieldSchema, len(pt.SystemFields))
	copy(result, pt.SystemFields)
	return result, nil
}

// GetUserFields returns the custom user field schemas
func (s *Service) GetUserFields() []customfield.FieldSchema {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]customfield.FieldSchema, len(s.userFields.Fields))
	copy(result, s.userFields.Fields)
	return result
}

// GetUserSystemFields returns the system user field schemas
func (s *Service) GetUserSystemFields() []customfield.FieldSchema {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]customfield.FieldSchema, len(s.userFields.SystemFields))
	copy(result, s.userFields.SystemFields)
	return result
}

// GetUserSystemFieldSlugs returns only the slugs of system user fields
func (s *Service) GetUserSystemFieldSlugs() []string {
	fields := s.GetUserSystemFields()
	slugs := make([]string, len(fields))
	for i, f := range fields {
		slugs[i] = f.Slug
	}
	return slugs
}

// LoadConfigFromFile loads post types from a TOML configuration file
// If the file doesn't exist, it uses default post types (no error returned)
func (s *Service) LoadConfigFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use defaults (already loaded in NewService)
			return nil
		}
		return fmt.Errorf("reading post types config %s: %w", path, err)
	}

	var cfg config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing post types config %s: %w", path, err)
	}

	// Validate and register all post types from config
	for _, pt := range cfg.PostTypes {
		if err := s.Register(pt); err != nil {
			return fmt.Errorf("validating post types config %s: %w", path, err)
		}
	}

	if err := cfg.UserFields.Validate(); err != nil {
		return fmt.Errorf("validating user fields config %s: %w", path, err)
	}
	s.mu.Lock()
	s.userFields = cfg.UserFields
	s.mu.Unlock()

	return nil
}

// NewService creates a new post type service initialized with default post types
func NewService() *Service {
	s := &Service{
		registry:     make(map[string]PostType),
		defaultSlugs: make(map[string]bool),
	}

	// Register default post types
	defaultTypes := GetDefaultPostTypes()
	for _, pt := range defaultTypes {
		_ = s.registerUnsafe(pt)
		// Track default slugs to prevent overriding
		s.defaultSlugs[pt.Slug] = true
	}

	return s
}

// GetDefaultPostTypes returns the default post types
func GetDefaultPostTypes() []PostType {
	return []PostType{
		{
			Name:        "Post",
			Slug:        "post",
			Description: "Blog posts and articles",
			Supports:    []string{"title", "content", "tags", "featured_image"},
		},
		{
			Name:        "Page",
			Slug:        "page",
			Description: "Static pages",
			Supports:    []string{"title", "content", "featured_image"},
		},
		{
			Name:        "Media",
			Slug:        "media",
			Description: "Media library items",
			Supports:    []string{"title", "featured_image"},
		},
		{
			Name:        "Comment",
			Slug:        "comment",
			Description: "User comments",
			Supports:    []string{"content"},
		},
	}
}
