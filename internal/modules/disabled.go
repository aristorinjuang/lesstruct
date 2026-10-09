package modules

import (
	"slices"

	"github.com/aristorinjuang/lesstruct/kernel"
)

// EffectiveCommentsEnabled folds the legacy [comments] toggle and the v1
// [modules] disabled list into the single flag every consumer reads.
// Disabling the comments module is equivalent to the legacy toggle; a
// cascaded drop (kept lacks comments) unmounts the routes the same way.
// Headless mode is independent: it never reaches this function.
func EffectiveCommentsEnabled(legacyEnabled bool, modulesDisabled []string, kept map[string]bool) bool {
	return legacyEnabled && !slices.Contains(modulesDisabled, "comments") && kept["comments"]
}

// ApplyDisabled prunes the disabled modules (plus cascade) from the
// candidates, reports each drop and each unknown name through warnf, and
// registers only the kept modules. It returns the kept name set for
// back-compat flag mapping. Register errors return; the composition root
// fails startup on them. Unknown names warn and are ignored so configs stay
// portable across editions.
func ApplyDisabled(
	registry *kernel.Registry,
	candidates []kernel.Module,
	disabled []string,
	warnf func(format string, args ...any),
) (map[string]bool, error) {
	pruned := kernel.Prune(candidates, disabled)
	kept := make(map[string]bool, len(pruned.Kept))
	for _, module := range pruned.Kept {
		kept[module.Name()] = true
	}
	for _, drop := range pruned.Dropped {
		if drop.Direct {
			warnf("module %q disabled by [modules] config — skipping registration", drop.Name)
		} else {
			warnf("module %q disabled: requires disabled module %q — skipping registration", drop.Name, drop.Cause)
		}
	}
	known := make(map[string]bool, len(candidates))
	for _, module := range candidates {
		if module == nil {
			continue
		}
		known[module.Name()] = true
	}
	for _, name := range disabled {
		if !known[name] {
			warnf("unknown module %q in [modules] disabled — ignoring", name)
		}
	}
	for _, module := range pruned.Kept {
		if err := registry.Register(module); err != nil {
			return nil, err
		}
	}
	return kept, nil
}
