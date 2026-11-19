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
	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/go-chi/chi/v5"
)

const (
	// maxBriefChars caps the generation brief at 5,000 runes.
	maxBriefChars = 5000
	// maxGenerateBodyBytes caps the generate request body at 32 KiB.
	maxGenerateBodyBytes = 32 << 10
	// generateTimeout bounds a plugin AI call at 120 seconds.
	generateTimeout = 120 * time.Second
	// maxConcurrentGenerations bounds server-wide parallel generations.
	maxConcurrentGenerations = 2
	// buildTimeout bounds one server-side plugin build end to end: TinyGo
	// cold compiles can run past a minute, so the budget carries headroom.
	buildTimeout = 5 * time.Minute
	// maxConcurrentBuilds bounds server-wide parallel builds to one: a
	// compiler saturates a core, and builds are rare admin operations.
	maxConcurrentBuilds = 1
)

var (
	// generateSemaphore bounds parallel AI generations server-wide so one
	// curious admin cannot torch the AI quota.
	generateSemaphore = make(chan struct{}, maxConcurrentGenerations)
	// buildSemaphore bounds parallel server-side builds server-wide.
	buildSemaphore = make(chan struct{}, maxConcurrentBuilds)
)

// generateRequest is the JSON body for the generate endpoint.
type generateRequest struct {
	Description  string   `json:"description"`
	Language     string   `json:"language"`
	Hooks        []string `json:"hooks"`
	Capabilities []string `json:"capabilities"`
}

// projectResponse is the JSON projection of a generated project.
type projectResponse struct {
	Name          string            `json:"name"`
	Language      string            `json:"language"`
	Hooks         []string          `json:"hooks"`
	Capabilities  []string          `json:"capabilities"`
	HTTPPatterns  []string          `json:"httpPatterns,omitempty"`
	DBPermissions []string          `json:"dbPermissions,omitempty"`
	Rationale     string            `json:"rationale"`
	Files         map[string]string `json:"files"`
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
	Language  string   `json:"language"`
	Hooks     []string `json:"hooks"`
	CreatedAt string   `json:"createdAt"`
	Source    string   `json:"source"`
}

// discardLogger drops log lines. It backs the nil-logger fallback so the
// handler is safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

func toProjectResponse(project plugingendomain.Project) projectResponse {
	return projectResponse{
		Name:          project.Name,
		Language:      project.Language,
		Hooks:         project.Hooks,
		Capabilities:  project.Capabilities,
		HTTPPatterns:  project.HTTPPatterns,
		DBPermissions: project.DBPermissions,
		Rationale:     project.Rationale,
		Files:         project.Files,
	}
}

func toValidationResponse(report plugingendomain.ValidationReport) validationResponse {
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

func toInstalledProjectResponse(project plugingendomain.InstalledProject) installedProjectResponse {
	return installedProjectResponse{
		Name:      project.Name,
		Language:  project.Language,
		Hooks:     project.Hooks,
		CreatedAt: project.CreatedAt.Format(time.RFC3339),
		Source:    project.Source,
	}
}

func mapPluginError(err error) (int, string, string) {
	switch {
	case errors.Is(err, plugingendomain.ErrInvalidBrief):
		return http.StatusBadRequest, "invalid_brief", "Invalid brief"
	case errors.Is(err, plugingendomain.ErrInvalidHooks):
		return http.StatusBadRequest, "invalid_hooks", "Invalid hooks"
	case errors.Is(err, plugingendomain.ErrInvalidCapabilities):
		return http.StatusBadRequest, "invalid_capabilities", "Invalid capabilities"
	case errors.Is(err, plugingendomain.ErrInvalidProjectName):
		return http.StatusBadRequest, "invalid_project_name", "Invalid project name"
	case errors.Is(err, plugingendomain.ErrPathNotAllowed):
		return http.StatusBadRequest, "invalid_project", "Project contains disallowed paths"
	case errors.Is(err, plugingendomain.ErrSizeLimitExceeded):
		return http.StatusBadRequest, "project_too_large", "Project exceeds size limits"
	case errors.Is(err, plugingendomain.ErrProjectExists):
		return http.StatusConflict, "project_exists", "Project already exists"
	case errors.Is(err, plugingendomain.ErrProjectNotFound):
		return http.StatusNotFound, "project_not_found", "Project not found"
	case errors.Is(err, plugingendomain.ErrPluginInstalled):
		return http.StatusConflict, "plugin_exists", "Plugin is already installed"
	case errors.Is(err, plugingendomain.ErrBuildUnavailable):
		return http.StatusServiceUnavailable, "build_unavailable", "Plugin builds are not configured"
	case errors.Is(err, plugingendomain.ErrBuildFailed):
		return http.StatusInternalServerError, "build_failed", "Failed to build plugin"
	case errors.Is(err, plugingendomain.ErrGenerationUnavailable):
		return http.StatusServiceUnavailable, "generation_unavailable", "Plugin generation is not configured"
	case errors.Is(err, plugingendomain.ErrGenerationFailed):
		return http.StatusInternalServerError, "generation_failed", "Failed to generate plugin"
	default:
		return http.StatusInternalServerError, "generation_failed", "Failed to generate plugin"
	}
}

// PluginService is the generation orchestration seam the handler drives. It
// mirrors the domain Service's persistence-facing methods so mocks stay
// narrow.
type PluginService interface {
	Generate(ctx context.Context, brief plugingendomain.Brief) (plugingendomain.Project, plugingendomain.ValidationReport, error)
	Build(ctx context.Context, name string) (plugingendomain.BuildResult, error)
	List(ctx context.Context) ([]plugingendomain.InstalledProject, error)
	Get(ctx context.Context, name string) (plugingendomain.Project, error)
	Delete(ctx context.Context, name string) error
	Zip(ctx context.Context, name string) ([]byte, error)
}

// PluginStatusReader exposes loaded-plugin inspection for the read-only
// status endpoint. It is satisfied by the edition over the plugin system.
type PluginStatusReader interface {
	LoadedPlugins(ctx context.Context) []PluginStatus
}

// PluginStatus is one loaded plugin for the status projection.
type PluginStatus struct {
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Hooks  []string `json:"hooks"`
}

// PluginGenHandler serves the AI plugin generator surface: generate, list,
// download, delete, and status.
type PluginGenHandler struct {
	service      PluginService
	status       PluginStatusReader
	logger       kernel.Logger
	buildTimeout time.Duration
}

func (h *PluginGenHandler) requirePrincipal(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := kernel.PrincipalFromContext(r.Context()); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return false
	}
	return true
}

// Generate creates a plugin project from a natural-language brief, validates
// it, and installs it as a source project.
func (h *PluginGenHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Plugin generation is not configured", nil)
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
	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = "go"
	}
	brief := plugingendomain.NewBrief(description, language, req.Hooks, req.Capabilities)
	select {
	case generateSemaphore <- struct{}{}:
		defer func() { <-generateSemaphore }()
	default:
		sendErrorResponse(w, http.StatusTooManyRequests, "generation_busy", "Plugin generation is busy", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), generateTimeout)
	defer cancel()
	project, report, err := h.service.Generate(ctx, brief)
	if err != nil {
		status, code, message := mapPluginError(err)
		h.logger.Error("Failed to generate plugin: %v", err)
		sendErrorResponse(w, status, code, message, toValidationResponse(report))
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"project":    toProjectResponse(project),
		"validation": toValidationResponse(report),
	})
}

// List returns the installed plugin projects.
func (h *PluginGenHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Plugin generation is not configured", nil)
		return
	}
	projects, err := h.service.List(r.Context())
	if err != nil {
		status, code, message := mapPluginError(err)
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
func (h *PluginGenHandler) Download(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Plugin generation is not configured", nil)
		return
	}
	name := chi.URLParam(r, "name")
	data, err := h.service.Zip(r.Context(), strings.TrimSpace(name))
	if err != nil {
		status, code, message := mapPluginError(err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+".zip\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Build compiles an installed project to WASM and installs the .wasm plus
// the synthesized manifest into plugins/. In DEV_MODE the watcher
// hot-reloads it; otherwise the response asks for a restart.
func (h *PluginGenHandler) Build(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Plugin generation is not configured", nil)
		return
	}
	select {
	case buildSemaphore <- struct{}{}:
		defer func() { <-buildSemaphore }()
	default:
		sendErrorResponse(w, http.StatusTooManyRequests, "build_busy", "Plugin build is busy", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.buildTimeoutOrDefault())
	defer cancel()
	result, err := h.service.Build(ctx, strings.TrimSpace(chi.URLParam(r, "name")))
	if err != nil {
		status, code, message := mapPluginError(err)
		h.logger.Error("Failed to build plugin: %v", err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"installed":       result.Installed,
		"requiresRestart": result.RequiresRestart,
	})
}

// Delete removes an installed project.
func (h *PluginGenHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Plugin generation is not configured", nil)
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if err := h.service.Delete(r.Context(), name); err != nil {
		status, code, message := mapPluginError(err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"deleted": true,
	})
}

// Status returns loaded-plugin inspection: the .wasm files the host sees.
func (h *PluginGenHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.status == nil {
		sendSuccessResponse(w, http.StatusOK, map[string]any{
			"plugins": []any{},
		})
		return
	}
	plugins := h.status.LoadedPlugins(r.Context())
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"plugins": plugins,
	})
}

func (h *PluginGenHandler) buildTimeoutOrDefault() time.Duration {
	if h.buildTimeout > 0 {
		return h.buildTimeout
	}
	return buildTimeout
}

// NewPluginGenHandler builds the plugin generator handler. A non-positive
// buildTimeout selects the default server-side build budget.
func NewPluginGenHandler(service PluginService, status PluginStatusReader, logger kernel.Logger, buildTimeout time.Duration) *PluginGenHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	return &PluginGenHandler{
		service:      service,
		status:       status,
		logger:       logger,
		buildTimeout: buildTimeout,
	}
}
