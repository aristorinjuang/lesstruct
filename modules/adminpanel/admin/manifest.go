// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for the shell itself.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions. The shell renders
// what the other modules declare, so it contributes no menu entries,
// settings sections, or editor extensions of its own.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}
