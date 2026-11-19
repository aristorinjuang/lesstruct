package plugingen

import (
	"context"
	"time"
)

// PluginGenerationService generates plugin projects from a brief. Sources
// carries the readable SDK contract and hook examples the prompts ground on.
// RepairProject feeds validation problems back to the model and receives the
// same brief and sources so the contract stays identical.
type PluginGenerationService interface {
	GenerateProject(ctx context.Context, brief Brief, sources PluginSources) (Project, error)
	RepairProject(ctx context.Context, brief Brief, sources PluginSources, project Project, problems []string) (Project, error)
}

// Finding is one validation result: a rule hit with severity and detail.
// Errors block install; warnings surface in the UI only.
type Finding struct {
	Rule     string
	Severity string
	Message  string
}

// NewFinding assembles a validation finding.
func NewFinding(rule, severity, message string) Finding {
	return Finding{
		Rule:     rule,
		Severity: severity,
		Message:  message,
	}
}

// ValidationReport is the outcome of validating a project before install.
// Valid is false when any error-severity finding exists.
type ValidationReport struct {
	Valid    bool
	Findings []Finding
}

// NewValidationReport assembles a validation report.
func NewValidationReport(valid bool, findings []Finding) ValidationReport {
	return ValidationReport{
		Valid:    valid,
		Findings: findings,
	}
}

// NewFindingsReport assembles a report whose Valid is derived from the
// findings: false when any error-severity finding exists.
func NewFindingsReport(findings []Finding) ValidationReport {
	for _, finding := range findings {
		if finding.Severity == FindingSeverityError {
			return NewValidationReport(false, findings)
		}
	}
	return NewValidationReport(true, findings)
}

// ProjectValidator checks a generated project before install: size caps, Go
// parse shape, hook and host-function policy.
type ProjectValidator interface {
	Validate(ctx context.Context, project Project) ValidationReport
}

// BuildResult is the outcome of a server-side plugin build: whether the
// .wasm and manifest installed into plugins/, and whether the host needs a
// restart to load them (production) or hot-reloaded them already (DEV_MODE).
type BuildResult struct {
	Installed       bool
	RequiresRestart bool
}

// NewBuildResult assembles a build result.
func NewBuildResult(installed, requiresRestart bool) BuildResult {
	return BuildResult{
		Installed:       installed,
		RequiresRestart: requiresRestart,
	}
}

// Builder compiles a validated project to WASM and installs it into the
// host's plugins directory.
type Builder interface {
	Build(ctx context.Context, project Project) (BuildResult, error)
}

// ProjectStore persists generated projects on disk and builds download zips.
type ProjectStore interface {
	Install(ctx context.Context, project Project) error
	List(ctx context.Context) ([]InstalledProject, error)
	Get(ctx context.Context, name string) (Project, error)
	Delete(ctx context.Context, name string) error
	Zip(ctx context.Context, name string) ([]byte, error)
}

// ProjectManifest is the on-disk project.json metadata: identity,
// provenance, and the capability grants the build step synthesizes the TOML
// sidecar from.
type ProjectManifest struct {
	Name          string    `json:"name"`
	Language      string    `json:"language"`
	Hooks         []string  `json:"hooks"`
	Capabilities  []string  `json:"capabilities"`
	HTTPPatterns  []string  `json:"httpPatterns,omitempty"`
	DBPermissions []string  `json:"dbPermissions,omitempty"`
	Generator     string    `json:"generator"`
	CreatedAt     time.Time `json:"createdAt"`
	Source        string    `json:"source"`
}

// NewProjectManifest assembles a manifest for an installed project.
func NewProjectManifest(
	name, language string,
	hooks []string,
	capabilities []string,
	httpPatterns []string,
	dbPermissions []string,
	generator string,
	createdAt time.Time,
	source string,
) ProjectManifest {
	return ProjectManifest{
		Name:          name,
		Language:      language,
		Hooks:         hooks,
		Capabilities:  capabilities,
		HTTPPatterns:  httpPatterns,
		DBPermissions: dbPermissions,
		Generator:     generator,
		CreatedAt:     createdAt,
		Source:        source,
	}
}
