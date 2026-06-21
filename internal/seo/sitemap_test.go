package seo

import (
	"strings"
	"testing"
)

func TestRenderSitemapXML(t *testing.T) {
	entries := []SitemapEntry{
		NewHomepageEntry("http://localhost:3000"),
		ContentToSitemapEntry("http://localhost:3000", ContentItem{Slug: "my-post", UpdatedAt: "2026-04-18T10:00:00Z"}, "post"),
		ContentToSitemapEntry("http://localhost:3000", ContentItem{Slug: "about", UpdatedAt: "2026-04-15T08:00:00Z"}, "page"),
	}
	// A page with two published translations (en + id) → hreflang alternates.
	translated := ContentToSitemapEntry("http://localhost:3000", ContentItem{Slug: "about", UpdatedAt: "2026-04-15T08:00:00Z"}, "page")
	translated.Alternates = []SitemapAlternate{
		{Hreflang: "en", Href: "http://localhost:3000/about"},
		{Hreflang: "id", Href: "http://localhost:3000/tentang"},
	}
	entries = append(entries, translated)

	body := string(RenderSitemapXML(entries))

	if !strings.HasPrefix(body, `<?xml`) {
		t.Errorf("expected XML declaration first, got %q", body)
	}
	if !strings.Contains(body, `xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`) {
		t.Errorf("expected the sitemaps.org namespace, got %q", body)
	}
	if !strings.Contains(body, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`) {
		t.Errorf("expected the xhtml namespace for hreflang, got %q", body)
	}
	if !strings.Contains(body, "<loc>http://localhost:3000/my-post</loc>") {
		t.Errorf("expected the post loc in the XML, got %q", body)
	}
	if !strings.Contains(body, "<priority>0.8</priority>") {
		t.Errorf("expected the post priority element, got %q", body)
	}
	if strings.Contains(body, "/posts/") {
		t.Errorf("sitemap XML must not emit /posts/ URLs, got %q", body)
	}
	// hreflang alternates render as <xhtml:link rel="alternate" ...> children
	// (the opening tag is asserted without the close, so it matches both the
	// self-closing and explicit-close forms encoding/xml may emit).
	if !strings.Contains(body, `<xhtml:link rel="alternate" hreflang="en" href="http://localhost:3000/about"`) {
		t.Errorf("expected an en hreflang alternate, got %q", body)
	}
	if !strings.Contains(body, `<xhtml:link rel="alternate" hreflang="id" href="http://localhost:3000/tentang"`) {
		t.Errorf("expected an id hreflang alternate, got %q", body)
	}
}

func TestContentToSitemapEntry(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		content  ContentItem
		postType string
		wantLoc  string
	}{
		{
			name:     "post type generates root URL (served at /<slug>, not /posts/<slug>)",
			baseURL:  "http://localhost:3000",
			content:  ContentItem{Slug: "my-post", UpdatedAt: "2026-04-18T10:00:00Z"},
			postType: "post",
			wantLoc:  "http://localhost:3000/my-post",
		},
		{
			name:     "page type generates root URL",
			baseURL:  "http://localhost:3000",
			content:  ContentItem{Slug: "about", UpdatedAt: "2026-04-15T08:00:00Z"},
			postType: "page",
			wantLoc:  "http://localhost:3000/about",
		},
		{
			name:     "custom post type generates root URL",
			baseURL:  "http://localhost:3000",
			content:  ContentItem{Slug: "my-tutorial", UpdatedAt: "2026-04-18T10:00:00Z"},
			postType: "tutorial",
			wantLoc:  "http://localhost:3000/my-tutorial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ContentToSitemapEntry(tt.baseURL, tt.content, tt.postType)
			if entry.Loc != tt.wantLoc {
				t.Errorf("ContentToSitemapEntry() Loc = %v, want %v", entry.Loc, tt.wantLoc)
			}
		})
	}
}

func TestGetChangeFrequency(t *testing.T) {
	tests := []struct {
		name     string
		postType string
		want     string
	}{
		{
			name:     "post type returns daily",
			postType: "post",
			want:     "daily",
		},
		{
			name:     "page type returns weekly",
			postType: "page",
			want:     "weekly",
		},
		{
			name:     "unknown type defaults to weekly",
			postType: "unknown",
			want:     "weekly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetChangeFrequency(tt.postType); got != tt.want {
				t.Errorf("GetChangeFrequency() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPriority(t *testing.T) {
	tests := []struct {
		name     string
		postType string
		want     string
	}{
		{
			name:     "post type returns 0.8",
			postType: "post",
			want:     "0.8",
		},
		{
			name:     "page type returns 0.6",
			postType: "page",
			want:     "0.6",
		},
		{
			name:     "unknown type defaults to 0.6",
			postType: "unknown",
			want:     "0.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPriority(tt.postType); got != tt.want {
				t.Errorf("GetPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}
