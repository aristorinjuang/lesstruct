// Package posttypesbasic is the official post + page content-type module.
//
// It hosts the post-type registry (adopted from the composition root, which
// builds it from config), declares the post and page definitions to the
// kernel discovery index, and serves the post-type HTTP surface. It carries
// no database tables and no migrations: the registry is in-memory.
//
// The /roles and /user_fields endpoints are served here only until the users
// module lands, which takes them over unchanged.
package posttypesbasic

import (
	"github.com/go-chi/chi/v5"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/posttypes-basic/admin"
	"github.com/aristorinjuang/lesstruct/modules/posttypes-basic/api"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
)

const (
	// Name is the registry key for this module.
	Name = "posttypes-basic"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

var (
	// ownedSlugs are the content types this module defines and reserves in the
	// kernel index.
	ownedSlugs = map[string]bool{
		"post": true,
		"page": true,
	}

	// deferredSlugs are registry placeholders claimed by later modules. They
	// stay resolvable through the rich registry for back-compat but are never
	// declared to the kernel index here: the media module declares "media" and
	// the comments module declares "comment" when they land.
	deferredSlugs = map[string]bool{
		"media":   true,
		"comment": true,
	}
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module with the
// default registry.
type Option func(*Module)

// WithService adopts a config-built registry (composition root). Without it
// the module seeds its own default registry.
func WithService(service *posttype.Service) Option {
	return func(m *Module) {
		if service != nil {
			m.service = service
		}
	}
}

// WithCapabilityResolver attaches the authorizer for role-scoped surfaces.
// Without one every authenticated caller sees everything (legacy behavior).
func WithCapabilityResolver(resolve kernel.CapabilityResolver) Option {
	return func(m *Module) {
		m.resolve = resolve
	}
}

// WithCommentsEnabled toggles the "comment" sentinel in list surfaces,
// mirroring the [comments] config block.
func WithCommentsEnabled(enabled bool) Option {
	return func(m *Module) {
		m.commentsEnabled = enabled
	}
}

// WithRoles attaches the role registry for the roles endpoint. Without one
// the endpoint answers 404 roles_unavailable (fail closed).
func WithRoles(roles api.RoleCapabilities) Option {
	return func(m *Module) {
		m.roles = roles
	}
}

// Module is the posttypes-basic kernel module.
type Module struct {
	service         *posttype.Service
	handler         *api.Handler
	resolve         kernel.CapabilityResolver
	roles           api.RoleCapabilities
	commentsEnabled bool
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. This module has none: it serves every
// authenticated caller when no resolver is attached.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. None: role scoping degrades to
// allow-all without a resolver by design.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The handler is already built by New; Init only
// validates the adopted service is present.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// PostTypes declares this module's content types to the kernel discovery
// index: post and page as protected, plus every custom type in the adopted
// registry unprotected. The media/comment placeholders stay out (see
// deferredSlugs).
func (m *Module) PostTypes() []kernel.PostTypeDef {
	all := m.service.GetAll()
	defs := make([]kernel.PostTypeDef, 0, len(all))
	for _, pt := range all {
		if deferredSlugs[pt.Slug] {
			continue
		}
		defs = append(defs, kernel.PostTypeDef{
			Slug:      pt.Slug,
			Name:      pt.Name,
			Protected: ownedSlugs[pt.Slug],
		})
	}
	return defs
}

// RegisterRoutes mounts the authenticated post-type surface. Core passes the
// protected router (auth pre-chained).
func (m *Module) RegisterRoutes(router chi.Router) {
	router.Get("/post_types", m.handler.GetPostTypes)
	router.Get("/roles", m.handler.GetRoles)
	router.Get("/user_fields", m.handler.GetUserFieldsEndpoint)
}

// RegisterPublicRoutes mounts the anonymous post-type surface. Core passes
// the public router (no auth chain).
func (m *Module) RegisterPublicRoutes(router chi.Router) {
	router.Get("/post_types", m.handler.GetPublicPostTypes)
}

// AdminManifest declares this module's admin-shell extensions. Post types
// surface in the admin UI data-driven (content tabs and the editor dropdown
// read the post-types API), so the module declares no menu entries, settings
// sections, or editor extensions.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// Service exposes the adopted registry for composition-root wiring (content
// adapter, importers, templates, role loading). It stays valid for the
// module's lifetime.
func (m *Module) Service() *posttype.Service {
	return m.service
}

// New builds the module, adopting the composition root's wiring. The handler
// is built once here so routes serve the same instance the provider reads.
func New(opts ...Option) *Module {
	m := &Module{
		service:         posttype.NewService(),
		commentsEnabled: true,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.handler = api.New(
		m.service,
		m.commentsEnabled,
		m.resolve,
		m.roles,
	)
	return m
}
