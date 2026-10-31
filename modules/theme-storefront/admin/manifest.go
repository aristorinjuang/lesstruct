// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for the storefront theme.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions. A theme ships no
// admin UI of its own — the admin panel and content editor are engine
// concerns — so the module declares no menu entries, settings sections, or
// editor extensions. The empty manifest still validates, which is the
// contract the shell checks.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}
