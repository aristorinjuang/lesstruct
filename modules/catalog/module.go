// Package catalog is the official commerce catalog module: native pricing,
// variant matrices, stock, and SKUs keyed to a configured product post type.
// Products are content items; this module owns the commerce structure and
// its admin/public surfaces.
package catalog

import (
	"fmt"
	"io/fs"

	"github.com/aristorinjuang/lesstruct/kernel"
	catalogapi "github.com/aristorinjuang/lesstruct/modules/catalog/api"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	"github.com/go-chi/chi/v5"
)

const (
	// Name is the registry key for this module.
	Name = "catalog"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that fails
// its Init (no config) and serves nothing.
type Option func(*Module)

// WithConfig sets the operator knobs: the product post-type slug and the
// display currency for public payloads.
func WithConfig(config catalogapi.Config) Option {
	return func(m *Module) {
		m.config = config
	}
}

// WithService adopts the catalog domain service (composition root).
func WithService(service *catalogdomain.Service) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithPostTypes attaches the post-type registry check. Without one Init
// fails: the module refuses to start when it cannot verify its product type.
func WithPostTypes(checker catalogapi.PostTypeChecker) Option {
	return func(m *Module) {
		m.postTypes = checker
	}
}

// WithSlugResolver attaches the published-product slug lookup for the
// public surface. Without one the public endpoint resolves nothing.
func WithSlugResolver(resolver catalogapi.ProductSlugResolver) Option {
	return func(m *Module) {
		m.slug = resolver
	}
}

// WithCapabilityResolver attaches the capability resolver gating admin
// mutations. Without one every mutation is denied (fail closed).
func WithCapabilityResolver(resolve kernel.CapabilityResolver) Option {
	return func(m *Module) {
		m.resolve = resolve
	}
}

// WithLogger attaches the module logger. Without one failures are dropped
// (discard sink).
func WithLogger(logger kernel.Logger) Option {
	return func(m *Module) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// Module is the catalog kernel module.
type Module struct {
	config    catalogapi.Config
	service   *catalogdomain.Service
	postTypes catalogapi.PostTypeChecker
	slug      catalogapi.ProductSlugResolver
	resolve   kernel.CapabilityResolver
	logger    kernel.Logger
	handler   *catalogapi.Handler
}

// Name returns the registry key.
func (m *Module) Name() string { return Name }

// Version returns the module version.
func (m *Module) Version() string { return Version }

// Requires lists hard dependencies. Products are content items: the content
// engine is core, and the post-type registry is adopted through the checker
// seam, so the module depends on nothing in the module graph.
func (m *Module) Requires() []string { return nil }

// Optional lists soft dependencies. None.
func (m *Module) Optional() []string { return nil }

// Init validates the adopted configuration: the configured product type
// must exist in the registry and the currency must be set — a misconfigured
// commerce edition fails startup loudly instead of serving an empty catalog.
func (m *Module) Init(_ kernel.Kernel) error {
	if m.config.PostType == "" {
		return fmt.Errorf("catalog: [catalog] post_type is required")
	}
	if m.postTypes != nil && !m.postTypes.Exists(m.config.PostType) {
		return fmt.Errorf("catalog: post type %q is not registered; declare it in config.toml", m.config.PostType)
	}
	if m.config.Currency == "" {
		return fmt.Errorf("catalog: [catalog] currency is required")
	}
	return nil
}

// Shutdown releases nothing: the module holds no runtime resources.
func (m *Module) Shutdown() error { return nil }

// Migrations returns the embedded migration filesystem, laid out per driver
// subdirectory (the core migration runner selects via Driver.MigrationDir).
func (m *Module) Migrations() fs.FS {
	sub, _ := fs.Sub(migrationsFS, "migrations")
	return sub
}

// RegisterDualRoutes declares the admin surface for dual-realm mounting:
// core serves every path on both the agent stack (Bearer API key) and the
// browser stack (CSRF plus JWT), dispatched by credential — so AI agents
// maintain catalog records with the same API key they use for content
// while the admin SPA keeps working unchanged. Same handlers, same
// in-handler capability checks on both realms.
func (m *Module) RegisterDualRoutes(r kernel.DualAuthRegistrar) {
	r.Get("/api/v1/catalog/products/{contentID}", m.handler.GetProduct)
	r.Put("/api/v1/catalog/products/{contentID}", m.handler.UpsertProduct)
}

// RegisterPublicRoutes mounts the public product payload on the anonymous
// group (public rate limit applied by core): commerce data by slug for
// themes and headless frontends.
func (m *Module) RegisterPublicRoutes(router chi.Router) {
	router.Get("/catalog/products/{slug}", m.handler.GetPublicProduct)
}

// Capabilities contributes the module's permission to the registry so the
// resolver can grant it (Admin implicitly, custom roles via config when
// declared).
func (m *Module) Capabilities() []string {
	return []string{string(catalogdomain.CapabilityManage)}
}

// New builds the module and constructs both handlers once, so the admin and
// public surfaces share the adopted service.
func New(options ...Option) *Module {
	m := &Module{}
	for _, option := range options {
		option(m)
	}
	m.handler = catalogapi.New(m.service, m.config, m.postTypes, m.slug, m.resolve)
	return m
}
