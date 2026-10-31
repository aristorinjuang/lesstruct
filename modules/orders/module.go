// Package orders is the official commerce order module: first-class orders
// snapshotted at checkout, a domain-enforced status lifecycle, guest order
// tracking, an admin order surface, and a pending-expiry sweeper that
// ages unpaid orders off the books. Checkout consumes the cart through
// the CartSource seam the composition root adapts; the module's only
// knob is the sweeper cadence the composition root adopts.
package orders

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	ordersapi "github.com/aristorinjuang/lesstruct/modules/orders/api"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	"github.com/go-chi/chi/v5"
)

const (
	// Name is the registry key for this module.
	Name = "orders"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"

	// sweepBatch bounds one sweeper pass: expiration flips carry
	// customer emails, so a huge backlog drains over ticks instead of
	// one long send.
	sweepBatch = 100
)

// discardLogger is the no-op sink New installs when no logger was
// adopted, mirroring the kernel host convention that a nil logger
// degrades to discard — a background sweeper must never panic on its
// own log line.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that
// serves nothing (handlers built over nil dependencies must not be
// invoked).
type Option func(*Module)

// WithService adopts the order domain service (composition root). The same
// instance backs every endpoint so checkout, tracking, and the admin
// surface speak one snapshot vocabulary.
func WithService(service *ordersdomain.Service) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithCapabilityResolver attaches the capability resolver gating the admin
// surface. Without one every admin request is refused (fail closed).
func WithCapabilityResolver(resolve kernel.CapabilityResolver) Option {
	return func(m *Module) {
		m.resolve = resolve
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

// WithSweeper runs the pending-expiry sweep in the module background:
// every interval the module expires pendings older than the TTL. A
// non-positive interval leaves the sweeper off (tests, standalone
// modules). The TTL mirrors into gateway session lifetimes so a hosted
// checkout cannot outlive the order it pays.
func WithSweeper(interval time.Duration, ttl time.Duration) Option {
	return func(m *Module) {
		m.sweepInterval = interval
		m.sweepTTL = ttl
	}
}

// WithInvoiceSettings adopts the seller identity and PPN rate stamped on
// every invoice.
func WithInvoiceSettings(settings ordersdomain.InvoiceSettings) Option {
	return func(m *Module) {
		m.invoiceSettings = settings
	}
}

// WithInvoiceSettingsSaver adopts the persistence seam behind the admin
// invoice-settings surface: the composition root supplies the config.toml
// write-back, so an admin edit survives a restart. Without one, edits stay
// in memory for the running process.
func WithInvoiceSettingsSaver(saver func(ordersdomain.InvoiceSettings) error) Option {
	return func(m *Module) {
		m.invoiceSaver = saver
	}
}

// WithInvoiceRenderer adopts the PDF renderer backing the invoice
// endpoint. Without one the endpoint answers unavailable.
func WithInvoiceRenderer(renderer ordersdomain.InvoiceRenderer) Option {
	return func(m *Module) {
		m.invoiceRenderer = renderer
	}
}

// WithManualPayee adopts the manual payee block for guest tracking
// payloads. Without one tracking payloads carry no payee section.
func WithManualPayee(payee ordersapi.ManualPayee) Option {
	return func(m *Module) {
		m.manualPayee = &payee
	}
}

// Module is the orders kernel module.
type Module struct {
	service         *ordersdomain.Service
	resolve         kernel.CapabilityResolver
	logger          kernel.Logger
	handler         *ordersapi.Handler
	sweepInterval   time.Duration
	sweepTTL        time.Duration
	sweepWG         sync.WaitGroup
	sweepCancel     context.CancelFunc
	invoiceMu       sync.RWMutex
	invoiceSettings ordersdomain.InvoiceSettings
	invoiceSaver    func(ordersdomain.InvoiceSettings) error
	invoiceRenderer ordersdomain.InvoiceRenderer
	manualPayee     *ordersapi.ManualPayee
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Checkout consumes the cart through the
// CartSource seam, and carts price from the catalog: without either there
// is nothing to check out, so disabling catalog cascades orders off the
// registry (catalog → cart → orders). The module covers both the checkout
// surface and the admin lifecycle views, so it is coherent to disable it
// as one unit.
func (m *Module) Requires() []string {
	return []string{"cart"}
}

// Optional lists soft dependencies. The mailer is structural best-effort:
// email failures never fail checkout or transitions.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The handler is already built by New; Init only
// validates that the adopted service and resolver are present — an orders
// module without every seam fails startup loudly instead of serving an
// empty surface. A configured sweeper starts its background loop here.
// Migration execution is core-driven (it needs the live *sql.DB, which
// modules never touch directly).
func (m *Module) Init(_ kernel.Kernel) error {
	if m.service == nil {
		return errors.New("orders: the order service is required")
	}
	if m.sweepInterval > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		m.sweepCancel = cancel
		m.sweepWG.Go(func() {
			m.sweepLoop(ctx)
		})
	}
	return nil
}

// sweepLoop expires stale pendings on the tick until shutdown. Sweep
// failures log and wait for the next tick: the store's conditional flips
// make every sweep safe to repeat.
func (m *Module) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(m.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids, err := m.service.ExpireStale(ctx, time.Now().UTC().Add(-m.sweepTTL), sweepBatch)
			switch {
			case err != nil:
				m.logger.Error("orders sweeper: %v", err)
			case len(ids) > 0:
				m.logger.Info("orders sweeper: expired %d stale pendings", len(ids))
			}
		}
	}
}

// Shutdown stops the sweeper and waits for its loop to return, so a
// graceful shutdown never leaks a tick mid-write.
func (m *Module) Shutdown() error {
	if m.sweepCancel != nil {
		m.sweepCancel()
		m.sweepWG.Wait()
		m.sweepCancel = nil
	}
	return nil
}

// RegisterPublicRoutes mounts the public surface (rate-limited, no auth
// chain): checkout, and guest order tracking.
func (m *Module) RegisterPublicRoutes(router chi.Router) {
	router.Post("/checkout", m.handler.Checkout)
	router.Get("/orders/{orderNumber}", m.handler.TrackOrder)
}

// RegisterDualRoutes declares the admin surface for dual-realm mounting:
// core serves every path on both the agent stack (Bearer API key) and the
// browser stack (CSRF plus JWT), dispatched by credential — so AI agents
// operate orders with the same API key they use for content while the
// admin SPA keeps working unchanged. Same handlers, same in-handler Admin
// checks on both realms.
func (m *Module) RegisterDualRoutes(r kernel.DualAuthRegistrar) {
	r.Get("/api/v1/orders", m.handler.ListOrders)
	r.Get("/api/v1/orders/invoice-settings", m.handler.GetInvoiceSettings)
	r.Put("/api/v1/orders/invoice-settings", m.handler.UpdateInvoiceSettings)
	r.Get("/api/v1/orders/{orderID}", m.handler.GetOrder)
	r.Post("/api/v1/orders/{orderID}/status", m.handler.TransitionOrder)
	r.Post("/api/v1/orders/{orderID}/shipping", m.handler.SetShippingOrder)
	r.Get("/api/v1/orders/{orderID}/invoice", m.handler.InvoiceOrder)
}

// Capabilities contributes the module's permission to the registry so the
// resolver can grant it (Admin implicitly, custom roles via config when
// declared).
func (m *Module) Capabilities() []string {
	return []string{string(ordersdomain.CapabilityManage)}
}

// Migrations returns the embedded migration filesystem, laid out per driver
// subdirectory (the core migration runner selects via Driver.MigrationDir).
func (m *Module) Migrations() fs.FS {
	sub, _ := fs.Sub(migrationsFS, "migrations")
	return sub
}

// InvoiceSettings returns the live invoice settings the next rendered
// invoice will use.
func (m *Module) InvoiceSettings() ordersdomain.InvoiceSettings {
	m.invoiceMu.RLock()
	defer m.invoiceMu.RUnlock()
	return m.invoiceSettings
}

// SetInvoiceSettings validates nothing itself (the API surface does) but
// persists the block through the adopted saver first: a failed write leaves
// the running settings untouched, so the admin sees the error instead of a
// value that would vanish on restart. The write lock spans the saver and
// the assignment so parallel admin saves cannot diverge the file from the
// live settings; invoice renders take the read lock and block briefly.
func (m *Module) SetInvoiceSettings(settings ordersdomain.InvoiceSettings) error {
	m.invoiceMu.Lock()
	defer m.invoiceMu.Unlock()
	if m.invoiceSaver != nil {
		if err := m.invoiceSaver(settings); err != nil {
			return err
		}
	}
	m.invoiceSettings = settings
	return nil
}

// New builds the module and constructs the handler once, so every endpoint
// shares the adopted service. A module built without a logger degrades to
// a discard sink: the background sweeper logs unconditionally and must
// never panic for lack of one.
func New(options ...Option) *Module {
	m := &Module{}
	for _, option := range options {
		option(m)
	}
	if m.logger == nil {
		m.logger = discardLogger{}
	}
	var support *ordersapi.InvoiceSupport
	if m.invoiceRenderer != nil {
		support = &ordersapi.InvoiceSupport{Settings: m.invoiceSettings, Renderer: m.invoiceRenderer, Store: m}
	}
	m.handler = ordersapi.New(m.service, m.resolve, support)
	if m.manualPayee != nil {
		m.handler.SetManualPayee(m.manualPayee)
	}
	return m
}
