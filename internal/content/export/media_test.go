package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMediaURLs(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []string
	}{
		{
			name:     "empty body",
			body:     "",
			expected: nil,
		},
		{
			name:     "body without images",
			body:     "<p>Hello world</p>",
			expected: nil,
		},
		{
			name: "single media image",
			body: `<p><img src="/uploads/media/abc123.webp" alt="test"></p>`,
			expected: []string{
				"/uploads/media/abc123.webp",
			},
		},
		{
			name: "multiple media images",
			body: `<p><img src="/uploads/media/abc123.webp" alt="first"></p>
<p><img src="/uploads/media/def456.jpg" alt="second"></p>`,
			expected: []string{
				"/uploads/media/abc123.webp",
				"/uploads/media/def456.jpg",
			},
		},
		{
			name: "images with various attributes",
			body: `<img src="/uploads/media/img1.png" width="800" height="600" class="responsive">`,
			expected: []string{
				"/uploads/media/img1.png",
			},
		},
		{
			name: "non-media images are ignored",
			body: `<img src="https://example.com/image.jpg"><img src="/uploads/media/local.webp">`,
			expected: []string{
				"/uploads/media/local.webp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMediaURLs(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}
