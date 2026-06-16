package seo_test

import (
	"testing"

	appseo "github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/stretchr/testify/assert"
)

func TestExtractPlainText(t *testing.T) {
	tests := []struct {
		name       string
		tiptapJSON string
		want       string
	}{
		{
			name:       "empty JSON",
			tiptapJSON: "",
			want:       "",
		},
		{
			name:       "invalid JSON",
			tiptapJSON: "invalid json",
			want:       "",
		},
		{
			name:       "simple paragraph",
			tiptapJSON: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`,
			want:       "Hello world",
		},
		{
			name:       "multiple paragraphs",
			tiptapJSON: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First paragraph"}]},{"type":"paragraph","content":[{"type":"text","text":"Second paragraph"}]}]}`,
			want:       "First paragraph Second paragraph",
		},
		{
			name:       "heading",
			tiptapJSON: `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"A heading"}]}]}`,
			want:       "A heading",
		},
		{
			name:       "bullet list",
			tiptapJSON: `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 1"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 2"}]}]}]}]}`,
			want:       "Item 1  Item 2",
		},
		{
			name:       "ordered list",
			tiptapJSON: `{"type":"doc","content":[{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 1"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 2"}]}]}]}]}`,
			want:       "Item 1  Item 2",
		},
		{
			name:       "code block",
			tiptapJSON: `{"type":"doc","content":[{"type":"codeBlock","content":[{"type":"text","text":"const x = 1;"}]}]}`,
			want:       "const x = 1;",
		},
		{
			name:       "mixed content",
			tiptapJSON: `{"type":"doc","content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Title"}]},{"type":"paragraph","content":[{"type":"text","text":"Paragraph text"}]},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"List item"}]}]}]}]}`,
			want:       "Title Paragraph text List item",
		},
		{
			name:       "content with no text nodes",
			tiptapJSON: `{"type":"doc","content":[{"type":"paragraph"}]}`,
			want:       "",
		},
		{
			name:       "empty content array",
			tiptapJSON: `{"type":"doc","content":[]}`,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appseo.ExtractPlainText(tt.tiptapJSON)
			assert.Equal(t, tt.want, got, "ExtractPlainText() mismatch")
		})
	}
}

func TestExtractImageURL(t *testing.T) {
	tests := []struct {
		name       string
		tiptapJSON string
		want       string
	}{
		{
			name:       "empty JSON",
			tiptapJSON: "",
			want:       "",
		},
		{
			name:       "invalid JSON",
			tiptapJSON: "invalid json",
			want:       "",
		},
		{
			name:       "single image",
			tiptapJSON: `{"type":"doc","content":[{"type":"image","attrs":{"src":"/uploads/media/test.webp"}}]}`,
			want:       "/uploads/media/test.webp",
		},
		{
			name:       "image with alt text",
			tiptapJSON: `{"type":"doc","content":[{"type":"image","attrs":{"src":"/uploads/media/test.webp","alt":"Test image"}}]}`,
			want:       "/uploads/media/test.webp",
		},
		{
			name:       "paragraph followed by image",
			tiptapJSON: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Text before"}]},{"type":"image","attrs":{"src":"/uploads/media/test.webp"}}]}`,
			want:       "/uploads/media/test.webp",
		},
		{
			name:       "image followed by paragraph",
			tiptapJSON: `{"type":"doc","content":[{"type":"image","attrs":{"src":"/uploads/media/test.webp"}},{"type":"paragraph","content":[{"type":"text","text":"Text after"}]}]}`,
			want:       "/uploads/media/test.webp",
		},
		{
			name:       "multiple images - returns first",
			tiptapJSON: `{"type":"doc","content":[{"type":"image","attrs":{"src":"/uploads/media/first.webp"}},{"type":"image","attrs":{"src":"/uploads/media/second.webp"}}]}`,
			want:       "/uploads/media/first.webp",
		},
		{
			name:       "no images",
			tiptapJSON: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Just text"}]}]}`,
			want:       "",
		},
		{
			name:       "image with URL src",
			tiptapJSON: `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.com/image.jpg"}}]}`,
			want:       "https://example.com/image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appseo.ExtractImageURL(tt.tiptapJSON)
			assert.Equal(t, tt.want, got, "ExtractImageURL() mismatch")
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		maxLength  int
		want       string
	}{
		{
			name:      "text shorter than max",
			text:      "Short text",
			maxLength: 100,
			want:      "Short text",
		},
		{
			name:      "text equal to max",
			text:      "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostru",
			maxLength: 160,
			want:      "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostru",
		},
		{
			name:      "text longer than max",
			text:      "This is a longer text that needs to be truncated because it exceeds the maximum length",
			maxLength: 50,
			want:      "This is a longer text that needs to be truncate...",
		},
		{
			name:      "truncate at exactly max length",
			text:      "1234567890",
			maxLength: 10,
			want:      "1234567890",
		},
		{
			name:      "truncate with one extra char",
			text:      "12345678901",
			maxLength: 10,
			want:      "1234567...",
		},
		{
			name:      "max length less than ellipsis length",
			text:      "Long text",
			maxLength: 2,
			want:      "Lo",
		},
		{
			name:      "max length equal to ellipsis length",
			text:      "Long text",
			maxLength: 3,
			want:      "Lon",
		},
		{
			name:      "empty string",
			text:      "",
			maxLength: 10,
			want:      "",
		},
		{
			name:      "unicode characters",
			text:      "Hello 世界 🌍 This is a test",
			maxLength: 14,
			want:      "Hello 世界 🌍 ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appseo.TruncateText(tt.text, tt.maxLength)
			assert.Equal(t, tt.want, got, "TruncateText() mismatch")
		})
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{
			name:    "relative path",
			baseURL: "https://example.com",
			path:    "/posts/my-article",
			want:    "https://example.com/posts/my-article",
		},
		{
			name:    "base URL with trailing slash",
			baseURL: "https://example.com/",
			path:    "/posts/my-article",
			want:    "https://example.com/posts/my-article",
		},
		{
			name:    "absolute URL with https",
			baseURL: "https://example.com",
			path:    "https://cdn.example.com/image.jpg",
			want:    "https://cdn.example.com/image.jpg",
		},
		{
			name:    "absolute URL with http",
			baseURL: "https://example.com",
			path:    "http://cdn.example.com/image.jpg",
			want:    "http://cdn.example.com/image.jpg",
		},
		{
			name:    "path without leading slash",
			baseURL: "https://example.com",
			path:    "posts/my-article",
			want:    "https://example.com/posts/my-article",
		},
		{
			name:    "empty path",
			baseURL: "https://example.com",
			path:    "",
			want:    "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appseo.BuildURL(tt.baseURL, tt.path)
			assert.Equal(t, tt.want, got, "BuildURL() mismatch")
		})
	}
}
