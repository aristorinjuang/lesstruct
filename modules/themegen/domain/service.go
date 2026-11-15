package themegen

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

const (
	// maxRepairAttempts caps how many times a failing theme is sent back to
	// the model with its validation findings before generation gives up.
	maxRepairAttempts = 2
)

var (
	briefWordPattern = regexp.MustCompile(`[a-z0-9]+`)
)

// slugifyBrief derives a theme slug from the brief keywords plus a random
// suffix. It is the fallback when the model returns an unusable name.
func slugifyBrief(description string) string {
	lowered := strings.ToLower(description)
	words := briefWordPattern.FindAllString(lowered, -1)
	kept := make([]string, 0, 3)
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		kept = append(kept, word)
		if len(kept) == 3 {
			break
		}
	}
	base := strings.Join(kept, "-")
	if base == "" {
		base = "theme"
	}
	var suffix [3]byte
	_, _ = rand.Read(suffix[:])
	extra := fmt.Sprintf("%02x%02x%02x", suffix[0], suffix[1], suffix[2])
	candidate := strings.Trim(base+"-"+extra, "-")
	if len(candidate) > maxThemeNameRunes {
		candidate = candidate[:maxThemeNameRunes]
	}
	if err := ValidateThemeName(candidate); err != nil {
		return "theme-" + extra
	}
	return candidate
}

// errorProblems extracts error-severity findings as repair instructions:
// "<rule>: <message>" when a rule is present, the message alone otherwise.
// Warnings are guidance, not blockers, and never reach the repair prompt.
func errorProblems(report ValidationReport) []string {
	problems := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if finding.Severity != FindingSeverityError {
			continue
		}
		if finding.Rule != "" {
			problems = append(problems, finding.Rule+": "+finding.Message)
			continue
		}
		problems = append(problems, finding.Message)
	}
	return problems
}

// Service orchestrates theme generation: brief validation, AI generation,
// validation gate with a bounded repair loop, and auto-install as inactive.
type Service struct {
	generation ThemeGenerationService
	validator  ThemeValidator
	store      ThemeStore
	sources    func(ctx context.Context, edition Edition) ThemeSources
	edition    Edition
}

func (s *Service) themeSources(ctx context.Context, edition Edition) ThemeSources {
	if s.sources == nil {
		return ThemeSources{}
	}
	return s.sources(ctx, edition)
}

// validateGenerated runs the composed validator plus the generated-content
// scan. It is generation-only: manual operator themes are trusted and pass
// through boot, reload, and activation with structural validation alone.
func (s *Service) validateGenerated(ctx context.Context, theme Theme) ValidationReport {
	report := s.validator.Validate(ctx, theme)
	if scanned := ScanGeneratedThemeFiles(theme.Files); len(scanned) > 0 {
		report = NewFindingsReport(append(report.Findings, scanned...))
	}
	return report
}

// Generate validates the brief, asks the model, validates the result, and
// installs it as inactive. A failing result is sent back to the model up to
// maxRepairAttempts times with its findings; a still-failing theme is
// refused. The final report is returned alongside the theme even on success
// so the UI can surface warnings.
func (s *Service) Generate(
	ctx context.Context,
	brief Brief,
	edition Edition,
) (Theme, ValidationReport, error) {
	if err := ValidateBrief(brief); err != nil {
		return Theme{}, ValidationReport{}, err
	}
	if s.generation == nil {
		return Theme{}, ValidationReport{}, ErrGenerationUnavailable
	}
	sources := s.themeSources(ctx, edition)
	theme, err := s.generation.GenerateTheme(ctx, brief, edition, sources)
	if err != nil {
		return Theme{}, ValidationReport{}, err
	}
	if theme.Edition == "" {
		theme.Edition = edition
	}
	if err := ValidateThemeName(theme.Name); err != nil {
		theme.Name = slugifyBrief(brief.Description)
	}
	report := s.validateGenerated(ctx, theme)
	for attempts := 0; !report.Valid && attempts < maxRepairAttempts; attempts++ {
		repaired, err := s.generation.RepairTheme(ctx, brief, sources, theme, errorProblems(report))
		if err != nil {
			return Theme{}, report, err
		}
		if repaired.Edition == "" {
			repaired.Edition = theme.Edition
		}
		if err := ValidateThemeName(repaired.Name); err != nil {
			repaired.Name = theme.Name
		}
		theme = repaired
		report = s.validateGenerated(ctx, theme)
	}
	if !report.Valid {
		return Theme{}, report, fmt.Errorf("%w: theme failed validation after %d repair attempts", ErrGenerationFailed, maxRepairAttempts)
	}
	if s.store != nil {
		if err := s.store.Install(ctx, theme); err != nil {
			return Theme{}, report, err
		}
	}
	return theme, report, nil
}

// List returns the installed-theme projections.
func (s *Service) List(ctx context.Context) ([]InstalledTheme, error) {
	if s.store == nil {
		return nil, ErrThemeNotFound
	}
	return s.store.List(ctx)
}

// Validate runs the validation gate without persisting.
func (s *Service) Validate(ctx context.Context, theme Theme) ValidationReport {
	return s.validator.Validate(ctx, theme)
}

// Get returns one installed theme by name.
func (s *Service) Get(ctx context.Context, name string) (Theme, error) {
	if s.store == nil {
		return Theme{}, ErrThemeNotFound
	}
	return s.store.Get(ctx, name)
}

// Delete removes an installed theme.
func (s *Service) Delete(ctx context.Context, name string) error {
	if s.store == nil {
		return ErrThemeNotFound
	}
	return s.store.Delete(ctx, name)
}

// Zip builds the download archive for one installed theme.
func (s *Service) Zip(ctx context.Context, name string) ([]byte, error) {
	if s.store == nil {
		return nil, ErrThemeNotFound
	}
	return s.store.Zip(ctx, name)
}

// NewService builds the orchestration service. A nil store disables
// persistence (generation still validates); a nil sources func generates
// from the distilled contract alone.
func NewService(
	generation ThemeGenerationService,
	validator ThemeValidator,
	store ThemeStore,
	sources func(ctx context.Context, edition Edition) ThemeSources,
	edition Edition,
) *Service {
	if validator == nil {
		validator = NewValidator()
	}
	return &Service{
		generation: generation,
		validator:  validator,
		store:      store,
		sources:    sources,
		edition:    edition,
	}
}
