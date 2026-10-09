// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for SEO.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions. SEO surfaces in the
// admin UI data-driven — the content editor's collapsible SEO settings read
// the content SEO API — so the module declares no menu entries, settings
// sections, or editor extensions. The empty manifest still validates, which
// is the contract the shell checks.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}
