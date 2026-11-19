package plugingen

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	// maxRepairAttempts caps how many times a failing project is sent back to
	// the model with its validation findings before generation gives up.
	maxRepairAttempts = 2
)

var (
	briefWordPattern = regexp.MustCompile(`[a-z0-9]+`)
)

// slugifyBrief derives a project slug from the brief keywords plus a random
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
		base = "plugin"
	}
	var suffix [3]byte
	_, _ = rand.Read(suffix[:])
	extra := fmt.Sprintf("%02x%02x%02x", suffix[0], suffix[1], suffix[2])
	candidate := strings.Trim(base+"-"+extra, "-")
	if len(candidate) > maxProjectNameRunes {
		candidate = candidate[:maxProjectNameRunes]
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

// Service orchestrates plugin generation: brief validation, AI generation,
// validation gate with a bounded repair loop, and install as a source
// project. Build compiles an installed project to WASM through the builder
// seam (nil builder disables builds).
type Service struct {
	generation PluginGenerationService
	validator  ProjectValidator
	store      ProjectStore
	builder    Builder
	sources    func(ctx context.Context) PluginSources
}

func (s *Service) pluginSources(ctx context.Context) PluginSources {
	if s.sources == nil {
		return PluginSources{}
	}
	return s.sources(ctx)
}

// validateGenerated runs the composed validator plus the generated-content
// scan. It is generation-only: manual operator projects are trusted. Hooks
// and capabilities the brief never requested surface as warnings so the
// admin sees what the model added beyond the brief.
func (s *Service) validateGenerated(ctx context.Context, brief Brief, project Project) ValidationReport {
	report := s.validator.Validate(ctx, project)
	if scanned := ScanGeneratedProjectFiles(project.Files); len(scanned) > 0 {
		report = NewFindingsReport(append(report.Findings, scanned...))
	}
	var unrequested []Finding
	for _, hook := range project.Hooks {
		if !slices.Contains(brief.Hooks, strings.TrimSpace(hook)) {
			unrequested = append(unrequested, NewFinding("project-hooks", FindingSeverityWarning, fmt.Sprintf("hook %q was not requested in the brief", hook)))
		}
	}
	for _, capability := range project.Capabilities {
		if !slices.Contains(brief.Capabilities, strings.TrimSpace(capability)) {
			unrequested = append(unrequested, NewFinding("project-capabilities", FindingSeverityWarning, fmt.Sprintf("capability %q was not requested in the brief", capability)))
		}
	}
	if len(unrequested) > 0 {
		report = NewFindingsReport(append(report.Findings, unrequested...))
	}
	return report
}

// Generate validates the brief, asks the model, validates the result, and
// installs it as a source project. A failing result is sent back to the model
// up to maxRepairAttempts times with its findings; a still-failing project is
// refused. The final report is returned alongside the project even on success
// so the UI can surface warnings.
func (s *Service) Generate(
	ctx context.Context,
	brief Brief,
) (Project, ValidationReport, error) {
	if err := ValidateBrief(brief); err != nil {
		return Project{}, ValidationReport{}, err
	}
	if s.generation == nil {
		return Project{}, ValidationReport{}, ErrGenerationUnavailable
	}
	sources := s.pluginSources(ctx)
	project, err := s.generation.GenerateProject(ctx, brief, sources)
	if err != nil {
		return Project{}, ValidationReport{}, err
	}
	if err := ValidateProjectName(project.Name); err != nil {
		project.Name = slugifyBrief(brief.Description)
	}
	if project.Language == "" {
		project.Language = "go"
	}
	if len(project.Hooks) == 0 {
		project.Hooks = brief.Hooks
	}
	report := s.validateGenerated(ctx, brief, project)
	for attempts := 0; !report.Valid && attempts < maxRepairAttempts; attempts++ {
		repaired, err := s.generation.RepairProject(ctx, brief, sources, project, errorProblems(report))
		if err != nil {
			return Project{}, report, err
		}
		if err := ValidateProjectName(repaired.Name); err != nil {
			repaired.Name = project.Name
		}
		if repaired.Language == "" {
			repaired.Language = project.Language
		}
		project = repaired
		report = s.validateGenerated(ctx, brief, project)
	}
	if !report.Valid {
		return Project{}, report, fmt.Errorf("%w: project failed validation after %d repair attempts", ErrGenerationFailed, maxRepairAttempts)
	}
	if s.store != nil {
		if err := s.store.Install(ctx, project); err != nil {
			return Project{}, report, err
		}
	}
	return project, report, nil
}

// Build compiles one installed project to WASM and installs the .wasm plus
// the synthesized capability manifest into the host's plugins directory.
// The resolved name hides nothing: an unknown or malformed name reads as
// not-found, like Get.
func (s *Service) Build(ctx context.Context, name string) (BuildResult, error) {
	trimmed := strings.TrimSpace(name)
	if err := ValidateProjectName(trimmed); err != nil {
		return BuildResult{}, ErrProjectNotFound
	}
	if s.builder == nil {
		return BuildResult{}, ErrBuildUnavailable
	}
	if s.store == nil {
		return BuildResult{}, ErrProjectNotFound
	}
	project, err := s.store.Get(ctx, trimmed)
	if err != nil {
		return BuildResult{}, err
	}
	return s.builder.Build(ctx, project)
}

// List returns the installed-project projections.
func (s *Service) List(ctx context.Context) ([]InstalledProject, error) {
	if s.store == nil {
		return nil, ErrProjectNotFound
	}
	return s.store.List(ctx)
}

// Validate runs the validation gate without persisting.
func (s *Service) Validate(ctx context.Context, project Project) ValidationReport {
	return s.validator.Validate(ctx, project)
}

// Get returns one installed project by name.
func (s *Service) Get(ctx context.Context, name string) (Project, error) {
	if s.store == nil {
		return Project{}, ErrProjectNotFound
	}
	return s.store.Get(ctx, name)
}

// Delete removes an installed project.
func (s *Service) Delete(ctx context.Context, name string) error {
	if s.store == nil {
		return ErrProjectNotFound
	}
	return s.store.Delete(ctx, name)
}

// Zip builds the download archive for one installed project.
func (s *Service) Zip(ctx context.Context, name string) ([]byte, error) {
	if s.store == nil {
		return nil, ErrProjectNotFound
	}
	return s.store.Zip(ctx, name)
}

// NewService builds the orchestration service. A nil store disables
// persistence (generation still validates); a nil builder disables builds; a
// nil sources func generates from the distilled contract alone.
func NewService(
	generation PluginGenerationService,
	validator ProjectValidator,
	store ProjectStore,
	sources func(ctx context.Context) PluginSources,
) *Service {
	return NewServiceWithBuilder(generation, validator, store, nil, sources)
}

// NewServiceWithBuilder builds the orchestration service with a WASM build
// seam. It is the constructor the composition root uses; NewService is the
// builder-less specialization tests and minimal embeddings use.
func NewServiceWithBuilder(
	generation PluginGenerationService,
	validator ProjectValidator,
	store ProjectStore,
	builder Builder,
	sources func(ctx context.Context) PluginSources,
) *Service {
	if validator == nil {
		validator = NewValidator()
	}
	return &Service{
		generation: generation,
		validator:  validator,
		store:      store,
		builder:    builder,
		sources:    sources,
	}
}
