// Package users is the official user management module: admin user
// management (approval, suspension, soft delete, admin creation),
// self-service profile (email, password, name, custom fields, data export,
// account deletion), and profile pictures over the handler surface core
// mounts explicitly.
package users

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	"github.com/aristorinjuang/lesstruct/modules/users/admin"
	"github.com/aristorinjuang/lesstruct/modules/users/api"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
)

// Name is the registry key for this module.
const Name = "users"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module whose
// handlers serve nil services (they must not be invoked unconfigured).
type Option func(*Module)

// WithUserManagementService adopts the admin user management service.
func WithUserManagementService(service *userdomain.UserManagementService) Option {
	return func(m *Module) {
		m.userManagementService = service
	}
}

// WithAdminCreateUserService adopts the admin user creation service.
func WithAdminCreateUserService(service *userdomain.AdminCreateUserService) Option {
	return func(m *Module) {
		m.adminCreateService = service
	}
}

// WithProfileService adopts the self-service profile service.
func WithProfileService(service *userdomain.ProfileService) Option {
	return func(m *Module) {
		m.profileService = service
	}
}

// WithAccountDeletionService adopts the account deletion service.
func WithAccountDeletionService(service *userdomain.AccountDeletionService) Option {
	return func(m *Module) {
		m.accountDeletionService = service
	}
}

// WithProfilePictureService adopts the profile picture service.
func WithProfilePictureService(service *profilepicture.Service) Option {
	return func(m *Module) {
		m.profilePictureService = service
	}
}

// WithUserStore adopts the user store for handler-level lookups.
func WithUserStore(store api.UserStore) Option {
	return func(m *Module) {
		m.userStore = store
	}
}

// WithSoftDeleteStore adopts the soft-delete store.
func WithSoftDeleteStore(store api.SoftDeleteStore) Option {
	return func(m *Module) {
		m.softDeleteStore = store
	}
}

// WithMailer adopts the account-email mailer.
func WithMailer(mailer api.Mailer) Option {
	return func(m *Module) {
		m.mailer = mailer
	}
}

// WithUserFieldsProvider adopts the user field schema provider for
// self-service profile editing.
func WithUserFieldsProvider(provider UserFieldsProvider) Option {
	return func(m *Module) {
		m.userFieldsProvider = provider
	}
}

// WithProfilePictureURLBuilder adopts the filename-to-URL builder.
func WithProfilePictureURLBuilder(builder func(string) string) Option {
	return func(m *Module) {
		m.profilePictureURLBuilder = builder
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

// UserFieldsProvider provides user field schema access for self-service
// profile editing. The core post-type service satisfies it structurally.
type UserFieldsProvider interface {
	GetUserFields() []customfield.FieldSchema
	GetUserSystemFields() []customfield.FieldSchema
}

// Module is the users kernel module.
type Module struct {
	userManagementService    *userdomain.UserManagementService
	adminCreateService       *userdomain.AdminCreateUserService
	profileService           *userdomain.ProfileService
	accountDeletionService   *userdomain.AccountDeletionService
	profilePictureService    *profilepicture.Service
	userStore                api.UserStore
	softDeleteStore          api.SoftDeleteStore
	mailer                   api.Mailer
	userFieldsProvider       UserFieldsProvider
	profilePictureURLBuilder func(string) string
	logger                   kernel.Logger
	userManagementHandler    *api.UserManagementHandler
	profileHandler           *api.ProfileHandler
	profilePictureHandler    *api.ProfilePictureHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Users is a leaf: every collaborator is
// adopted, matching the retired wrapper edge.
func (m *Module) Requires() []string {
	return nil
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

// AdminManifest declares this module's admin-shell extensions. The Users
// navigation surface is static SPA chrome today, so the module declares no
// menu entries, settings sections, or editor extensions.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// UserManagementHandler exposes the user management handler for core
// mounting. Core mounts it explicitly (rather than via RouteProvider)
// because the surface spans the admin group, which a single RegisterRoutes
// call cannot express.
func (m *Module) UserManagementHandler() *api.UserManagementHandler {
	return m.userManagementHandler
}

// ProfileHandler exposes the self-service profile handler for core
// mounting. Same explicit-mount deviation as the management surface.
func (m *Module) ProfileHandler() *api.ProfileHandler {
	return m.profileHandler
}

// ProfilePictureHandler exposes the profile picture handler for core
// mounting. Same explicit-mount deviation as the management surface.
func (m *Module) ProfilePictureHandler() *api.ProfilePictureHandler {
	return m.profilePictureHandler
}

// New builds the module, adopting the composition root's wiring. The
// handlers are built once here so every realm serves the same instances.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.userManagementHandler = api.NewUserManagementHandler(
		m.userManagementService,
		m.adminCreateService,
		m.userStore,
		m.softDeleteStore,
		m.mailer,
		m.logger,
		m.profilePictureURLBuilder,
	)
	m.profileHandler = api.NewProfileHandler(
		m.profileService,
		m.accountDeletionService,
		m.logger,
		m.userFieldsProvider,
		m.profilePictureURLBuilder,
	)
	m.profilePictureHandler = api.NewProfilePictureHandler(
		m.profilePictureService,
		m.userStore,
		m.logger,
	)
	return m
}
