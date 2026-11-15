// Package themegen is the official AI theme module: the generator over
// OpenAI-compatible APIs, the filesystem store, and the browser handler.
package themegen

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/themegen/admin"
	"github.com/aristorinjuang/lesstruct/modules/themegen/api"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

// Name is the registry key for this module.
const Name = "themegen"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that serves
// nothing (the handler built over nil dependencies must not be invoked).
type Option func(*Module)

// WithService adopts the orchestration service (composition root, nil when
// unconfigured). The handler is only mounted when generation is enabled.
func WithService(service *themegendomain.Service) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithActivations adopts the active-theme store (the [theme] active value).
// Without one activation and deactivation stay unavailable.
func WithActivations(store api.ThemeActivationStore) Option {
	return func(m *Module) {
		m.activations = store
	}
}

// WithEdition sets the default edition the generator derives from (blog or
// storefront). Empty defaults to blog.
func WithEdition(edition themegendomain.Edition) Option {
	return func(m *Module) {
		m.edition = edition
	}
}

// WithReloader attaches the edition's reload closure so activation
// hot-applies the theme without a restart. Without one activation keeps the
// restart-hint behavior.
func WithReloader(reloader api.ThemeReloader) Option {
	return func(m *Module) {
		m.reloader = reloader
	}
}

// WithPreviewKey installs the cookie-signing key for the preview endpoints
// (derived from the server secret by the composition root; Secure mirrors
// the site origin). Without one Preview answers 503.
func WithPreviewKey(key []byte, secure bool) Option {
	return func(m *Module) {
		m.previewKey = key
		m.previewSecure = secure
	}
}

// WithSnapshotRenderer attaches the edition's snapshot closure so the admin
// panel can frame an installed theme's rendered homepage. Without one the
// snapshot endpoint answers 503.
func WithSnapshotRenderer(renderer api.ThemeSnapshotter) Option {
	return func(m *Module) {
		m.snapshotter = renderer
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

// Module is the themegen kernel module.
type Module struct {
	service       *themegendomain.Service
	activations   api.ThemeActivationStore
	reloader      api.ThemeReloader
	previewKey    []byte
	previewSecure bool
	snapshotter   api.ThemeSnapshotter
	edition       themegendomain.Edition
	logger        kernel.Logger
	handler       *api.ThemGenHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Generation degrades without theme
// sources, so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. Theme sources enrich prompts; absent,
// generation uses the distilled contract alone.
func (m *Module) Optional() []string {
	return []string{"site-ssr"}
}

// Init wires the module. The handler is already built by New; Init only
// validates presence.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// AdminManifest declares this module's admin-shell extensions.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// Handler exposes the theme generator handler for core mounting. Core mounts
// it explicitly (rather than via RouteProvider) because the surface is gated
// by the themGenEnabled feature flag, which a single RegisterRoutes call
// cannot express.
func (m *Module) Handler() *api.ThemGenHandler {
	return m.handler
}

// Service exposes the orchestration service. It stays valid for the
// module's lifetime.
func (m *Module) Service() *themegendomain.Service {
	return m.service
}

// New builds the module, adopting the composition root's wiring. The handler
// is built once here so routes serve the same instance.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	if m.edition == "" {
		m.edition = themegendomain.EditionBlog
	}
	var svc api.ThemeService
	if m.service != nil {
		svc = m.service
	}
	m.handler = api.NewThemGenHandler(svc, m.activations, m.logger, m.edition, m.reloader)
	if m.previewKey != nil {
		m.handler.SetPreviewKey(m.previewKey, m.previewSecure)
	}
	if m.snapshotter != nil {
		m.handler.SetSnapshotRenderer(m.snapshotter)
	}
	return m
}
