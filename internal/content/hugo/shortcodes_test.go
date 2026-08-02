package hugo_test

import (
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
			name:     "transformHighlight with language",
			body:     `{{< highlight "go" >}}package main{{< /highlight >}}`,
			expected: `<pre><code class="language-go">package main</code></pre>`,
			wantErr:  false,
		},
		{
			name:     "transformHighlight without language",
			body:     `{{< highlight >}}code{{< /highlight >}}`,
			expected: `{{< highlight >}}code{{< /highlight >}}`,
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
		{
			name:     "combined highlight and iframe",
			body:     `{{< highlight "go" >}}package main{{< /highlight >}}{{< iframe src="https://example.com" >}}`,
			expected: `<pre><code class="language-go">package main</code></pre><iframe width="740" height="416" src="https://example.com" title="Embedded content"></iframe>`,
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
