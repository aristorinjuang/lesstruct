package export

import (
	"strings"
	"testing"
	"time"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestBuildFrontmatter(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		content  *contentdomain.Content
		aliases  []string
		contains []string
	}{
		{
			name: "basic published content",
			content: &contentdomain.Content{
				Title:     "My Article",
				Slug:      "my-article",
				Status:    contentdomain.StatusPublished,
				PostType:  "post",
				Language:  "en",
				CreatedAt: now,
			},
			contains: []string{
				"title: My Article",
				`date: "2024-01-15T10:30:00Z"`,
				"url: /posts/my-article",
				"language: en",
			},
		},
		{
			name: "content with tags and description",
			content: &contentdomain.Content{
				Title:           "Tagged Post",
				Slug:            "tagged-post",
				Status:          contentdomain.StatusPublished,
				PostType:        "post",
				Language:        "en",
				Tags:            []string{"go", "testing"},
				MetaDescription: "A great post",
				CreatedAt:       now,
			},
			contains: []string{
				"title: Tagged Post",
				"description: A great post",
				"tags:",
				"- go",
				"- testing",
			},
		},
		{
			name: "draft content",
			content: &contentdomain.Content{
				Title:     "Draft Post",
				Slug:      "draft-post",
				Status:    contentdomain.StatusDraft,
				PostType:  "post",
				Language:  "en",
				CreatedAt: now,
			},
			contains: []string{
				"draft: true",
			},
		},
		{
			name: "content with aliases",
			content: &contentdomain.Content{
				Title:     "Aliased Post",
				Slug:      "aliased-post",
				Status:    contentdomain.StatusPublished,
				PostType:  "post",
				Language:  "en",
				CreatedAt: now,
			},
			aliases: []string{"old-url.html", "another-old-url.html"},
			contains: []string{
				"aliases:",
				"- old-url.html",
				"- another-old-url.html",
			},
		},
		{
			name: "custom post type",
			content: &contentdomain.Content{
				Title:     "Custom Item",
				Slug:      "custom-item",
				Status:    contentdomain.StatusPublished,
				PostType:  "portfolio",
				Language:  "en",
				CreatedAt: now,
			},
			contains: []string{
				"url: /portfolio/custom-item",
			},
		},
		{
			name: "content with author",
			content: &contentdomain.Content{
				Title:     "Authored Post",
				Slug:      "authored-post",
				Status:    contentdomain.StatusPublished,
				PostType:  "post",
				Language:  "en",
				Author:    "John Doe",
				CreatedAt: now,
			},
			contains: []string{
				"author: John Doe",
			},
		},
		{
			name: "content with custom fields",
			content: &contentdomain.Content{
				Title:    "Custom Fields Post",
				Slug:     "custom-fields-post",
				Status:   contentdomain.StatusPublished,
				PostType: "post",
				Language: "en",
				CustomFields: map[string]any{
					"hasMath":  true,
					"hasChart": false,
				},
				CreatedAt: now,
			},
			contains: []string{
				"hasMath: true",
				"hasChart: false",
			},
		},
		{
			name: "content with lastmod",
			content: &contentdomain.Content{
				Title:     "Updated Post",
				Slug:      "updated-post",
				Status:    contentdomain.StatusPublished,
				PostType:  "post",
				Language:  "en",
				CreatedAt: now,
				UpdatedAt: now.Add(2 * time.Hour),
			},
			contains: []string{
				`date: "2024-01-15T10:30:00Z"`,
				`lastmod: "2024-01-15T12:30:00Z"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildFrontmatter(tt.content, tt.aliases)
			assert.True(t, strings.HasPrefix(result, "---\n"), "should start with YAML delimiter")
			assert.True(t, strings.HasSuffix(result, "---\n"), "should end with YAML delimiter")

			// Extract YAML between delimiters and verify it parses
			yamlPart := strings.TrimPrefix(result, "---\n")
			yamlPart = strings.TrimSuffix(yamlPart, "---\n")
			yamlPart = strings.TrimSuffix(yamlPart, "---")

			var parsed map[string]any
			err := yaml.Unmarshal([]byte(yamlPart), &parsed)
			assert.NoError(t, err, "frontmatter should be valid YAML")

			// Verify content-specific values
			for _, s := range tt.contains {
				assert.Contains(t, result, s, "frontmatter should contain %q", s)
			}
		})
	}
}
