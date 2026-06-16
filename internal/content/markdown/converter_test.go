package markdown_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/markdown"
	"github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tNode mirrors the converter's TipTap node shape for assertions.
type tNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []tNode        `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []tMark        `json:"marks,omitempty"`
}

// tMark mirrors the converter's TipTap mark shape.
type tMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// mustValidate ensures the converter output is accepted by the same TipTap
// sanitizer used when content is created through the normal API — copied from
// internal/content/wordpress/converter_test.go so no test can ship output the
// domain layer would reject.
func mustValidate(t *testing.T, doc string) {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(doc), &m), "output must be valid JSON: %s", doc)
	require.NoError(t, sanitize.ValidateTipTapDocument(m), "output must pass sanitizer: %s", doc)
}

// decode converts a Tiptap JSON string into a navigable doc node.
func decode(t *testing.T, doc string) tNode {
	t.Helper()
	var n tNode
	require.NoError(t, json.Unmarshal([]byte(doc), &n))
	require.Equal(t, "doc", n.Type, "root must be a doc: %s", doc)
	return n
}

// hasMark reports whether a node carries a mark of the given type, returning it.
func hasMark(n tNode, markType string) (tMark, bool) {
	for _, m := range n.Marks {
		if m.Type == markType {
			return m, true
		}
	}
	return tMark{}, false
}

// hasChildType reports whether a node has any direct child of the given type.
func hasChildType(n tNode, typ string) bool {
	for _, c := range n.Content {
		if c.Type == typ {
			return true
		}
	}
	return false
}

func TestConvert_MappingTable(t *testing.T) {
	tests := []struct {
		name string
		md   string
		// check asserts the structural expectation on the decoded doc.
		check func(t *testing.T, doc tNode)
	}{
		{
			name: "paragraph with text",
			md:   "Hello world",
			check: func(t *testing.T, doc tNode) {
				p := doc.Content[0]
				assert.Equal(t, "paragraph", p.Type)
				require.Len(t, p.Content, 1)
				assert.Equal(t, "text", p.Content[0].Type)
				assert.Equal(t, "Hello world", p.Content[0].Text)
			},
		},
		{
			name: "heading level 1",
			md:   "# Title",
			check: func(t *testing.T, doc tNode) {
				h := doc.Content[0]
				assert.Equal(t, "heading", h.Type)
				assert.Equal(t, float64(1), h.Attrs["level"])
				require.Len(t, h.Content, 1)
				assert.Equal(t, "Title", h.Content[0].Text)
			},
		},
		{
			name: "heading level 3",
			md:   "### Section",
			check: func(t *testing.T, doc tNode) {
				h := doc.Content[0]
				assert.Equal(t, "heading", h.Type)
				assert.Equal(t, float64(3), h.Attrs["level"])
			},
		},
		{
			name: "heading level 6",
			md:   "###### Deepest",
			check: func(t *testing.T, doc tNode) {
				assert.Equal(t, float64(6), doc.Content[0].Attrs["level"])
			},
		},
		{
			name: "italic emphasis",
			md:   "*em*",
			check: func(t *testing.T, doc tNode) {
				txt := doc.Content[0].Content[0]
				assert.Equal(t, "em", txt.Text)
				_, ok := hasMark(txt, "italic")
				assert.True(t, ok, "expected italic mark")
			},
		},
		{
			name: "bold emphasis",
			md:   "**strong**",
			check: func(t *testing.T, doc tNode) {
				txt := doc.Content[0].Content[0]
				assert.Equal(t, "strong", txt.Text)
				_, ok := hasMark(txt, "bold")
				assert.True(t, ok, "expected bold mark")
			},
		},
		{
			name: "inline code",
			md:   "`code`",
			check: func(t *testing.T, doc tNode) {
				txt := doc.Content[0].Content[0]
				assert.Equal(t, "code", txt.Text)
				_, ok := hasMark(txt, "code")
				assert.True(t, ok, "expected code mark")
			},
		},
		{
			name: "bold and italic combined",
			md:   "***bi***",
			check: func(t *testing.T, doc tNode) {
				txt := doc.Content[0].Content[0]
				assert.Equal(t, "bi", txt.Text)
				_, hasBold := hasMark(txt, "bold")
				_, hasItalic := hasMark(txt, "italic")
				assert.True(t, hasBold && hasItalic, "expected both bold and italic marks")
			},
		},
		{
			name: "fenced code block with language",
			md:   "```go\nfmt.Println(\"hi\")\n```",
			check: func(t *testing.T, doc tNode) {
				cb := doc.Content[0]
				assert.Equal(t, "codeBlock", cb.Type)
				assert.Equal(t, "go", cb.Attrs["language"])
				require.Len(t, cb.Content, 1)
				assert.Equal(t, "text", cb.Content[0].Type)
				assert.Equal(t, "fmt.Println(\"hi\")", cb.Content[0].Text)
			},
		},
		{
			name: "fenced code block without language",
			md:   "```\nplain\n```",
			check: func(t *testing.T, doc tNode) {
				cb := doc.Content[0]
				assert.Equal(t, "codeBlock", cb.Type)
				assert.Nil(t, cb.Attrs, "no language attr when info string is empty")
				assert.Equal(t, "plain", cb.Content[0].Text)
			},
		},
		{
			name: "indented code block",
			md:   "    indented",
			check: func(t *testing.T, doc tNode) {
				cb := doc.Content[0]
				assert.Equal(t, "codeBlock", cb.Type)
				assert.Nil(t, cb.Attrs, "indented code has no language")
				assert.Equal(t, "indented", cb.Content[0].Text)
			},
		},
		{
			name: "thematic break",
			md:   "a\n\n---\n\nb",
			check: func(t *testing.T, doc tNode) {
				assert.True(t, hasChildType(doc, "horizontalRule"), "expected a horizontalRule")
			},
		},
		{
			name: "blockquote",
			md:   "> quoted",
			check: func(t *testing.T, doc tNode) {
				bq := doc.Content[0]
				assert.Equal(t, "blockquote", bq.Type)
				require.NotEmpty(t, bq.Content)
				assert.Equal(t, "paragraph", bq.Content[0].Type)
			},
		},
		{
			name: "bullet list",
			md:   "- one\n- two",
			check: func(t *testing.T, doc tNode) {
				ul := doc.Content[0]
				assert.Equal(t, "bulletList", ul.Type)
				require.Len(t, ul.Content, 2)
				assert.Equal(t, "listItem", ul.Content[0].Type)
				assert.Equal(t, "paragraph", ul.Content[0].Content[0].Type)
			},
		},
		{
			name: "ordered list",
			md:   "1. one\n2. two",
			check: func(t *testing.T, doc tNode) {
				ol := doc.Content[0]
				assert.Equal(t, "orderedList", ol.Type)
				require.Len(t, ol.Content, 2)
				assert.Equal(t, "listItem", ol.Content[0].Type)
			},
		},
		{
			name: "image standalone unwraps to block image",
			md:   "![alt text](https://example.com/img.png \"the title\")",
			check: func(t *testing.T, doc tNode) {
				img := doc.Content[0]
				assert.Equal(t, "image", img.Type, "standalone image unwraps to a block image")
				assert.Equal(t, "https://example.com/img.png", img.Attrs["src"])
				assert.Equal(t, "alt text", img.Attrs["alt"])
				assert.Equal(t, "the title", img.Attrs["title"])
			},
		},
		{
			name: "link mark",
			md:   "[Lesstruct](https://lesstruct.example \"official\")",
			check: func(t *testing.T, doc tNode) {
				txt := doc.Content[0].Content[0]
				assert.Equal(t, "Lesstruct", txt.Text)
				m, ok := hasMark(txt, "link")
				require.True(t, ok, "expected link mark")
				assert.Equal(t, "https://lesstruct.example", m.Attrs["href"])
				assert.Equal(t, "official", m.Attrs["title"])
			},
		},
		{
			name: "hard line break",
			md:   "line1  \nline2",
			check: func(t *testing.T, doc tNode) {
				assert.True(t, hasChildType(doc.Content[0], "hardBreak"), "expected a hardBreak node")
			},
		},
		{
			name: "autolink maps to link mark",
			md:   "<https://example.com>",
			check: func(t *testing.T, doc tNode) {
				txt := doc.Content[0].Content[0]
				m, ok := hasMark(txt, "link")
				require.True(t, ok, "expected link mark from autolink")
				assert.Equal(t, "https://example.com", m.Attrs["href"])
				assert.Equal(t, "https://example.com", txt.Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := markdown.Convert(tt.md)
			require.NoError(t, err)
			mustValidate(t, doc)
			tt.check(t, decode(t, doc))
		})
	}
}

func TestConvert_EmptyAndWhitespace(t *testing.T) {
	tests := []struct {
		name string
		md   string
	}{
		{name: "empty string", md: ""},
		{name: "spaces only", md: "   "},
		{name: "newlines only", md: "\n\n"},
		{name: "tabs and spaces", md: "\t  \n\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := markdown.Convert(tt.md)
			require.NoError(t, err)
			mustValidate(t, doc)
			parsed := decode(t, doc)
			require.Len(t, parsed.Content, 1, "empty input yields a single paragraph")
			assert.Equal(t, "paragraph", parsed.Content[0].Type)
		})
	}
}

func TestConvert_RawHTMLSanitized(t *testing.T) {
	// Raw HTML is reduced to safe plain text via bluemonday; no markup is ever
	// stored in a text field.
	tests := []struct {
		name string
		md   string
	}{
		{name: "inline script tag", md: "Hello <script>alert(1)</script> world"},
		{name: "inline bold tag", md: "This is <b>bold</b> text"},
		{name: "block html", md: "<div class=\"x\">block content</div>"},
		{name: "html comment", md: "<!-- a comment -->"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := markdown.Convert(tt.md)
			require.NoError(t, err)
			mustValidate(t, doc)

			// No text field anywhere in the document may contain raw markup.
			assertNoRawHTML(t, decode(t, doc), tt.md)
		})
	}
}

// assertNoRawHTML walks the document and fails if any text node carries the
// `<` or `>` characters — the guarantee that raw HTML is never persisted.
func assertNoRawHTML(t *testing.T, n tNode, src string) {
	t.Helper()
	if n.Type == "text" && (strings.Contains(n.Text, "<") || strings.Contains(n.Text, ">")) {
		t.Fatalf("raw HTML markup leaked into text %q (source: %q)", n.Text, src)
	}
	for _, c := range n.Content {
		assertNoRawHTML(t, c, src)
	}
}

func TestConvert_NeverErrorsOnContent(t *testing.T) {
	// The conversion degrades gracefully on any content: it never returns an
	// error, and always yields a valid (sanitizer-passing) non-empty document.
	// Unsupported node kinds are stripped + logged; since core CommonMark (no
	// GFM) is fully mapped, this table also confirms no standard construct
	// trips the defensive strip path into producing invalid output.
	tests := []struct {
		name string
		md   string
	}{
		{name: "only raw html", md: "<p>just html</p>"},
		{name: "nested blockquotes", md: "> > deeply\n> > nested"},
		{name: "nested lists", md: "- a\n  - b\n  - c\n- d"},
		{name: "mixed everything", md: "# Title\n\n**bold** and *italic* and `code`.\n\n- one\n- two\n\n> quote\n\n```go\nx := 1\n```\n\n![alt](https://e/i.png)\n\n[link](https://e)"},
		{name: "deeply emphasized", md: "_a *b `c` d* e_"},
		{name: "many headings", md: "# 1\n## 2\n### 3\n#### 4\n##### 5\n###### 6"},
		{name: "link reference then use", md: "[ref]: https://example.com\n\nSee [ref]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := markdown.Convert(tt.md)
			require.NoError(t, err, "Convert must never error on content")
			mustValidate(t, doc)
			parsed := decode(t, doc)
			assert.NotEmpty(t, parsed.Content, "doc must have content")
		})
	}
}
