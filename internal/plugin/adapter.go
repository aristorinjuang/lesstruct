package plugin

import (
	"context"
	"errors"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
)

type HookExecutorAdapter struct {
	registry *registry.Registry
}

func (a *HookExecutorAdapter) Execute(
	ctx context.Context,
	hookName plugin.HookName,
	data []byte,
) ([]byte, error) {
	if a.registry == nil {
		return nil, nil
	}
	result, err := a.registry.Execute(ctx, hookName, data)
	if errors.Is(err, plugin.ErrHookNotFound) {
		return nil, nil
	}
	return result, err
}

func NewHookExecutorAdapter(registry *registry.Registry) *HookExecutorAdapter {
	return &HookExecutorAdapter{registry: registry}
}
