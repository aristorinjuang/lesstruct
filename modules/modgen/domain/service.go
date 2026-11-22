package modgen

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	// maxRepairAttempts caps how many times a failing module is sent back to
	// the model with its validation findings before generation gives up.
	maxRepairAttempts = 2
)

var (
	briefWordPattern = regexp.MustCompile(`[a-z0-9]+`)
)

// slugifyBrief derives a module name from the brief keywords plus a random
// suffix. It is the fallback when the model returns an unusable name.
// Module names stay hyphen-free so the package clause needs no rename.
func slugifyBrief(description string) string {
	lowered := strings.ToLower(description)
	words := briefWordPattern.FindAllString(lowered, -1)
	kept := make([]string, 0, 3)
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		kept = append(kept, word)
		if len(kept) == 2 {
			break
		}
	}
	base := strings.Join(kept, "")
	if base == "" {
		base = "mymodule"
	}
	var suffix [2]byte
	_, _ = rand.Read(suffix[:])
	extra := fmt.Sprintf("%02x%02x", suffix[0], suffix[1])
	candidate := strings.Trim(base+extra, "-")
	if len(candidate) > maxProjectNameRunes {
		candidate = candidate[:maxProjectNameRunes]
	}
	if err := ValidateProjectName(candidate); err != nil {
		return "mymodule" + extra
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

// checkTierGuidance nudges hook-only briefs toward Tier 2 plugins: a module
// that declares no kernel surface is likely a runtime hook in disguise.
func checkTierGuidance(brief Brief, project Project) string {
	if len(project.Surfaces) > 0 {
		return ""
	}
	lowered := strings.ToLower(brief.Description)
	for _, hint := range []string{"hook", "before_save", "after_create", "after_publish", "wasm", "plugin"} {
		if strings.Contains(lowered, hint) {
			return "this brief reads like a runtime hook; a Tier 2 WASM plugin may fit better than a Tier 1 module"
		}
	}
	return ""
}

// renameProjectForCollision re-homes a validated project under a fresh
// slug after an install collision: the go.mod module line, the root and
// domain package clauses, and self-references to the old module path are
// rewritten so the tree stays internally consistent. It reports false when
// the project does not carry the expected declarations (defense in depth:
// only repair-validated projects reach it, so this is unreachable in
// practice).
func renameProjectForCollision(project Project, name string) (Project, bool) {
	oldModule := "github.com/aristorinjuang/lesstruct/modules/" + project.Name
	newModule := "github.com/aristorinjuang/lesstruct/modules/" + name
	renamed := NewProject(name, slices.Clone(project.Surfaces), make(map[string]string, len(project.Files)), project.Rationale)
	for path, content := range project.Files {
		updated := strings.ReplaceAll(content, oldModule, newModule)
		if path == "module.go" || strings.HasPrefix(path, "domain/") {
			decl := "package " + project.Name
			lines := strings.Split(updated, "\n")
			replaced := false
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if rest, ok := strings.CutPrefix(trimmed, decl); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
					lines[i] = "package " + name + rest
					replaced = true
					break
				}
			}
			if !replaced {
				return Project{}, false
			}
			updated = strings.Join(lines, "\n")
		}
		renamed.Files[path] = updated
	}
	return renamed, true
}

// Service orchestrates module generation: brief validation, AI generation,
// validation gate with a bounded repair loop, and install as an inert source
// project under modules-gen/. Apply stages an installed project into a
// source checkout through the applier seam (nil applier disables applies).
type Service struct {
	generation ModuleGenerationService
	validator  ProjectValidator
	store      ProjectStore
	applier    Applier
	sources    func(ctx context.Context) ModuleSources
}

func (s *Service) moduleSources(ctx context.Context) ModuleSources {
	if s.sources == nil {
		return ModuleSources{}
	}
	return s.sources(ctx)
}

// validateGenerated runs the composed validator plus the generated-content
// scan. Hooks the brief never requested surface as warnings so the admin
// sees what the model added beyond the brief.
func (s *Service) validateGenerated(ctx context.Context, brief Brief, project Project) ValidationReport {
	report := s.validator.Validate(ctx, project)
	if scanned := ScanGeneratedProjectFiles(project.Files); len(scanned) > 0 {
		report = NewFindingsReport(append(report.Findings, scanned...))
	}
	var unrequested []Finding
	for _, surface := range project.Surfaces {
		if !slices.Contains(brief.Surfaces, strings.TrimSpace(surface)) {
			unrequested = append(unrequested, NewFinding("module-surfaces", FindingSeverityWarning, fmt.Sprintf("surface %q was not requested in the brief", surface)))
		}
	}
	if len(unrequested) > 0 {
		report = NewFindingsReport(append(report.Findings, unrequested...))
	}
	if handshake := checkTierGuidance(brief, project); handshake != "" {
		report = NewFindingsReport(append(report.Findings, NewFinding("module-tier", FindingSeverityWarning, handshake)))
	}
	return report
}

// Generate validates the brief, asks the model, validates the result, and
// installs it as an inert source project. A failing result is sent back to
// the model up to maxRepairAttempts times with its findings; a still-failing
// project is refused. The final report is returned alongside the project even
// on success so the UI can surface warnings.
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
	sources := s.moduleSources(ctx)
	project, err := s.generation.GenerateProject(ctx, brief, sources)
	if err != nil {
		return Project{}, ValidationReport{}, err
	}
	if err := ValidateProjectName(project.Name); err != nil {
		project.Name = slugifyBrief(brief.Description)
	}
	if len(project.Surfaces) == 0 && len(brief.Surfaces) > 0 {
		project.Surfaces = slices.Clone(brief.Surfaces)
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
		if len(repaired.Surfaces) == 0 {
			repaired.Surfaces = project.Surfaces
		}
		project = repaired
		report = s.validateGenerated(ctx, brief, project)
	}
	if !report.Valid {
		return Project{}, report, fmt.Errorf("%w: project failed validation after %d repair attempts", ErrGenerationFailed, maxRepairAttempts)
	}
	if s.store != nil {
		if err := s.store.Install(ctx, project); err != nil {
			if !errors.Is(err, ErrProjectExists) {
				return Project{}, report, err
			}
			// A valid-but-taken name discards the whole paid generation.
			// Re-home the validated project under a fresh slug once instead.
			renamed, ok := renameProjectForCollision(project, slugifyBrief(brief.Description))
			if !ok {
				return Project{}, report, err
			}
			if err := s.store.Install(ctx, renamed); err != nil {
				return Project{}, report, err
			}
			return renamed, report, nil
		}
	}
	return project, report, nil
}

// List returns the installed-project projections, marking each project the
// checkout applier reports as applied. A failing applied lookup degrades to
// unmarked rather than failing the list.
func (s *Service) List(ctx context.Context) ([]InstalledProject, error) {
	if s.store == nil {
		return nil, ErrProjectNotFound
	}
	projects, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	if s.applier == nil {
		return projects, nil
	}
	applied, err := s.applier.Applied(ctx)
	if err != nil {
		return projects, nil
	}
	marked := make(map[string]struct{}, len(applied))
	for _, name := range applied {
		marked[strings.TrimSpace(name)] = struct{}{}
	}
	for i := range projects {
		if _, ok := marked[projects[i].Name]; ok {
			projects[i].Applied = true
		}
	}
	return projects, nil
}

// Apply stages one installed project into the source checkout. The name
// resolves like Get; a nil applier reads as unavailable, like generation
// without a key.
func (s *Service) Apply(ctx context.Context, name string, replace, confirm bool) (Project, ApplyResult, error) {
	trimmed := strings.TrimSpace(name)
	if err := ValidateProjectName(trimmed); err != nil {
		return Project{}, ApplyResult{}, ErrProjectNotFound
	}
	if s.applier == nil {
		return Project{}, ApplyResult{}, ErrApplyUnavailable
	}
	if s.store == nil {
		return Project{}, ApplyResult{}, ErrProjectNotFound
	}
	project, err := s.store.Get(ctx, trimmed)
	if err != nil {
		return Project{}, ApplyResult{}, err
	}
	// Re-run the validation gate on the staged tree: staged projects are
	// inert disk state, and anything that reached modules-gen/ outside the
	// generation path (hand edits, restored backups) never saw the
	// content scan or the module contract. Refuse before touching the
	// checkout.
	report := s.validator.Validate(ctx, project)
	if scanned := ScanGeneratedProjectFiles(project.Files); len(scanned) > 0 {
		report = NewFindingsReport(append(report.Findings, scanned...))
	}
	if !report.Valid {
		reason := "project failed validation"
		if problems := errorProblems(report); len(problems) > 0 {
			reason = problems[0]
		}
		return Project{}, ApplyResult{}, fmt.Errorf("%w: %s", ErrApplyFailed, reason)
	}
	result, err := s.applier.Apply(ctx, project, replace, confirm)
	if err != nil {
		return Project{}, ApplyResult{}, err
	}
	return project, result, nil
}

// Unapply removes an applied module from the source checkout. Confirmation
// is enforced here so no caller can skip the destructive guard; the applier
// owns the registry-membership matrix.
func (s *Service) Unapply(ctx context.Context, name string, confirm bool) (UnapplyResult, error) {
	trimmed := strings.TrimSpace(name)
	if err := ValidateProjectName(trimmed); err != nil {
		return UnapplyResult{}, ErrProjectNotFound
	}
	if !confirm {
		return UnapplyResult{}, ErrConfirmationRequired
	}
	if s.applier == nil {
		return UnapplyResult{}, ErrApplyUnavailable
	}
	return s.applier.Unapply(ctx, trimmed, confirm)
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

// Delete removes an installed project, reporting whether the module is
// still applied in the checkout: the staged copy and the applied tree have
// independent lifecycles, and deleting the former must warn about the
// latter. The applied lookup degrades to unmarked on failure rather than
// failing the delete.
func (s *Service) Delete(ctx context.Context, name string) (bool, error) {
	if s.store == nil {
		return false, ErrProjectNotFound
	}
	trimmed := strings.TrimSpace(name)
	if err := ValidateProjectName(trimmed); err != nil {
		return false, ErrProjectNotFound
	}
	if err := s.store.Delete(ctx, trimmed); err != nil {
		return false, err
	}
	return s.isApplied(ctx, trimmed), nil
}

// isApplied reports whether name is in the checkout's applied registry,
// degrading to false on any applier failure.
func (s *Service) isApplied(ctx context.Context, name string) bool {
	if s.applier == nil {
		return false
	}
	applied, err := s.applier.Applied(ctx)
	if err != nil {
		return false
	}
	for _, entry := range applied {
		if strings.TrimSpace(entry) == name {
			return true
		}
	}
	return false
}

// Zip builds the download archive for one installed project.
func (s *Service) Zip(ctx context.Context, name string) ([]byte, error) {
	if s.store == nil {
		return nil, ErrProjectNotFound
	}
	return s.store.Zip(ctx, name)
}

// NewService builds the orchestration service. A nil store disables
// persistence (generation still validates); a nil sources func generates
// from the distilled contract alone.
func NewService(
	generation ModuleGenerationService,
	validator ProjectValidator,
	store ProjectStore,
	sources func(ctx context.Context) ModuleSources,
) *Service {
	if validator == nil {
		validator = NewValidator()
	}
	return &Service{
		generation: generation,
		validator:  validator,
		store:      store,
		sources:    sources,
	}
}

// NewServiceWithApplier builds the orchestration service with a checkout
// applier. It is the constructor the composition root uses;
// NewService is the applier-less specialization tests and minimal
// embeddings use.
func NewServiceWithApplier(
	generation ModuleGenerationService,
	validator ProjectValidator,
	store ProjectStore,
	applier Applier,
	sources func(ctx context.Context) ModuleSources,
) *Service {
	svc := NewService(generation, validator, store, sources)
	svc.applier = applier
	return svc
}
