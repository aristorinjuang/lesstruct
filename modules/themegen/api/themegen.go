package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/kernel"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
	"github.com/go-chi/chi/v5"
)

const (
	// maxBriefChars caps the generation brief at 5,000 runes.
	maxBriefChars = 5000
	// maxGenerateBodyBytes caps the generate request body at 32 KiB.
	maxGenerateBodyBytes = 32 << 10
	// generateTimeout bounds a stylesheet-only AI call at 120 seconds.
	generateTimeout = 120 * time.Second
	// generateTemplateTimeout bounds a template-override AI call at 180
	// seconds: templates are larger, and the repair loop may run twice.
	generateTemplateTimeout = 180 * time.Second
	// maxConcurrentGenerations bounds server-wide parallel generations.
	maxConcurrentGenerations = 2

	// snapshotCacheControl bounds snapshot freshness: installed theme files
	// carry no content hash, so a short private TTL keeps a re-installed
	// theme from serving a stale frame.
	snapshotCacheControl = "private, max-age=300"

	// snapshotFrameOptions lets the admin panel frame the snapshot. The root
	// security middleware stamps every response DENY, so the handler must
	// override it — and the inherited frame-ancestors 'none' — before the
	// headers are written.
	snapshotFrameOptions = "SAMEORIGIN"

	// snapshotCSP replaces the global content policy for the snapshot document.
	// frame-ancestors 'self' is the actual unlock (the admin panel is
	// same-origin); script-src 'none' keeps a model-injected script in a
	// generated template from ever executing; style-src preserves the CDN
	// stylesheets the embedded layouts carry.
	snapshotCSP = "default-src 'none'; style-src 'unsafe-inline' https://cdn.jsdelivr.net; " +
		"img-src 'self' data:; font-src 'self'; frame-ancestors 'self'; " +
		"base-uri 'none'; form-action 'none'; script-src 'none'"

	// snapshotPlaceholder is the degraded snapshot document: a visual
	// affordance must never break the themes listing, so an unknown or
	// broken theme renders this instead of an error.
	snapshotPlaceholder = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Snapshot unavailable</title></head>
<body><p>Theme snapshot unavailable.</p></body>
</html>`
)

var (
	// generateSemaphore bounds parallel AI generations server-wide so one
	// curious admin cannot torch the AI quota.
	generateSemaphore = make(chan struct{}, maxConcurrentGenerations)
)

// generateRequest is the JSON body for the generate endpoint.
type generateRequest struct {
	Description       string `json:"description"`
	Style             string `json:"style"`
	TemplateOverrides bool   `json:"templateOverrides"`
	Edition           string `json:"edition"`
}

// themeResponse is the JSON projection of a generated theme.
type themeResponse struct {
	Name      string            `json:"name"`
	Edition   string            `json:"edition"`
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

// installedThemeResponse is the JSON projection of an installed theme.
type installedThemeResponse struct {
	Name      string `json:"name"`
	Edition   string `json:"edition"`
	CreatedAt string `json:"createdAt"`
	Active    bool   `json:"active"`
	Source    string `json:"source"`
}

// discardLogger drops log lines. It backs the nil-logger fallback so the
// handler is safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

func toThemeResponse(theme themegendomain.Theme) themeResponse {
	return themeResponse{
		Name:      theme.Name,
		Edition:   string(theme.Edition),
		Rationale: theme.Rationale,
		Files:     theme.Files,
	}
}

func toValidationResponse(report themegendomain.ValidationReport) validationResponse {
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

func toInstalledThemeResponse(theme themegendomain.InstalledTheme) installedThemeResponse {
	return installedThemeResponse{
		Name:      theme.Name,
		Edition:   string(theme.Edition),
		CreatedAt: theme.CreatedAt.Format(time.RFC3339),
		Active:    theme.Active,
		Source:    theme.Source,
	}
}

func mapThemeError(err error) (int, string, string) {
	switch {
	case errors.Is(err, themegendomain.ErrInvalidBrief):
		return http.StatusBadRequest, "invalid_brief", "Invalid brief"
	case errors.Is(err, themegendomain.ErrInvalidThemeName):
		return http.StatusBadRequest, "invalid_theme_name", "Invalid theme name"
	case errors.Is(err, themegendomain.ErrPathNotAllowed):
		return http.StatusBadRequest, "invalid_theme", "Theme contains disallowed paths"
	case errors.Is(err, themegendomain.ErrSizeLimitExceeded):
		return http.StatusBadRequest, "theme_too_large", "Theme exceeds size limits"
	case errors.Is(err, themegendomain.ErrThemeExists):
		return http.StatusConflict, "theme_exists", "Theme already exists"
	case errors.Is(err, themegendomain.ErrThemeNotFound):
		return http.StatusNotFound, "theme_not_found", "Theme not found"
	case errors.Is(err, themegendomain.ErrThemeActive):
		return http.StatusConflict, "theme_active", "Theme is active"
	case errors.Is(err, themegendomain.ErrActivationFailed):
		return http.StatusInternalServerError, "activation_failed", "Failed to activate theme"
	case errors.Is(err, themegendomain.ErrGenerationUnavailable):
		return http.StatusServiceUnavailable, "generation_unavailable", "Theme generation is not configured"
	case errors.Is(err, themegendomain.ErrGenerationFailed):
		return http.StatusInternalServerError, "generation_failed", "Failed to generate theme"
	default:
		return http.StatusInternalServerError, "generation_failed", "Failed to generate theme"
	}
}

// ThemeService is the generation orchestration seam the handler drives. It
// mirrors the domain Service's persistence-facing methods so mocks stay
// narrow.
type ThemeService interface {
	Generate(ctx context.Context, brief themegendomain.Brief, edition themegendomain.Edition) (themegendomain.Theme, themegendomain.ValidationReport, error)
	Validate(ctx context.Context, theme themegendomain.Theme) themegendomain.ValidationReport
	List(ctx context.Context) ([]themegendomain.InstalledTheme, error)
	Get(ctx context.Context, name string) (themegendomain.Theme, error)
	Delete(ctx context.Context, name string) error
	Zip(ctx context.Context, name string) ([]byte, error)
}

// ThemeActivationStore persists the active installed theme: the [theme]
// active value the edition composes over the embedded default.
type ThemeActivationStore interface {
	ActiveTheme(ctx context.Context) string
	SaveActive(ctx context.Context, name string) error
	ClearActive(ctx context.Context) error
}

// ThemeReloader swaps the live content-site surfaces to the active theme
// without a restart. It is satisfied by the edition; a nil reloader keeps
// the restart-hint behavior.
type ThemeReloader interface {
	Reload(ctx context.Context) error
}

// ThemeSnapshotter renders an installed theme's homepage to a standalone
// HTML document for admin snapshots. It is satisfied by the edition; a nil
// snapshotter keeps the snapshot endpoint unavailable.
type ThemeSnapshotter interface {
	SnapshotTheme(name string) (string, error)
}

// ThemGenHandler serves the AI theme generator surface: generate, list,
// download, activate, deactivate, delete, preview, preview status, and
// snapshot.
type ThemGenHandler struct {
	service        ThemeService
	activations    ThemeActivationStore
	reloader       ThemeReloader
	previewKey     []byte
	previewSecure  bool
	snapshotter    ThemeSnapshotter
	logger         kernel.Logger
	defaultEdition themegendomain.Edition
}

func (h *ThemGenHandler) requirePrincipal(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := kernel.PrincipalFromContext(r.Context()); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return false
	}
	return true
}

// restoreActive writes name back as the active theme, clearing to the
// embedded default when name is empty. It backs the transactional revert
// when a reload fails after the config write.
func (h *ThemGenHandler) restoreActive(ctx context.Context, name string) error {
	if name == "" {
		return h.activations.ClearActive(ctx)
	}
	return h.activations.SaveActive(ctx, name)
}

// SetPreviewKey installs the cookie-signing key for the preview endpoints.
// Without it Preview answers 503; StopPreview still clears the cookie.
func (h *ThemGenHandler) SetPreviewKey(key []byte, secure bool) {
	h.previewKey = key
	h.previewSecure = secure
}

// SetSnapshotRenderer installs the edition's snapshot closure for the
// snapshot endpoint. Without it Snapshot answers 503.
func (h *ThemGenHandler) SetSnapshotRenderer(snapshotter ThemeSnapshotter) {
	h.snapshotter = snapshotter
}

// Generate creates a theme from a natural-language brief, validates it, and
// auto-installs it as inactive.
func (h *ThemGenHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Theme generation is not configured", nil)
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
	edition := h.defaultEdition
	if trimmed := themegendomain.Edition(strings.TrimSpace(req.Edition)); trimmed != "" {
		edition = trimmed
	}
	brief := themegendomain.NewBrief(description, strings.TrimSpace(req.Style), req.TemplateOverrides)
	timeout := generateTimeout
	if brief.TemplateOverrides {
		timeout = generateTemplateTimeout
	}
	select {
	case generateSemaphore <- struct{}{}:
		defer func() { <-generateSemaphore }()
	default:
		sendErrorResponse(w, http.StatusTooManyRequests, "generation_busy", "Theme generation is busy", nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), timeout)
	defer cancel()
	theme, report, err := h.service.Generate(ctx, brief, edition)
	if err != nil {
		status, code, message := mapThemeError(err)
		h.logger.Error("Failed to generate theme: %v", err)
		sendErrorResponse(w, status, code, message, toValidationResponse(report))
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"theme":      toThemeResponse(theme),
		"validation": toValidationResponse(report),
	})
}

// List returns the installed themes with active markers.
func (h *ThemGenHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Theme generation is not configured", nil)
		return
	}
	themes, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("Failed to list themes: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to list themes", nil)
		return
	}
	items := make([]installedThemeResponse, 0, len(themes))
	for _, theme := range themes {
		items = append(items, toInstalledThemeResponse(theme))
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{"themes": items})
}

// Download streams the theme zip with a THEME_DIR install README.
func (h *ThemGenHandler) Download(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Theme generation is not configured", nil)
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_theme_name", "Theme name is required", nil)
		return
	}
	payload, err := h.service.Zip(r.Context(), name)
	if err != nil {
		status, code, message := mapThemeError(err)
		h.logger.Error("Failed to download theme: %v", err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+".zip\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

// Activate validates the theme, writes [theme] active, and hot-reloads the
// content-site surfaces when a reloader is attached (restart to apply
// otherwise). A reload failure reverts the config write and reports an
// error so activation stays transactional.
func (h *ThemGenHandler) Activate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Theme generation is not configured", nil)
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_theme_name", "Theme name is required", nil)
		return
	}
	if h.activations == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "activation_failed", "Theme activation is not configured", nil)
		return
	}
	prev := h.activations.ActiveTheme(r.Context())
	theme, err := h.service.Get(r.Context(), name)
	if err != nil {
		status, code, message := mapThemeError(err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	report := h.service.Validate(r.Context(), theme)
	if !report.Valid {
		h.logger.Error("Theme activation refused by validation: %s", name)
		sendErrorResponse(w, http.StatusBadRequest, "activation_failed", "Theme failed validation", toValidationResponse(report))
		return
	}
	if err := h.activations.SaveActive(r.Context(), name); err != nil {
		h.logger.Error("Failed to activate theme: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "activation_failed", "Failed to activate theme", nil)
		return
	}
	requiresRestart := true
	if h.reloader != nil {
		if err := h.reloader.Reload(r.Context()); err != nil {
			h.logger.Error("Failed to reload theme after activation: %v", err)
			if rerr := h.restoreActive(r.Context(), prev); rerr != nil {
				h.logger.Error("Failed to revert theme activation: %v", rerr)
			}
			sendErrorResponse(w, http.StatusInternalServerError, "activation_failed", "Failed to activate theme", nil)
			return
		}
		requiresRestart = false
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{"activated": true, "requiresRestart": requiresRestart})
}

// Deactivate clears [theme] active back to the embedded default and
// hot-reloads the content-site surfaces when a reloader is attached. A
// reload failure restores the previous value and reports an error.
func (h *ThemGenHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.activations == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "activation_failed", "Theme activation is not configured", nil)
		return
	}
	prev := h.activations.ActiveTheme(r.Context())
	if err := h.activations.ClearActive(r.Context()); err != nil {
		h.logger.Error("Failed to deactivate theme: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "activation_failed", "Failed to deactivate theme", nil)
		return
	}
	requiresRestart := true
	if h.reloader != nil {
		if err := h.reloader.Reload(r.Context()); err != nil {
			h.logger.Error("Failed to reload theme after deactivation: %v", err)
			if rerr := h.restoreActive(r.Context(), prev); rerr != nil {
				h.logger.Error("Failed to revert theme deactivation: %v", rerr)
			}
			sendErrorResponse(w, http.StatusInternalServerError, "activation_failed", "Failed to deactivate theme", nil)
			return
		}
		requiresRestart = false
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{"activated": false, "requiresRestart": requiresRestart})
}

// Delete removes an installed theme (409 while active).
func (h *ThemGenHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "generation_unavailable", "Theme generation is not configured", nil)
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_theme_name", "Theme name is required", nil)
		return
	}
	if err := h.service.Delete(r.Context(), name); err != nil {
		status, code, message := mapThemeError(err)
		h.logger.Error("Failed to delete theme: %v", err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	// A deleted theme leaves a dangling preview cookie behind: clear it when
	// it names the theme just removed so the browser falls back immediately
	// instead of idling until the cookie's TTL.
	h.clearPreviewForTheme(w, r, name)
	sendSuccessResponse(w, http.StatusOK, map[string]any{"deleted": true})
}

// clearPreviewForTheme clears the preview cookie when it carries a valid
// signature for name. Forged, malformed, or other-theme cookies are left
// alone; without a key nothing is verifiable and nothing is cleared.
func (h *ThemGenHandler) clearPreviewForTheme(w http.ResponseWriter, r *http.Request, name string) {
	if h.previewKey == nil {
		return
	}
	cookie, err := r.Cookie(PreviewCookieName)
	if err != nil {
		return
	}
	previewed, ok := VerifyPreviewCookie(cookie.Value, h.previewKey)
	if !ok || previewed != name {
		return
	}
	ClearPreviewCookie(w, h.previewSecure)
}

// Preview validates the theme and sets a short-lived signed preview cookie
// so the content site renders that theme for this browser only. It changes
// nothing server-side: activation still applies the theme to everyone.
func (h *ThemGenHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.service == nil || h.previewKey == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "preview_unavailable", "Theme preview is not configured", nil)
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_theme_name", "Theme name is required", nil)
		return
	}
	theme, err := h.service.Get(r.Context(), name)
	if err != nil {
		status, code, message := mapThemeError(err)
		sendErrorResponse(w, status, code, message, nil)
		return
	}
	report := h.service.Validate(r.Context(), theme)
	if !report.Valid {
		h.logger.Error("Theme preview refused by validation: %s", name)
		sendErrorResponse(w, http.StatusBadRequest, "activation_failed", "Theme failed validation", toValidationResponse(report))
		return
	}
	value, expiresAt := SignPreviewCookie(name, h.previewKey)
	SetPreviewCookie(w, value, expiresAt, h.previewSecure)
	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"preview":   true,
		"expiresAt": expiresAt.Format(time.RFC3339),
	})
}

// StopPreview clears the preview cookie so the content site renders the
// active theme again.
func (h *ThemGenHandler) StopPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	ClearPreviewCookie(w, h.previewSecure)
	sendSuccessResponse(w, http.StatusOK, map[string]any{"preview": false})
}

// PreviewStatus reports whether this browser holds a valid preview cookie.
// The UI polls it on mount so the stop-preview affordance only renders while
// a preview is actually active; without a key every cookie is unverifiable
// and the answer is always false.
func (h *ThemGenHandler) PreviewStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.previewKey == nil {
		sendSuccessResponse(w, http.StatusOK, map[string]any{"preview": false})
		return
	}
	cookie, err := r.Cookie(PreviewCookieName)
	if err != nil {
		sendSuccessResponse(w, http.StatusOK, map[string]any{"preview": false})
		return
	}
	name, ok := VerifyPreviewCookie(cookie.Value, h.previewKey)
	if !ok {
		sendSuccessResponse(w, http.StatusOK, map[string]any{"preview": false})
		return
	}
	sendSuccessResponse(w, http.StatusOK, map[string]any{"preview": true, "theme": name})
}

// Snapshot renders the installed theme's homepage to a standalone HTML
// document for the admin panel's snapshot frames. Unknown or broken themes
// degrade to a placeholder document; without a renderer the endpoint is
// unavailable.
func (h *ThemGenHandler) Snapshot(w http.ResponseWriter, r *http.Request) {
	if !h.requirePrincipal(w, r) {
		return
	}
	if h.snapshotter == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "snapshot_unavailable", "Theme snapshots are not configured", nil)
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_theme_name", "Theme name is required", nil)
		return
	}
	doc, err := h.snapshotter.SnapshotTheme(name)
	if err != nil {
		h.logger.Error("Failed to render theme snapshot: %v", err)
		doc = snapshotPlaceholder
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", snapshotCacheControl)
	w.Header().Set("X-Frame-Options", snapshotFrameOptions)
	// The override is mode-independent: in report-only deployments the
	// middleware's report header carries the same frame-ancestors 'none',
	// so it goes and the enforcing snapshot policy stands alone.
	w.Header().Del("Content-Security-Policy-Report-Only")
	w.Header().Set("Content-Security-Policy", snapshotCSP)
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, doc)
}

// NewThemGenHandler builds the theme generator handler. A nil logger
// degrades to a discard sink so the handler is safe in any context. A nil
// reloader keeps the restart-hint activation behavior.
func NewThemGenHandler(
	service ThemeService,
	activations ThemeActivationStore,
	logger kernel.Logger,
	defaultEdition themegendomain.Edition,
	reloader ThemeReloader,
) *ThemGenHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	if defaultEdition == "" {
		defaultEdition = themegendomain.EditionBlog
	}
	return &ThemGenHandler{
		service:        service,
		activations:    activations,
		reloader:       reloader,
		logger:         logger,
		defaultEdition: defaultEdition,
	}
}
