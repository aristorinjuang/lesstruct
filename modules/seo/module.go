// Package seo is the official SEO module: metadata generation, sitemap and
// robots discovery surfaces, and legacy-URL aliases.
package seo

import (
	"io/fs"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/seo/admin"
	"github.com/aristorinjuang/lesstruct/modules/seo/api"
)

const (
	// Name is the registry key for this module.
	Name = "seo"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that serves
// nothing (the handler built over a nil pages source must not be invoked).
type Option func(*Module)

// WithPages attaches the published-content source for sitemap generation.
func WithPages(pages api.PublishedPages) Option {
	return func(m *Module) {
		m.pages = pages
	}
}

// WithBaseURL attaches the canonical site URL for absolute sitemap URLs.
// A trailing slash is trimmed at handler construction.
func WithBaseURL(baseURL string) Option {
	return func(m *Module) {
		m.baseURL = baseURL
	}
}

// WithHeadless disables the XML sitemap (no server-rendered site exists to
// advertise) and blocks all crawling in robots.txt.
func WithHeadless(headless bool) Option {
	return func(m *Module) {
		m.headless = headless
	}
}

// WithLogger attaches the module logger. Without one handler failures are
// dropped (discard sink).
func WithLogger(logger kernel.Logger) Option {
	return func(m *Module) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// Module is the seo kernel module.
type Module struct {
	pages    api.PublishedPages
	baseURL  string
	headless bool
	logger   kernel.Logger
	handler  *api.SEOHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Aliases reference content items loosely
// (content IDs without foreign-key enforcement), so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. None.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. Migration execution is core-driven (it needs the
// live *sql.DB, which modules never touch directly).
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// Migrations returns the embedded migration filesystem, laid out per driver
// ({sqlite,postgresql,mysql}/NNN_name.{up,down}.sql). The core runner tracks
// each module in its own schema_migrations_<module> table.
func (m *Module) Migrations() fs.FS {
	// The embed directive roots paths at migrations/; strip the prefix so the
	// runner sees bare driver dirs. The subtree always exists (embedded), so
	// the error is impossible — it is explicitly ignored, never panicked on.
	sub, _ := fs.Sub(migrationsFS, "migrations")
	return sub
}

// AdminManifest declares this module's admin-shell extensions. SEO surfaces
// in the admin UI data-driven (the content editor's collapsible SEO settings
// read the content SEO API), so the module declares no menu entries, settings
// sections, or editor extensions.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// Handler exposes the discovery-surface handler for core mounting. Core
// mounts it explicitly (rather than via RouteProvider) because the three
// leaves live in three different middleware groups (root, /api, /api/v1),
// which a single RegisterRoutes call cannot express.
func (m *Module) Handler() *api.SEOHandler {
	return m.handler
}

// New builds the module, adopting the composition root's wiring. The handler
// is built once here so routes serve the same instance.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.handler = api.NewSEOHandler(m.pages, m.baseURL, m.headless, m.logger)
	return m
}
