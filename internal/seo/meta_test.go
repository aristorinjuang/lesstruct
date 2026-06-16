package seo_test

import (
	"html/template"
	"strings"
	"testing"

	appseo "github.com/aristorinjuang/lesstruct/internal/seo"
	"github.com/stretchr/testify/assert"
)

func TestMetaTag_Render(t *testing.T) {
	tests := []struct {
		name string
		tag  appseo.MetaTag
		want string
	}{
		{
			name: "meta tag with name",
			tag:  appseo.NewMetaTag("description", "This is a test description"),
			want: `<meta name="description" content="This is a test description">`,
		},
		{
			name: "meta tag with property",
			tag:  appseo.NewPropertyMetaTag("og:title", "Test Title"),
			want: `<meta property="og:title" content="Test Title">`,
		},
		{
			name: "meta tag with name and special characters",
			tag:  appseo.NewMetaTag("description", "Test with \"quotes\" and <tags>"),
			want: `<meta name="description" content="Test with &#34;quotes&#34; and &lt;tags&gt;">`,
		},
		{
			name: "meta tag with property and special characters",
			tag:  appseo.NewPropertyMetaTag("og:title", "Title with & ampersand"),
			want: `<meta property="og:title" content="Title with &amp; ampersand">`,
		},
		{
			name: "empty meta tag",
			tag:  appseo.MetaTag{},
			want: `<meta>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tag.Render()
			assert.Equal(t, template.HTML(tt.want), got, "MetaTag.Render() mismatch")
		})
	}
}

func TestNewMetaTag(t *testing.T) {
	tag := appseo.NewMetaTag("description", "Test description")

	assert.Equal(t, "description", tag.Name, "NewMetaTag() Name")
	assert.Equal(t, "Test description", tag.Content, "NewMetaTag() Content")
	assert.Empty(t, tag.Property, "NewMetaTag() Property should be empty")
}

func TestNewPropertyMetaTag(t *testing.T) {
	tag := appseo.NewPropertyMetaTag("og:title", "Test Title")

	assert.Equal(t, "og:title", tag.Property, "NewPropertyMetaTag() Property")
	assert.Equal(t, "Test Title", tag.Content, "NewPropertyMetaTag() Content")
	assert.Empty(t, tag.Name, "NewPropertyMetaTag() Name should be empty")
}

func TestRenderMetaTags(t *testing.T) {
	tags := []appseo.MetaTag{
		appseo.NewMetaTag("description", "Test description"),
		appseo.NewPropertyMetaTag("og:title", "Test Title"),
		appseo.NewPropertyMetaTag("og:type", "article"),
	}

	got := appseo.RenderMetaTags(tags)
	lines := strings.Split(string(got), "\n")

	assert.Len(t, lines, 4, "RenderMetaTags() should return 4 lines")

	expectedTags := []string{
		`<meta name="description" content="Test description">`,
		`<meta property="og:title" content="Test Title">`,
		`<meta property="og:type" content="article">`,
	}

	for i, expected := range expectedTags {
		assert.Contains(t, lines[i], expected, "RenderMetaTags() line %d mismatch", i)
	}
}

func TestMetaTag_Escaping(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		shouldNotContain []string
		shouldContain   []string
	}{
		{
			name:            "escape quotes",
			input:           `Text with "quotes"`,
			shouldNotContain: []string{`content="Text with "`},
			shouldContain:    []string{"&#34;", "&quot;"},
		},
		{
			name:            "escape angle brackets",
			input:           `<script>alert('xss')</script>`,
			shouldNotContain: []string{`<script>`},
			shouldContain:    []string{"&lt;script&gt;"},
		},
		{
			name:            "escape ampersand",
			input:           "Text & more",
			shouldNotContain: []string{"content=\"Text & more\""},
			shouldContain:    []string{"&amp;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := appseo.NewMetaTag("description", tt.input)
			got := string(tag.Render())

			for _, forbidden := range tt.shouldNotContain {
				assert.NotContains(t, got, forbidden, "MetaTag.Render() contains unescaped content")
			}

			hasEscaped := false
			for _, allowed := range tt.shouldContain {
				if strings.Contains(got, allowed) {
					hasEscaped = true
					break
				}
			}
			if len(tt.shouldContain) > 0 {
				assert.True(t, hasEscaped, "MetaTag.Render() should contain escaped content")
			}
		})
	}
}
