// Package posttype is the kernel's content-type discovery index. Modules
// declare the types they provide via kernel.ContentTypeProvider; core
// collects the declarations into this registry after InitAll. It answers
// only "which types exist and which slugs are reserved" — rich definitions
// (fields, validation, templates) stay in module domains.
package posttype

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/aristorinjuang/lesstruct/kernel"
)

var (
	ErrDuplicatePostType = errors.New("kernel: duplicate post type")
	ErrEmptyPostTypeSlug = errors.New("kernel: post type slug cannot be empty")
)

// Registry is the process-wide index of content types. Register order never
// matters; All returns slugs sorted. Protected slugs (claimed by built-in
// modules) are recorded so later phases can refuse overrides.
type Registry struct {
	mu        sync.RWMutex
	types     map[string]kernel.PostTypeDef
	protected map[string]bool
}

// Register adds a type definition. Duplicate slugs are rejected.
func (r *Registry) Register(def kernel.PostTypeDef) error {
	if def.Slug == "" {
		return ErrEmptyPostTypeSlug
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.types[def.Slug]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicatePostType, def.Slug)
	}
	r.types[def.Slug] = def
	if def.Protected {
		r.protected[def.Slug] = true
	}
	return nil
}

// GetBySlug returns the definition registered under slug.
func (r *Registry) GetBySlug(slug string) (kernel.PostTypeDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.types[slug]
	return def, ok
}

// IsProtected reports whether the slug was claimed as protected by a module.
func (r *Registry) IsProtected(slug string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.protected[slug]
}

// All returns every registered definition in slug-sorted order.
func (r *Registry) All() []kernel.PostTypeDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]kernel.PostTypeDef, 0, len(r.types))
	for _, def := range r.types {
		defs = append(defs, def)
	}
	slices.SortFunc(defs, func(a, b kernel.PostTypeDef) int {
		return strings.Compare(a.Slug, b.Slug)
	})
	return defs
}

// CollectFrom gathers every ContentTypeProvider in the module registry into
// r, in module-name order. The first duplicate slug aborts the collection.
func (r *Registry) CollectFrom(reg *kernel.Registry) error {
	if reg == nil {
		return errors.New("kernel: module registry cannot be nil")
	}
	for _, provider := range kernel.Capabilities[kernel.ContentTypeProvider](reg) {
		for _, def := range provider.PostTypes() {
			if err := r.Register(def); err != nil {
				return err
			}
		}
	}
	return nil
}

// Slugs returns every registered slug in sorted order.
func (r *Registry) Slugs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.types))
}

// NewRegistry creates an empty post-type registry.
func NewRegistry() *Registry {
	return &Registry{
		types:     make(map[string]kernel.PostTypeDef),
		protected: make(map[string]bool),
	}
}
