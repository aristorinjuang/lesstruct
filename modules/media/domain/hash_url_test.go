package media_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/stretchr/testify/assert"
)

func TestExtractHashFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal media URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "thumb variant URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a_thumb.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "medium variant URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a_medium.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "large variant URL",
			input:    "http://localhost:8080/uploads/media/abc123def456789a_large.webp",
			expected: "abc123def456789a",
		},
		{
			name:     "external URL extracts filename stem",
			input:    "https://example.com/photo.jpg",
			expected: "photo",
		},
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "malformed URL returns empty",
			input:    "://bad-url",
			expected: "",
		},
		{
			name:     "URL ending with directory",
			input:    "http://localhost:8080/uploads/media/",
			expected: "media",
		},
		{
			name:     "percent-encoded path",
			input:    "http://localhost:8080/uploads/media/abc%20def.webp",
			expected: "abc def",
		},
		{
			name:     "path that fails to unescape returns empty",
			input:    "http://localhost:8080/uploads/media/100%25zz.webp",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := media.ExtractHashFromURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
