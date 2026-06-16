package wordpress_test

import (
	"encoding/json"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	"github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustValidate ensures the converter output is accepted by the same TipTap
// sanitizer used when content is created through the normal API.
func mustValidate(t *testing.T, doc string) {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(doc), &m), "output must be valid JSON: %s", doc)
	require.NoError(t, sanitize.ValidateTipTapDocument(m), "output must pass sanitizer: %s", doc)
}

func TestConvertBlocks_EachBlockType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string // type of the first content node
	}{
		{
			name:     "paragraph with bold and link",
			input:    `<!-- wp:paragraph --><p>Hello <strong>world</strong> and <a href="https://example.com">link</a></p><!-- /wp:paragraph -->`,
			wantType: "paragraph",
		},
		{
			name:     "heading with level from attrs",
			input:    `<!-- wp:heading {"level":3} --><h3 class="wp-block-heading">Title</h3><!-- /wp:heading -->`,
			wantType: "heading",
		},
		{
			name:     "heading level detected from tag",
			input:    `<!-- wp:heading --><h2 class="wp-block-heading">Title</h2><!-- /wp:heading -->`,
			wantType: "heading",
		},
		{
			name:     "unordered list",
			input:    `<!-- wp:list --><ul><!-- wp:list-item --><li>A</li><!-- /wp:list-item --><!-- wp:list-item --><li>B</li><!-- /wp:list-item --></ul><!-- /wp:list -->`,
			wantType: "bulletList",
		},
		{
			name:     "ordered list from attrs",
			input:    `<!-- wp:list {"ordered":true} --><ol><!-- wp:list-item --><li>1</li><!-- /wp:list-item --></ol><!-- /wp:list -->`,
			wantType: "orderedList",
		},
		{
			name:     "quote with nested paragraph",
			input:    `<!-- wp:quote --><blockquote><!-- wp:paragraph --><p>Quoted text</p><!-- /wp:paragraph --></blockquote><!-- /wp:quote -->`,
			wantType: "blockquote",
		},
		{
			name:     "code block",
			input:    `<!-- wp:code --><pre class="wp-block-code"><code>console.log("hi")</code></pre><!-- /wp:code -->`,
			wantType: "codeBlock",
		},
		{
			name:     "table",
			input:    `<!-- wp:table --><figure class="wp-block-table"><table><tbody><tr><td>a</td><td>b</td></tr></tbody></table></figure><!-- /wp:table -->`,
			wantType: "table",
		},
		{
			name:     "youtube embed",
			input:    `<!-- wp:embed {"url":"https://www.youtube.com/watch?v=iJ5QOmja4f4","providerNameSlug":"youtube"} --><figure><div>https://www.youtube.com/watch?v=iJ5QOmja4f4</div></figure><!-- /wp:embed -->`,
			wantType: "youtube",
		},
		{
			name:     "math block",
			input:    `<!-- wp:math {"latex":"x^2"} --><div>...</div><!-- /wp:math -->`,
			wantType: "blockMath",
		},
		{
			name:     "more separator",
			input:    `<!-- wp:more --><!--more--><!-- /wp:more -->`,
			wantType: "horizontalRule",
		},
		{
			name:     "image with mapped url",
			input:    `<!-- wp:image --><figure><img src="http://wp.local/img.jpg" alt="pic"/></figure><!-- /wp:image -->`,
			wantType: "image",
		},
		{
			name:     "unsupported block falls back to paragraph",
			input:    `<!-- wp:audio --><figure class="wp-block-audio"><audio controls src="http://wp.local/song.mp3"></audio></figure><!-- /wp:audio -->`,
			wantType: "paragraph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.ConvertBlocks(tt.input, nil)
			require.NoError(t, err)
			mustValidate(t, doc)

			var parsed struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
			}
			require.NoError(t, json.Unmarshal([]byte(doc), &parsed))
			require.Equal(t, "doc", parsed.Type)
			require.NotEmpty(t, parsed.Content)
			assert.Equal(t, tt.wantType, parsed.Content[0].Type)
		})
	}
}

func TestConvertBlocks_ImageURLRemap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		imageMap map[string]string
		wantSrc  string
	}{
		{
			name:     "success - remapped to local url",
			input:    `<!-- wp:image --><figure><img src="http://wp.local/img.jpg" alt=""/></figure><!-- /wp:image -->`,
			imageMap: map[string]string{"http://wp.local/img.jpg": "http://localhost:8080/uploads/media/abc.webp"},
			wantSrc:  "http://localhost:8080/uploads/media/abc.webp",
		},
		{
			name:     "success - unmapped keeps original",
			input:    `<!-- wp:image --><figure><img src="http://wp.local/img.jpg" alt=""/></figure><!-- /wp:image -->`,
			imageMap: map[string]string{},
			wantSrc:  "http://wp.local/img.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.ConvertBlocks(tt.input, tt.imageMap)
			require.NoError(t, err)
			mustValidate(t, doc)

			var parsed struct {
				Content []struct {
					Type  string         `json:"type"`
					Attrs map[string]any `json:"attrs"`
				} `json:"content"`
			}
			require.NoError(t, json.Unmarshal([]byte(doc), &parsed))
			require.NotEmpty(t, parsed.Content)
			assert.Equal(t, "image", parsed.Content[0].Type)
			assert.Equal(t, tt.wantSrc, parsed.Content[0].Attrs["src"])
		})
	}
}

func TestConvertBlocks_EmptyInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "whitespace only", input: "   \n\n  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := wordpress.ConvertBlocks(tt.input, nil)
			require.NoError(t, err)
			mustValidate(t, doc)

			var parsed struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
			}
			require.NoError(t, json.Unmarshal([]byte(doc), &parsed))
			assert.Equal(t, "doc", parsed.Type)
			assert.NotEmpty(t, parsed.Content, "empty input must still produce a valid doc with a paragraph")
		})
	}
}

func TestConvertBlocks_RealSampleFirstPost(t *testing.T) {
	// Excerpt of the "My First Post" content from samples/wordpress-export.xml
	// exercising many block types at once.
	input := `<!-- wp:paragraph -->
<p>This is my first post.</p>
<!-- /wp:paragraph -->

<!-- wp:heading -->
<h2 class="wp-block-heading">Heading 2</h2>
<!-- /wp:heading -->

<!-- wp:heading {"level":3} -->
<h3 class="wp-block-heading">Heading 3</h3>
<!-- /wp:heading -->

<!-- wp:list -->
<ul class="wp-block-list"><!-- wp:list-item -->
<li>Unordered List 1</li>
<!-- /wp:list-item --></ul>
<!-- /wp:list -->

<!-- wp:code -->
<pre class="wp-block-code"><code>// Log to Hello World!
console.log("Hello World")</code></pre>
<!-- /wp:code -->

<!-- wp:pullquote -->
<figure class="wp-block-pullquote"><blockquote><p>Migrate WordPress to Lesstruct!!!</p><cite>Aristo Rinjuang</cite></blockquote></figure>
<!-- /wp:pullquote -->`

	doc, err := wordpress.ConvertBlocks(input, nil)
	require.NoError(t, err)
	mustValidate(t, doc)

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
		} `json:"content"`
	}
	require.NoError(t, json.Unmarshal([]byte(doc), &parsed))
	// Expect paragraph, heading, heading, bulletList, codeBlock, blockquote
	require.GreaterOrEqual(t, len(parsed.Content), 6)
	assert.Equal(t, "paragraph", parsed.Content[0].Type)
	assert.Equal(t, "heading", parsed.Content[1].Type)
	assert.Equal(t, "codeBlock", parsed.Content[4].Type)
	assert.Equal(t, "blockquote", parsed.Content[5].Type)
}

func TestExtractImageURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name: "success - image and cover urls",
			input: `<!-- wp:image --><figure><img src="http://wp.local/a.jpg"/></figure><!-- /wp:image -->
<!-- wp:cover {"url":"http://wp.local/cover.jpg"} --><div></div><!-- /wp:cover -->`,
			want: []string{"http://wp.local/a.jpg", "http://wp.local/cover.jpg"},
		},
		{
			name:  "no images",
			input: `<!-- wp:paragraph --><p>just text</p><!-- /wp:paragraph -->`,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordpress.ExtractImageURLs(tt.input)
			if len(tt.want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
