package seo

import (
	"strings"
)

// OpenGraphTags represents Open Graph meta tags
type OpenGraphTags struct {
	Title       string
	Description string
	Image       string
	URL         string
	Type        string
	SiteName    string
}

// ToMetaTags converts Open Graph tags to meta tags
func (og OpenGraphTags) ToMetaTags() []MetaTag {
	var tags []MetaTag

	if og.Title != "" {
		tags = append(tags, NewPropertyMetaTag("og:title", og.Title))
	}

	if og.Description != "" {
		tags = append(tags, NewPropertyMetaTag("og:description", og.Description))
	}

	if og.Image != "" {
		tags = append(tags, NewPropertyMetaTag("og:image", og.Image))
	}

	if og.URL != "" {
		tags = append(tags, NewPropertyMetaTag("og:url", og.URL))
	}

	if og.Type != "" {
		tags = append(tags, NewPropertyMetaTag("og:type", og.Type))
	}

	if og.SiteName != "" {
		tags = append(tags, NewPropertyMetaTag("og:site_name", og.SiteName))
	}

	return tags
}

// DefaultOpenGraphTags returns default Open Graph tags
func DefaultOpenGraphTags() OpenGraphTags {
	return OpenGraphTags{
		Type:     "website",
		SiteName: "Lesstruct",
	}
}

// TwitterCardType represents Twitter card types
type TwitterCardType string

const (
	// TwitterCardSummary is the summary card type
	TwitterCardSummary TwitterCardType = "summary"
	// TwitterCardSummaryLargeImage is the summary card with large image type
	TwitterCardSummaryLargeImage TwitterCardType = "summary_large_image"
)

// TwitterCardTags represents Twitter Card meta tags
type TwitterCardTags struct {
	Card        TwitterCardType
	Title       string
	Description string
	Image       string
}

// ToMetaTags converts Twitter Card tags to meta tags
func (tc TwitterCardTags) ToMetaTags() []MetaTag {
	var tags []MetaTag

	if tc.Card != "" {
		tags = append(tags, NewMetaTag("twitter:card", string(tc.Card)))
	}

	if tc.Title != "" {
		tags = append(tags, NewMetaTag("twitter:title", tc.Title))
	}

	if tc.Description != "" {
		tags = append(tags, NewMetaTag("twitter:description", tc.Description))
	}

	if tc.Image != "" {
		tags = append(tags, NewMetaTag("twitter:image", tc.Image))
	}

	return tags
}

// DefaultTwitterCardTags returns default Twitter Card tags
func DefaultTwitterCardTags() TwitterCardTags {
	return TwitterCardTags{
		Card: TwitterCardSummaryLargeImage,
	}
}

// RenderAllSEOMetaTags renders all SEO meta tags (description, Open Graph, Twitter Card)
func RenderAllSEOMetaTags(metaDescription string, og OpenGraphTags, tc TwitterCardTags) []MetaTag {
	var tags []MetaTag

	if metaDescription != "" {
		tags = append(tags, NewMetaTag("description", metaDescription))
	}

	tags = append(tags, og.ToMetaTags()...)
	tags = append(tags, tc.ToMetaTags()...)

	return tags
}

// RenderAllSEOMetaTagsAsHTML renders all SEO meta tags as HTML
func RenderAllSEOMetaTagsAsHTML(metaDescription string, og OpenGraphTags, tc TwitterCardTags) string {
	tags := RenderAllSEOMetaTags(metaDescription, og, tc)
	var sb strings.Builder

	for _, tag := range tags {
		sb.WriteString(string(tag.Render()))
		sb.WriteString("\n")
	}

	return sb.String()
}
