package seo

import (
	"testing"
)

func TestContentToSitemapEntry(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		content  ContentItem
		postType string
		wantLoc  string
	}{
		{
			name:     "post type generates /posts/ URL",
			baseURL:  "http://localhost:3000",
			content:  ContentItem{Slug: "my-post", UpdatedAt: "2026-04-18T10:00:00Z"},
			postType: "post",
			wantLoc:  "http://localhost:3000/posts/my-post",
		},
		{
			name:     "page type generates root URL",
			baseURL:  "http://localhost:3000",
			content:  ContentItem{Slug: "about", UpdatedAt: "2026-04-15T08:00:00Z"},
			postType: "page",
			wantLoc:  "http://localhost:3000/about",
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
