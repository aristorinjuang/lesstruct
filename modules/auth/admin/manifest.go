// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for authentication.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions. Auth is chrome,
// not a navigation surface: login and setup screens render outside the
// shell, so the module declares no menu entries, settings sections, or
// editor extensions.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}
