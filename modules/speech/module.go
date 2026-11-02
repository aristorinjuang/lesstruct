// Package speech is the official voice-dictation module: speech-to-text
// transcription over Meta Model API or any OpenAI-compatible endpoint,
// enhanced into TipTap content by the text generation service.
package speech

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/speech/admin"
	"github.com/aristorinjuang/lesstruct/modules/speech/api"
	speechdomain "github.com/aristorinjuang/lesstruct/modules/speech/domain"
)

// Name is the registry key for this module.
const Name = "speech"

// Version is the module version reported via Module.Version.
const Version = "0.1.0"

// Option configures a Module. The composition root supplies every option;
// direct construction without options yields a standalone module that serves
// nothing (the handler built over nil dependencies must not be invoked).
type Option func(*Module)

// WithTranscriber adopts the speech-to-text service (composition root, nil
// when unconfigured). The handler is only mounted when a transcriber exists.
func WithTranscriber(service speechdomain.SpeechToTextService) Option {
	return func(m *Module) {
		m.transcriber = service
	}
}

// WithEnhancer adopts the transcription enhancer (the text generation
// service from the composition root, nil when unconfigured). Without one the
// handler cannot shape transcripts into content.
func WithEnhancer(enhancer api.TranscriptionEnhancer) Option {
	return func(m *Module) {
		m.enhancer = enhancer
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

// Module is the speech kernel module.
type Module struct {
	transcriber speechdomain.SpeechToTextService
	enhancer    api.TranscriptionEnhancer
	logger      kernel.Logger
	handler     *api.SpeechHandler
}

// Name returns the registry key.
func (m *Module) Name() string {
	return Name
}

// Version returns the module version.
func (m *Module) Version() string {
	return Version
}

// Requires lists hard dependencies. Dictation degrades to unmounted when the
// providers are unconfigured, so nothing is required.
func (m *Module) Requires() []string {
	return nil
}

// Optional lists soft dependencies. The text generation service is adopted
// directly (not via the registry), so no module coordination is needed.
func (m *Module) Optional() []string {
	return nil
}

// Init wires the module. The handler is already built by New; Init only
// validates presence.
func (m *Module) Init(_ kernel.Kernel) error {
	return nil
}

// Shutdown releases module resources. There are none beyond memory.
func (m *Module) Shutdown() error {
	return nil
}

// AdminManifest declares this module's admin-shell extensions. The Dictate
// button renders data-driven from the /api/health feature flags, so the
// module declares no menu entries, settings sections, or editor extensions.
func (m *Module) AdminManifest() kernel.AdminExtensionManifest {
	return admin.Manifest()
}

// Handler exposes the speech handler for core mounting. Core mounts it
// explicitly (rather than via RouteProvider) because the surface is gated by
// the speechEnabled feature flag, which a single RegisterRoutes call cannot
// express.
func (m *Module) Handler() *api.SpeechHandler {
	return m.handler
}

// Service exposes the adopted transcription service. It stays valid for the
// module's lifetime.
func (m *Module) Service() speechdomain.SpeechToTextService {
	return m.transcriber
}

// New builds the module, adopting the composition root's wiring. The handler
// is built once here so routes serve the same instance.
func New(opts ...Option) *Module {
	m := &Module{}
	for _, opt := range opts {
		opt(m)
	}
	m.handler = api.NewSpeechHandler(m.transcriber, m.enhancer, m.logger)
	return m
}
