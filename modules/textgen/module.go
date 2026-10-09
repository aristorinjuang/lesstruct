// Package textgen is the official AI text module: the Enhancer and the
// Translator over OpenAI-compatible APIs.
package textgen

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/textgen/admin"
	"github.com/aristorinjuang/lesstruct/modules/textgen/api"
	textgendomain "github.com/aristorinjuang/lesstruct/modules/textgen/domain"
)

// Name is the registry key for this module.
const Name = "textgen"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that serves
// nothing (the handler built over nil dependencies must not be invoked).
type Option func(*Module)

// WithService adopts the text generation service (composition root, nil when
// unconfigured). The handler is only mounted when a service exists.
func WithService(service textgendomain.TextGenerationService) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithMediaLister attaches the media library for AI context injection. The
// media domain service satisfies it structurally; without one the handler
// generates without media context (the legacy behavior when media is absent).
func WithMediaLister(lister api.MediaLister) Option {
	return func(m *Module) {
		m.media = lister
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

// Module is the textgen kernel module.
type Module struct {
	service textgendomain.TextGenerationService
	media   api.MediaLister
	logger  kernel.Logger
	handler *api.TextGenHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Text generation degrades gracefully
// without media context or theme design tokens, so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. Media context enriches HTML prompts and
// the site-ssr theme bakes design tokens into them; absent either,
// enhance/translate still work.
func (m *Module) Optional() []string {
	return []string{"media", "site-ssr"}
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

// AdminManifest declares this module's admin-shell extensions. The TipTap
// toolbar buttons render data-driven from the /api/health feature flags, so
// the module declares no menu entries, settings sections, or editor
// extensions.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// Handler exposes the text generation handler for core mounting. Core mounts
// it explicitly (rather than via RouteProvider) because the surface is gated
// by the textGenEnabled feature flag, which a single RegisterRoutes call
// cannot express.
func (m *Module) Handler() *api.TextGenHandler {
	return m.handler
}

// Service exposes the adopted text generation service. It stays valid for
// the module's lifetime.
func (m *Module) Service() textgendomain.TextGenerationService {
	return m.service
}

// New builds the module, adopting the composition root's wiring. The handler
// is built once here so routes serve the same instance.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.handler = api.NewTextGenHandler(m.service, m.media, m.logger)
	return m
}
