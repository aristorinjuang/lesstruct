// Package modgen is the AI module generator module's domain layer: the
// brief, project, and validation value types, the generation/store ports, the
// envelope parser, and the Go/module-contract validator. Provider adapters
// (the OpenAI-compatible client) live in infrastructure; the filesystem store
// lives in store; neither is imported by the domain.
package modgen

import (
	"errors"
	"time"
)

const (
	// maxBriefRunes caps the natural-language brief at 5,000 characters.
	maxBriefRunes = 5000
	// maxProjectNameRunes bounds the module name length (2–50 runes).
	maxProjectNameRunes = 50
	// minProjectNameRunes is the minimum module name length.
	minProjectNameRunes = 2
	// maxFileBytes caps a single project file at 512 KiB.
	maxFileBytes = 512 << 10
	// maxTotalBytes caps a project at 4 MiB total (modules carry more source
	// and migrations than plugins).
	maxTotalBytes = 4 << 20
	// maxFileCount caps the number of files in one generated module.
	maxFileCount = 64
)

var (
	// ErrInvalidBrief reports an empty or over-long generation brief.
	ErrInvalidBrief = errors.New("invalid brief")
	// ErrInvalidProjectName reports a module name outside the slug grammar.
	ErrInvalidProjectName = errors.New("invalid module name")
	// ErrPathNotAllowed reports a project file path outside the allowlist.
	ErrPathNotAllowed = errors.New("path not allowed")
	// ErrSizeLimitExceeded reports a project file or total exceeding the caps.
	ErrSizeLimitExceeded = errors.New("size limit exceeded")
	// ErrGenerationFailed reports an AI generation or repair failure.
	ErrGenerationFailed = errors.New("generation failed")
	// ErrProjectExists reports an install colliding with an existing project.
	ErrProjectExists = errors.New("project exists")
	// ErrProjectNotFound reports a lookup for a missing project.
	ErrProjectNotFound = errors.New("project not found")
	// ErrGenerationUnavailable reports generation without an AI key.
	ErrGenerationUnavailable = errors.New("generation unavailable")
	// ErrInvalidSurfaces reports an unknown surface declaration.
	ErrInvalidSurfaces = errors.New("invalid surfaces")
	// ErrApplyUnavailable reports an apply without an enabled applier: the
	// feature flag is off, no Go toolchain is on PATH, or the server does
	// not run from a source checkout.
	ErrApplyUnavailable = errors.New("apply unavailable")
	// ErrAlreadyApplied reports an apply for a module already present in
	// the checkout's modules tree.
	ErrAlreadyApplied = errors.New("already applied")
	// ErrTreeDirty reports an apply refused because a shared registry file
	// carries uncommitted operator changes, or a machine-owned path holds
	// hand-written files without the generated marker.
	ErrTreeDirty = errors.New("tree dirty")
	// ErrNotCheckout reports an apply outside a Lesstruct source checkout.
	ErrNotCheckout = errors.New("not a checkout")
	// ErrNotApplied reports an unapply for a module the checkout registry
	// does not hold. Only applied trees may be removed.
	ErrNotApplied = errors.New("not applied")
	// ErrConfirmationRequired reports a destructive call without its
	// explicit confirmation.
	ErrConfirmationRequired = errors.New("confirmation required")
	// ErrApplyFailed reports an apply whose writes or verification failed.
	ErrApplyFailed = errors.New("apply failed")
)

// ModuleSources is the prompt grounding: the distilled kernel contract, the
// module grammar (file layout and style rules), one exemplar module's
// sources, and the host versions the generated go.mod must pin.
type ModuleSources struct {
	Contract      string
	Grammar       string
	Examples      string
	KernelVersion string
}

// NewModuleSources assembles the prompt grounding material.
func NewModuleSources(
	contract, grammar, examples, kernelVersion string,
) ModuleSources {
	return ModuleSources{
		Contract:      contract,
		Grammar:       grammar,
		Examples:      examples,
		KernelVersion: kernelVersion,
	}
}

// Brief is the natural-language module request from the admin UI. Surfaces
// selects the kernel capability interfaces the module should implement; an
// empty list lets the model infer them from the description.
type Brief struct {
	Description string
	Surfaces    []string
}

// NewBrief assembles a generation brief from its parts.
func NewBrief(description string, surfaces []string) Brief {
	return Brief{
		Description: description,
		Surfaces:    surfaces,
	}
}

// Project is a validated generated module project: an installable name, the
// kernel surfaces it implements, the relative-path → content file map the
// store persists, and the model's design rationale.
type Project struct {
	Name      string
	Surfaces  []string
	Files     map[string]string
	Rationale string
}

// NewProject assembles a project from its validated parts.
func NewProject(
	name string,
	surfaces []string,
	files map[string]string,
	rationale string,
) Project {
	return Project{
		Name:      name,
		Surfaces:  surfaces,
		Files:     files,
		Rationale: rationale,
	}
}

// InstalledProject is the manifest projection the list endpoint serves:
// identity, declared surfaces, file count, creation time, provenance, and
// whether the project is applied to the checkout's modules tree.
type InstalledProject struct {
	Name      string
	Surfaces  []string
	FileCount int
	CreatedAt time.Time
	Source    string
	Applied   bool
}

// NewInstalledProject assembles an installed-project projection.
func NewInstalledProject(
	name string,
	surfaces []string,
	fileCount int,
	createdAt time.Time,
	source string,
	applied bool,
) InstalledProject {
	return InstalledProject{
		Name:      name,
		Surfaces:  surfaces,
		FileCount: fileCount,
		CreatedAt: createdAt,
		Source:    source,
		Applied:   applied,
	}
}

// ProjectManifest is the on-disk project.json metadata: identity, declared
// surfaces, the kernel version the generated go.mod pins, and provenance.
type ProjectManifest struct {
	Name          string    `json:"name"`
	Surfaces      []string  `json:"surfaces"`
	KernelVersion string    `json:"kernelVersion"`
	Generator     string    `json:"generator"`
	CreatedAt     time.Time `json:"createdAt"`
	Source        string    `json:"source"`
}

// NewProjectManifest assembles a manifest for an installed project.
func NewProjectManifest(
	name string,
	surfaces []string,
	kernelVersion, generator string,
	createdAt time.Time,
	source string,
) ProjectManifest {
	return ProjectManifest{
		Name:          name,
		Surfaces:      surfaces,
		KernelVersion: kernelVersion,
		Generator:     generator,
		CreatedAt:     createdAt,
		Source:        source,
	}
}

// ApplyStep is one verification outcome of an apply: the step name, whether
// it passed (or was skipped), and the failure-signaling output tail.
type ApplyStep struct {
	Name    string
	Passed  bool
	Skipped bool
	Output  string
}

// NewApplyStep assembles an apply verification step outcome.
func NewApplyStep(name string, passed, skipped bool, output string) ApplyStep {
	return ApplyStep{
		Name:    name,
		Passed:  passed,
		Skipped: skipped,
		Output:  output,
	}
}

// ApplyResult is the outcome of applying a staged project to a source
// checkout: whether the tree was written, whether verification passed, the
// per-step outcomes, and every module the custom edition now carries.
type ApplyResult struct {
	Applied         bool
	Verified        bool
	Steps           []ApplyStep
	AppliedModules  []string
	RequiresRestart bool
}

// NewApplyResult assembles an apply result.
func NewApplyResult(
	applied, verified bool,
	steps []ApplyStep,
	appliedModules []string,
	requiresRestart bool,
) ApplyResult {
	return ApplyResult{
		Applied:         applied,
		Verified:        verified,
		Steps:           steps,
		AppliedModules:  appliedModules,
		RequiresRestart: requiresRestart,
	}
}

// UnapplyResult is the outcome of removing an applied module from a source
// checkout: whether anything was removed, whether the post-removal build
// passed, the per-step outcomes, the remaining edition modules, and whether
// the module tree itself was deleted (false when it was already gone).
type UnapplyResult struct {
	Removed          bool
	Verified         bool
	Steps            []ApplyStep
	AppliedModules   []string
	ModuleDirRemoved bool
}

// NewUnapplyResult assembles an unapply result.
func NewUnapplyResult(
	removed, verified bool,
	steps []ApplyStep,
	appliedModules []string,
	moduleDirRemoved bool,
) UnapplyResult {
	return UnapplyResult{
		Removed:          removed,
		Verified:         verified,
		Steps:            steps,
		AppliedModules:   appliedModules,
		ModuleDirRemoved: moduleDirRemoved,
	}
}
