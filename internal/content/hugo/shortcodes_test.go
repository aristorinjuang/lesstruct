package hugo_test

import (
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformShortcodes(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
		wantErr  bool
	}{
		{
			name:     "transformHighlight without arguments becomes a plain code block",
			body:     `{{< highlight >}}code{{< /highlight >}}`,
			expected: `<pre><code>code</code></pre>`,
			wantErr:  false,
		},
		{
			name:     "transformHighlight with no closing tag",
			body:     `{{< highlight "go" >}}package main`,
			expected: `{{< highlight "go" >}}package main`,
			wantErr:  false,
		},
		{
			name:     "transformHighlight with text language",
			body:     `{{< highlight "text" >}}plain{{< /highlight >}}`,
			expected: `<pre><code>plain</code></pre>`,
			wantErr:  false,
		},
		{
			name:     "transformHighlight with unknown language falls back to escaped code block",
			body:     `{{< highlight notalang42 "linenos=table" >}}a < b & c{{< /highlight >}}`,
			expected: `<pre><code class="language-notalang42">a &lt; b &amp; c</code></pre>`,
			wantErr:  false,
		},
		{
			name:     "transformIframe with src",
			body:     `{{< iframe src="https://example.com" >}}`,
			expected: `<iframe width="740" height="416" src="https://example.com" title="Embedded content"></iframe>`,
			wantErr:  false,
		},
		{
			name:     "transformIframe with youtube param",
			body:     `{{< iframe youtube="dQw4w9WgXcQ" >}}`,
			expected: `<iframe width="740" height="416" src="https://www.youtube.com/embed/dQw4w9WgXcQ" title="YouTube video player" frameborder="0" allowfullscreen></iframe>`,
			wantErr:  false,
		},
		{
			name:     "transformIframe with src AND title",
			body:     `{{< iframe src="https://example.com" title="My Video" >}}`,
			expected: `<iframe width="740" height="416" src="https://example.com" title="My Video"></iframe>`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hugo.TransformShortcodes(tt.body)
			require.NotPanics(t, func() { hugo.TransformShortcodes(tt.body) })
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransformShortcodes_HighlightChroma(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "whitespace-tolerant closing tag converts the block",
			body: `{{< highlight go >}}package main{{< / highlight >}}`,
			wantContains: []string{
				`<div class="highlight">`,
				`<pre class="chroma"><code class="language-go nohighlight" data-lang="go">`,
				`package`,
			},
		},
		{
			name: "quoted language and quoted linenos=table option",
			body: `{{< highlight go "linenos=table" >}}package main{{< /highlight >}}`,
			wantContains: []string{
				`class="lntable"`,
				`class="lntd"`,
				`class="lnt"`,
				`class="language-go nohighlight" data-lang="go"`,
			},
		},
		{
			name: "unquoted language and unquoted linenos=inline option",
			body: `{{< highlight js linenos=inline >}}const a = 1;{{< /highlight >}}`,
			wantContains: []string{
				`class="language-js nohighlight" data-lang="js"`,
				`class="ln"`,
			},
			wantNotContains: []string{"lntable"},
		},
		{
			name: "linenos=false disables line numbers",
			body: `{{< highlight go linenos=false >}}package main{{< /highlight >}}`,
			wantContains: []string{
				`<div class="highlight">`,
				`class="language-go nohighlight"`,
			},
			wantNotContains: []string{"lntable", `"ln"`},
		},
		{
			name: "code body is HTML-escaped",
			body: `{{< highlight go >}}if a < b && c > d { x("&y") }{{< /highlight >}}`,
			wantContains: []string{
				`&lt;`,
				`&amp;&amp;`,
				`&gt;`,
				`&#34;&amp;y&#34;`,
			},
		},
		{
			name: "multiple blocks in one body all convert",
			body: `before {{< highlight go >}}one{{< /highlight >}} middle {{< highlight python >}}two{{< / highlight >}} after`,
			wantContains: []string{
				`class="language-go nohighlight"`,
				`class="language-python nohighlight"`,
				"before",
				"middle",
				"after",
			},
		},
		{
			name: "surrounding text is preserved verbatim",
			body: `<p>intro</p>{{< highlight go >}}x{{< /highlight >}}<p>outro</p>`,
			wantContains: []string{
				"<p>intro</p>",
				`<div class="highlight">`,
				"<p>outro</p>",
			},
		},
		{
			name: "unknown options are ignored without breaking conversion",
			body: `{{< highlight go style=monokai guessSyntax=true >}}x{{< /highlight >}}`,
			wantContains: []string{
				`<div class="highlight">`,
				`class="language-go nohighlight"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hugo.TransformShortcodes(tt.body)

			for _, want := range tt.wantContains {
				assert.Contains(t, result, want)
			}
			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, result, notWant)
			}

			// Shortcode soup must never survive a matched pair.
			assert.NotContains(t, result, "{{<")
		})
	}
}

func TestTransformShortcodes_CombinedHighlightAndIframe(t *testing.T) {
	result := hugo.TransformShortcodes(
		`{{< highlight go >}}package main{{< / highlight >}}{{< iframe src="https://example.com" >}}`,
	)

	assert.Contains(t, result, `<div class="highlight">`)
	assert.Contains(t, result, `title="Embedded content"`)
	assert.False(t, strings.Contains(result, "{{<"), "no shortcode may survive")
}
