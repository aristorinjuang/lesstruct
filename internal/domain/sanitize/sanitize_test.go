package sanitize_test

import (
	"testing"

	sanitizedomain "github.com/aristorinjuang/lesstruct/internal/domain/sanitize"
)

func TestSanitizePlainText(t *testing.T) {
	s := sanitizedomain.NewSanitizer()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "strips script tags",
			input: `<script>alert("xss")</script>`,
			want:  "",
		},
		{
			name:  "strips img onerror",
			input: `<img src="x" onerror="alert(1)">`,
			want:  "",
		},
		{
			name:  "strips javascript href",
			input: `<a href="javascript:alert(1)">click</a>`,
			want:  "click",
		},
		{
			name:  "strips iframe tags",
			input: `<iframe src="evil.com"></iframe>`,
			want:  "",
		},
		{
			name:  "strips object tags",
			input: `<object data="evil.swf"></object>`,
			want:  "",
		},
		{
			name:  "strips embed tags",
			input: `<embed src="evil.swf">`,
			want:  "",
		},
		{
			name:  "strips form tags",
			input: `<form action="/steal"><input type="text"></form>`,
			want:  "",
		},
		{
			name:  "strips style tags",
			input: `<style>body{display:none}</style>`,
			want:  "",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "HTML entities preserved as text",
			input: "5 &lt; 10",
			want:  "5 &lt; 10",
		},
		{
			name:  "strips event handlers on allowed tags",
			input: `<b onclick="alert(1)">bold</b>`,
			want:  "bold",
		},
		{
			name:  "strips nested script tags",
			input: `<div><script>alert(1)</script></div>`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SanitizePlainText(tt.input)
			if got != tt.want {
				t.Errorf("SanitizePlainText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeRichHTML(t *testing.T) {
	s := sanitizedomain.NewSanitizer()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "preserves paragraph tags",
			input: "<p>Hello world</p>",
			want:  "<p>Hello world</p>",
		},
		{
			name:  "preserves strong and em tags",
			input: "<strong>bold</strong> and <em>italic</em>",
			want:  "<strong>bold</strong> and <em>italic</em>",
		},
		{
			name:  "preserves headings",
			input: "<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>",
			want:  "<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>",
		},
		{
			name:  "preserves list tags",
			input: "<ul><li>item</li></ul><ol><li>item</li></ol>",
			want:  "<ul><li>item</li></ul><ol><li>item</li></ol>",
		},
		{
			name:  "preserves blockquote",
			input: "<blockquote>A quote</blockquote>",
			want:  "<blockquote>A quote</blockquote>",
		},
		{
			name:  "preserves code and pre tags",
			input: "<pre><code>func main() {}</code></pre>",
			want:  "<pre><code>func main() {}</code></pre>",
		},
		{
			name:  "preserves links with safe href",
			input: `<a href="https://example.com">link</a>`,
			want:  `<a href="https://example.com">link</a>`,
		},
		{
			name:  "preserves image tags",
			input: `<img src="https://example.com/img.jpg" alt="test">`,
			want:  `<img src="https://example.com/img.jpg" alt="test">`,
		},
		{
			name:  "strips script tags",
			input: "<p>Hello</p><script>alert(1)</script>",
			want:  "<p>Hello</p>",
		},
		{
			name:  "strips iframe tags",
			input: "<p>Hello</p><iframe src=\"evil.com\"></iframe>",
			want:  "<p>Hello</p>",
		},
		{
			name:  "strips object tags",
			input: "<p>Hello</p><object data=\"evil.swf\"></object>",
			want:  "<p>Hello</p>",
		},
		{
			name:  "strips embed tags",
			input: "<p>Hello</p><embed src=\"evil.swf\">",
			want:  "<p>Hello</p>",
		},
		{
			name:  "strips form tags",
			input: "<p>Hello</p><form action=\"evil\"><input></form>",
			want:  "<p>Hello</p>",
		},
		{
			name:  "strips onclick attribute",
			input: `<p onclick="alert(1)">Hello</p>`,
			want:  "<p>Hello</p>",
		},
		{
			name:  "strips onerror attribute",
			input: `<img src="x" onerror="alert(1)">`,
			want:  `<img src="x">`,
		},
		{
			name:  "strips onload attribute",
			input: `<img src="x" onload="alert(1)">`,
			want:  `<img src="x">`,
		},
		{
			name:  "strips onmouseover attribute",
			input: `<a href="https://example.com" onmouseover="alert(1)">link</a>`,
			want:  `<a href="https://example.com">link</a>`,
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "plain text without HTML",
			input: "Just some text",
			want:  "Just some text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SanitizeRichHTML(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeRichHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContainsHTML(t *testing.T) {
	s := sanitizedomain.NewSanitizer()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "plain text no HTML",
			input: "Hello world",
			want:  false,
		},
		{
			name:  "contains script tag",
			input: `<script>alert(1)</script>`,
			want:  true,
		},
		{
			name:  "contains img tag",
			input: `<img src="test.jpg">`,
			want:  true,
		},
		{
			name:  "contains bold tag",
			input: "<b>bold</b>",
			want:  true,
		},
		{
			name:  "empty string no HTML",
			input: "",
			want:  false,
		},
		{
			name:  "contains anchor tag",
			input: `<a href="https://example.com">link</a>`,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.ContainsHTML(tt.input)
			if got != tt.want {
				t.Errorf("ContainsHTML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeHTMLWithSafeLinks(t *testing.T) {
	s := sanitizedomain.NewSanitizer()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "preserves https link",
			input: `<a href="https://example.com">link</a>`,
			want:  `<a href="https://example.com">link</a>`,
		},
		{
			name:  "preserves http link",
			input: `<a href="http://example.com">link</a>`,
			want:  `<a href="http://example.com">link</a>`,
		},
		{
			name:  "removes javascript href keeping text",
			input: `<a href="javascript:alert(1)">evil</a>`,
			want:  "evil",
		},
		{
			name:  "removes data href keeping text",
			input: `<a href="data:text/html,evil">evil</a>`,
			want:  "evil",
		},
		{
			name:  "removes vbscript href keeping text",
			input: `<a href="vbscript:alert(1)">evil</a>`,
			want:  "evil",
		},
		{
			name:  "preserves plain text with no links",
			input: "Just text",
			want:  "Just text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SanitizeHTMLWithSafeLinks(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeHTMLWithSafeLinks() = %q, want %q", got, tt.want)
			}
		})
	}
}
