package kernel

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

var (
	ErrDuplicateModule    = errors.New("kernel: duplicate module")
	ErrMissingDependency  = errors.New("kernel: missing module dependency")
	ErrCircularDependency = errors.New("kernel: circular module dependency")
	ErrNilKernel          = errors.New("kernel: host cannot be nil")
	ErrNilModule          = errors.New("kernel: module cannot be nil")
)

// Capabilities collects every registered module implementing capability T in
// registry name order. It is the discovery mechanism modules use instead of a
// service locator.
func Capabilities[T any](r *Registry) []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []T
	for _, name := range slices.Sorted(maps.Keys(r.modules)) {
		if capability, ok := any(r.modules[name]).(T); ok {
			out = append(out, capability)
		}
	}
	return out
}

// Registry is the module graph: it stores modules, resolves their Requires
// edges into a deterministic init order, and runs lifecycle transitions.
// Register order never matters; InitAll always topologically sorts.
type Registry struct {
	mu          sync.RWMutex
	modules     map[string]Module
	order       []string
	initialized bool
}

func (r *Registry) resolveOrder() ([]Module, error) {
	r.mu.RLock()
	modules := make(map[string]Module, len(r.modules))
	maps.Copy(modules, r.modules)
	r.mu.RUnlock()
	const (
		unvisited = 0
		inStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(modules))
	var ordered []Module
	var visit func(name string) error
	visit = func(name string) error {
		switch state[name] {
		case done:
			return nil
		case inStack:
			return fmt.Errorf("%w: %q", ErrCircularDependency, name)
		}
		state[name] = inStack
		module := modules[name]
		optional := module.Optional()
		for _, dep := range slices.Sorted(slices.Values(module.Requires())) {
			if _, ok := modules[dep]; !ok {
				if slices.Contains(optional, dep) {
					continue
				}
				return fmt.Errorf("%w: module %q requires %q", ErrMissingDependency, name, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = done
		ordered = append(ordered, module)
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(modules)) {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

// Register adds a module under its Name. Duplicate names are rejected.
func (r *Registry) Register(module Module) error {
	if module == nil {
		return ErrNilModule
	}
	name := module.Name()
	if name == "" {
		return errors.New("kernel: module name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateModule, name)
	}
	r.modules[name] = module
	return nil
}

// Get returns the module registered under name.
func (r *Registry) Get(name string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, ok := r.modules[name]
	return module, ok
}

// Names returns every registered module name in sorted order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.modules))
}

// Initialized reports whether InitAll completed successfully.
func (r *Registry) Initialized() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.initialized
}

// InitOrder returns the topological init order from the last InitAll,
// or nil when InitAll has not run.
func (r *Registry) InitOrder() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.order)
}

// InitAll resolves the dependency graph and calls Init on every module in
// topological order. A hard Requires edge to an unregistered module fails
// fast; an Optional edge to one is silently skipped. The first Init error
// aborts the transition and leaves Initialized false; already-initialized
// modules are NOT rolled back — the caller owns that (at startup the process
// exits, so there is nothing to roll back to). InitAll is a single-shot
// startup transition, not safe for concurrent use.
func (r *Registry) InitAll(kernel Kernel) error {
	if kernel == nil {
		return ErrNilKernel
	}
	ordered, err := r.resolveOrder()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(ordered))
	for _, module := range ordered {
		if err := module.Init(kernel); err != nil {
			return fmt.Errorf("kernel: init module %q: %w", module.Name(), err)
		}
		names = append(names, module.Name())
	}
	r.mu.Lock()
	r.order = names
	r.initialized = true
	r.mu.Unlock()
	return nil
}

// ShutdownAll calls Shutdown in reverse init order, collecting every error
// with errors.Join. Modules never initialized are skipped.
func (r *Registry) ShutdownAll() error {
	r.mu.RLock()
	ordered := make([]Module, 0, len(r.order))
	for _, name := range r.order {
		if module, ok := r.modules[name]; ok {
			ordered = append(ordered, module)
		}
	}
	r.mu.RUnlock()
	var errs []error
	for i := len(ordered) - 1; i >= 0; i-- {
		if err := ordered[i].Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("kernel: shutdown module %q: %w", ordered[i].Name(), err))
		}
	}
	return errors.Join(errs...)
}

// NewRegistry creates an empty module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
	}
}
