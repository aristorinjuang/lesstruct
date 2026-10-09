// Package comments is the official comment-system module: per-content
// discussion with moderation (pending/approved/rejected/spam).
package comments

import (
	"io/fs"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/comments/admin"
	"github.com/aristorinjuang/lesstruct/modules/comments/api"
	"github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
)

const (
	// Name is the registry key for this module.
	Name = "comments"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that serves
// nothing (handlers built over nil dependencies must not be invoked).
type Option func(*Module)

// WithService adopts the comments domain service (composition root). The
// same instance backs the browser and agent handlers so validation, the
// AllowComments gate, and moderation transitions run identically in both
// realms.
func WithService(service *commentdomain.Service) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithContentResolver attaches the published-content lookup for the browser
// handlers. Without one content-scoped endpoints fail at lookup time.
func WithContentResolver(resolve api.ContentResolver) Option {
	return func(m *Module) {
		m.content = resolve
	}
}

// WithAgentContentLookup attaches the content lookup for the agent handlers.
// Without one content-scoped endpoints fail at lookup time.
func WithAgentContentLookup(lookup agent.ContentLookup) Option {
	return func(m *Module) {
		m.agentContent = lookup
	}
}

// WithRoles attaches the role registry for the browser comment gate. Without
// one every authenticated caller may post comments (the legacy behavior).
func WithRoles(roles api.RoleCapabilities) Option {
	return func(m *Module) {
		m.roles = roles
	}
}

// WithAgentRoles attaches the role registry for the agent comment gate. It
// is a separate option (rather than sharing WithRoles) because the two api
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

// Module is the comments kernel module.
type Module struct {
	service      *commentdomain.Service
	content      api.ContentResolver
	agentContent agent.ContentLookup
	roles        api.RoleCapabilities
	agentRoles   agent.RoleCapabilities
	logger       kernel.Logger
	browser      *api.CommentHandler
	agent        *agent.CommentHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Comments reference content items and
// users loosely (owner/content IDs without foreign-key enforcement), but the
// wrapper graph this module replaces declared users — keep the edge so the
// composition resolves identically.
func (m *Module) Requires() []string {
	return []string{"users"}
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

// AdminManifest declares this module's admin-shell extensions: the comments
// moderation navigation entry.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// BrowserHandler exposes the JWT/CSRF-realm handler for core mounting. Core
// mounts it explicitly (rather than via RouteProvider) because the comment
// surface spans four middleware groups plus the agent realm, which a single
// RegisterRoutes call cannot express.
func (m *Module) BrowserHandler() *api.CommentHandler {
	return m.browser
}

// AgentHandler exposes the Bearer-realm handler for core mounting, alongside
// the browser handler for the same per-group reason.
func (m *Module) AgentHandler() *agent.CommentHandler {
	return m.agent
}

// Service exposes the adopted domain service for composition-root wiring.
// It stays valid for the module's lifetime.
func (m *Module) Service() *commentdomain.Service {
	return m.service
}

// New builds the module, adopting the composition root's wiring. Both realm
// handlers are built once here so routes serve the same instances.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.browser = api.NewCommentHandler(
		m.service,
		m.content,
		api.WithCommentRoleService(m.roles),
	)
	m.agent = agent.NewCommentHandler(
		m.service,
		m.agentContent,
		m.logger,
		agent.WithCommentRoleService(m.agentRoles),
	)
	return m
}
