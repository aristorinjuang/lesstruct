package template

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// readEmbeddedPage reads a template file from the embedded pages filesystem.
// It maps .html filenames to .gohtml extensions used by the embedded files.
func readEmbeddedPage(filename string) string {
	embeddedPath := "pages/" + strings.TrimSuffix(filename, ".html") + ".gohtml"
	data, err := fs.ReadFile(pagesFS, embeddedPath)
	if err != nil {
		return ""
	}
	return string(data)
}

// Theme holds the path to a custom theme directory on disk.
// When Dir is empty, embedded defaults are used.
type Theme struct {
	Dir string
}

// readThemeFile reads a template file from the theme's templates directory,
// falling back to the embedded page if the theme is nil, empty,
// or the file does not exist.
func readThemeFile(theme *Theme, filename string) string {
	if theme == nil || theme.Dir == "" {
		return readEmbeddedPage(filename)
	}

	data, err := os.ReadFile(filepath.Join(theme.Dir, "templates", filename))
	if err != nil {
		return readEmbeddedPage(filename)
	}

	return string(data)
}

// compositeFS implements fs.FS by checking a primary filesystem first,
// then falling back to a secondary filesystem.
type compositeFS struct {
	primary   fs.FS
	secondary fs.FS
}

// Open tries the primary filesystem first, then falls back to secondary.
func (c *compositeFS) Open(name string) (fs.File, error) {
	f, err := c.primary.Open(name)
	if err == nil {
		return f, nil
	}

	return c.secondary.Open(name)
}

// NewCompositeFSForTest creates a compositeFS for testing purposes.
func NewCompositeFSForTest(primary, secondary fs.FS) fs.FS {
	return &compositeFS{
		primary:   primary,
		secondary: secondary,
	}
}

// resolveFS returns a filesystem that checks the theme directory on disk
// first, then falls back to the embedded filesystem.
// If theme is nil or theme.Dir is empty, the embedded filesystem is returned directly.
func resolveFS(theme *Theme, embedded fs.FS, subPath string) fs.FS {
	if theme == nil || theme.Dir == "" {
		if subPath == "" {
			return embedded
		}

		sub, _ := fs.Sub(embedded, subPath)

		return sub
	}

	var primary fs.FS
	if subPath != "" {
		primary = os.DirFS(theme.Dir + "/" + subPath)
	} else {
		primary = os.DirFS(theme.Dir)
	}

	secondary := embedded
	if subPath != "" {
		sub, _ := fs.Sub(embedded, subPath)
		secondary = sub
	}

	return &compositeFS{
		primary:   primary,
		secondary: secondary,
	}
}

// ResolveFSForTest exposes resolveFS for testing purposes.
func ResolveFSForTest(theme *Theme, embedded fs.FS, subPath string) fs.FS {
	return resolveFS(theme, embedded, subPath)
}
