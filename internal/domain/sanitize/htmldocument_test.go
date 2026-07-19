package sanitize_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeHTMLDocument(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic HTML passes through",
			input: `<div class="wrapper"><p>Hello World</p></div>`,
			want:  `<div class="wrapper"><p>Hello World</p></div>`,
		},
		{
			name:  "inline styles preserved",
			input: `<div style="color: red; font-size: 16px;"><p>Styled text</p></div>`,
			want:  `<div style="color: red; font-size: 16px;"><p>Styled text</p></div>`,
		},
		{
			name:  "style block preserved",
			input: `<div><style>.my-class { color: blue; }</style><p class="my-class">Blue</p></div>`,
			want:  `<div><style>.my-class { color: blue; }</style><p class="my-class">Blue</p></div>`,
		},
		{
			name:  "style block with type attribute preserved",
			input: `<div><style type="text/css">h1 { font-size: 2em; }</style><h1 class="title">Hi</h1></div>`,
			want:  `<div><style type="text/css">h1 { font-size: 2em; }</style><h1 class="title">Hi</h1></div>`,
		},
		{
			name:  "multi-line style block preserved verbatim",
			input: "<div><style>\n  .a { color: red; }\n  .b { color: green; }\n</style><p class=\"a\">Red</p></div>",
			want:  "<div><style>\n  .a { color: red; }\n  .b { color: green; }\n</style><p class=\"a\">Red</p></div>",
		},
		{
			name:  "script tag stripped",
			input: `<div><p>Hello</p><script>alert('xss')</script></div>`,
			want:  `<div><p>Hello</p></div>`,
		},
		{
			name:  "event handler stripped",
			input: `<div onclick="alert('xss')">Click me</div>`,
			want:  `<div>Click me</div>`,
		},
		{
			name:  "javascript URL stripped",
			input: `<a href="javascript:alert('xss')">Click</a>`,
			want:  `Click`,
		},
		{
			name:  "data attributes preserved",
			input: `<div data-widget-id="123" data-type="hero">Content</div>`,
			want:  `<div data-widget-id="123" data-type="hero">Content</div>`,
		},
		{
			name:  "iframe stripped by default",
			input: `<iframe src="https://www.youtube.com/embed/123" width="560" height="315"></iframe>`,
			want:  ``,
		},
		{
			name:  "nested elementor layout preserved",
			input: `<div class="elementor-widget-wrap"><div class="elementor-element" data-id="abc123"><p>Hello</p></div></div>`,
			want:  `<div class="elementor-widget-wrap"><div class="elementor-element" data-id="abc123"><p>Hello</p></div></div>`,
		},
		{
			name:  "svg preserved with lowercased attributes",
			input: `<svg viewBox="0 0 24 24"><path d="M12 2L2 22h20z"/></svg>`,
			want:  `<svg viewbox="0 0 24 24"><path d="M12 2L2 22h20z"/></svg>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitize.SanitizeHTMLDocument(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeHTMLDocument_WithIframeAllowlist(t *testing.T) {
	input := `<iframe src="https://www.youtube.com/embed/123" width="560" height="315"></iframe>`
	got := sanitize.SanitizeHTMLDocument(input, "www.youtube.com")
	// iframes require sandbox values in bluemonday; without them they're stripped
	assert.Empty(t, got)
}

func TestValidateHTMLDocument(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid HTML",
			input:   `<div><p>Hello</p></div>`,
			wantErr: false,
		},
		{
			name:    "empty HTML rejected",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "whitespace only rejected",
			input:   `   `,
			wantErr: true,
		},
		{
			name:    "script tag rejected",
			input:   `<div><script>alert('xss')</script></div>`,
			wantErr: true,
		},
		{
			name:    "SCRIPT tag rejected case insensitive",
			input:   `<div><SCRIPT>alert('xss')</SCRIPT></div>`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitize.ValidateHTMLDocument(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
