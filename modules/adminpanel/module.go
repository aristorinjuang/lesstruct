// Package adminpanel is the official admin module: the admin SPA shell
// served at /admin plus the frozen admin API surface and the manifest
// endpoint aggregating every feature module's admin extensions.
package adminpanel

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel/admin"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel/spa"
)

// Name is the registry key for this module.
const Name = "adminpanel"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module serving
// the embedded production build.
type Option func(*Module)

// WithDevMode serves the admin SPA from the Vite dev server (reverse proxy)
// instead of the embedded production build.
func WithDevMode(devMode bool) Option {
	return func(m *Module) {
		m.devMode = devMode
	}
}

// WithAdminDevURL sets the Vite dev server target used when dev mode is on.
func WithAdminDevURL(adminURL string) Option {
	return func(m *Module) {
		m.adminURL = adminURL
	}
}

// Module is the adminpanel kernel module.
type Module struct {
	devMode    bool
	adminURL   string
	spaHandler *spa.Handler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. The shell authenticates through the auth
// module, matching the retired wrapper edge.
func (m *Module) Requires() []string {
	return []string{"auth"}
}

// Optional lists soft dependencies. There are none: every feature module's
// admin extensions are discovered through the AdminExtensionProvider
// capability, never through a declared edge.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The shell is static file serving plus the manifest
// aggregation, both attached in later slices; Init only validates presence.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// AdminManifest declares this module's admin-shell extensions. The shell
// itself contributes no navigation: it renders what the other modules
// declare.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// SPAHandler exposes the admin SPA handler for core mounting. Core mounts
// it explicitly (rather than via RouteProvider) because the /admin premises
// share one mount point with the static server's nil-gated behavior, which
// a single RegisterRoutes call cannot express.
func (m *Module) SPAHandler() *spa.Handler {
	return m.spaHandler
}

// New builds the module, adopting the composition root's wiring. The SPA
// handler is built once here so every premise serves the same instance.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.spaHandler = spa.NewHandler(m.devMode, m.adminURL)
	return m
}
