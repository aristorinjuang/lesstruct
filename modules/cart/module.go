// Package cart is the official commerce cart module: guest-first,
// server-side carts with live catalog pricing. Carts are keyed by opaque
// bearer tokens and served on the public router; the module carries no
// admin surface and no config of its own.
package cart

import (
	"io/fs"

	"github.com/aristorinjuang/lesstruct/kernel"
	cartapi "github.com/aristorinjuang/lesstruct/modules/cart/api"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	"github.com/go-chi/chi/v5"
)

const (
	// Name is the registry key for this module.
	Name = "cart"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that
// serves nothing (handlers built over nil dependencies must not be
// invoked).
type Option func(*Module)

// WithService adopts the cart domain service (composition root). The same
// instance backs every endpoint so token resolution and totals run
// identically across the surface.
func WithService(service *cartdomain.Service) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithCurrency sets the display currency for cart payloads, adopted from
// the catalog configuration. Empty renders payloads without a currency.
func WithCurrency(currency string) Option {
	return func(m *Module) {
		m.currency = currency
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

// Module is the cart kernel module.
type Module struct {
	service  *cartdomain.Service
	currency string
	logger   kernel.Logger
	handler  *cartapi.Handler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Carts price from the catalog: the
// module resolves sellable lines through the catalog service it adopts,
// so disabling catalog cascades the cart off the registry.
func (m *Module) Requires() []string {
	return []string{"catalog"}
}

// Optional lists soft dependencies. None.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The handler is already built by New; Init only
// validates presence. Migration execution is core-driven (it needs the
// live *sql.DB, which modules never touch directly).
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases nothing: the handler holds no runtime resources.
func (m *Module) Shutdown() error {
	return nil
}

// RegisterPublicRoutes mounts the cart surface on the public router
// (rate-limited, no auth chain): token minting, cart reads, line
// management, and cart deletion.
func (m *Module) RegisterPublicRoutes(router chi.Router) {
	router.Post("/cart", m.handler.CreateCart)
	router.Get("/cart", m.handler.GetCart)
	router.Delete("/cart", m.handler.DeleteCart)
	router.Post("/cart/items", m.handler.AddItem)
	router.Patch("/cart/items/{itemId}", m.handler.UpdateItem)
	router.Delete("/cart/items/{itemId}", m.handler.RemoveItem)
}

// Migrations returns the embedded migration filesystem, laid out per driver
// subdirectory (the core migration runner selects via Driver.MigrationDir).
func (m *Module) Migrations() fs.FS {
	sub, _ := fs.Sub(migrationsFS, "migrations")
	return sub
}

// New builds the module and constructs the handler once, so every endpoint
// shares the adopted service.
func New(options ...Option) *Module {
	m := &Module{}
	for _, option := range options {
		option(m)
	}
	m.handler = cartapi.New(m.service, m.currency)
	return m
}
