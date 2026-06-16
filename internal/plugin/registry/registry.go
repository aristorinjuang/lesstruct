package registry

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
)

func insertSorted(regs []plugin.HookRegistration) []plugin.HookRegistration {
	if len(regs) <= 1 {
		return regs
	}

	sort.SliceStable(regs, func(i, j int) bool {
		return regs[i].Priority < regs[j].Priority
	})
	return regs
}

type hookKey struct {
	pluginName string
	hookName   plugin.HookName
}

type Registry struct {
	mu     sync.RWMutex
	hooks  map[plugin.HookName][]plugin.HookRegistration
	keys   map[hookKey]struct{}
	logger func(string)
}

func (r *Registry) Register(reg plugin.HookRegistration) error {
	if reg.Handler == nil {
		return fmt.Errorf("handler cannot be nil for plugin %q hook %q", reg.PluginName, reg.HookName)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := hookKey{pluginName: reg.PluginName, hookName: reg.HookName}
	if _, exists := r.keys[key]; exists {
		return plugin.ErrHookAlreadyRegistered
	}

	r.keys[key] = struct{}{}
	r.hooks[reg.HookName] = insertSorted(
		append(r.hooks[reg.HookName], reg),
	)

	return nil
}

func (r *Registry) Execute(
	ctx context.Context,
	hookName plugin.HookName,
	data []byte,
) ([]byte, error) {
	r.mu.RLock()
	registrations, exists := r.hooks[hookName]
	r.mu.RUnlock()

	if !exists || len(registrations) == 0 {
		return nil, plugin.ErrHookNotFound
	}

	for _, reg := range registrations {
		inputCopy := make([]byte, len(data))
		copy(inputCopy, data)
		snapshot := make([]byte, len(inputCopy))
		copy(snapshot, inputCopy)

		result, err := reg.Handler(ctx, inputCopy)
		if err != nil {
			switch reg.FailureMode {
			case plugin.FailFast:
				r.logger(fmt.Sprintf(
					"Hook execution failed: %s",
					err.Error(),
				))
				return nil, fmt.Errorf(
					"%w: %s.%s: %v",
					plugin.ErrHookExecutionFailed,
					reg.PluginName,
					reg.HookName,
					err,
				)
			case plugin.LogAndContinue:
				r.logger(fmt.Sprintf(
					"Hook execution failed: %s",
					err.Error(),
				))
				continue
			case plugin.Fallback:
				if reg.FallbackHandler != nil {
					r.logger("Hook failed, using fallback")
					fallbackResult, fbErr := reg.FallbackHandler(ctx, snapshot)
					if fbErr != nil {
						r.logger(fmt.Sprintf(
							"Hook execution failed: %s",
							fbErr.Error(),
						))
						continue
					}
					if len(fallbackResult) == 0 {
						data = snapshot
					} else {
						data = fallbackResult
					}
					continue
				}
				r.logger(fmt.Sprintf(
					"Hook execution failed: %s",
					err.Error(),
				))
				continue
			default:
				r.logger(fmt.Sprintf(
					"Hook execution failed: %s",
					err.Error(),
				))
				return nil, fmt.Errorf(
					"%w: %s.%s: %v",
					plugin.ErrHookExecutionFailed,
					reg.PluginName,
					reg.HookName,
					err,
				)
			}
		}

		if !bytes.Equal(inputCopy, snapshot) {
			r.logger("Plugin attempted to modify immutable data")
		}

		if len(result) == 0 {
			r.logger("Hook did not return data, using original")
			data = snapshot
		} else {
			data = result
		}
	}

	return data, nil
}

func (r *Registry) Unregister(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, regs := range r.hooks {
		filtered := make([]plugin.HookRegistration, 0, len(regs))
		for _, reg := range regs {
			if reg.PluginName != pluginName {
				filtered = append(filtered, reg)
			} else {
				delete(r.keys, hookKey{pluginName: pluginName, hookName: reg.HookName})
			}
		}
		if len(filtered) > 0 {
			r.hooks[name] = filtered
		} else {
			delete(r.hooks, name)
		}
	}
}

func (r *Registry) Hooks() map[plugin.HookName][]plugin.HookRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[plugin.HookName][]plugin.HookRegistration, len(r.hooks))
	for name, regs := range r.hooks {
		cp := make([]plugin.HookRegistration, len(regs))
		copy(cp, regs)
		result[name] = cp
	}
	return result
}

func (r *Registry) HasHook(hookName plugin.HookName) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.hooks[hookName]) > 0
}

func NewRegistry(logger func(string)) *Registry {
	if logger == nil {
		logger = func(string) {}
	}
	return &Registry{
		hooks:  make(map[plugin.HookName][]plugin.HookRegistration),
		keys:   make(map[hookKey]struct{}),
		logger: logger,
	}
}
