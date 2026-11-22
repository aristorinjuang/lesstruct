package blog

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/config"
	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	modgeninfra "github.com/aristorinjuang/lesstruct/modules/modgen/infrastructure"
)

// resolveKernelVersion reports the kernel module version the host binary
// built against, or "" when it cannot be determined (workspace checkouts
// report no version). Generated go.mod files must require exactly this
// version; an empty version degrades the generator to the distilled
// contract alone without a pin.
func resolveKernelVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/aristorinjuang/lesstruct/kernel" {
			if dep.Version == "" || dep.Version == "(devel)" {
				return ""
			}
			return dep.Version
		}
	}
	return ""
}

// resolveKernelVersionFromGoMod derives the kernel version from a checkout
// root's go.mod require, for workspace builds where debug.ReadBuildInfo
// reports no usable version. The directory must declare the Lesstruct
// module; anything else reads as not-a-checkout and yields "".
func resolveKernelVersionFromGoMod(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "module github.com/aristorinjuang/lesstruct" {
		return ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "require":
			if len(fields) >= 3 && strings.Contains(fields[1], "lesstruct/kernel") {
				return strings.TrimSpace(fields[2])
			}
		case "replace", "exclude", "retract":
		default:
			// require-block line: "<path> <version>"
			if len(fields) >= 2 && strings.Contains(fields[0], "lesstruct/kernel") {
				return strings.TrimSpace(fields[1])
			}
		}
	}
	return ""
}

// newModuleSources builds the modgen grounding material: the host kernel
// version for the go.mod pin. The distilled contract, grammar, and exemplar
// sources stay baked into the infrastructure prompts; the edition only
// supplies what varies per binary.
func newModuleSources(kernelVersion string) func(context.Context) modgendomain.ModuleSources {
	sources := modgendomain.NewModuleSources("", "", "", kernelVersion)
	return func(context.Context) modgendomain.ModuleSources {
		return sources
	}
}

// newModuleApplier builds the checkout applier when applies are explicitly
// opted in and the host can honor them: a source checkout at the apply
// root plus a Go toolchain on PATH. Anything else reads as not-ready —
// the operator gets a boot warning and the apply surface answers 503 —
// never a half-wired applier.
func newModuleApplier(cfg *config.Config, kernelVersion string) (modgendomain.Applier, bool) {
	if !cfg.IsModuleApplyEnabled() {
		return nil, false
	}
	root := strings.TrimSpace(cfg.ModuleApplyRoot)
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		log.Printf("Warning: MODULE_APPLY_ROOT is not usable (%v): module applies answer 503 apply_unavailable", err)
		return nil, false
	}
	goPath := strings.TrimSpace(cfg.GoPath)
	if goPath == "" {
		goPath = "go"
	}
	if _, err := exec.LookPath(goPath); err != nil {
		log.Printf("Warning: Go toolchain not found (%s): module applies answer 503 apply_unavailable", goPath)
		return nil, false
	}
	if !modgeninfra.IsCheckoutDir(abs) {
		log.Printf("Warning: %s is not a Lesstruct source checkout: module applies answer 503 apply_unavailable", abs)
		return nil, false
	}
	return modgeninfra.NewCheckoutApplier(abs, goPath, kernelVersion, cfg.ModuleApplyTimeout, nil), true
}
