package posttype

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
)

var (
	// ErrInvalidPostTypeName is returned when post type name validation fails
	ErrInvalidPostTypeName = errors.New("post type name is required and must be between 1 and 200 characters")
	// ErrInvalidPostTypeSlug is returned when post type slug validation fails
	ErrInvalidPostTypeSlug = errors.New("post type slug must be between 1 and 200 characters and contain only lowercase letters, numbers, hyphens, and underscores")
	// ErrInvalidSupports is returned when supports validation fails
	ErrInvalidSupports = errors.New("supports must contain at least one valid feature")
	// ErrDuplicatePostType is returned when attempting to register a duplicate post type
	ErrDuplicatePostType = errors.New("post type with this slug already exists")
	// ErrPostTypeNotFound is returned when a post type is not found
	ErrPostTypeNotFound = errors.New("post type not found")
	// ErrInvalidPostTypeNameID is returned when the optional Indonesian name fails validation
	ErrInvalidPostTypeNameID = errors.New("post type name_id must be between 1 and 200 characters when set")
	// ErrInvalidPostTypeDescriptionID is returned when the optional Indonesian description fails validation
	ErrInvalidPostTypeDescriptionID = errors.New("post type description_id must be at most 500 characters when set")
)

// SupportedFeatures returns the list of valid feature names
var SupportedFeatures = []string{
	"title",
	"content",
	"tags",
	"featured_image",
	"excerpt",
}

// GetSupportedFeatures returns a copy of the supported features list
func GetSupportedFeatures() []string {
	features := make([]string, len(SupportedFeatures))
	copy(features, SupportedFeatures)
	return features
}

// UserFields holds global custom and system fields for user profiles
type UserFields struct {
	Fields       []customfield.FieldSchema `json:"fields,omitempty" toml:"fields,omitempty"`
	SystemFields []customfield.FieldSchema `json:"systemFields,omitempty" toml:"system_fields,omitempty"`
}

func (uf UserFields) Validate() error {
	if err := customfield.ValidateFields(uf.Fields); err != nil {
		return err
	}
	if err := customfield.ValidateFields(uf.SystemFields); err != nil {
		return err
	}
	return nil
}

// PostType represents a custom post type configuration
type PostType struct {
	Name          string                    `json:"name" toml:"name"`
	Slug          string                    `json:"slug" toml:"slug"`
	Description   string                    `json:"description,omitempty" toml:"description,omitempty"`
	NameID        string                    `json:"name_id,omitempty" toml:"name_id,omitempty"`
	DescriptionID string                    `json:"description_id,omitempty" toml:"description_id,omitempty"`
	Supports      []string                  `json:"supports" toml:"supports"`
	Fields        []customfield.FieldSchema `json:"fields,omitempty" toml:"fields,omitempty"`
	SystemFields  []customfield.FieldSchema `json:"systemFields,omitempty" toml:"system_fields,omitempty"`
	// Hidden hides the post type from the admin panel and the public post-type
	// API. The type stays fully registered (content validation, templates, and
	// the WordPress importer still resolve it) — hiding is presentation only.
	// Built-in types other than "post" (i.e. page, media, comment) cannot be
	// hidden; setting hidden=true on them is rejected at startup.
	Hidden bool `json:"hidden,omitempty" toml:"hidden,omitempty"`
}

// Validate validates the entire PostType struct
func Validate(pt PostType) error {
	if err := ValidateName(pt.Name); err != nil {
		return err
	}

	if err := ValidateSlug(pt.Slug); err != nil {
		return err
	}

	if err := ValidateSupports(pt.Supports); err != nil {
		return err
	}

	if err := ValidateNameID(pt.NameID); err != nil {
		return err
	}

	if err := ValidateDescriptionID(pt.DescriptionID); err != nil {
		return err
	}

	if err := customfield.ValidateFields(pt.Fields); err != nil {
		return err
	}

	if err := customfield.ValidateFields(pt.SystemFields); err != nil {
		return err
	}

	return nil
}

// ValidateName validates the post type name field
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 200 {
		return ErrInvalidPostTypeName
	}
	return nil
}

// ValidateSlug validates the post type slug field
func ValidateSlug(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" || utf8.RuneCountInString(slug) > 200 {
		return ErrInvalidPostTypeSlug
	}

	// Check for invalid characters
	for _, r := range slug {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return ErrInvalidPostTypeSlug
		}
	}

	// Slug cannot start or end with a hyphen
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return ErrInvalidPostTypeSlug
	}

	// Slug cannot have consecutive hyphens
	if strings.Contains(slug, "--") {
		return ErrInvalidPostTypeSlug
	}

	return nil
}

// ValidateSupports validates the supports array
func ValidateSupports(supports []string) error {
	if len(supports) == 0 {
		return ErrInvalidSupports
	}

	featureMap := make(map[string]bool)
	for _, f := range SupportedFeatures {
		featureMap[f] = true
	}

	for _, support := range supports {
		if !featureMap[support] {
			return ErrInvalidSupports
		}
	}

	return nil
}

// ValidateNameID validates the optional Indonesian name override. Empty is
// allowed (falls back to Name); when set it must be 1-200 characters.
func ValidateNameID(nameID string) error {
	trimmed := strings.TrimSpace(nameID)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > 200 {
		return ErrInvalidPostTypeNameID
	}
	return nil
}

// ValidateDescriptionID validates the optional Indonesian description override.
// Empty is allowed (falls back to Description); when set it must be at most
// 500 characters.
func ValidateDescriptionID(descriptionID string) error {
	trimmed := strings.TrimSpace(descriptionID)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > 500 {
		return ErrInvalidPostTypeDescriptionID
	}
	return nil
}
