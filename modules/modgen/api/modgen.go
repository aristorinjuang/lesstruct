package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/kernel"
	modgendomain "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/go-chi/chi/v5"
)

const (
	// maxBriefChars caps the generation brief at 5,000 runes.
	maxBriefChars = 5000
	// maxGenerateBodyBytes caps the generate request body at 32 KiB.
	maxGenerateBodyBytes = 32 << 10
	// generateTimeout bounds one module generation end to end: the envelope
	// carries an entire module (domain/api/store/admin overlays), and the
	// budget covers up to four model calls — the generation, one re-ask of
	// a malformed response, and two repair attempts. Five minutes matches
	// the CLI and MCP client budgets.
	generateTimeout = 5 * time.Minute
	// applyTimeout bounds one module apply end to end: module tests in both
	// workspaces plus the custom-binary build share it, so the budget
	// carries headroom over the applier's own verification deadline.
	applyTimeout = 15 * time.Minute
	// maxConcurrentGenerations bounds server-wide parallel generations.
	maxConcurrentGenerations = 2
	// maxConcurrentApplies bounds server-wide parallel applies to one: a
	// compiler saturates a core, and applies are rare admin operations that
	// all edit the same shared tree.
	maxConcurrentApplies = 1
)

var (
	// generateSemaphore bounds parallel AI generations server-wide so one
	// curious admin cannot torch the AI quota.
	generateSemaphore = make(chan struct{}, maxConcurrentGenerations)
	// applySemaphore bounds parallel applies server-wide.
	applySemaphore = make(chan struct{}, maxConcurrentApplies)
)

// generateRequest is the JSON body for the generate endpoint.
type generateRequest struct {
	Description string   `json:"description"`
	Surfaces    []string `json:"surfaces"`
}

// projectResponse is the JSON projection of a generated module.
type projectResponse struct {
	Name      string            `json:"name"`
	Surfaces  []string          `json:"surfaces"`
	Rationale string            `json:"rationale"`
	Files     map[string]string `json:"files"`
}

// findingResponse is the JSON projection of a validation finding.
type findingResponse struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// validationResponse is the JSON projection of a validation report.
type validationResponse struct {
	Valid    bool              `json:"valid"`
	Findings []findingResponse `json:"findings"`
}

// installedProjectResponse is the JSON projection of an installed project.
type installedProjectResponse struct {
	Name      string   `json:"name"`
	Surfaces  []string `json:"surfaces"`
	FileCount int      `json:"fileCount"`
	CreatedAt string   `json:"createdAt"`
	Source    string   `json:"source"`
	Applied   bool     `json:"applied"`
}

// discardLogger drops log lines. It backs the nil-logger fallback so the
// handler is safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

func toProjectResponse(project modgendomain.Project) projectResponse {
	return projectResponse{
		Name:      project.Name,
		Surfaces:  project.Surfaces,
		Rationale: project.Rationale,
		Files:     project.Files,
	}
}

func toValidationResponse(report modgendomain.ValidationReport) validationResponse {
	findings := make([]findingResponse, 0, len(report.Findings))
	for _, finding := range report.Findings {
		findings = append(findings, findingResponse{
			Rule:     finding.Rule,
			Severity: finding.Severity,
			Message:  finding.Message,
		})
	}
	return validationResponse{
		Valid:    report.Valid,
		Findings: findings,
	}
}

func toInstalledProjectResponse(project modgendomain.InstalledProject) installedProjectResponse {
	return installedProjectResponse{
		Name:      project.Name,
		Surfaces:  project.Surfaces,
		FileCount: project.FileCount,
		CreatedAt: project.CreatedAt.Format(time.RFC3339),
		Source:    project.Source,
		Applied:   project.Applied,
	}
}

// applyStepResponse is the JSON projection of one verification step.
type applyStepResponse struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Skipped bool   `json:"skipped"`
	Output  string `json:"output,omitempty"`
}

// applyResultResponse is the JSON projection of an apply outcome.
type applyResultResponse struct {
	Applied         bool                `json:"applied"`
	Verified        bool                `json:"verified"`
	Steps           []applyStepResponse `json:"steps"`
	AppliedModules  []string            `json:"appliedModules"`
	RequiresRestart bool                `json:"requiresRestart"`
}

func toApplyResultResponse(result modgendomain.ApplyResult) applyResultResponse {
	steps := make([]applyStepResponse, 0, len(result.Steps))
	for _, step := range result.Steps {
		steps = append(steps, applyStepResponse{
			Name:    step.Name,
			Passed:  step.Passed,
			Skipped: step.Skipped,
			Output:  step.Output,
		})
	}
	return applyResultResponse{
		Applied:         result.Applied,
		Verified:        result.Verified,
		Steps:           steps,
		AppliedModules:  result.AppliedModules,
		RequiresRestart: result.RequiresRestart,
	}
}

// unapplyResultResponse is the JSON projection of an unapply outcome.
type unapplyResultResponse struct {
	Removed          bool                `json:"removed"`
	Verified         bool                `json:"verified"`
	Steps            []applyStepResponse `json:"steps"`
	AppliedModules   []string            `json:"appliedModules"`
	ModuleDirRemoved bool                `json:"moduleDirRemoved"`
}

func toUnapplyResultResponse(result modgendomain.UnapplyResult) unapplyResultResponse {
	steps := make([]applyStepResponse, 0, len(result.Steps))
	for _, step := range result.Steps {
		steps = append(steps, applyStepResponse{
			Name:    step.Name,
			Passed:  step.Passed,
			Skipped: step.Skipped,
			Output:  step.Output,
		})
	}
	return unapplyResultResponse{
		Removed:          result.Removed,
		Verified:         result.Verified,
		Steps:            steps,
		AppliedModules:   result.AppliedModules,
		ModuleDirRemoved: result.ModuleDirRemoved,
	}
}

func mapModuleError(err error) (int, string, string) {
	switch {
	case errors.Is(err, modgendomain.ErrInvalidBrief):
		return http.StatusBadRequest, "invalid_brief", "Invalid brief"
	case errors.Is(err, modgendomain.ErrInvalidSurfaces):
		return http.StatusBadRequest, "invalid_surfaces", "Invalid surfaces"
	case errors.Is(err, modgendomain.ErrInvalidProjectName):
		return http.StatusBadRequest, "invalid_module_name", "Invalid module name"
	case errors.Is(err, modgendomain.ErrPathNotAllowed):
		return http.StatusBadRequest, "invalid_module", "Module contains disallowed paths"
	case errors.Is(err, modgendomain.ErrSizeLimitExceeded):
		return http.StatusBadRequest, "module_too_large", "Module exceeds size limits"
	case errors.Is(err, modgendomain.ErrProjectExists):
		return http.StatusConflict, "module_exists", "Module project already exists"
	case errors.Is(err, modgendomain.ErrProjectNotFound):
		return http.StatusNotFound, "module_not_found", "Module project not found"
	case errors.Is(err, modgendomain.ErrGenerationUnavailable):
		return http.StatusServiceUnavailable, "generation_unavailable", "Module generation is not configured"
	case errors.Is(err, modgendomain.ErrGenerationFailed):
		return http.StatusInternalServerError, "generation_failed", "Failed to generate module"
	case errors.Is(err, modgendomain.ErrApplyUnavailable):
		return http.StatusServiceUnavailable, "apply_unavailable", "Module apply is not configured"
	case errors.Is(err, modgendomain.ErrNotCheckout):
		return http.StatusServiceUnavailable, "apply_not_checkout", "Server does not run from a source checkout"
	case errors.Is(err, modgendomain.ErrNotApplied):
		return http.StatusNotFound, "module_not_applied", "Module is not applied in the checkout"
	case errors.Is(err, modgendomain.ErrConfirmationRequired):
		return http.StatusBadRequest, "confirmation_required", "Confirmation is required"
	case errors.Is(err, modgendomain.ErrAlreadyApplied):
		return http.StatusConflict, "module_applied", "Module is already applied"
	case errors.Is(err, modgendomain.ErrTreeDirty):
		return http.StatusConflict, "tree_dirty", "Checkout tree carries uncommitted changes"
	case errors.Is(err, modgendomain.ErrApplyFailed):
		return http.StatusInternalServerError, "apply_failed", "Failed to apply module"
	default:
		return http.StatusInternalServerError, "generation_failed", "Failed to generate module"
	}
}

// ModuleService is the generation orchestration seam the handler drives. It
// mirrors the domain Service's persistence-facing methods so mocks stay
// narrow.
type ModuleService interface {
	Generate(ctx context.Context, brief modgendomain.Brief) (modgendomain.Project, modgendomain.ValidationReport, error)
	Apply(ctx context.Context, name string, replace, confirm bool) (modgendomain.Project, modgendomain.ApplyResult, error)
	Unapply(ctx context.Context, name string, confirm bool) (modgendomain.UnapplyResult, error)
	List(ctx context.Context) ([]modgendomain.InstalledProject, error)
	Get(ctx context.Context, name string) (modgendomain.Project, error)
	Delete(ctx context.Context, name string) (bool, error)
	Zip(ctx context.Context, name string) ([]byte, error)
}

// applyRequest is the optional JSON body for the apply endpoint: replace
// with confirm restages over an applied tree the registry owns.
type applyRequest struct {
	Replace bool `json:"replace"`
	Confirm bool `json:"confirm"`
}

// unapplyRequest is the JSON body for the unapply endpoint: removal of an
// applied module tree always requires explicit confirmation.
type unapplyRequest struct {
	Confirm bool `json:"confirm"`
}

// ModuleGenHandler serves the AI module generator surface: generate,
// apply, unapply, list, download, and delete.
type ModuleGenHandler struct {
	service      ModuleService
	logger       kernel.Logger
	applyTimeout time.Duration
}

func (h *ModuleGenHandler) requirePrincipal(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := kernel.PrincipalFromContext(r.Context()); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return false
	}
	return true
}

// Generate creates a module project from a natural-language brief, validates
// it, and installs it as an inert source project.
func (h *ModuleGenHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Module generation is not configured", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_brief", "Description is required", nil)
		return
	}
	if utf8.RuneCountInString(description) > maxBriefChars {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_brief", "Description is too long", nil)
		return
	}
	brief := modgendomain.NewBrief(description, req.Surfaces)
	select {
	case generateSemaphore <- struct{}{}:
		defer func() { <-generateSemaphore }()
	default:
		sendErrorResponse(w, http.StatusTooManyRequests, "generation_busy", "Module generation is busy", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), generateTimeout)
	defer cancel()
	project, report, err := h.service.Generate(ctx, brief)
	if err != nil {
		status, code, message := mapModuleError(err)
		h.logger.Error("Failed to generate module: %v", err)
		sendErrorResponse(w, status, code, message, toValidationResponse(report))
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"project":    toProjectResponse(project),
		"validation": toValidationResponse(report),
	})
}

// List returns the installed module projects.
// Apply stages an installed module project into the source checkout:
// tree, registries, regenerated custom edition, and verification. The
// handler serializes applies server-wide and bounds the whole call; the
// applier's own verification deadline applies inside.
func (h *ModuleGenHandler) Apply(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Module generation is not configured", nil)
		return
	}
	select {
	case applySemaphore <- struct{}{}:
		defer func() { <-applySemaphore }()
	default:
		sendErrorResponse(w, http.StatusTooManyRequests, "apply_busy", "Module apply is busy", nil)
		return
	}
	var req applyRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.applyTimeoutOrDefault())
	defer cancel()
	project, result, err := h.service.Apply(ctx, strings.TrimSpace(chi.URLParam(r, "name")), req.Replace, req.Confirm)
	if err != nil {
		status, code, message := mapModuleError(err)
		h.logger.Error("Failed to apply module: %v", err)
		sendErrorResponse(w, status, code, message, toApplyResultResponse(result))
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"project": toProjectResponse(project),
		"apply":   toApplyResultResponse(result),
	})
}

// Unapply removes an applied module from the source checkout: registry,
// custom edition, and tree. Without explicit confirmation the call is
// refused before touching anything.
func (h *ModuleGenHandler) Unapply(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Module generation is not configured", nil)
		return
	}
	var req unapplyRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	select {
	case applySemaphore <- struct{}{}:
		defer func() { <-applySemaphore }()
	default:
		sendErrorResponse(w, http.StatusTooManyRequests, "apply_busy", "Module apply is busy", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.applyTimeoutOrDefault())
	defer cancel()
	result, err := h.service.Unapply(ctx, strings.TrimSpace(chi.URLParam(r, "name")), req.Confirm)
	if err != nil {
		status, code, message := mapModuleError(err)
		h.logger.Error("Failed to unapply module: %v", err)
		sendErrorResponse(w, status, code, message, toUnapplyResultResponse(result))
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"unapply": toUnapplyResultResponse(result),
	})
}

// List returns the installed module projects.
func (h *ModuleGenHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Module generation is not configured", nil)
		return
	}
	projects, err := h.service.List(r.Context())
	if err != nil {
		status, code, message := mapModuleError(err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	items := make([]installedProjectResponse, 0, len(projects))
	for _, project := range projects {
		items = append(items, toInstalledProjectResponse(project))
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"projects": items,
	})
}

// Download streams the project zip.
func (h *ModuleGenHandler) Download(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Module generation is not configured", nil)
		return
	}
	name := chi.URLParam(r, "name")
	data, err := h.service.Zip(r.Context(), strings.TrimSpace(name))
	if err != nil {
		status, code, message := mapModuleError(err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+".zip\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Delete removes an installed project. The response reports whether the
// module stays applied in the checkout: staged and applied have independent
// lifecycles, so deleting the former must warn about the latter.
func (h *ModuleGenHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Module generation is not configured", nil)
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	applied, err := h.service.Delete(r.Context(), name)
	if err != nil {
		status, code, message := mapModuleError(err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"deleted":           true,
		"appliedInCheckout": applied,
	})
}

func (h *ModuleGenHandler) applyTimeoutOrDefault() time.Duration {
	if h.applyTimeout > 0 {
		return h.applyTimeout
	}
	return applyTimeout
}

// NewModuleGenHandler builds the module generator handler. A non-positive
// applyTimeout selects the default server-side apply budget.
func NewModuleGenHandler(service ModuleService, logger kernel.Logger, applyTimeout time.Duration) *ModuleGenHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	return &ModuleGenHandler{
		service:      service,
		logger:       logger,
		applyTimeout: applyTimeout,
	}
}
