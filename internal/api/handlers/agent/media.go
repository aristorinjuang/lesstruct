package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// handleMediaError maps a media-domain error to the agent API envelope using UPPER_SNAKE
// codes. It mirrors the content handleError structure but is media-specific: it emits the
// agent error catalog via response.Error (never the legacy lowercase admin codes), and maps
// a *mediadomain.DuplicateMediaError to 409 CONFLICT (the correct REST status for a
// duplicate resource). Auth-path errors (UNAUTHORIZED/INVALID_API_KEY/...) are emitted by
// the middleware, not here.
func handleMediaError(w http.ResponseWriter, err error) {
	if _, ok := errors.AsType[*mediadomain.DuplicateMediaError](err); ok {
		response.Error(w, http.StatusConflict, "CONFLICT", "media already exists", nil)
		return
	}

	switch {
	case errors.Is(err, mediadomain.ErrMediaNotFound):
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Media not found", nil)
	case errors.Is(err, mediadomain.ErrUnauthorized):
		response.Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this media", nil)
	case errors.Is(err, mediadomain.ErrInvalidFile),
		errors.Is(err, mediadomain.ErrInvalidMimeType),
		errors.Is(err, mediadomain.ErrInvalidFileContent),
		errors.Is(err, mediadomain.ErrFileTooLarge),
		errors.Is(err, mediadomain.ErrInvalidAltText):
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// MediaService is the narrow slice of the media domain service the agent handlers depend
// on. *mediadomain.Service satisfies it. It is exported so mockery can generate an exported
// constructor callable from the package's *_test files (CLAUDE.md mandates *_test packages,
// which cannot reach an unexported mock).
type MediaService interface {
	Upload(ctx context.Context, req mediadomain.UploadRequest) (*mediadomain.Media, error)
	GetByID(ctx context.Context, id int) (*mediadomain.Media, error)
	ListByCursor(ctx context.Context, userID int, limit int, beforeID int) ([]*mediadomain.Media, error)
}

// MediaHandler exposes the Bearer-authenticated agent media endpoints. It reuses the
// existing MediaService (which already validates files, hashes, dedups, converts to WebP,
// and builds thumbnail variants) — it never duplicates that logic.
type MediaHandler struct {
	mediaService MediaService
	logger       *util.Logger
}

// Upload handles POST /api/v1/media. It parses a required `file` part and an optional JSON
// `metadata` part (carrying altText) from multipart/form-data, maps them onto a
// mediadomain.UploadRequest, and delegates to the existing MediaService.Upload. A missing
// `file` part returns 400 VALIDATION_ERROR; the service's own validation (mime/size/alt
// text) and dedup flow through handleMediaError.
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	if err := r.ParseMultipartForm(mediadomain.MaxFileSize); err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to parse multipart form", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "file part is required", nil)
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

	response.Success(w, NewMediaResponse(uploaded))
}

// Get handles GET /api/v1/media/{id}. It returns the media in the envelope, or 404
// NOT_FOUND when it does not exist OR the caller is not allowed to see it (ownership
// scoping: only the owner or an Admin may read a given item — existence is never disclosed,
// consistent with the content visibility model).
func (h *MediaHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}
	role := authenticatedRole(r)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid media ID", nil)
		return
	}

	mediaItem, err := h.mediaService.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("agent get media failed: id=%d err=%v", id, err)
		handleMediaError(w, err)
		return
	}

	if mediaItem.UserID != userID && role != middleware.RoleAdmin {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Media not found", nil)
		return
	}

	response.Success(w, NewMediaResponse(mediaItem))
}

// List handles GET /api/v1/media. It returns the caller's own media in newest-first order
// using opaque keyset (cursor) pagination — the SAME contract as the content list. It
// reuses the agent package's cursor helpers and the response package's SuccessList/
// Pagination/ListMeta types (introduced for this reuse in Story 2.2).
func (h *MediaHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", nil)
		return
	}

	limit := parseListLimit(r)
	beforeID, err := decodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid cursor", nil)
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
		nextCursor = encodeCursor(items[len(items)-1].ID)
	}

	projections := make([]MediaProjection, 0, len(items))
	for _, m := range items {
		projections = append(projections, NewMediaResponse(m).Media)
	}

	response.SuccessList(
		w,
		projections,
		response.ListMeta{Pagination: response.Pagination{NextCursor: nextCursor, HasMore: hasMore}},
	)
}

// NewMediaHandler constructs a MediaHandler backed by the given media service. A nil logger
// degrades to a discard sink so the handler is safe to construct in any context (mirrors
// NewContentHandler).
func NewMediaHandler(s MediaService, logger *util.Logger) *MediaHandler {
	if logger == nil {
		logger = util.NewLogger(io.Discard)
	}
	return &MediaHandler{
		mediaService: s,
		logger:       logger,
	}
}
