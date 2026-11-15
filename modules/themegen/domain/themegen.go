package themegen

import (
	"context"
	"time"
)

// ThemeGenerationService generates theme files from a brief. The edition
// selects the source theme; sources carries the readable base material
// (stylesheets and, for template overrides, page templates) the prompts
// ground on. RepairTheme feeds validation problems back to the model and
// receives the same brief and sources so the contract stays identical.
type ThemeGenerationService interface {
	GenerateTheme(ctx context.Context, brief Brief, edition Edition, sources ThemeSources) (Theme, error)
	RepairTheme(ctx context.Context, brief Brief, sources ThemeSources, theme Theme, problems []string) (Theme, error)
}

// Finding is one validation result: a rule hit with severity and detail.
// Errors block install and activation; warnings surface in the UI only.
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

// ValidationReport is the outcome of validating a theme before install or
// activation. Valid is false when any error-severity finding exists.
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
// findings: false when any error-severity finding exists. Decorators and
// whole-pipeline validators use it so severity handling stays in one place.
func NewFindingsReport(findings []Finding) ValidationReport {
	for _, finding := range findings {
		if finding.Severity == FindingSeverityError {
			return NewValidationReport(false, findings)
		}
	}
	return NewValidationReport(true, findings)
}

// ThemeValidator checks a generated theme before install and again before
// activation: size caps, CSS parse, and url() policy.
type ThemeValidator interface {
	Validate(ctx context.Context, theme Theme) ValidationReport
}

// ThemeStore persists installed themes on disk and builds download zips.
type ThemeStore interface {
	Install(ctx context.Context, theme Theme) error
	List(ctx context.Context) ([]InstalledTheme, error)
	Get(ctx context.Context, name string) (Theme, error)
	Delete(ctx context.Context, name string) error
	Zip(ctx context.Context, name string) ([]byte, error)
}

// ActiveThemeState resolves the currently active installed theme: the
// [theme] active value the edition composes over the embedded default.
type ActiveThemeState interface {
	ActiveTheme(ctx context.Context) string
}

// ThemeManifest is the on-disk theme.json metadata: identity and
// provenance. Active state lives in config, never in the manifest.
type ThemeManifest struct {
	Name      string    `json:"name"`
	Edition   string    `json:"edition"`
	Generator string    `json:"generator"`
	CreatedAt time.Time `json:"createdAt"`
	Source    string    `json:"source"`
}

// NewThemeManifest assembles a manifest for an installed theme.
func NewThemeManifest(
	name, edition, generator string,
	createdAt time.Time,
	source string,
) ThemeManifest {
	return ThemeManifest{
		Name:      name,
		Edition:   edition,
		Generator: generator,
		CreatedAt: createdAt,
		Source:    source,
	}
}
