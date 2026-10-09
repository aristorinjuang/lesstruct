// Package modules hosts the core-side adapters between the composition root
// and the real v1 modules under modules/: repository-to-seam bridges whose
// only layer may import both sides (user policy, auth notifications, soft
// delete, profile pictures), plus the logger adapter. The Phase 1
// transitional wrappers are fully retired — every first-party feature is a
// real module — so only DefaultEdition (kept for the Phase 4.1
// disabled/cascade work) and the adapters remain.
package modules

import (
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/aristorinjuang/lesstruct/kernel"
)

type loggerAdapter struct {
	logger *util.Logger
}

func (a *loggerAdapter) Info(format string, args ...any) {
	a.logger.Info(format, args...)
}

func (a *loggerAdapter) Error(format string, args ...any) {
	a.logger.Error(format, args...)
}

// DefaultEdition returns every first-party wrapper — the default composition
// root. Registration order is irrelevant; the registry topologically sorts.
// All wrappers are retired: every first-party feature is a real module, so
// the edition is empty until Phase 4.1 rebuilds it on the disabled/cascade
// matrix.
func DefaultEdition() []kernel.Module {
	return []kernel.Module{}
}

// NewLogger adapts *util.Logger to kernel.Logger. A nil logger yields nil,
// and kernel.NewHost substitutes its discard sink.
func NewLogger(logger *util.Logger) kernel.Logger {
	if logger == nil {
		return nil
	}
	return &loggerAdapter{
		logger: logger,
	}
}
