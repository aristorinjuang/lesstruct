package kernel

// DroppedModule names one module Prune removed from the candidate list.
type DroppedModule struct {
	// Name is the dropped module's registry key.
	Name string
	// Direct is true when the module itself was named in disabled, false
	// when it cascaded from a disabled requirement.
	Direct bool
	// Cause names the disabled module that forced the drop: the module
	// itself when Direct, otherwise the unavailable requirement.
	Cause string
}

// PruneResult is the outcome of Prune: the modules to register plus every
// dropped module with its reason.
type PruneResult struct {
	Kept    []Module
	Dropped []DroppedModule
}

// Prune removes disabled modules and their transitive dependents from the
// candidate list. It is a pure function over Requires edges: no registry
// mutation, no I/O, deterministic output in candidate order.
//
// A module named in disabled drops as Direct. Any other module whose
// Requires names a dropped or disabled module drops as a cascade, naming
// the unavailable requirement as Cause. Modules whose hard dependency is
// merely absent (not disabled) are KEPT: the registry still fails fast on
// those at InitAll, unchanged. Nil candidates are skipped.
func Prune(candidates []Module, disabled []string) PruneResult {
	off := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		off[name] = true
	}
	dropped := make(map[string]DroppedModule)
	changed := true
	for changed {
		changed = false
		for _, module := range candidates {
			if module == nil {
				continue
			}
			name := module.Name()
			if _, ok := dropped[name]; ok {
				continue
			}
			if off[name] {
				dropped[name] = DroppedModule{Name: name, Direct: true, Cause: name}
				changed = true
				continue
			}
			for _, dep := range module.Requires() {
				if _, ok := dropped[dep]; ok || off[dep] {
					dropped[name] = DroppedModule{Name: name, Cause: dep}
					changed = true
					break
				}
			}
		}
	}
	var result PruneResult
	for _, module := range candidates {
		if module == nil {
			continue
		}
		if drop, ok := dropped[module.Name()]; ok {
			result.Dropped = append(result.Dropped, drop)
			continue
		}
		result.Kept = append(result.Kept, module)
	}
	return result
}
