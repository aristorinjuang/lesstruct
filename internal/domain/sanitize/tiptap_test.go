package sanitize_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	sanitizedomain "github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
)

func TestValidateTipTapDocument(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid simple document",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "Hello world"}]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with heading",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "heading",
						"attrs": {"level": 1},
						"content": [{"type": "text", "text": "Title"}]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with bullet list",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "bulletList",
						"content": [
							{
								"type": "listItem",
								"content": [
									{"type": "paragraph", "content": [{"type": "text", "text": "Item 1"}]}
								]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with bold and italic marks",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "bold and italic",
								"marks": [
									{"type": "bold"},
									{"type": "italic"}
								]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with link mark",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "click here",
								"marks": [
									{"type": "link", "attrs": {"href": "https://example.com"}}
								]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with image",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "image",
						"attrs": {"src": "https://example.com/img.jpg", "alt": "test"}
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with blockquote",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "blockquote",
						"content": [
							{"type": "paragraph", "content": [{"type": "text", "text": "A quote"}]}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with code block",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "codeBlock",
						"attrs": {"language": "go"},
						"content": [{"type": "text", "text": "func main() {}"}]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with hard break",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{"type": "text", "text": "line 1"},
							{"type": "hardBreak"},
							{"type": "text", "text": "line 2"}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid empty document",
			input: `{"type": "doc"}`,
			wantErr: false,
		},
		{
			name: "valid ordered list",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "orderedList",
						"content": [
							{
								"type": "listItem",
								"content": [
									{"type": "paragraph", "content": [{"type": "text", "text": "First"}]}
								]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid with http link",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "link",
								"marks": [{"type": "link", "attrs": {"href": "http://example.com"}}]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid with relative link",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "link",
								"marks": [{"type": "link", "attrs": {"href": "/page"}}]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid with horizontal rule",
			input: `{
				"type": "doc",
				"content": [
					{"type": "paragraph", "content": [{"type": "text", "text": "above"}]},
					{"type": "horizontalRule"},
					{"type": "paragraph", "content": [{"type": "text", "text": "below"}]}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid with youtube node",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "youtube",
						"attrs": {"src": "https://www.youtube.com/embed/dQw4w9WgXcQ", "width": 640, "height": 360}
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid with table cell colspan",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "table",
						"content": [
							{
								"type": "tableRow",
								"content": [
									{
										"type": "tableCell",
										"attrs": {"colspan": 2},
										"content": [{"type": "paragraph", "content": [{"type": "text", "text": "wide"}]}]
									}
								]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid with table header colwidth",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "table",
						"content": [
							{
								"type": "tableRow",
								"content": [
									{
										"type": "tableHeader",
										"attrs": {"colwidth": [100, 200]},
										"content": [{"type": "paragraph", "content": [{"type": "text", "text": "header"}]}]
									}
								]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "rejects unknown node type",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "script",
						"content": [{"type": "text", "text": "alert(1)"}]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects unknown attribute on mark",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "styled",
								"marks": [{"type": "underline", "attrs": {"style": "color:red"}}]
							}
						]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects javascript href in link mark",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "evil",
								"marks": [{"type": "link", "attrs": {"href": "javascript:alert(1)"}}]
							}
						]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects data href in link mark",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "evil",
								"marks": [{"type": "link", "attrs": {"href": "data:text/html,<script>alert(1)</script>"}}]
							}
						]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects vbscript href in link mark",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "evil",
								"marks": [{"type": "link", "attrs": {"href": "vbscript:MsgBox(1)"}}]
							}
						]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects unknown attribute on node",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "heading",
						"attrs": {"level": 1, "onclick": "alert(1)"},
						"content": [{"type": "text", "text": "Title"}]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects youtube with unknown attr",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "youtube",
						"attrs": {"src": "https://www.youtube.com/embed/dQw4w9WgXcQ", "onclick": "alert(1)"}
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects table cell with unknown attr",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "table",
						"content": [
							{
								"type": "tableRow",
								"content": [
									{
										"type": "tableCell",
										"attrs": {"onclick": "alert(1)"},
										"content": [{"type": "paragraph", "content": [{"type": "text", "text": "bad"}]}]
									}
								]
							}
						]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects deeply nested document",
			input: buildDeeplyNestedDoc(55),
			wantErr: true,
		},
		{
			name: "accepts max depth document",
			input: buildDeeplyNestedDoc(48),
			wantErr: false,
		},
		{
			name: "rejects oversized document",
			input: buildOversizedDoc(10001),
			wantErr: true,
		},
		{
			name: "rejects text node with non-string text",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": 123}]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects node missing type",
			input: `{
				"type": "doc",
				"content": [
					{"content": [{"type": "text", "text": "hello"}]}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects unknown mark type completely",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "styled",
								"marks": [{"type": "superscript"}]
							}
						]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects iframe node type",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "iframe",
						"attrs": {"src": "https://evil.com"}
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects unknown attribute on image",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "image",
						"attrs": {"src": "https://example.com/img.jpg", "onerror": "alert(1)"}
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "valid with strike and code marks",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "deleted code",
								"marks": [{"type": "strike"}, {"type": "code"}]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid with mailto link",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{
								"type": "text",
								"text": "email",
								"marks": [{"type": "link", "attrs": {"href": "mailto:test@example.com"}}]
							}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid document with emoji node",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{"type": "text", "text": "hello "},
							{"type": "emoji", "attrs": {"name": "smile"}},
							{"type": "text", "text": " world"}
						]
					}
				]
			}`,
			wantErr: false,
		},
		{
			name: "rejects emoji node with unknown attr",
			input: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{"type": "emoji", "attrs": {"name": "grin", "onclick": "alert(1)"}}
						]
					}
				]
			}`,
			wantErr: true,
		},
		{
			name: "rejects doc missing type",
			input: `{"content": []}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse input JSON: %v", err)
			}

			err := sanitizedomain.ValidateTipTapDocument(doc)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTipTapDocument() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func buildDeeplyNestedDoc(depth int) string {
	inner := `{"type": "paragraph", "content": [{"type": "text", "text": "deep"}]}`
	for i := 0; i < depth; i++ {
		inner = fmt.Sprintf(`{"type": "blockquote", "content": [%s]}`, inner)
	}
	return fmt.Sprintf(`{"type": "doc", "content": [%s]}`, inner)
}

func buildOversizedDoc(count int) string {
	nodes := make([]string, count)
	for i := range nodes {
		nodes[i] = `{"type": "paragraph", "content": [{"type": "text", "text": "x"}]}`
	}
	return fmt.Sprintf(`{"type": "doc", "content": [%s]}`, strings.Join(nodes, ","))
}
