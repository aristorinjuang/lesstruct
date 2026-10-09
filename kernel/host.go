package kernel

// discardLogger is the no-op sink NewHost installs when no logger is given,
// mirroring the middleware convention that a nil logger degrades to discard.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// Host is the default Kernel implementation handed to Module.Init.
type Host struct {
	logger   Logger
	registry *Registry
}

// Logger returns the shared logger.
func (h *Host) Logger() Logger {
	return h.logger
}

// Registry returns the module registry for capability discovery.
func (h *Host) Registry() *Registry {
	return h.registry
}

// NewHost creates a Kernel host with a fresh bus and registry.
func NewHost(logger Logger) *Host {
	if logger == nil {
		logger = discardLogger{}
	}
	return &Host{
		logger:   logger,
		registry: NewRegistry(),
	}
}
