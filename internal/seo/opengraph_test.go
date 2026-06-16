package seo_test

import (
	"strings"
	"testing"

	appseo "github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/stretchr/testify/assert"
)

func TestDefaultOpenGraphTags(t *testing.T) {
	og := appseo.DefaultOpenGraphTags()

	assert.Equal(t, "website", og.Type, "DefaultOpenGraphTags() Type")
	assert.Equal(t, "Lesstruct", og.SiteName, "DefaultOpenGraphTags() SiteName")
}

func TestOpenGraphTags_ToMetaTags(t *testing.T) {
	tests := []struct {
		name string
		og   appseo.OpenGraphTags
		want []string
	}{
		{
			name: "all fields populated",
			og: appseo.OpenGraphTags{
				Title:       "Test Title",
				Description: "Test Description",
				Image:       "https://example.com/image.jpg",
				URL:         "https://example.com/page",
				Type:        "article",
				SiteName:    "Test Site",
			},
			want: []string{
				`<meta property="og:title" content="Test Title">`,
				`<meta property="og:description" content="Test Description">`,
				`<meta property="og:image" content="https://example.com/image.jpg">`,
				`<meta property="og:url" content="https://example.com/page">`,
				`<meta property="og:type" content="article">`,
				`<meta property="og:site_name" content="Test Site">`,
			},
		},
		{
			name: "only title",
			og: appseo.OpenGraphTags{
				Title: "Test Title",
			},
			want: []string{
				`<meta property="og:title" content="Test Title">`,
			},
		},
		{
			name: "empty tags",
			og:   appseo.OpenGraphTags{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.og.ToMetaTags()

			assert.Len(t, got, len(tt.want), "OpenGraphTags.ToMetaTags() returned wrong number of tags")

			for i, expected := range tt.want {
				if i >= len(got) {
					break
				}
				assert.Contains(t, string(got[i].Render()), expected, "OpenGraphTags.ToMetaTags() tag %d mismatch", i)
			}
		})
	}
}

func TestDefaultTwitterCardTags(t *testing.T) {
	tc := appseo.DefaultTwitterCardTags()

	assert.Equal(t, appseo.TwitterCardSummaryLargeImage, tc.Card, "DefaultTwitterCardTags() Card")
}

func TestTwitterCardTags_ToMetaTags(t *testing.T) {
	tests := []struct {
		name string
		tc   appseo.TwitterCardTags
		want []string
	}{
		{
			name: "all fields populated",
			tc: appseo.TwitterCardTags{
				Card:        appseo.TwitterCardSummaryLargeImage,
				Title:       "Test Title",
				Description: "Test Description",
				Image:       "https://example.com/image.jpg",
			},
			want: []string{
				`<meta name="twitter:card" content="summary_large_image">`,
				`<meta name="twitter:title" content="Test Title">`,
				`<meta name="twitter:description" content="Test Description">`,
				`<meta name="twitter:image" content="https://example.com/image.jpg">`,
			},
		},
		{
			name: "only card type",
			tc: appseo.TwitterCardTags{
				Card: appseo.TwitterCardSummary,
			},
			want: []string{
				`<meta name="twitter:card" content="summary">`,
			},
		},
		{
			name: "empty tags",
			tc:   appseo.TwitterCardTags{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tc.ToMetaTags()

			assert.Len(t, got, len(tt.want), "TwitterCardTags.ToMetaTags() returned wrong number of tags")

			for i, expected := range tt.want {
				if i >= len(got) {
					break
				}
				assert.Contains(t, string(got[i].Render()), expected, "TwitterCardTags.ToMetaTags() tag %d mismatch", i)
			}
		})
	}
}

func TestRenderAllSEOMetaTags(t *testing.T) {
	metaDesc := "Test meta description"
	og := appseo.OpenGraphTags{
		Title:       "OG Title",
		Description: "OG Description",
		Image:       "https://example.com/image.jpg",
		URL:         "https://example.com/page",
		Type:        "article",
		SiteName:    "Test Site",
	}
	tc := appseo.TwitterCardTags{
		Card:        appseo.TwitterCardSummaryLargeImage,
		Title:       "Twitter Title",
		Description: "Twitter Description",
		Image:       "https://example.com/twitter-image.jpg",
	}

	tags := appseo.RenderAllSEOMetaTags(metaDesc, og, tc)

	expectedTags := []string{
		`<meta name="description" content="Test meta description">`,
		`<meta property="og:title" content="OG Title">`,
		`<meta property="og:description" content="OG Description">`,
		`<meta property="og:image" content="https://example.com/image.jpg">`,
		`<meta property="og:url" content="https://example.com/page">`,
		`<meta property="og:type" content="article">`,
		`<meta property="og:site_name" content="Test Site">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`<meta name="twitter:title" content="Twitter Title">`,
		`<meta name="twitter:description" content="Twitter Description">`,
		`<meta name="twitter:image" content="https://example.com/twitter-image.jpg">`,
	}

	assert.Len(t, tags, len(expectedTags), "RenderAllSEOMetaTags() returned wrong number of tags")

	for i, expected := range expectedTags {
		if i >= len(tags) {
			break
		}
		assert.Contains(t, string(tags[i].Render()), expected, "RenderAllSEOMetaTags() tag %d mismatch", i)
	}
}

func TestRenderAllSEOMetaTagsAsHTML(t *testing.T) {
	metaDesc := "Test meta description"
	og := appseo.OpenGraphTags{
		Title:       "OG Title",
		Description: "OG Description",
	}
	tc := appseo.TwitterCardTags{
		Card:  appseo.TwitterCardSummary,
		Title: "Twitter Title",
	}

	html := appseo.RenderAllSEOMetaTagsAsHTML(metaDesc, og, tc)

	expectedContains := []string{
		`<meta name="description" content="Test meta description">`,
		`<meta property="og:title" content="OG Title">`,
		`<meta property="og:description" content="OG Description">`,
		`<meta name="twitter:card" content="summary">`,
		`<meta name="twitter:title" content="Twitter Title">`,
	}

	for _, expected := range expectedContains {
		assert.Contains(t, html, expected, "RenderAllSEOMetaTagsAsHTML() HTML missing expected tag")
	}

	lines := strings.Split(strings.TrimSpace(html), "\n")
	assert.Len(t, lines, len(expectedContains), "RenderAllSEOMetaTagsAsHTML() returned wrong number of lines")
}
