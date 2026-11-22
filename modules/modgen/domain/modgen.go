package modgen

import (
	"context"
)

// ModuleGenerationService generates module projects from a brief. Sources
// carries the readable kernel contract, the module grammar, and the exemplar
// sources the prompts ground on. RepairProject feeds validation problems back
// to the model and receives the same brief and sources so the contract stays
// identical.
type ModuleGenerationService interface {
	GenerateProject(ctx context.Context, brief Brief, sources ModuleSources) (Project, error)
	RepairProject(ctx context.Context, brief Brief, sources ModuleSources, project Project, problems []string) (Project, error)
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
// parse shape, the kernel.Module contract, layer import policy, and migration
// hygiene.
type ProjectValidator interface {
	Validate(ctx context.Context, project Project) ValidationReport
}

// ProjectStore persists generated projects on disk and builds download zips.
type ProjectStore interface {
	Install(ctx context.Context, project Project) error
	List(ctx context.Context) ([]InstalledProject, error)
	Get(ctx context.Context, name string) (Project, error)
	Delete(ctx context.Context, name string) error
	Zip(ctx context.Context, name string) ([]byte, error)
}

// Applier stages a generated project into a source checkout: it copies the
// project tree, registers the module in the machine-writable registries,
// regenerates the custom-edition wiring, and verifies the result compiles
// and tests green. Apply is refused — never partial — when the checkout
// guard fails; verification failures keep the applied tree and report
// per-step outcomes instead of rolling back.
type Applier interface {
	Apply(ctx context.Context, project Project, replace, confirm bool) (ApplyResult, error)
	Unapply(ctx context.Context, name string, confirm bool) (UnapplyResult, error)
	Applied(ctx context.Context) ([]string, error)
}
