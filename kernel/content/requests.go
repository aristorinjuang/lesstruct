package content

import (
	"time"
)

// CreateContentRequest represents a request to create content
type CreateContentRequest struct {
	Title              string         `json:"title"`
	Slug               string         `json:"slug,omitempty"`
	Content            string         `json:"content"`
	Tags               []string       `json:"tags"`
	Status             Status         `json:"status"`
	Format             Format         `json:"format,omitempty"`
	PostType           string         `json:"postType"`
	MetaDescription    string         `json:"metaDescription,omitempty"`
	OGTitle            string         `json:"ogTitle,omitempty"`
	OGDescription      string         `json:"ogDescription,omitempty"`
	FeaturedImage      string         `json:"featuredImage,omitempty"`
	AllowComments      *bool          `json:"allowComments,omitempty"`
	CustomFields       map[string]any `json:"customFields,omitempty"`
	Language           string         `json:"language,omitempty"`
	TranslationGroupID *int           `json:"translationGroupId,omitempty"`
	PublishedAt        *time.Time     `json:"publishedAt,omitempty"`
}
