package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

const (
	mediaSearchParam     = "search"
	mediaDateFilterParam = "date_filter"
	maxAIPromptLength    = 1000
)

// generateImageRequest is the JSON body for the GenerateImage endpoint.
type generateImageRequest struct {
	Prompt string `json:"prompt"`
}

func handleMediaError(w http.ResponseWriter, err error) {
	if dupErr, ok := errors.AsType[*media.DuplicateMediaError](err); ok {
		sendSuccessResponse(w, http.StatusOK, map[string]any{
			"existingMedia": dupErr.Existing,
			"duplicate":     true,
		})
		return
	}

	statusCode := http.StatusInternalServerError
	code := "internal_error"
	message := "An internal error occurred"

	switch {
	case errors.Is(err, media.ErrInvalidFile), errors.Is(err, media.ErrInvalidMimeType), errors.Is(err, media.ErrInvalidFileContent):
		statusCode = http.StatusBadRequest
		code = "invalid_file"
		message = "Invalid file type. Please upload an image file (JPG, PNG, GIF, WebP)"
	case errors.Is(err, media.ErrFileTooLarge):
		statusCode = http.StatusBadRequest
		code = "file_too_large"
		message = "File size exceeds 10MB limit. Please upload a smaller image"
	case errors.Is(err, media.ErrInvalidAltText):
		statusCode = http.StatusBadRequest
		code = "invalid_alt_text"
		message = "Alt text is required and must be less than 500 characters"
	case errors.Is(err, media.ErrDuplicateMedia):
		statusCode = http.StatusConflict
		code = "duplicate_media"
		message = "Media already exists"
	case errors.Is(err, media.ErrMediaNotFound):
		statusCode = http.StatusNotFound
		code = "media_not_found"
		message = "Media not found"
	case errors.Is(err, media.ErrUnauthorized):
		statusCode = http.StatusForbidden
		code = "forbidden"
		message = "You are not authorized to perform this action"
	}

	sendErrorResponse(w, statusCode, code, message, nil)
}

type MediaServiceInterface interface {
	Upload(ctx context.Context, req media.UploadRequest) (*media.Media, error)
	ForceUpload(ctx context.Context, req media.UploadRequest) (*media.Media, error)
	GenerateFromBytes(ctx context.Context, imageBytes []byte, userID int, altText string, originalFilename string) (*media.Media, error)
	GetByID(ctx context.Context, id int) (*media.Media, error)
	GetAll(ctx context.Context, limit int, offset int) ([]*media.Media, error)
	Delete(ctx context.Context, id int, userID int, userRole string) error
	SearchMedia(ctx context.Context, search string, dateFilter string, limit int, offset int) ([]*media.Media, error)
}

type MediaHandler struct {
	mediaService    MediaServiceInterface
	imageGenService media.ImageGenerationService
	logger          *util.Logger
}

func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
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

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		h.logger.Error("Failed to parse multipart form: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form data", nil)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		h.logger.Error("Failed to get form file: %v", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Image file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()

	altText := r.FormValue("alt_text")

	req := media.UploadRequest{
		File:       file,
		FileHeader: header,
		UserID:     userID,
		AltText:    altText,
	}

	var uploadedMedia *media.Media
	if r.URL.Query().Get("force") == "true" {
		uploadedMedia, err = h.mediaService.ForceUpload(r.Context(), req)
	} else {
		uploadedMedia, err = h.mediaService.Upload(r.Context(), req)
	}
	if err != nil {
		h.logger.Error("Failed to upload media: %v", err)
		handleMediaError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusCreated, uploadedMedia)
}

func (h *MediaHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserID(r); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	limit := 100
	limitQuery := r.URL.Query().Get("limit")
	if limitQuery != "" {
		if l, err := strconv.Atoi(limitQuery); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0
	offsetQuery := r.URL.Query().Get("offset")
	if offsetQuery != "" {
		if o, err := strconv.Atoi(offsetQuery); err == nil && o >= 0 {
			offset = o
		}
	}

	search := r.URL.Query().Get(mediaSearchParam)
	dateFilter := r.URL.Query().Get(mediaDateFilterParam)

	mediaList, err := h.mediaService.SearchMedia(
		r.Context(),
		search,
		dateFilter,
		limit,
		offset,
	)
	if err != nil {
		h.logger.Error("Failed to get media: %v", err)
		handleMediaError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, map[string]any{
		"media": mediaList,
	})
}

func (h *MediaHandler) GetMediaByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserID(r); !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_media_id", "Invalid media ID", nil)
		return
	}

	mediaItem, err := h.mediaService.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get media: %v", err)
		handleMediaError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusOK, mediaItem)
}

func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
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

	role, _ := middleware.GetRole(r)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_media_id", "Invalid media ID", nil)
		return
	}

	if err := h.mediaService.Delete(r.Context(), id, userID, role); err != nil {
		h.logger.Error("Failed to delete media: %v", err)
		handleMediaError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *MediaHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
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

	if h.imageGenService == nil {
		sendErrorResponse(w, http.StatusServiceUnavailable, "not_configured", "AI image generation is not configured", nil)
		return
	}

	var req generateImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_prompt", "Prompt is required", nil)
		return
	}
	if utf8.RuneCountInString(prompt) > maxAIPromptLength {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_prompt", "Prompt must be less than 1000 characters", nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	imageBytes, err := h.imageGenService.GenerateImage(ctx, prompt)
	if err != nil {
		h.logger.Error("Failed to generate image: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate image", nil)
		return
	}

	originalFilename := fmt.Sprintf("ai-generated-%s.webp", time.Now().Format("20060102-150405"))

	generatedMedia, err := h.mediaService.GenerateFromBytes(ctx, imageBytes, userID, prompt, originalFilename)
	if err != nil {
		h.logger.Error("Failed to save generated image: %v", err)
		handleMediaError(w, err)
		return
	}

	sendSuccessResponse(w, http.StatusCreated, generatedMedia)
}

func NewMediaHandler(
	mediaService MediaServiceInterface,
	imageGenService media.ImageGenerationService,
	logger *util.Logger,
) *MediaHandler {
	return &MediaHandler{
		mediaService:    mediaService,
		imageGenService: imageGenService,
		logger:          logger,
	}
}
