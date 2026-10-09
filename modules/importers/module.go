// Package importers is the official content-migration module: the WordPress
// WXR importer and the Hugo archive importer, each with its async admin
// handler over a shared in-memory job store.
package importers

import (
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/importers/admin"
	"github.com/aristorinjuang/lesstruct/modules/importers/api"
	"github.com/aristorinjuang/lesstruct/modules/importers/hugo"
	"github.com/aristorinjuang/lesstruct/modules/importers/wordpress"
)

// Name is the registry key for this module.
const Name = "importers"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module whose
// handlers serve the adopted (possibly nil) importers.
type Option func(*Module)

// WithWordPressImporter adopts the WordPress importer (composition root).
func WithWordPressImporter(importer *wordpress.Importer) Option {
	return func(m *Module) {
		m.wordpressImporter = importer
	}
}

// WithHugoImporter adopts the Hugo importer (composition root).
func WithHugoImporter(importer *hugo.Importer) Option {
	return func(m *Module) {
		m.hugoImporter = importer
	}
}

// WithMaxImportBytes sets the total upload ceiling applied via
// MaxBytesReader by both handlers.
func WithMaxImportBytes(maxBytes int64) Option {
	return func(m *Module) {
		m.maxImportBytes = maxBytes
	}
}

// WithWordPressTimeout sets the per-job ceiling for WordPress imports.
func WithWordPressTimeout(timeout time.Duration) Option {
	return func(m *Module) {
		m.wordpressTimeout = timeout
	}
}

// WithHugoTimeout sets the per-job ceiling for Hugo imports.
func WithHugoTimeout(timeout time.Duration) Option {
	return func(m *Module) {
		m.hugoTimeout = timeout
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

// Module is the importers kernel module.
type Module struct {
	wordpressImporter *wordpress.Importer
	hugoImporter      *hugo.Importer
	maxImportBytes    int64
	wordpressTimeout  time.Duration
	hugoTimeout       time.Duration
	logger            kernel.Logger
	wordpressHandler  *api.WordPressHandler
	hugoHandler       *api.HugoHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. The importers degrade to standalone
// operation on the adopted services, so nothing is required: a disabled
// media or seo edition must not fail the boot.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. Media re-upload, post-type schemas, and
// alias repointing enrich imports; absent either, the importers still work
// within the adopted services.
func (m *Module) Optional() []string {
	return []string{"media", "posttypes-basic", "seo"}
}

// Init wires the module. The handlers are already built by New; Init only
// validates presence.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory (the job
// stores are in-memory by design — a restart loses polling state, matching
// the pre-module behavior).
func (m *Module) Shutdown() error {
	return nil
}

// AdminManifest declares this module's admin-shell extensions: the Import
// section (with its WordPress and Hugo pages rendered from the static
// router) and nothing else.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// WordPressHandler exposes the WordPress import handler for core mounting.
// Core mounts it explicitly (rather than via RouteProvider) because the
// surface spans the browser admin group and the agent realm, which a single
// RegisterRoutes call cannot express.
func (m *Module) WordPressHandler() *api.WordPressHandler {
	return m.wordpressHandler
}

// HugoHandler exposes the Hugo import handler for core mounting. Same
// explicit-mount deviation as the WordPress surface.
func (m *Module) HugoHandler() *api.HugoHandler {
	return m.hugoHandler
}

// New builds the module, adopting the composition root's wiring. The
// handlers are built once here so both realms serve the same instances —
// and the same in-memory job stores — as the pre-module code.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.wordpressHandler = api.NewWordPressHandler(
		m.wordpressImporter,
		m.logger,
		m.maxImportBytes,
		m.wordpressTimeout,
	)
	m.hugoHandler = api.NewHugoHandler(
		m.hugoImporter,
		m.logger,
		m.maxImportBytes,
		m.hugoTimeout,
	)
	return m
}
