// Package themedefault is the official default theme module: the embedded
// layouts, static assets, and template funcs the site-ssr engine consumes.
package themedefault

import (
	"html/template"
	"io/fs"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/theme-default/admin"
)

// Name is the registry key for this module.
const Name = "theme-default"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Module is the theme-default kernel module.
type Module struct{}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. The theme is a leaf artifact consumed by
// the site-ssr engine, so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. None.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. There is nothing to wire beyond the embedded files.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// ThemeFiles returns the default theme artifact filesystem (layouts under
// pages/, static assets under static/). The site-ssr engine resolves
// THEME_DIR overrides over it.
func (m *Module) ThemeFiles() fs.FS {
	return Files()
}

// ThemeFuncs returns the template funcs this theme contributes. The default
// theme adds none beyond the engine's own (urlpath, t, assetURL, formatDate,
// formatDateTime) — the empty map keeps the provider contract explicit.
func (m *Module) ThemeFuncs() template.FuncMap {
	return template.FuncMap{}
}

// AdminManifest declares this module's admin-shell extensions. A theme ships
// no admin UI of its own, so the manifest is empty.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// New builds the module. The theme artifact is static; construction takes no
// options.
func New() *Module {
	return &Module{}
}
