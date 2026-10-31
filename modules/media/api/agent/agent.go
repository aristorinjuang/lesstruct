package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
)

// adminRoleName is the reserved administrator role. It matches the users
// module's reserved name; the two reconcile when that module lands.
const adminRoleName = "Admin"

// authenticatedUserID reads the owning user id from the kernel principal the
// Bearer middleware injects. It returns ok=false when no auth middleware ran
// or the id is not a decimal int — a defensive case the handler maps to
// 401 UNAUTHORIZED.
func authenticatedUserID(r *http.Request) (int, bool) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		return 0, false
	}
	userID, err := strconv.Atoi(principal.ID)
	if err != nil {
		return 0, false
	}
	return userID, true
}

// authenticatedRole reads the caller's first role from the kernel principal.
// It returns "" when absent (a defensive case — the Bearer middleware always
// sets identity and role together). "" never equals the admin name, so a
// caller that is neither owner nor admin is treated as unauthorized.
func authenticatedRole(r *http.Request) string {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok || len(principal.Roles) == 0 {
		return ""
	}
	return principal.Roles[0]
}

// handleMediaError maps a media-domain error to the agent API envelope using UPPER_SNAKE
// codes. It mirrors the content handleError structure but is media-specific: it emits the
// agent error catalog via response.Error (never the legacy lowercase admin codes), and maps
// a *mediadomain.DuplicateMediaError to 409 CONFLICT (the correct REST status for a
// duplicate resource). Auth-path errors (UNAUTHORIZED/INVALID_API_KEY/...) are emitted by
// the middleware, not here.
func handleMediaError(w http.ResponseWriter, err error) {
	if _, ok := errors.AsType[*mediadomain.DuplicateMediaError](err); ok {
		httpx.Error(w, http.StatusConflict, "CONFLICT", "media already exists", nil)
		return
	}

	switch {
	case errors.Is(err, mediadomain.ErrMediaNotFound):
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Media not found", nil)
	case errors.Is(err, mediadomain.ErrUnauthorized):
		httpx.Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this media", nil)
	case errors.Is(err, mediadomain.ErrInvalidFile),
		errors.Is(err, mediadomain.ErrInvalidMimeType),
		errors.Is(err, mediadomain.ErrInvalidFileContent),
		errors.Is(err, mediadomain.ErrFileTooLarge),
		errors.Is(err, mediadomain.ErrInvalidAltText):
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	default:
		httpx.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// RoleCapabilities is the narrow role-registry seam this handler depends on.
// Core satisfies it with an adapter over *role.Service (which cannot be
// imported here); the users module implements it natively when it lands.
type RoleCapabilities interface {
	CanMedia(name string) bool
}

// discardLogger drops log lines. It backs the nil-logger fallback in
// NewMediaHandler so the handler is safe to construct in any context.
type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// MediaService is the narrow slice of the media domain service the agent handlers depend
// on. *mediadomain.Service satisfies it. It is exported so mockery can generate an exported
// constructor callable from the package's *_test files (CLAUDE.md mandates *_test packages,
// which cannot reach an unexported mock).
type MediaService interface {
	Upload(ctx context.Context, req mediadomain.UploadRequest) (*mediadomain.Media, error)
	GetByID(ctx context.Context, id int) (*mediadomain.Media, error)
	ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*mediadomain.Media, error)
	Delete(ctx context.Context, id int, principal kernel.Principal) error
}

// MediaHandlerOption configures a MediaHandler. A role service makes the upload
// capability config-driven; without one every authenticated user may upload
// (the legacy behavior).
type MediaHandlerOption func(*MediaHandler)

// WithMediaRoleService attaches the config-driven role registry so uploads are gated
// by the caller's CanMedia capability.
func WithMediaRoleService(rs RoleCapabilities) MediaHandlerOption {
	return func(h *MediaHandler) {
		h.roleService = rs
	}
}

// MediaHandler exposes the Bearer-authenticated agent media endpoints. It reuses the
// existing MediaService (which already validates files, hashes, dedups, converts to WebP,
// and builds thumbnail variants) — it never duplicates that logic.
type MediaHandler struct {
	mediaService MediaService
	logger       kernel.Logger
	roleService  RoleCapabilities
}

// uploadAllowed reports whether the caller's role may upload media. Without a
// role service every authenticated user may, matching the legacy behavior.
func (h *MediaHandler) uploadAllowed(r *http.Request) bool {
	if h.roleService == nil {
		return true
	}
	return h.roleService.CanMedia(authenticatedRole(r))
}

// Upload handles POST /api/v1/media. It parses a required `file` part and an optional JSON
// `metadata` part (carrying altText) from multipart/form-data, maps them onto a
// mediadomain.UploadRequest, and delegates to the existing MediaService.Upload. A missing
// `file` part returns 400 VALIDATION_ERROR; the service's own validation (mime/size/alt
// text) and dedup flow through handleMediaError.
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	if err := r.ParseMultipartForm(mediadomain.MaxFileSize); err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to parse multipart form", nil)
		return
	}

	if !h.uploadAllowed(r) {
		httpx.Error(w, http.StatusForbidden, "FORBIDDEN", "Your role cannot manage media", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "file part is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	// The optional JSON `metadata` part carries altText (required by the service for
	// accessibility). An absent/unparseable metadata part leaves altText "" → the service
	// returns ErrInvalidAltText → VALIDATION_ERROR.
	altText := ""
	if meta := r.FormValue("metadata"); meta != "" {
		var md MediaMetadata
		if err := json.Unmarshal([]byte(meta), &md); err == nil {
			altText = md.AltText
		}
	}

	req := mediadomain.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     userID,
		AltText:    altText,
	}

	uploaded, err := h.mediaService.Upload(r.Context(), req)
	if err != nil {
		h.logger.Error("agent upload media failed: userID=%d err=%v", userID, err)
		handleMediaError(w, err)
		return
	}

	httpx.Success(w, NewMediaResponse(uploaded))
}

// Get handles GET /api/v1/media/{id}. It returns the media in the envelope, or 404
// NOT_FOUND when it does not exist OR the caller is not allowed to see it (ownership
// scoping: only the owner or an Admin may read a given item — existence is never disclosed,
// consistent with the content visibility model).
func (h *MediaHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid media ID", nil)
		return
	}

	mediaItem, err := h.mediaService.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("agent get media failed: id=%d err=%v", id, err)
		handleMediaError(w, err)
		return
	}

	if mediaItem.UserID != userID && role != adminRoleName {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "Media not found", nil)
		return
	}

	httpx.Success(w, NewMediaResponse(mediaItem))
}

// List handles GET /api/v1/media. It returns the caller's own media in newest-first order
// using opaque keyset (cursor) pagination — the SAME contract as the content list. It
// reuses the agent package's cursor helpers and the response package's SuccessList/
// Pagination/ListMeta types (introduced for this reuse in Story 2.2).
func (h *MediaHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	limit := httpx.ParseListLimit(r)
	beforeID, err := httpx.DecodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid cursor", nil)
		return
	}

	items, err := h.mediaService.ListByCursor(r.Context(), userID, limit+1, beforeID)
	if err != nil {
		h.logger.Error("agent list media failed: userID=%d err=%v", userID, err)
		handleMediaError(w, err)
		return
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextCursor := ""
	if hasMore && len(items) > 0 {
		nextCursor = httpx.EncodeCursor(items[len(items)-1].ID)
	}

	projections := make([]MediaProjection, 0, len(items))
	for _, m := range items {
		projections = append(projections, NewMediaResponse(m).Media)
	}

	httpx.SuccessList(
		w,
		projections,
		httpx.ListMeta{Pagination: httpx.Pagination{NextCursor: nextCursor, HasMore: hasMore}},
	)
}

// Delete handles DELETE /api/v1/media/{id}. It mirrors the browser admin's
// delete: the caller's role must manage media, then the domain deletes
// (owner-or-admin — a non-owner gets 403, a ghost id 404). Success answers
// 204 with no body.
func (h *MediaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	principal, ok := kernel.PrincipalFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	if !h.uploadAllowed(r) {
		httpx.Error(w, http.StatusForbidden, "FORBIDDEN", "Your role cannot manage media", nil)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid media ID", nil)
		return
	}

	if err := h.mediaService.Delete(r.Context(), id, principal); err != nil {
		h.logger.Error("agent delete media failed: id=%d err=%v", id, err)
		handleMediaError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// NewMediaHandler constructs a MediaHandler backed by the given media service. A nil logger
// degrades to a discard sink so the handler is safe to construct in any context (mirrors
// NewContentHandler).
func NewMediaHandler(s MediaService, logger kernel.Logger, opts ...MediaHandlerOption) *MediaHandler {
	if logger == nil {
		logger = discardLogger{}
	}
	h := &MediaHandler{
		mediaService: s,
		logger:       logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
