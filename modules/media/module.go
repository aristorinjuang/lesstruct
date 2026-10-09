// Package media is the official media-library module: uploads, the media
// library, AI image generation, and thumbnail variants.
package media

import (
	"io/fs"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/media/admin"
	"github.com/aristorinjuang/lesstruct/modules/media/api"
	"github.com/aristorinjuang/lesstruct/modules/media/api/agent"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
)

const (
	// Name is the registry key for this module.
	Name = "media"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that serves
// nothing (handlers built over nil dependencies must not be invoked).
type Option func(*Module)

// WithService adopts the media domain service (composition root). The same
// instance backs the browser and agent handlers so hashing, dedup, WebP
// conversion, and thumbnail variants run identically in both realms.
func WithService(service *mediadomain.Service) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithImageGeneration adopts the AI image provider (composition root, nil
// when unconfigured). Without one the generate endpoint answers
// 503 not_configured (fail closed).
func WithImageGeneration(service mediadomain.ImageGenerationService) Option {
	return func(m *Module) {
		m.imageGen = service
	}
}

// WithRoles attaches the role registry for the browser upload/generate/delete
// gates. Without one every authenticated caller may manage media (the legacy
// behavior).
func WithRoles(roles api.RoleCapabilities) Option {
	return func(m *Module) {
		m.roles = roles
	}
}

// WithAgentRoles attaches the role registry for the agent upload gate. It is
// a separate option (rather than sharing WithRoles) because the two api
// packages own their seams independently; core satisfies both with one
// adapter.
func WithAgentRoles(roles agent.RoleCapabilities) Option {
	return func(m *Module) {
		m.agentRoles = roles
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

// Module is the media kernel module.
type Module struct {
	service    *mediadomain.Service
	imageGen   mediadomain.ImageGenerationService
	roles      api.RoleCapabilities
	agentRoles agent.RoleCapabilities
	logger     kernel.Logger
	browser    *api.MediaHandler
	agent      *agent.MediaHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Media references users loosely (owner
// IDs without foreign-key enforcement), so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. None.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The handlers are already built by New; Init only
// validates presence. Migration execution is core-driven (it needs the
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

// AdminManifest declares this module's admin-shell extensions: the media
// library navigation entry.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// BrowserHandler exposes the JWT/CSRF-realm handler for core mounting. Core
// mounts it explicitly (rather than via RouteProvider) because the media
// surface spans three body-limit groups plus the dual-auth dispatch — a
// single RegisterRoutes call cannot express per-route body limits.
func (m *Module) BrowserHandler() *api.MediaHandler {
	return m.browser
}

// AgentHandler exposes the Bearer-realm handler for core mounting, alongside
// the browser handler for the same per-route-limit reason.
func (m *Module) AgentHandler() *agent.MediaHandler {
	return m.agent
}

// Service exposes the adopted domain service for composition-root wiring
// (importers, text generation, templates). It stays valid for the module's
// lifetime.
func (m *Module) Service() *mediadomain.Service {
	return m.service
}

// New builds the module, adopting the composition root's wiring. Both realm
// handlers are built once here so routes serve the same instances.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.browser = api.NewMediaHandler(
		m.service,
		m.imageGen,
		m.logger,
		api.WithMediaRoleService(m.roles),
	)
	m.agent = agent.NewMediaHandler(
		m.service,
		m.logger,
		agent.WithMediaRoleService(m.agentRoles),
	)
	return m
}
