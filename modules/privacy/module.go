// Package privacy is the official privacy redaction module: the SAM 3.1
// scan that blurs faces, plates, and nudity before media and profile
// pictures are stored. It owns no routes: the media and users domains call
// it through narrow seams the composition root adapts.
package privacy

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	privacydomain "github.com/aristorinjuang/lesstruct/modules/privacy/domain"
)

const (
	// Name is the registry key for this module.
	Name = "privacy"

	// Version is the module version reported via Module.Version.
	Version = "0.1.0"
)

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module with an
// inert service (nil Segmenter returns its input unchanged).
type Option func(*Module)

// WithService adopts the redaction service (composition root, built over
// the SAM client when AI_SAM_API_KEY is set).
func WithService(service *privacydomain.Service) Option {
	return func(m *Module) {
		if service != nil {
			m.service = service
		}
	}
}

// WithLogger attaches the module logger. Without one there is nothing to
// log: the service reports failures to its caller, which fails closed.
func WithLogger(logger kernel.Logger) Option {
	return func(m *Module) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// Module is the privacy kernel module.
type Module struct {
	service *privacydomain.Service
	logger  kernel.Logger
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Redaction degrades to inert without a
// segmenter, so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. None: the SAM client is built by the
// composition root, not discovered from another module.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The service is already adopted by New; Init only
// validates presence.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// AdminManifest declares this module's admin-shell extensions. Redaction is
// env-driven with no UI toggle, so the module declares nothing.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return kernel.AdminExtensionManifest{}
}

// Service exposes the adopted redaction service for composition-root wiring
// (media and profile-picture adapters). It stays valid for the module's
// lifetime.
func (m *Module) Service() *privacydomain.Service {
	return m.service
}

// New builds the module, adopting the composition root's wiring.
func New(opts ...Option) *Module {
	m := &Module{
		service: privacydomain.NewService(nil, nil),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}
