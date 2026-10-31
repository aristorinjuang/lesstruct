package themestorefront

import (
	"embed"
	"io/fs"
)

// files is the storefront theme artifact: embedded layouts (pages/) and
// static assets (static/). Core resolves THEME_DIR overrides over this
// filesystem, so a child theme can shadow any page or asset by name.

//go:embed all:pages all:static
var files embed.FS

// Files returns the storefront artifact filesystem, rooted at the directories
// above (paths like "pages/product.gohtml" and "static/storefront.js"). Core
// installs it once at startup via the template package's setter.
func Files() fs.FS {
	return files
}
