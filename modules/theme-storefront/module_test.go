package themestorefront_test

import (
	"io/fs"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	themestorefront "github.com/aristorinjuang/lesstruct/modules/theme-storefront"
	"github.com/aristorinjuang/lesstruct/modules/theme-storefront/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModule_ProductGalleryContract pins the product gallery wiring: the main
// image prefers the resolved _large rendition, thumbs carry _thumb with a
// data-image swap target, and the legacy lines list stays as the fallback for
// textarea-shaped fields.
func TestModule_ProductGalleryContract(t *testing.T) {
	files := testutil.NewModule().ThemeFiles()
	raw, err := fs.ReadFile(files, "pages/product.gohtml")
	require.NoError(t, err)
	body := string(raw)

	tests := []struct {
		name   string
		needle string
	}{
		{
			name:   "gallery main renders the large rendition",
			needle: "product-main-image",
		},
		{
			name:   "main prefers the resolved Large URL",
			needle: ".Large",
		},
		{
			name:   "thumbs carry the Thumb rendition",
			needle: ".Thumb",
		},
		{
			name:   "thumb swap targets the large rendition via data-image",
			needle: "data-image",
		},
		{
			name:   "thumb swap carries the srcset via data-srcset",
			needle: "data-srcset",
		},
		{
			name:   "resolved galleries come from CustomFieldGalleries",
			needle: "CustomFieldGalleries",
		},
		{
			name:   "legacy lines list stays as the fallback",
			needle: "lines (index .CustomFields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, body, tt.needle)
		})
	}
}

func TestModule_Lifecycle(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, themestorefront.Name, module.Name())
	assert.Equal(t, themestorefront.Version, module.Version())
	assert.Empty(t, module.Requires())
	assert.Empty(t, module.Optional())
	require.NoError(t, module.Init(testutil.Kernel()))
	require.NoError(t, module.Shutdown())
}

func TestModule_ThemeFiles(t *testing.T) {
	module := testutil.NewModule()

	files := module.ThemeFiles()
	require.NotNil(t, files)

	// Every layout, shell, and asset the engine resolves must be present under
	// the historical paths.
	for _, path := range []string{
		"pages/layout.gohtml",
		"pages/post.gohtml",
		"pages/product.gohtml",
		"pages/index.gohtml",
		"pages/homepage.gohtml",
		"pages/author.gohtml",
		"pages/tag.gohtml",
		"pages/not_found.gohtml",
		"pages/login.gohtml",
		"pages/register.gohtml",
		"pages/forgot_password.gohtml",
		"pages/verify_email.gohtml",
		"pages/reset_password.gohtml",
		"pages/cart.gohtml",
		"pages/checkout.gohtml",
		"pages/orders-track.gohtml",
		"static/base.css",
		"static/style.css",
		"static/storefront.js",
		"static/nav-auth.js",
		"static/comments.js",
		"static/favicon.svg",
		"static/favicon.ico",
		"static/favicon-96x96.png",
		"static/apple-touch-icon.png",
	} {
		_, err := fs.Stat(files, path)
		assert.NoError(t, err, "theme file %q must exist", path)
	}
}

func TestModule_ThemeFuncs(t *testing.T) {
	module := testutil.NewModule()

	funcs := module.ThemeFuncs()
	require.NotNil(t, funcs)
	lines, ok := funcs["lines"].(func(any) []string)
	require.True(t, ok, "lines must be a template func")

	tests := []struct {
		name     string
		input    any
		expected []string
	}{
		{
			name:     "newline separated values are trimmed",
			input:    " /media/a.jpg \n\n/media/b.jpg\n",
			expected: []string{"/media/a.jpg", "/media/b.jpg"},
		},
		{
			name:     "blank lines are dropped",
			input:    "\n  \n",
			expected: []string{},
		},
		{
			name:     "missing field yields no lines",
			input:    nil,
			expected: nil,
		},
		{
			name:     "non-string values yield no lines",
			input:    42,
			expected: nil,
		},
		{
			name: "canonical gallery array extracts urls in order",
			input: []any{
				map[string]any{"url": "/media/a.jpg", "alt": "A"},
				map[string]any{"url": "/media/b.jpg", "alt": ""},
			},
			expected: []string{"/media/a.jpg", "/media/b.jpg"},
		},
		{
			name: "gallery blanks are dropped and malformed items skipped",
			input: []any{
				map[string]any{"url": "  ", "alt": "blank"},
				map[string]any{"alt": "missing url"},
				map[string]any{"url": 42},
				"  /media/c.jpg  ",
				map[string]any{"url": "/media/d.jpg", "alt": "D", "extra": "ignored-still-valid"},
			},
			expected: []string{"/media/c.jpg", "/media/d.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, lines(tt.input))
		})
	}
}

func TestModule_ThemePages(t *testing.T) {
	module := testutil.NewModule()

	assert.Equal(t, []string{"cart", "checkout", "orders/track"}, module.ThemePages())
}

func TestModule_AdminManifest(t *testing.T) {
	module := testutil.NewModule()

	manifest := module.AdminManifest()
	require.NoError(t, manifest.Validate())
	assert.Empty(t, manifest.MenuEntries)
}

func TestModule_ImplementsThemeProvider(t *testing.T) {
	var _ kernel.ThemeProvider = testutil.NewModule()
}

func TestModule_ImplementsThemePageProvider(t *testing.T) {
	var _ kernel.ThemePageProvider = testutil.NewModule()
}
