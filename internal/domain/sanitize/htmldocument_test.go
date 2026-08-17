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
			name:  "root-relative image src preserved",
			input: `<img src="/uploads/media/dd3bbc8a44d5fe89.webp" alt="PlantUML Editor">`,
			want:  `<img src="/uploads/media/dd3bbc8a44d5fe89.webp" alt="PlantUML Editor">`,
		},
		{
			name:  "root-relative link href preserved",
			input: `<a href="/plantuml-editor.html">PlantUML Editor</a>`,
			want:  `<a href="/plantuml-editor.html">PlantUML Editor</a>`,
		},
		{
			name:  "page-relative link href preserved",
			input: `<a href="../about.html">About</a>`,
			want:  `<a href="../about.html">About</a>`,
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
	tests := []struct {
		name      string
		input     string
		allowlist []string
		want      string
	}{
		{
			name:      "allowed host iframe preserved",
			input:     `<iframe src="https://www.youtube.com/embed/123" width="560" height="315" allowfullscreen></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      `<iframe src="https://www.youtube.com/embed/123" width="560" height="315" allowfullscreen=""></iframe>`,
		},
		{
			name:      "wildcard host subdomain iframe preserved",
			input:     `<iframe src="https://aristorinjuang.disqus.com/embed/comments/"></iframe>`,
			allowlist: []string{"*.disqus.com"},
			want:      `<iframe src="https://aristorinjuang.disqus.com/embed/comments/"></iframe>`,
		},
		{
			name:      "multiple allowlisted hosts preserved",
			input:     `<iframe src="https://www.youtube.com/embed/123"></iframe><iframe src="https://aristorinjuang.disqus.com/embed/comments/"></iframe>`,
			allowlist: []string{"www.youtube.com", "*.disqus.com"},
			want:      `<iframe src="https://www.youtube.com/embed/123"></iframe><iframe src="https://aristorinjuang.disqus.com/embed/comments/"></iframe>`,
		},
		{
			name:      "protocol-relative iframe preserved",
			input:     `<iframe src="//www.youtube.com/embed/123"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      `<iframe src="//www.youtube.com/embed/123"></iframe>`,
		},
		{
			name:      "relative iframe src preserved",
			input:     `<iframe src="/demo/embed"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      `<iframe src="/demo/embed"></iframe>`,
		},
		{
			name:      "unknown host iframe stripped",
			input:     `<iframe src="https://evil.example.com/embed/123"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      ``,
		},
		{
			name:      "host substring spoofing stripped",
			input:     `<iframe src="https://www.youtube.com.evil.example/embed/123"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      ``,
		},
		{
			name:      "protocol-relative unknown host stripped",
			input:     `<iframe src="//evil.com/embed/1"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      ``,
		},
		{
			name:      "userinfo in allowed host stripped",
			input:     `<iframe src="https://www.youtube.com:80@evil.com/x"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      ``,
		},
		{
			name:      "scheme-relative userinfo stripped",
			input:     `<iframe src="//www.youtube.com@evil.com/x"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      ``,
		},
		{
			name:      "wildcard host does not allow apex",
			input:     `<iframe src="https://disqus.com/embed/"></iframe>`,
			allowlist: []string{"*.disqus.com"},
			want:      ``,
		},
		{
			name:      "uppercase src preserved",
			input:     `<iframe src="HTTPS://WWW.YOUTUBE.COM/EMBED/1"></iframe>`,
			allowlist: []string{"www.youtube.com"},
			want:      `<iframe src="https://WWW.YOUTUBE.COM/EMBED/1"></iframe>`,
		},
		{
			name:      "port entry preserves matching port",
			input:     `<iframe src="https://localhost:8080/embed"></iframe>`,
			allowlist: []string{"localhost:8080"},
			want:      `<iframe src="https://localhost:8080/embed"></iframe>`,
		},
		{
			name:      "port entry rejects other ports",
			input:     `<iframe src="https://localhost:9999/embed"></iframe>`,
			allowlist: []string{"localhost:8080"},
			want:      ``,
		},
		{
			name:      "path entry preserves matching path",
			input:     `<iframe src="https://media.example.com/path/embed/1"></iframe>`,
			allowlist: []string{"media.example.com/path/embed"},
			want:      `<iframe src="https://media.example.com/path/embed/1"></iframe>`,
		},
		{
			name:      "path entry rejects other paths",
			input:     `<iframe src="https://media.example.com/other"></iframe>`,
			allowlist: []string{"media.example.com/path/embed"},
			want:      ``,
		},
		{
			name:      "no allowlist strips iframe",
			input:     `<iframe src="https://www.youtube.com/embed/123"></iframe>`,
			allowlist: []string{},
			want:      ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitize.SanitizeHTMLDocument(tt.input, tt.allowlist...)
			assert.Equal(t, tt.want, got)
		})
	}
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
