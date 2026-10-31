package media

import (
	"net/url"
	"path/filepath"
	"strings"
)

// ExtractHashFromURL pulls the media hash (or foreign filename stem) out of a
// stored media URL: the file base name minus any thumbnail-variant suffix
// this package generates (_large, _medium, _thumb). Foreign URLs (external
// images) degrade to their filename stem, so callers can still use the value
// as a best-effort lookup key against FindByHashPrefix.
func ExtractHashFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return ""
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for _, suffix := range []string{"_large", "_medium", "_thumb"} {
		if rest, ok := strings.CutSuffix(name, suffix); ok {
			name = rest
			break
		}
	}
	return name
}
