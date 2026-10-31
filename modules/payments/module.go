// Package payments is the official commerce payment module: payment
// methods adopted from store configuration, manual transfer proof
// submission for guests, and the admin payment decisions that flip orders
// to paid or keep them waiting. The module never imports the orders
// packages — it resolves and confirms orders through the Orders seam with
// the composition root providing the adapter.
package payments

import (
	"errors"
	"io/fs"

	"github.com/aristorinjuang/lesstruct/kernel"
	paymentsapi "github.com/aristorinjuang/lesstruct/modules/payments/api"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
	"github.com/go-chi/chi/v5"
)

const (
	// Name is the registry key for this module.
	Name = "payments"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that
// serves nothing (handlers built over nil dependencies must not be
// invoked).
type Option func(*Module)

// WithService adopts the payments domain service (composition root). The
// same instance backs every endpoint.
func WithService(service *paymentsdomain.Service) Option {
	return func(m *Module) {
		m.service = service
	}
}

// WithMethods adopts the offered payment methods from store
// configuration, in the configured order.
func WithMethods(methods []paymentsdomain.Method) Option {
	return func(m *Module) {
		m.methods = methods
	}
}

// WithCapabilityResolver attaches the capability resolver gating the admin
// surface. Without one every admin request is refused (fail closed).
func WithCapabilityResolver(resolve kernel.CapabilityResolver) Option {
	return func(m *Module) {
		m.resolve = resolve
	}
}

// WithGateway adopts the hosted-checkout driver. Without one the mint
// surface answers 503 and webhooks refuse validation.
func WithGateway(gateway paymentsdomain.Gateway) Option {
	return func(m *Module) {
		m.gateway = gateway
	}
}

// Module is the payments kernel module.
type Module struct {
	service *paymentsdomain.Service
	gateway paymentsdomain.Gateway
	methods []paymentsdomain.Method
	resolve kernel.CapabilityResolver
	handler *paymentsapi.Handler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Payment proofs resolve and confirm
// orders through the Orders seam, and orders own the checkout flow: with
// orders disabled there is nothing to pay for, so the module drops from
// the registry (orders → payments).
func (m *Module) Requires() []string {
	return []string{"orders"}
}

// Optional lists soft dependencies. The mailer is structural best-effort:
// email failures never fail a submission or a decision.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The handler is already built by New; Init only
// validates that the adopted service is present — a payments module
// without its service fails startup loudly instead of serving an empty
// surface. Migration execution is core-driven (it needs the live
// *sql.DB, which modules never touch directly).
func (m *Module) Init(_ kernel.Kernel) error {
	if m.service == nil {
		return errors.New("payments: the payment service is required")
	}
	return nil
}

// Shutdown releases nothing: the handler holds no runtime resources.
func (m *Module) Shutdown() error { return nil }

// RegisterPublicRoutes mounts the public surface (rate-limited, no auth
// chain): the offered-method enumeration, the guest proof submission,
// the hosted-session mint, and the gateway webhook (signature-gated).
func (m *Module) RegisterPublicRoutes(router chi.Router) {
	router.Get("/payments/methods", m.handler.ListMethods)
	router.Post("/orders/{orderNumber}/payments", m.handler.SubmitPayment)
	router.Post("/orders/{orderNumber}/payments/session", m.handler.MintPayment)
	router.Post("/payments/midtrans/webhook", m.handler.MidtransWebhook)
}

// RegisterDualRoutes declares the admin surface for dual-realm mounting:
// core serves every path on both the agent stack (Bearer API key) and the
// browser stack (CSRF plus JWT), dispatched by credential — so AI agents
// decide payment proofs with the same API key they use for content while
// the admin SPA keeps working unchanged. Same handlers, same in-handler
// capability checks on both realms.
func (m *Module) RegisterDualRoutes(r kernel.DualAuthRegistrar) {
	r.Get("/api/v1/payments", m.handler.ListPayments)
	r.Get("/api/v1/payments/{paymentID}", m.handler.GetPayment)
	r.Post("/api/v1/payments/{paymentID}/verify", m.handler.VerifyPayment)
	r.Post("/api/v1/payments/{paymentID}/reject", m.handler.RejectPayment)
}

// Capabilities contributes the module's permission to the registry so the
// resolver can grant it (Admin implicitly, custom roles via config when
// declared).
func (m *Module) Capabilities() []string {
	return []string{string(paymentsdomain.CapabilityManage)}
}

// Migrations returns the embedded migration filesystem, laid out per driver
// subdirectory (the core migration runner selects via Driver.MigrationDir).
func (m *Module) Migrations() fs.FS {
	sub, _ := fs.Sub(migrationsFS, "migrations")
	return sub
}

// New builds the module and constructs the handler once, so every endpoint
// shares the adopted service and gateway driver.
func New(options ...Option) *Module {
	m := &Module{}
	for _, option := range options {
		option(m)
	}
	m.handler = paymentsapi.New(m.service, m.gateway, m.resolve)
	return m
}
