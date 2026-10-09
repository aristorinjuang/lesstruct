package seo

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidMetaDescription is returned when meta description validation fails
	ErrInvalidMetaDescription = errors.New("meta description must be between 1 and 160 characters")
	// ErrInvalidOGTitle is returned when OG title validation fails
	ErrInvalidOGTitle = errors.New("og title must be between 1 and 60 characters")
	// ErrInvalidOGDescription is returned when OG description validation fails
	ErrInvalidOGDescription = errors.New("og description must be between 1 and 160 characters")
)

// SEOMetadata represents SEO metadata for content
type SEOMetadata struct {
	MetaDescription string `json:"metaDescription"`
	OGTitle         string `json:"ogTitle"`
	OGDescription   string `json:"ogDescription"`
}

// ValidateMetaDescription validates the meta description field
func ValidateMetaDescription(metaDescription string) error {
	metaDescription = strings.TrimSpace(metaDescription)
	if metaDescription == "" || utf8.RuneCountInString(metaDescription) > 160 {
		return ErrInvalidMetaDescription
	}
	return nil
}

// ValidateOGTitle validates the OG title field
func ValidateOGTitle(ogTitle string) error {
	ogTitle = strings.TrimSpace(ogTitle)
	if ogTitle == "" || utf8.RuneCountInString(ogTitle) > 60 {
		return ErrInvalidOGTitle
	}
	return nil
}

// ValidateOGDescription validates the OG description field
func ValidateOGDescription(ogDescription string) error {
	ogDescription = strings.TrimSpace(ogDescription)
	if ogDescription == "" || utf8.RuneCountInString(ogDescription) > 160 {
		return ErrInvalidOGDescription
	}
	return nil
}
