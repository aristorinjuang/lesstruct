package template

import (
	"io/fs"
	"net/http"
	"os"
	"path"
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

// readContentTemplate resolves the content template for a given post type slug.
// The lookup chain is:
//  1. Theme <slug>.html
//  2. Embedded <slug>.gohtml
//  3. Theme post.html
//  4. Embedded post.gohtml
//
// This handles the per-post-type template only; per-slug overrides
// (<postType>-<slug>.html) are discovered separately by
// findPerSlugTemplateOverrides at startup.
func readContentTemplate(theme *Theme, slug string) string {
	if theme != nil && theme.Dir != "" {
		if data, err := os.ReadFile(filepath.Join(theme.Dir, "templates", slug+".html")); err == nil {
			return string(data)
		}
	}

	if content := readEmbeddedPage(slug + ".html"); content != "" {
		return content
	}

	if theme != nil && theme.Dir != "" {
		if data, err := os.ReadFile(filepath.Join(theme.Dir, "templates", "post.html")); err == nil {
			return string(data)
		}
	}

	return readEmbeddedPage("post.html")
}

// findPerSlugTemplateOverrides scans the theme's templates directory for
// per-slug content template overrides. A per-slug override is a file named
// <postType>-<slug>.html (e.g. page-about.html, menu-item-special.html),
// where <postType> is one of the known post-type slugs. The longest matching
// post-type prefix wins, so a file menu-item-special.html is attributed to
// post type "menu-item" rather than "menu" when both are registered.
//
// Returns a map keyed by "<postType>:<slug>" whose values are the raw file
// contents. Returns an empty map when the theme is nil, the theme directory
// is empty, the templates directory cannot be read, or no per-slug overrides
// are present. Individual per-slug files are skipped on read error — a
// missing or unreadable file is treated as if it did not exist.
func findPerSlugTemplateOverrides(theme *Theme, knownPostTypes map[string]bool) map[string]string {
	overrides := make(map[string]string)
	if theme == nil || theme.Dir == "" || len(knownPostTypes) == 0 {
		return overrides
	}

	templatesDir := filepath.Join(theme.Dir, "templates")
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return overrides
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".html") {
			continue
		}

		base := strings.TrimSuffix(name, ".html")
		if base == "" {
			continue
		}

		matchedPostType := longestPostTypePrefix(base, knownPostTypes)
		if matchedPostType == "" {
			continue
		}

		slug := base[len(matchedPostType)+1:]
		if slug == "" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(templatesDir, name))
		if err != nil {
			continue
		}

		overrides[matchedPostType+":"+slug] = string(data)
	}

	return overrides
}

// longestPostTypePrefix returns the longest entry in knownPostTypes that
// matches base up to a hyphen boundary. Returns "" when no post type is a
// prefix of base followed by a hyphen (or when base is itself an exact
// post-type slug, which denotes a per-post-type template rather than a
// per-slug override).
func longestPostTypePrefix(base string, knownPostTypes map[string]bool) string {
	matched := ""
	for pt := range knownPostTypes {
		if len(base) <= len(pt) {
			continue
		}
		if !strings.HasPrefix(base, pt) {
			continue
		}
		if base[len(pt)] != '-' {
			continue
		}
		if len(pt) > len(matched) {
			matched = pt
		}
	}
	return matched
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

// rootFilesHandler serves files from the theme's optional root/ directory at
// the site root (e.g. /webpushr-sw.js for push service workers, which must
// live at a fixed root scope). Requests that do not map to an existing regular
// file fall through to next.
type rootFilesHandler struct {
	fsys fs.FS
	next http.Handler
}

func (h *rootFilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name != "" && !strings.Contains(name, "..") {
		if f, err := h.fsys.Open(name); err == nil {
			info, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !info.IsDir() {
				http.FileServerFS(h.fsys).ServeHTTP(w, r)
				return
			}
		}
	}

	h.next.ServeHTTP(w, r)
}

// RootFilesFS returns a filesystem over the theme's optional root/ directory,
// or nil when the theme is unset or ships no root/ directory. Files in that
// directory are served at the site root by the dynamic server and copied to
// the archive root by the static site generator.
func RootFilesFS(theme *Theme) fs.FS {
	if theme == nil || theme.Dir == "" {
		return nil
	}

	dir := filepath.Join(theme.Dir, "root")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	return os.DirFS(dir)
}

// RootFilesHandler wraps next with site-root serving of the theme's root/
// directory. When the theme provides no root/ directory, next is returned
// unchanged.
func RootFilesHandler(theme *Theme, next http.Handler) http.Handler {
	fsys := RootFilesFS(theme)
	if fsys == nil {
		return next
	}

	return &rootFilesHandler{fsys: fsys, next: next}
}
