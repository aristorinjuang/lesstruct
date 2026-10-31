// Package auth is the official authentication module: credential login,
// self-service registration, email verification, password reset, and
// first-login bootstrap over the admin handler surface.
package auth

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/auth/admin"
	"github.com/aristorinjuang/lesstruct/modules/auth/api"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
)

// Name is the registry key for this module.
const Name = "auth"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module whose
// handlers serve nil services (they must not be invoked unconfigured).
type Option func(*Module)

// WithAuthService adopts the default-credentials auth service.
func WithAuthService(service *authdomain.AuthService) Option {
	return func(m *Module) {
		m.authService = service
	}
}

// WithFirstLoginService adopts the first-login service.
func WithFirstLoginService(service *authdomain.FirstLoginService) Option {
	return func(m *Module) {
		m.firstLoginService = service
	}
}

// WithRegistrationService adopts the registration service.
func WithRegistrationService(service *authdomain.RegistrationService) Option {
	return func(m *Module) {
		m.registrationService = service
	}
}

// WithVerificationService adopts the verification service.
func WithVerificationService(service *authdomain.VerificationService) Option {
	return func(m *Module) {
		m.verificationService = service
	}
}

// WithLoginService adopts the login service.
func WithLoginService(service *authdomain.LoginService) Option {
	return func(m *Module) {
		m.loginService = service
	}
}

// WithMFAService adopts the login MFA service.
func WithMFAService(service *authdomain.MFAService) Option {
	return func(m *Module) {
		m.mfaService = service
	}
}

// WithPasswordResetService adopts the password-reset service.
func WithPasswordResetService(service *authdomain.PasswordResetService) Option {
	return func(m *Module) {
		m.passwordResetService = service
	}
}

// WithTokenIssuer adopts the JWT issuer for login sessions.
func WithTokenIssuer(issuer api.TokenIssuer) Option {
	return func(m *Module) {
		m.tokenIssuer = issuer
	}
}

// WithUserStore adopts the user store for handler-level lookups.
func WithUserStore(store api.UserStore) Option {
	return func(m *Module) {
		m.userStore = store
	}
}

// WithFailedLoginStore adopts the failed-login store for the lockout-email
// flow. The domain FailedLoginAttemptRepo covers it directly.
func WithFailedLoginStore(store authdomain.FailedLoginAttemptRepo) Option {
	return func(m *Module) {
		m.failedLoginStore = store
	}
}

// WithNotificationRecorder adopts the admin notification recorder.
func WithNotificationRecorder(recorder api.NotificationRecorder) Option {
	return func(m *Module) {
		m.notificationRecorder = recorder
	}
}

// WithMailer adopts the account mailer.
func WithMailer(mailer api.Mailer) Option {
	return func(m *Module) {
		m.mailer = mailer
	}
}

// WithMFADeliverer adopts the login-code deliverer (email in production, the
// dev log sink only under DEV_MODE without SMTP).
func WithMFADeliverer(deliverer api.MFACodeDeliverer) Option {
	return func(m *Module) {
		m.mfaDeliverer = deliverer
	}
}

// WithBlockedEmailChecker adopts the blocked-email checker.
func WithBlockedEmailChecker(checker api.BlockedEmailChecker) Option {
	return func(m *Module) {
		m.blockedEmailChecker = checker
	}
}

// WithCredentialPolicy adopts the credential policy for the first-login
// handler's password and email validation.
func WithCredentialPolicy(policy authdomain.CredentialPolicy) Option {
	return func(m *Module) {
		m.credentialPolicy = policy
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

// WithAdminApprovalRequired configures the admin-approval registration mode
// on the auth handler.
func WithAdminApprovalRequired() Option {
	return func(m *Module) {
		m.adminApprovalRequired = true
	}
}

// Module is the auth kernel module.
type Module struct {
	authService           *authdomain.AuthService
	firstLoginService     *authdomain.FirstLoginService
	registrationService   *authdomain.RegistrationService
	verificationService   *authdomain.VerificationService
	loginService          *authdomain.LoginService
	mfaService            *authdomain.MFAService
	passwordResetService  *authdomain.PasswordResetService
	tokenIssuer           api.TokenIssuer
	userStore             api.UserStore
	failedLoginStore      authdomain.FailedLoginAttemptRepo
	notificationRecorder  api.NotificationRecorder
	mailer                api.Mailer
	mfaDeliverer          api.MFACodeDeliverer
	blockedEmailChecker   api.BlockedEmailChecker
	credentialPolicy      authdomain.CredentialPolicy
	logger                kernel.Logger
	adminApprovalRequired bool
	authHandler           *api.AuthHandler
	firstLoginHandler     *api.FirstLoginHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Authentication resolves users, matching
// the retired wrapper edge.
func (m *Module) Requires() []string {
	return []string{"users"}
}

// Optional lists soft dependencies. There are none: every collaborator is
// either adopted or required.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The handlers are already built by New; Init only
// validates presence.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// AdminManifest declares this module's admin-shell extensions. Auth is
// chrome, not a navigation surface: login and setup screens render outside
// the shell.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// AuthHandler exposes the auth handler for core mounting. Core mounts it
// explicitly (rather than via RouteProvider) because the surface spans the
// public auth group, where a single RegisterRoutes call cannot express the
// per-route anonymous contract.
func (m *Module) AuthHandler() *api.AuthHandler {
	return m.authHandler
}

// FirstLoginHandler exposes the first-login handler for core mounting.
// Same explicit-mount deviation as the auth surface.
func (m *Module) FirstLoginHandler() *api.FirstLoginHandler {
	return m.firstLoginHandler
}

// New builds the module, adopting the composition root's wiring. The
// handlers are built once here so every realm serves the same instances.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	authOpts := []api.AuthHandlerOption{}
	if m.adminApprovalRequired {
		authOpts = append(authOpts, api.WithAdminApprovalRequired())
	}
	m.authHandler = api.NewAuthHandler(
		m.authService,
		m.tokenIssuer,
		m.logger,
		m.firstLoginService,
		m.registrationService,
		m.verificationService,
		m.loginService,
		m.mfaService,
		m.passwordResetService,
		m.userStore,
		m.failedLoginStore,
		m.notificationRecorder,
		m.mailer,
		m.mfaDeliverer,
		m.blockedEmailChecker,
		authOpts...,
	)
	m.firstLoginHandler = api.NewFirstLoginHandler(
		m.firstLoginService,
		m.userStore,
		m.credentialPolicy,
		m.logger,
	)
	return m
}
