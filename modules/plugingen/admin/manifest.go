// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for plugin generation.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions: the plugin
// generator and installed-projects navigation entries.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{
		MenuEntries: []kernel.AdminMenuEntry{
			{Label: "Plugin Generator", Path: "/plugin-generator", Icon: "puzzle"},
			{Label: "Plugins", Path: "/plugins", Icon: "blocks"},
		},
	}
}
