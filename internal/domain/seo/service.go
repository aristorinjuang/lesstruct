package seo

import (
	"fmt"

	"github.com/aristorinjuang/lesstruct/internal/seo"
)

// GenerateInput contains input data for SEO metadata generation
type GenerateInput struct {
	Title           string
	Content         string // TipTap JSON or HTML (see Format)
	Format          string // "html" → HTML extraction; anything else (including "") → TipTap extraction
	FeaturedImage   string
	URL             string
	DatePublished   string
	DateModified    string
	AuthorName      string
	Tags            []string
	MetaDescription string // Optional override
	OGTitle         string // Optional override
	OGDescription   string // Optional override
}

// GeneratedMetadata contains all generated SEO metadata
type GeneratedMetadata struct {
	MetaDescription    string
	OGTitle            string
	OGDescription      string
	OGImage            string
	OGURL              string
	OGType             string
	OGSiteName         string
	TwitterCard        string
	TwitterTitle       string
	TwitterDescription string
	TwitterImage       string
	JSONLD             map[string]any
}

// Service handles SEO metadata generation
type Service struct {
	baseURL  string
	siteName string
}

// Generate generates SEO metadata from the input data
func (s *Service) Generate(input GenerateInput) (*GeneratedMetadata, error) {
	// Extract plain text and first image URL in a format-aware way. Content
	// in "html" format (e.g. WordPress Elementor pages stored via the HTML
	// content format) uses tag-stripping and regex-based extraction; any
	// other format (including empty, the default) assumes TipTap JSON.
	// This branching is required because the HTML content format stores raw
	// HTML — calling the TipTap extractor on it silently returns empty,
	// which causes SEO meta-description validation to fail and the entire
	// import/publish to be skipped.
	var plainText, imageURL string
	if input.Format == "html" {
		plainText = seo.ExtractPlainTextFromHTML(input.Content)
		imageURL = seo.ExtractImageURLFromHTML(input.Content)
	} else {
		plainText = seo.ExtractPlainText(input.Content)
		imageURL = seo.ExtractImageURL(input.Content)
	}

	if input.FeaturedImage != "" {
		imageURL = input.FeaturedImage
	}

	metaDescription := input.MetaDescription
	if metaDescription == "" {
		metaDescription = seo.TruncateText(plainText, 160)
	}

	if err := ValidateMetaDescription(metaDescription); err != nil {
		return nil, fmt.Errorf("meta description validation failed: %w", err)
	}

	ogTitle := input.OGTitle
	if ogTitle == "" {
		ogTitle = input.Title
	}

	if err := ValidateOGTitle(ogTitle); err != nil {
		return nil, fmt.Errorf("og title validation failed: %w", err)
	}

	ogDescription := input.OGDescription
	if ogDescription == "" {
		ogDescription = seo.TruncateText(plainText, 160)
	}

	if err := ValidateOGDescription(ogDescription); err != nil {
		return nil, fmt.Errorf("og description validation failed: %w", err)
	}

	ogImage := imageURL
	if ogImage != "" {
		ogImage = seo.BuildURL(s.baseURL, ogImage)
	}

	ogURL := input.URL
	if ogURL != "" {
		ogURL = seo.BuildURL(s.baseURL, ogURL)
	}

	twitterImage := imageURL
	if twitterImage != "" {
		twitterImage = seo.BuildURL(s.baseURL, twitterImage)
	}

	authorName := input.AuthorName
	if authorName == "" {
		authorName = "Author"
	}

	keywords := ""
	if len(input.Tags) > 0 {
		for i, tag := range input.Tags {
			if i > 0 {
				keywords += ", "
			}
			keywords += tag
		}
	}

	jsonLD := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "Article",
		"headline":      input.Title,
		"description":   metaDescription,
		"datePublished": input.DatePublished,
		"dateModified":  input.DateModified,
		"author": map[string]any{
			"@type": "Person",
			"name":  authorName,
		},
	}

	if keywords != "" {
		jsonLD["keywords"] = keywords
	}

	if imageURL != "" {
		jsonLD["image"] = seo.BuildURL(s.baseURL, imageURL)
	}

	return &GeneratedMetadata{
		MetaDescription:    metaDescription,
		OGTitle:            ogTitle,
		OGDescription:      ogDescription,
		OGImage:            ogImage,
		OGURL:              ogURL,
		OGType:             "article",
		OGSiteName:         s.siteName,
		TwitterCard:        "summary_large_image",
		TwitterTitle:       ogTitle,
		TwitterDescription: ogDescription,
		TwitterImage:       twitterImage,
		JSONLD:             jsonLD,
	}, nil
}

// NewService creates a new SEO service
func NewService(baseURL, siteName string) *Service {
	return &Service{
		baseURL:  baseURL,
		siteName: siteName,
	}
}
