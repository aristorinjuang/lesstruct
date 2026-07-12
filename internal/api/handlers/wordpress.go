package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/content/wordpress"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

// maxWordPressImportDuration bounds how long a single import run may take,
// covering image downloads and content creation for large exports.
const maxWordPressImportDuration = 5 * time.Minute

// wordpressImportMaxMemory is the in-RAM threshold for ParseMultipartForm; the
// total upload is bounded separately by maxImportBytes (applied via
// MaxBytesReader). Keeping the RAM threshold modest means a large WXR file
// spills to temp files instead of sitting wholly in memory.
const wordpressImportMaxMemory = 32 << 20

// WordPressHandler exposes the WordPress import endpoint to administrators.
type WordPressHandler struct {
	importer       *wordpress.Importer
	logger         *util.Logger
	maxImportBytes int64
}

// Import accepts a WordPress WXR XML file upload and imports its posts and pages.
// The endpoint is admin-only (enforced by route middleware).
func (h *WordPressHandler) Import(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	// Bound the total upload size. Applied here (not at the router) so the
	// configured IMPORT_MAX_SIZE_MB is the single source of truth and there is
	// no competing outer MaxBytesReader that would silently cap it lower.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxImportBytes)
	if err := r.ParseMultipartForm(wordpressImportMaxMemory); err != nil {
		h.logger.Error("WordPress import: failed to parse multipart form: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form data", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.Error("WordPress import: missing file field: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "XML file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xml") {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_file", "File must be a WordPress export (.xml)", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), maxWordPressImportDuration)
	defer cancel()

	result, err := h.importer.Import(ctx, file, userID)
	if err != nil {
		h.logger.Error("WordPress import failed: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "import_failed", "Failed to import WordPress file", nil)
		return
	}

	h.logger.Info("WordPress import complete: %d imported, %d skipped", result.Imported, result.Skipped)
	sendSuccessResponse(w, http.StatusOK, result)
}

// NewWordPressHandler creates a WordPressHandler. maxImportBytes is the total
// upload ceiling (from IMPORT_MAX_SIZE_MB) applied via MaxBytesReader.
func NewWordPressHandler(importer *wordpress.Importer, logger *util.Logger, maxImportBytes int64) *WordPressHandler {
	return &WordPressHandler{
		importer:       importer,
		logger:         logger,
		maxImportBytes: maxImportBytes,
	}
}
