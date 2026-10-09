// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for post types.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions. Post types surface
// in the admin UI data-driven — content tabs and the editor type dropdown
// read the post-types API — so the module declares no menu entries, settings
// sections, or editor extensions. The empty manifest still validates, which
// is the contract the shell checks.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}
