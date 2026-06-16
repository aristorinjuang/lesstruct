package tiptap_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/tiptap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_EmptyDocument(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{"type":"doc","content":[]}`)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestRender_InvalidJSON(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	_, err := r.Render(`not json`)
	require.Error(t, err)
}

func TestRender_InvalidRootType(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	_, err := r.Render(`{"type":"paragraph","content":[]}`)
	require.Error(t, err)
}

func TestRender_Paragraph(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "Hello world"}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper">Hello world</p>`, result)
}

func TestRender_EmptyParagraph(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph"}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"></p>`, result)
}

func TestRender_Heading(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "h1",
			input:    `{"type":"doc","content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Title"}]}]}`,
			expected: `<h1 class="content-wrapper">Title</h1>`,
		},
		{
			name:     "h2",
			input:    `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Subtitle"}]}]}`,
			expected: `<h2 class="content-wrapper">Subtitle</h2>`,
		},
		{
			name:     "h3",
			input:    `{"type":"doc","content":[{"type":"heading","attrs":{"level":3},"content":[{"type":"text","text":"Section"}]}]}`,
			expected: `<h3 class="content-wrapper">Section</h3>`,
		},
	}

	r := tiptap.NewRenderer(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.Render(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRender_BoldText(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "bold", "marks": [{"type": "bold"}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><strong>bold</strong></p>`, result)
}

func TestRender_ItalicText(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "italic", "marks": [{"type": "italic"}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><em>italic</em></p>`, result)
}

func TestRender_UnderlineText(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "underline", "marks": [{"type": "underline"}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><u>underline</u></p>`, result)
}

func TestRender_StrikeText(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "strike", "marks": [{"type": "strike"}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><s>strike</s></p>`, result)
}

func TestRender_InlineCode(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "code", "marks": [{"type": "code"}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><code>code</code></p>`, result)
}

func TestRender_CombinedMarks(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "bold italic", "marks": [{"type": "bold"}, {"type": "italic"}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><strong><em>bold italic</em></strong></p>`, result)
}

func TestRender_Link(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "click here", "marks": [{"type": "link", "attrs": {"href": "https://example.com"}}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><a href="https://example.com">click here</a></p>`, result)
}

func TestRender_LinkWithTarget(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "link", "marks": [{"type": "link", "attrs": {"href": "https://example.com", "target": "_blank", "rel": "noopener"}}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"><a href="https://example.com" target="_blank" rel="noopener">link</a></p>`, result)
}

func TestRender_BulletList(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "bulletList", "content": [
			{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Item 1"}]}]},
			{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Item 2"}]}]}
		]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<ul class="content-wrapper"><li><p class="content-wrapper">Item 1</p></li><li><p class="content-wrapper">Item 2</p></li></ul>`, result)
}

func TestRender_OrderedList(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "orderedList", "content": [
			{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "First"}]}]},
			{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Second"}]}]}
		]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<ol class="content-wrapper"><li><p class="content-wrapper">First</p></li><li><p class="content-wrapper">Second</p></li></ol>`, result)
}

func TestRender_Blockquote(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "blockquote", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "quoted text"}]}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<blockquote class="content-wrapper"><p class="content-wrapper">quoted text</p></blockquote>`, result)
}

func TestRender_CodeBlock(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "codeBlock", "attrs": {"language": "go"}, "content": [{"type": "text", "text": "fmt.Println(\"hello\")"}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<pre class="content-wrapper"><code class="language-go">fmt.Println(&#34;hello&#34;)</code></pre>`, result)
}

func TestRender_CodeBlockWithoutLanguage(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "codeBlock", "content": [{"type": "text", "text": "some code"}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<pre class="content-wrapper"><code>some code</code></pre>`, result)
}

func TestRender_HardBreak(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [
			{"type": "text", "text": "line 1"},
			{"type": "hardBreak"},
			{"type": "text", "text": "line 2"}
		]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper">line 1<br>line 2</p>`, result)
}

func TestRender_HorizontalRule(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "above"}]},
			{"type": "horizontalRule"},
			{"type": "paragraph", "content": [{"type": "text", "text": "below"}]}
		]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper">above</p><hr class="content-wrapper"><p class="content-wrapper">below</p>`, result)
}

func TestRender_Image(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "image", "attrs": {"src": "/uploads/photo.webp", "alt": "A photo", "title": "My Photo"}}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<figure><img src="/uploads/photo.webp" alt="A photo" title="My Photo" loading="lazy"></figure>`, result)
}

func TestRender_ImageWithOnlySrc(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "image", "attrs": {"src": "/uploads/photo.webp"}}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<figure><img src="/uploads/photo.webp" loading="lazy"></figure>`, result)
}

func TestRender_ImageWithoutSrc(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "image", "attrs": {"alt": "missing"}}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestRender_Table(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "table", "content": [
			{"type": "tableRow", "content": [
				{"type": "tableHeader", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Header"}]}]},
				{"type": "tableCell", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Cell"}]}]}
			]}
		]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<div class="table-wrapper"><table><tr><th><p class="content-wrapper">Header</p></th><td><p class="content-wrapper">Cell</p></td></tr></table></div>`, result)
}

func TestRender_ComplexDocument(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [
			{"type": "heading", "attrs": {"level": 1}, "content": [{"type": "text", "text": "Welcome"}]},
			{"type": "paragraph", "content": [
				{"type": "text", "text": "This is "},
				{"type": "text", "text": "bold", "marks": [{"type": "bold"}]},
				{"type": "text", "text": " and "},
				{"type": "text", "text": "italic", "marks": [{"type": "italic"}]},
				{"type": "text", "text": " text."}
			]},
			{"type": "bulletList", "content": [
				{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "one"}]}]},
				{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "two"}]}]}
			]},
			{"type": "blockquote", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "quote"}]}]},
			{"type": "image", "attrs": {"src": "/img/photo.webp", "alt": "photo"}}
		]
	}`)
	require.NoError(t, err)
	expected := `<h1 class="content-wrapper">Welcome</h1>` +
		`<p class="content-wrapper">This is <strong>bold</strong> and <em>italic</em> text.</p>` +
		`<ul class="content-wrapper"><li><p class="content-wrapper">one</p></li><li><p class="content-wrapper">two</p></li></ul>` +
		`<blockquote class="content-wrapper"><p class="content-wrapper">quote</p></blockquote>` +
		`<figure><img src="/img/photo.webp" alt="photo" loading="lazy"></figure>`
	assert.Equal(t, expected, result)
}

func TestRender_HTMLEscaping(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [{"type": "text", "text": "<script>alert('xss')</script>"}]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper">&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</p>`, result)
}

func TestRender_ImageWithSrcset(t *testing.T) {
	resolver := func(src string) []tiptap.ImageVariant {
		if src == "/uploads/photo.webp" {
			return []tiptap.ImageVariant{
				{URL: "/uploads/photo_thumb.webp", Width: 370},
				{URL: "/uploads/photo_medium.webp", Width: 800},
				{URL: "/uploads/photo_large.webp", Width: 1600},
			}
		}
		return nil
	}

	r := tiptap.NewRenderer(resolver)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "image", "attrs": {"src": "/uploads/photo.webp", "alt": "A photo"}}]
	}`)
	require.NoError(t, err)
	assert.Contains(t, result, `src="/uploads/photo.webp"`)
	assert.Contains(t, result, `srcset="/uploads/photo_thumb.webp 370w, /uploads/photo_medium.webp 800w, /uploads/photo_large.webp 1600w"`)
	assert.Contains(t, result, `sizes="100vw"`)
	assert.Contains(t, result, `alt="A photo"`)
	assert.Contains(t, result, "<figure>")
	assert.Contains(t, result, "</figure>")
}

func TestRender_ImageNoResolverNoSrcset(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "image", "attrs": {"src": "/uploads/photo.webp"}}]
	}`)
	require.NoError(t, err)
	assert.Contains(t, result, `src="/uploads/photo.webp"`)
	assert.NotContains(t, result, "srcset")
	assert.NotContains(t, result, "sizes")
}

func TestRender_ImageResolverReturnsEmpty(t *testing.T) {
	resolver := func(src string) []tiptap.ImageVariant {
		return nil
	}

	r := tiptap.NewRenderer(resolver)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "image", "attrs": {"src": "/uploads/photo.webp"}}]
	}`)
	require.NoError(t, err)
	assert.Contains(t, result, `src="/uploads/photo.webp"`)
	assert.NotContains(t, result, "srcset")
}

func TestRender_ImageExternalURLNoSrcset(t *testing.T) {
	resolver := func(src string) []tiptap.ImageVariant {
		return nil
	}

	r := tiptap.NewRenderer(resolver)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "image", "attrs": {"src": "https://example.com/photo.jpg"}}]
	}`)
	require.NoError(t, err)
	assert.Contains(t, result, `src="https://example.com/photo.jpg"`)
	assert.NotContains(t, result, "srcset")
}

func TestRender_MultipleImagesWithSrcset(t *testing.T) {
	resolver := func(src string) []tiptap.ImageVariant {
		return []tiptap.ImageVariant{
			{URL: src + "_thumb", Width: 370},
		}
	}

	r := tiptap.NewRenderer(resolver)

	result, err := r.Render(`{
		"type": "doc",
		"content": [
			{"type": "image", "attrs": {"src": "/uploads/a.webp"}},
			{"type": "paragraph", "content": [{"type": "text", "text": "text"}]},
			{"type": "image", "attrs": {"src": "/uploads/b.webp"}}
		]
	}`)
	require.NoError(t, err)
	assert.Contains(t, result, `src="/uploads/a.webp"`)
	assert.Contains(t, result, `srcset="/uploads/a.webp_thumb 370w"`)
	assert.Contains(t, result, `src="/uploads/b.webp"`)
	assert.Contains(t, result, `srcset="/uploads/b.webp_thumb 370w"`)
}

func TestRender_Youtube(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "youtube", "attrs": {"src": "https://www.youtube.com/embed/dQw4w9WgXcQ"}}]
	}`)
	require.NoError(t, err)
	assert.Equal(
		t,
		`<div class="embed-wrapper"><iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ" frameborder="0" allowfullscreen allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"></iframe></div>`,
		result,
	)
}

func TestRender_Emoji(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [
			{"type": "text", "text": "hello "},
			{"type": "emoji", "attrs": {"name": "smile"}},
			{"type": "text", "text": " world"}
		]}]
	}`)
	require.NoError(t, err)
	assert.Equal(
		t,
		`<p class="content-wrapper">hello <span class="emoji" data-name="smile">😄</span> world</p>`,
		result,
	)
}

func TestRender_EmojiNoName(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [
			{"type": "emoji", "attrs": {}}
		]}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, `<p class="content-wrapper"></p>`, result)
}

func TestRender_EmojiBetweenParagraphs(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [
			{"type": "paragraph", "content": [{"type": "emoji", "attrs": {"name": "heart"}}]},
			{"type": "paragraph", "content": [{"type": "emoji", "attrs": {"name": "fire"}}]}
		]
	}`)
	require.NoError(t, err)
	assert.Equal(
		t,
		`<p class="content-wrapper"><span class="emoji" data-name="heart">❤</span></p><p class="content-wrapper"><span class="emoji" data-name="fire">🔥</span></p>`,
		result,
	)
}

func TestRender_EmojiFallbackUnknown(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "paragraph", "content": [
			{"type": "emoji", "attrs": {"name": "nonexistent_emoji"}}
		]}]
	}`)
	require.NoError(t, err)
	assert.Equal(
		t,
		`<p class="content-wrapper"><span class="emoji" data-name="nonexistent_emoji">:nonexistent_emoji:</span></p>`,
		result,
	)
}

func TestRender_YoutubeNoSrc(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [{"type": "youtube", "attrs": {"width": 640}}]
	}`)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestRender_YoutubeBetweenParagraphs(t *testing.T) {
	r := tiptap.NewRenderer(nil)

	result, err := r.Render(`{
		"type": "doc",
		"content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "before"}]},
			{"type": "youtube", "attrs": {"src": "https://www.youtube.com/embed/dQw4w9WgXcQ"}},
			{"type": "paragraph", "content": [{"type": "text", "text": "after"}]}
		]
	}`)
	require.NoError(t, err)
	assert.Contains(t, result, `<p class="content-wrapper">before</p>`)
	assert.Contains(t, result, `<div class="embed-wrapper"><iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ"`)
	assert.Contains(t, result, `<p class="content-wrapper">after</p>`)
}
