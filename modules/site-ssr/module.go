// Package sitessr is the official public-site SSR engine module: contentpage
// data assembly, template resolution, and the HTTP surfaces that render the
// content site (plus its auth pages).
package sitessr

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/site-ssr/admin"
)

// Name is the registry key for this module.
const Name = "site-ssr"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Module is the site-ssr kernel module.
type Module struct{}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. The engine degrades without any single
// content source at the call site (each lookup logs and continues), so
// nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. Any theme satisfies the engine; without
// one only THEME_DIR files resolve.
func (m *Module) Optional() []string {
	return []string{"theme-default"}
}

// Init wires the module. There is nothing to wire yet; the assembler and
// handler attach in later slices.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// AdminManifest declares this module's admin-shell extensions. The SSR engine
// ships no admin UI of its own, so the manifest is empty.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// New builds the module. The assembler, handler, and theme wiring attach in
// later slices (3.5d/3.5e); this slice carries lifecycle plus the content
// projection seam.
func New() *Module {
	return &Module{}
}
