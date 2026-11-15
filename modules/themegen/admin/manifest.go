// Package admin owns this module's admin-shell manifest: the declarative UI
// the single admin SPA renders for theme generation.
package admin

import (
	"github.com/aristorinjuang/lesstruct/kernel"
)

// Manifest returns this module's admin-shell extensions: the theme
// generator and installed-themes navigation entries.
func Manifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{
		MenuEntries: []kernel.AdminMenuEntry{
			{Label: "Theme Generator", Path: "/theme-generator", Icon: "palette"},
			{Label: "Themes", Path: "/themes", Icon: "swatch"},
		},
	}
}
