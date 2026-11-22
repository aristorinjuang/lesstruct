// Package modgen is the official AI module module: the generator over
// OpenAI-compatible APIs, the filesystem store, and the browser handler.
package modgen

import (
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/modgen/admin"
	"github.com/aristorinjuang/lesstruct/modules/modgen/api"
	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
)

// Name is the registry key for this module.
const Name = "modgen"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that serves
// nothing (the handler built over nil dependencies must not be invoked).
type Option func(*Module)

// WithService adopts the orchestration service (composition root, nil when
// unconfigured). The handler is only mounted when generation is enabled.
func WithService(service *modgendomain.Service) Option {
	return func(m *Module) {
		m.service = service
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

// WithApplyTimeout sets the server-side apply request budget. Non-positive
// selects the handler default.
func WithApplyTimeout(timeout time.Duration) Option {
	return func(m *Module) {
		m.applyTimeout = timeout
	}
}

// Module is the modgen kernel module.
type Module struct {
	service      *modgendomain.Service
	logger       kernel.Logger
	applyTimeout time.Duration
	handler      *api.ModuleGenHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Generation degrades without module
// sources, so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies.
func (m *Module) Optional() []string {
	return nil
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

// Handler exposes the module generator handler for core mounting. Core mounts
// it explicitly (rather than via RouteProvider) because the surface is gated
// by the modGenEnabled feature flag, which a single RegisterRoutes call
// cannot express.
func (m *Module) Handler() *api.ModuleGenHandler {
	return m.handler
}

// Service exposes the orchestration service. It stays valid for the
// module's lifetime.
func (m *Module) Service() *modgendomain.Service {
	return m.service
}

// New builds the module, adopting the composition root's wiring. The handler
// is built once here so routes serve the same instance.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	var svc api.ModuleService
	if m.service != nil {
		svc = m.service
	}
	m.handler = api.NewModuleGenHandler(svc, m.logger, m.applyTimeout)
	return m
}
