// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for user management.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions. The Users
// navigation surface is static SPA chrome today, so the module declares no
// menu entries, settings sections, or editor extensions.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}
