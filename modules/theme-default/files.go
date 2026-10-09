package themedefault

import (
	"embed"
	"io/fs"
)

// files is the default theme artifact: embedded layouts (pages/) and static
// assets (static/). Core resolves THEME_DIR overrides over this filesystem;
// the layout mirrors the historical internal/api/template tree so every
// "pages/<name>.gohtml" and "static/<name>" lookup keeps working unchanged.

//go:embed all:pages all:static
var files embed.FS

// Files returns the default theme artifact filesystem, rooted at the
// directories above (paths like "pages/post.gohtml" and "static/base.css").
// Core installs it once at startup via the template package's setter; without
// it (a custom edition omitting this module) only THEME_DIR files resolve.
func Files() fs.FS {
	return files
}
